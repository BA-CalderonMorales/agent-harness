package agent

import (
	"context"
	"encoding/json"
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

func (l *Loop) isAtBlockingLimit(msgs []types.Message) bool {
	return estimateTokens(msgs) > l.Config.BlockingTokenLimit
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
	limit := l.Config.BlockingTokenLimit
	if limit <= 0 {
		limit = 180000
	}
	target := limit * 8 / 10
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

	removedPrefix := append([]types.Message(nil), state.messages[:keepStart]...)
	recentSuffix := append([]types.Message(nil), state.messages[keepStart:]...)
	model := state.toolUseContext.Options.MainLoopModel
	if model == "" {
		model = config.DefaultModel
	}
	summarized, err := l.summarizeMessages(ctx, removedPrefix, model)
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

// summarizeMessages sends old messages to the LLM for summarization.
func (l *Loop) summarizeMessages(ctx context.Context, msgs []types.Message, model string) (string, error) {
	if l.Client == nil {
		return "", fmt.Errorf("no LLM client available")
	}

	var b strings.Builder
	b.WriteString("Create a durable handoff from the conversation prefix below.\n")
	b.WriteString("Preserve the original user goal, constraints, decisions and approvals, pending work, plans, files changed, tool findings or receipts, errors, and verification status.\n")
	b.WriteString("State uncertainty explicitly. Put the original goal and unresolved work first.\n\n")
	for _, msg := range msgs {
		b.WriteString(fmt.Sprintf("%s: ", msg.Role))
		for _, block := range msg.Content {
			switch blk := block.(type) {
			case types.TextBlock:
				b.WriteString(blk.Text)
			case types.ThinkingBlock:
				b.WriteString("[reasoning omitted from durable summary input]")
			case types.ToolUseBlock:
				input, _ := json.Marshal(blk.Input)
				b.WriteString(fmt.Sprintf("[tool id=%s name=%s input=%s]", blk.ID, blk.Name, input))
			case types.ToolResultBlock:
				b.WriteString(fmt.Sprintf("[tool result id=%s error=%t: %s]", blk.ToolUseID, blk.IsError, blk.Content))
			}
		}
		b.WriteString("\n")
	}

	req := llm.Request{
		Messages: []types.Message{
			{UUID: uuid.New().String(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: b.String()}}, Timestamp: time.Now()},
		},
		SystemPrompt: "You are a context summarizer producing a durable continuation record. Be concise, structured, and loss-averse. Preserve the original goal, constraints, decisions, approvals, pending tasks, plans, changed files, tool outcomes or receipts, errors, and verification state.",
		Model:        model,
		MaxTokens:    512,
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
