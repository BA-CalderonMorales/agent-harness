package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
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
		_ = terminal // caller can inspect final messages
	}()

	return out, nil
}

// queryLoop is the while-true agent loop.
func (l *Loop) queryLoop(ctx context.Context, params QueryParams, state *loopState, out chan<- types.StreamEvent) Terminal {
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

		// Build request
		messagesForQuery := state.messages
		sysPrompt := params.SystemPrompt
		for k, v := range params.SystemContext {
			sysPrompt += fmt.Sprintf("\n\n<%s>\n%s\n</%s>", k, v, k)
		}

		// Token blocking check with auto-compaction
		if l.Config.AutoCompactEnabled && l.isAtBlockingLimit(messagesForQuery) {
			compactMsg := l.autoCompactMessages(state)
			if compactMsg != "" {
				select {
				case out <- types.StreamMessage{Message: types.Message{
					Role:    types.RoleSystem,
					Content: []types.ContentBlock{types.TextBlock{Text: compactMsg}},
				}}:
				case <-ctx.Done():
					out <- types.StreamError{Error: ctx.Err()}
					return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
				}
			}
		}

		// Determine model
		model := params.ToolUseContext.Options.MainLoopModel
		if model == "" {
			model = "nvidia/nemotron-3-super-120b-a12b:free" // OpenRouter default
		}

		req := llm.Request{
			Messages:     messagesForQuery,
			SystemPrompt: sysPrompt,
			Tools:        params.ToolUseContext.Options.Tools,
			Model:        model,
			MaxTokens:    8192,
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
	total := 0
	for _, msg := range msgs {
		for _, block := range msg.Content {
			switch b := block.(type) {
			case types.TextBlock:
				total += len(b.Text) / 4
			case types.ToolUseBlock:
				total += len(b.Name) / 4
				if inputJSON, err := json.Marshal(b.Input); err == nil {
					total += len(inputJSON) / 4
				}
			case types.ToolResultBlock:
				total += len(fmt.Sprintf("%v", b.Content)) / 4
			}
		}
	}
	return total
}

// autoCompactMessages trims old messages when approaching the token limit.
// Returns a description of what was compacted, or empty string if no compaction needed.
func (l *Loop) autoCompactMessages(state *loopState) string {
	limit := l.Config.BlockingTokenLimit
	if limit <= 0 {
		limit = 180000
	}
	// Target: 80% of limit to leave headroom
	target := limit * 8 / 10
	current := estimateTokens(state.messages)
	if current <= target {
		return ""
	}

	// Preserve recent messages (last 20) and compact older ones
	preserve := 20
	if len(state.messages) <= preserve {
		// Not enough history to trim meaningfully
		return ""
	}

	removed := len(state.messages) - preserve
	state.messages = state.messages[removed:]

	// Summarize removed messages if LLM client is available
	summaryText := fmt.Sprintf("[Context compacted: removed %d older messages]", removed)
	if l.Client != nil {
		if summarized, err := l.summarizeMessages(context.Background(), state.messages[:removed]); err == nil && summarized != "" {
			summaryText = fmt.Sprintf("[Earlier conversation summarized]: %s", summarized)
		}
	}

	// Insert compaction summary
	summary := types.Message{
		UUID:      uuid.New().String(),
		Role:      types.RoleSystem,
		Timestamp: time.Now(),
		Content: []types.ContentBlock{
			types.TextBlock{Text: summaryText},
		},
	}
	state.messages = append([]types.Message{summary}, state.messages...)

	return fmt.Sprintf("[Auto-compacted: removed %d older messages, %d estimated tokens → %d]",
		removed, current, estimateTokens(state.messages))
}

// summarizeMessages sends old messages to the LLM for summarization.
func (l *Loop) summarizeMessages(ctx context.Context, msgs []types.Message) (string, error) {
	if l.Client == nil {
		return "", fmt.Errorf("no LLM client available")
	}

	var b strings.Builder
	b.WriteString("Summarize the following conversation concisely. Preserve key decisions, facts, and context:\n\n")
	for _, msg := range msgs {
		b.WriteString(fmt.Sprintf("%s: ", msg.Role))
		for _, block := range msg.Content {
			switch blk := block.(type) {
			case types.TextBlock:
				b.WriteString(blk.Text)
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("[tool: %s]", blk.Name))
			case types.ToolResultBlock:
				b.WriteString(fmt.Sprintf("[result: %v]", blk.Content))
			}
		}
		b.WriteString("\n")
	}

	req := llm.Request{
		Messages: []types.Message{
			{UUID: uuid.New().String(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: b.String()}}, Timestamp: time.Now()},
		},
		SystemPrompt: "You are a context summarizer. Summarize conversation history in 2-3 sentences. Be concise but preserve all key facts, decisions, and context.",
		Model:        "nvidia/nemotron-3-super-120b-a12b:free",
		MaxTokens:    512,
	}

	stream, err := l.Client.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for event := range stream {
		switch e := event.(type) {
		case types.LLMTextDelta:
			result.WriteString(e.Delta)
		case types.LLMError:
			return result.String(), e.Error
		}
	}
	return strings.TrimSpace(result.String()), nil
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
		// Try compacting context and retry
		if state.hasAttemptedReactiveCompact {
			return false, nil, nil, fmt.Errorf("already attempted compaction")
		}
		state.hasAttemptedReactiveCompact = true

		// In a full implementation, trigger context compaction here
		select {
		case out <- types.StreamMessage{Message: types.Message{
			Role:    types.RoleSystem,
			Content: []types.ContentBlock{types.TextBlock{Text: "[Recovering: compacting context]"}},
		}}:
		case <-ctx.Done():
			return false, nil, nil, ctx.Err()
		}

		// Retry with (potentially) compacted messages
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
		model = "nvidia/nemotron-3-super-120b-a12b:free"
	}

	req := llm.Request{
		Messages:     state.messages,
		SystemPrompt: sysPrompt,
		Tools:        params.ToolUseContext.Options.Tools,
		Model:        model,
		MaxTokens:    8192,
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
