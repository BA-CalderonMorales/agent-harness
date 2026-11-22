package agent

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
	"strings"
	"time"
)

// defaultContextLimit applies when the loop has no configured window.
const defaultContextLimit = 180000

// contextLimit is the window the loop works within: the limit recovery
// learned from a provider refusal, else the configured one.
func (l *Loop) contextLimit(state *loopState) int {
	if state.contextLimit > 0 {
		return state.contextLimit
	}
	if l.Config.BlockingTokenLimit > 0 {
		return l.Config.BlockingTokenLimit
	}
	return defaultContextLimit
}

func (l *Loop) isAtBlockingLimit(state *loopState) bool {
	return estimateTokens(state.messages)+state.promptTokens > l.contextLimit(state)
}

// estimateTokens provides a rough character-based token estimate.
func estimateTokens(msgs []types.Message) int {
	return messages.EstimateTokens(msgs)
}

type compactionOutcome struct {
	Messages     []types.Message
	RemovedCount int
	BeforeTokens int
	AfterTokens  int
	Notice       string
}

func (o compactionOutcome) changed() bool {
	return o.RemovedCount > 0
}

// autoCompactMessages trims old messages when approaching the token limit.
// Returns a description of what was compacted, or empty string if no compaction needed.
func (l *Loop) autoCompactMessages(state *loopState) string {
	outcome, err := l.compactMessages(context.Background(), state, false)
	if err != nil {
		return ""
	}
	return outcome.Notice
}

// compactMessages summarizes the exact removed prefix, then atomically swaps
// the loop state to a bounded summary plus a token-budgeted recent suffix.
func (l *Loop) compactMessages(ctx context.Context, state *loopState, force bool) (compactionOutcome, error) {
	limit := l.contextLimit(state)
	// The fixed prompt and tool schemas ride every request; history can
	// only use what those leave behind. Ignoring them let a request
	// exceed the window while the loop believed it was comfortably under.
	messageBudget := limit - state.promptTokens
	if messageBudget < 1 {
		return compactionOutcome{}, fmt.Errorf("prompt overhead (%d tokens) leaves no room in the %d-token context window", state.promptTokens, limit)
	}
	target := messageBudget * 8 / 10
	if target < 1 {
		target = 1
	}
	current := estimateTokens(state.messages)
	if !force && current <= target {
		return compactionOutcome{}, nil
	}
	if len(state.messages) < 2 {
		return compactionOutcome{}, fmt.Errorf("cannot compact a history with fewer than two messages")
	}

	// Reserve half of the target for the durable summary and fill the rest
	// from newest to oldest. At least one recent message is retained.
	recentBudget := target / 2
	if recentBudget < 1 {
		recentBudget = 1
	}
	keepStart := len(state.messages) - 1
	recentTokens := estimateTokens(state.messages[keepStart:])
	for keepStart > 0 {
		nextTokens := estimateTokens(state.messages[keepStart-1 : keepStart])
		if recentTokens+nextTokens > recentBudget {
			break
		}
		keepStart--
		recentTokens += nextTokens
	}
	if keepStart == 0 {
		if !force {
			return compactionOutcome{}, nil
		}
		keepStart = len(state.messages) / 2
		if keepStart < 1 {
			keepStart = 1
		}
	}

	// A usable session record stands in for the removed turns without a
	// model call, which a small window cannot afford.
	if outcome, handled, err := l.compactFromRecord(state, keepStart, current, target); handled {
		return outcome, err
	}

	removedPrefix := append([]types.Message(nil), state.messages[:keepStart]...)
	recentSuffix := append([]types.Message(nil), state.messages[keepStart:]...)
	model := state.toolUseContext.Options.MainLoopModel
	if model == "" {
		model = config.DefaultModel
	}
	summarized, err := l.summarizeMessages(ctx, removedPrefix, model, limit)
	if err != nil {
		return compactionOutcome{}, err
	}
	summarized = strings.TrimSpace(summarized)
	if summarized == "" {
		return compactionOutcome{}, fmt.Errorf("context summarizer returned an empty summary")
	}

	summary := types.Message{
		UUID:      uuid.New().String(),
		Role:      types.RoleSystem,
		Timestamp: time.Now(),
		Content: []types.ContentBlock{
			types.TextBlock{Text: "[Earlier conversation summarized]: " + summarized},
		},
	}
	compacted := make([]types.Message, 0, 1+len(recentSuffix))
	compacted = append(compacted, summary)
	compacted = append(compacted, recentSuffix...)
	after := estimateTokens(compacted)
	if after >= current {
		return compactionOutcome{}, fmt.Errorf(
			"context summarization did not reduce the history (%d estimated tokens before, %d after)",
			current,
			after,
		)
	}
	if after > target {
		return compactionOutcome{}, fmt.Errorf(
			"context summary exceeds the bounded target (%d estimated tokens, target %d)",
			after,
			target,
		)
	}

	state.messages = compacted
	notice := fmt.Sprintf(
		"[Context compacted: summarized %d older messages, %d estimated tokens → %d]",
		len(removedPrefix),
		current,
		after,
	)
	diag.Info("context.compacted", fmt.Sprintf(
		"summarized %d messages · tokens %d → %d · history now %d messages",
		len(removedPrefix), current, after, len(state.messages)-len(removedPrefix)+1,
	))
	return compactionOutcome{
		Messages:     append([]types.Message(nil), compacted...),
		RemovedCount: len(removedPrefix),
		BeforeTokens: current,
		AfterTokens:  after,
		Notice:       notice,
	}, nil
}

const (
	// maxReactiveCompactions bounds the compaction rounds one query may run
	// after provider refusals.
	maxReactiveCompactions = 3

	// maxSummaryShrinks bounds how often one round halves the working
	// window when the summarizer's own requests are refused.
	maxSummaryShrinks = 3
)

// compactAfterRefusal compacts after the provider refused a request as too
// long. The refusal proves the real window is smaller than the request the
// loop sent, whatever the configured window says, so the working limit is
// capped at that request's size for the rest of the query. If the
// summarizer's own requests are refused too, the limit halves and the
// round tries again, a bounded number of times.
func (l *Loop) compactAfterRefusal(ctx context.Context, state *loopState) (compactionOutcome, error) {
	refused := estimateTokens(state.messages) + state.promptTokens
	limit := l.contextLimit(state)
	if refused < limit {
		limit = refused
	}
	for shrink := 0; ; shrink++ {
		state.contextLimit = limit
		outcome, err := l.compactMessages(ctx, state, true)
		if err == nil {
			diag.Info("context.limit.learned", fmt.Sprintf(
				"provider refused %d estimated tokens · working window now %d", refused, limit,
			))
			return outcome, nil
		}
		if !isPromptTooLongError(err) || shrink >= maxSummaryShrinks {
			return compactionOutcome{}, err
		}
		limit /= 2
	}
}

func emitCompaction(ctx context.Context, out chan<- types.StreamEvent, outcome compactionOutcome) error {
	if !outcome.changed() {
		return nil
	}
	select {
	case out <- types.StreamContextCompacted{
		Messages:     append([]types.Message(nil), outcome.Messages...),
		RemovedCount: outcome.RemovedCount,
		Notice:       outcome.Notice,
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// summarizeMessages summarizes old messages within the context window.
// A prefix that fits goes to the model in one request. A larger one is
// split into window-sized parts, and each request carries the running
// summary of the parts before it, so no request can exceed the window
// the history overflowed.
func (l *Loop) summarizeMessages(ctx context.Context, msgs []types.Message, model string, window int) (string, error) {
	if l.Client == nil {
		return "", fmt.Errorf("no LLM client available")
	}
	lines := renderTranscriptLines(msgs)
	var chunks []string
	if budget := summaryChunkBudget(window); budget >= minSummaryChunkTokens {
		chunks = chunkTranscript(lines, budget)
	} else {
		// A window this small cannot hold the summarizer's own prompt
		// with room to spare, so splitting cannot make a request fit.
		// Send the prefix whole and let the provider decide.
		chunks = []string{strings.Join(lines, "\n") + "\n"}
	}
	summary := ""
	for i, chunk := range chunks {
		var b strings.Builder
		b.WriteString(summaryInstructions)
		if summary != "" {
			fmt.Fprintf(&b, summaryCarryForward, summary)
		}
		b.WriteString(chunk)

		part, err := l.requestSummary(ctx, b.String(), model)
		if err != nil {
			if len(chunks) > 1 {
				return "", fmt.Errorf("summarizing part %d of %d: %w", i+1, len(chunks), err)
			}
			return "", err
		}
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("context summarizer returned an empty summary")
		}
		// A provider may ignore MaxTokens; the running summary must still
		// leave the next part its room.
		summary = truncateMiddle(part, summaryOutputTokens*4)
	}
	return summary, nil
}

// requestSummary sends one summarizer request and returns its text.
func (l *Loop) requestSummary(ctx context.Context, prompt, model string) (string, error) {
	req := llm.Request{
		Messages: []types.Message{
			{UUID: uuid.New().String(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: prompt}}, Timestamp: time.Now()},
		},
		SystemPrompt: summarySystemPrompt,
		Model:        model,
		MaxTokens:    summaryOutputTokens,
	}

	stream, err := l.Client.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				return strings.TrimSpace(result.String()), nil
			}
			switch e := event.(type) {
			case types.LLMTextDelta:
				result.WriteString(e.Delta)
			case types.LLMError:
				return result.String(), e.Error
			}
		case <-ctx.Done():
			return result.String(), ctx.Err()
		}
	}
}

// attemptRecovery tries to recover from recoverable errors.
// Returns true with results if recovery succeeded, false with error if all attempts failed.
