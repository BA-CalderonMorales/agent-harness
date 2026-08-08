package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

// NewLoop creates an agent loop with the given LLM client.
func NewLoop(client llm.Client) *Loop {
	return &Loop{
		Client: client,
		Config: DefaultLoopConfig(),
	}
}

// Query runs the full agent loop for a single user turn.
// It yields StreamEvents so the caller can render progress in real time.
func (l *Loop) Query(ctx context.Context, params QueryParams) (<-chan types.StreamEvent, error) {
	out := make(chan types.StreamEvent, 16)

	go func() {
		defer close(out)
		state := loopState{
			messages:                     params.Messages,
			toolUseContext:               params.ToolUseContext,
			maxOutputTokensOverride:      params.MaxOutputTokens,
			maxOutputTokensRecoveryCount: 0,
			turnCount:                    1,
		}

		terminal := l.queryLoop(ctx, params, &state, out)
		out <- types.StreamTerminal{
			Reason:  string(terminal.Reason),
			Message: terminal.Message,
			Error:   terminal.Error,
		}
	}()

	return out, nil
}

// queryLoop is the while-true agent loop.
func (l *Loop) queryLoop(ctx context.Context, params QueryParams, state *loopState, out chan<- types.StreamEvent) Terminal {
	if params.MaxOutputTokens > 0 && state.maxOutputTokensOverride == 0 {
		state.maxOutputTokensOverride = params.MaxOutputTokens
	}

	maxTurns := params.MaxTurns
	if maxTurns == 0 {
		maxTurns = l.Config.DefaultMaxTurns
	}

	for state.turnCount <= maxTurns {
		state.turnCount++

		// 1. Task Start
		// (UI handled by stream renderer in caller)

		// Reset content replacement budget for this turn
		tools.ResetBudgetForNewTurn()

		// Yield stream start
		select {
		case out <- types.StreamRequestStart{}:
		case <-ctx.Done():
			out <- types.StreamError{Error: ctx.Err()}
			return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
		}

		// Token blocking check with auto-compaction
		if l.Config.AutoCompactEnabled && l.isAtBlockingLimit(state.messages) {
			compaction, err := l.compactMessages(ctx, state, false)
			if err != nil {
				compactionErr := fmt.Errorf("automatic context compaction failed: %w", err)
				out <- types.StreamError{Error: compactionErr}
				return Terminal{Reason: TerminalReasonError, Error: compactionErr}
			}
			if err := emitCompaction(ctx, out, compaction); err != nil {
				out <- types.StreamError{Error: err}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: err}
			}
		}

		// Build the request only after compaction so this call uses the same
		// bounded context that callers persist from StreamContextCompacted.
		messagesForQuery := state.messages
		sysPrompt := params.SystemPrompt
		for k, v := range params.SystemContext {
			sysPrompt += fmt.Sprintf("\n\n<%s>\n%s\n</%s>", k, v, k)
		}

		// Determine model
		model := params.ToolUseContext.Options.MainLoopModel
		if model == "" {
			model = config.DefaultModel
		}

		req := llm.Request{
			Messages:        messagesForQuery,
			SystemPrompt:    sysPrompt,
			Tools:           params.ToolUseContext.Options.Tools,
			Model:           model,
			MaxTokens:       8192,
			Temperature:     params.Temperature,
			ReasoningEffort: params.ReasoningEffort,
		}

		if state.maxOutputTokensOverride > 0 {
			req.MaxTokens = state.maxOutputTokensOverride
		}

		// Call LLM
		llmEvents, err := l.Client.Stream(ctx, req)
		if err != nil {
			out <- types.StreamError{Error: err}
			return Terminal{Reason: TerminalReasonError, Error: err}
		}

		assistantMsg, toolUses, streamErr := l.consumeStream(ctx, llmEvents, out)

		// Handle recoverable errors with retry logic
		if streamErr != nil {
			if recErr, ok := streamErr.(*recoverableError); ok {
				recovered, newMsg, newToolUses, recoverErr := l.attemptRecovery(ctx, params, state, recErr, out)
				if recovered {
					assistantMsg = newMsg
					toolUses = newToolUses
					streamErr = nil
				} else if recoverErr != nil {
					// Recovery failed - now yield the original error
					streamErr = fmt.Errorf("recovery failed after %d attempts: %w (original: %v)",
						state.maxOutputTokensRecoveryCount, recoverErr, recErr.err)
				}
			}

			if streamErr != nil {
				out <- types.StreamError{Error: streamErr}
				return Terminal{Reason: TerminalReasonError, Error: streamErr}
			}
		}

		if assistantMsg == nil {
			return Terminal{Reason: TerminalReasonComplete}
		}

		state.messages = append(state.messages, *assistantMsg)

		if len(toolUses) == 0 {
			// No tools requested; turn is complete
			return Terminal{Reason: TerminalReasonComplete, Message: assistantMsg}
		}

		// Enforce total tool call budget per query
		maxToolCalls := l.Config.MaxToolCalls
		if maxToolCalls <= 0 {
			maxToolCalls = 15
		}
		if state.toolCallCount+len(toolUses) > maxToolCalls {
			msg := fmt.Sprintf("[Tool call limit reached: %d total tools used. Stopping to prevent runaway exploration.]", state.toolCallCount)
			select {
			case out <- types.StreamMessage{Message: types.Message{
				Role:    types.RoleSystem,
				Content: []types.ContentBlock{types.TextBlock{Text: msg}},
			}}:
			case <-ctx.Done():
				out <- types.StreamError{Error: ctx.Err()}
				return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
			}
			return Terminal{Reason: TerminalReasonBlockingLimit, Message: assistantMsg}
		}
		state.toolCallCount += len(toolUses)

		// Execute tools
		if l.Config.StreamingToolExecution {
			executor := NewStreamingToolExecutor(params.ToolUseContext.Options.Tools, params.CanUseTool, params.ToolUseContext)
			for _, tu := range toolUses {
				executor.AddTool(tu, *assistantMsg)
			}

			// Consume events until all tools are done
			done := make(chan struct{})
			go func() {
				defer close(done)
				for ev := range executor.Events() {
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}

					// Update session messages for final results
					if sm, ok := ev.(types.StreamMessage); ok {
						l.mu.Lock()
						state.messages = append(state.messages, sm.Message)
						l.mu.Unlock()
					}
				}
			}()

			// Wait for completion
			_, execErr := executor.GetRemainingResults(ctx)
			executor.Close()
			<-done

			if execErr != nil {
				_ = execErr
			}
		} else {
			// Batch execution
			results, execErr := runToolsBatch(ctx, toolUses, *assistantMsg, params.ToolUseContext, params.CanUseTool)
			if execErr != nil {
				_ = execErr
			}
			for _, msg := range results {
				state.messages = append(state.messages, msg)
				select {
				case out <- types.StreamMessage{Message: msg}:
				case <-ctx.Done():
					return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
				}
			}
		}
	}

	return Terminal{Reason: TerminalReasonMaxTurns}
}

// consumeStream reads LLM events and builds an assistant message + tool uses.
// Handles max_output_tokens recovery with error withholding.
func (l *Loop) consumeStream(ctx context.Context, events <-chan types.LLMEvent, out chan<- types.StreamEvent) (*types.Message, []types.ToolUseBlock, error) {
	var msg types.Message
	msg.UUID = uuid.New().String()
	msg.Role = types.RoleAssistant
	msg.Timestamp = time.Now()

	var currentText string
	var pendingToolUse *types.ToolUseBlock
	var toolInputBuffer string
	var toolUses []types.ToolUseBlock

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if pendingToolUse != nil {
					if toolInputBuffer != "" {
						var input map[string]any
						if err := json.Unmarshal([]byte(toolInputBuffer), &input); err == nil {
							pendingToolUse.Input = input
						}
					}
					msg.Content = append(msg.Content, *pendingToolUse)
					toolUses = append(toolUses, *pendingToolUse)
				}
				if currentText != "" {
					msg.Content = append(msg.Content, types.TextBlock{Text: currentText})
				}
				if len(msg.Content) == 0 {
					return nil, nil, fmt.Errorf("empty response from LLM: no content received")
				}
				return &msg, toolUses, nil
			}

			switch e := ev.(type) {
			case types.LLMMessageStart:
				msg.UUID = e.ID
			case types.LLMTextDelta:
				currentText += e.Delta
			case types.LLMToolUseDelta:
				if pendingToolUse == nil {
					pendingToolUse = &types.ToolUseBlock{ID: e.ID, Name: e.Name}
					toolInputBuffer = ""
				} else if pendingToolUse.ID != e.ID {
					if toolInputBuffer != "" {
						var input map[string]any
						if err := json.Unmarshal([]byte(toolInputBuffer), &input); err == nil {
							pendingToolUse.Input = input
						}
					}
					msg.Content = append(msg.Content, *pendingToolUse)
					toolUses = append(toolUses, *pendingToolUse)

					pendingToolUse = &types.ToolUseBlock{ID: e.ID, Name: e.Name}
					toolInputBuffer = ""
				}
				toolInputBuffer += e.Delta
			case types.LLMMessageStop:
				msg.StopReason = e.StopReason
				msg.Model = e.Model
				l.LastUsage = e.Usage
				if pendingToolUse != nil {
					if toolInputBuffer != "" {
						var input map[string]any
						if err := json.Unmarshal([]byte(toolInputBuffer), &input); err == nil {
							pendingToolUse.Input = input
						}
					}
					msg.Content = append(msg.Content, *pendingToolUse)
					toolUses = append(toolUses, *pendingToolUse)
					pendingToolUse = nil
				}
				if currentText != "" {
					msg.Content = append(msg.Content, types.TextBlock{Text: currentText})
					currentText = ""
				}
				// Yield the assembled message
				select {
				case out <- types.StreamMessage{Message: msg}:
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
				return &msg, toolUses, nil
			case types.LLMError:
				// Check if this is a recoverable error
				if isMaxOutputTokensError(e.Error) {
					// Return special marker for recovery attempt
					return nil, nil, &recoverableError{err: e.Error, reason: "max_output_tokens"}
				}
				// For prompt_too_long, also mark as recoverable if compaction might help
				if isPromptTooLongError(e.Error) {
					return nil, nil, &recoverableError{err: e.Error, reason: "prompt_too_long"}
				}
				return nil, nil, e.Error
			}

		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}

// recoverableError indicates an error that might be resolved by retry.
type recoverableError struct {
	err    error
	reason string
}

func (e *recoverableError) Error() string {
	return e.err.Error()
}

func (e *recoverableError) Reason() string {
	return e.reason
}

func isMaxOutputTokensError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "max_output_tokens") || contains(errStr, "max_tokens")
}

func isPromptTooLongError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "prompt_too_long") || contains(errStr, "context length")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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
func (l *Loop) attemptRecovery(ctx context.Context, params QueryParams, state *loopState, recErr *recoverableError, out chan<- types.StreamEvent) (bool, *types.Message, []types.ToolUseBlock, error) {
	if state.maxOutputTokensRecoveryCount >= l.Config.MaxOutputTokensRecovery {
		return false, nil, nil, fmt.Errorf("max recovery attempts reached")
	}

	state.maxOutputTokensRecoveryCount++

	switch recErr.Reason() {
	case "max_output_tokens":
		// Increase token limit and retry
		if state.maxOutputTokensOverride == 0 {
			state.maxOutputTokensOverride = 8192 * 2 // Double from default
		} else {
			state.maxOutputTokensOverride *= 2
		}
		// Cap at reasonable maximum
		if state.maxOutputTokensOverride > 64000 {
			state.maxOutputTokensOverride = 64000
		}

		// Yield recovery attempt notice
		select {
		case out <- types.StreamMessage{Message: types.Message{
			Role:    types.RoleSystem,
			Content: []types.ContentBlock{types.TextBlock{Text: fmt.Sprintf("[Recovering: increasing output token limit to %d]", state.maxOutputTokensOverride)}},
		}}:
		case <-ctx.Done():
			return false, nil, nil, ctx.Err()
		}

		// Retry the request
		return l.retryQuery(ctx, params, state, out)

	case "prompt_too_long":
		if state.hasAttemptedReactiveCompact {
			return false, nil, nil, fmt.Errorf("already attempted compaction")
		}
		state.hasAttemptedReactiveCompact = true

		compaction, err := l.compactMessages(ctx, state, true)
		if err != nil {
			return false, nil, nil, fmt.Errorf("reactive context compaction failed: %w", err)
		}
		if !compaction.changed() {
			return false, nil, nil, fmt.Errorf("reactive context compaction did not reduce the prompt")
		}
		if err := emitCompaction(ctx, out, compaction); err != nil {
			return false, nil, nil, err
		}

		// Retry only after the state has been replaced and the exact snapshot has
		// been emitted for durable persistence.
		return l.retryQuery(ctx, params, state, out)

	default:
		return false, nil, nil, fmt.Errorf("unknown recoverable error: %s", recErr.Reason())
	}
}

// retryQuery re-executes the LLM query after recovery adjustments.
func (l *Loop) retryQuery(ctx context.Context, params QueryParams, state *loopState, out chan<- types.StreamEvent) (bool, *types.Message, []types.ToolUseBlock, error) {
	// Rebuild the request with potentially updated parameters
	sysPrompt := params.SystemPrompt
	for k, v := range params.SystemContext {
		sysPrompt += fmt.Sprintf("\n\n<%s>\n%s\n</%s>", k, v, k)
	}

	model := params.ToolUseContext.Options.MainLoopModel
	if model == "" {
		model = config.DefaultModel
	}

	req := llm.Request{
		Messages:     state.messages,
		SystemPrompt: sysPrompt,
		Tools:        params.ToolUseContext.Options.Tools,
		Model:        model,
		MaxTokens:    8192,
		Temperature:  params.Temperature,
	}

	if state.maxOutputTokensOverride > 0 {
		req.MaxTokens = state.maxOutputTokensOverride
	}

	llmEvents, err := l.Client.Stream(ctx, req)
	if err != nil {
		return false, nil, nil, err
	}

	assistantMsg, toolUses, streamErr := l.consumeStream(ctx, llmEvents, out)
	if streamErr != nil {
		// Check if we need nested recovery
		if recErr, ok := streamErr.(*recoverableError); ok {
			return l.attemptRecovery(ctx, params, state, recErr, out)
		}
		return false, nil, nil, streamErr
	}

	return true, assistantMsg, toolUses, nil
}
