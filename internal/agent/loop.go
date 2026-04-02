package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

// NewLoop creates an agent loop with the given LLM client.
func NewLoop(client llm.Client) *Loop {
	return &Loop{
		Client: client,
		Config: DefaultLoopConfig(),
		UI:     ui.NewHandler(),
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
		if state.turnCount == 2 { // first iteration
			l.UI.Status("◆", "Starting task...")
		}

		// Reset content replacement budget for this turn
		tools.ResetBudgetForNewTurn()

		// Yield stream start
		select {
		case out <- types.StreamRequestStart{}:
		case <-ctx.Done():
			return Terminal{Reason: TerminalReasonUserInterrupt, Error: ctx.Err()}
		}

		// Build request
		messagesForQuery := state.messages
		sysPrompt := params.SystemPrompt
		for k, v := range params.SystemContext {
			sysPrompt += fmt.Sprintf("\n\n<%s>\n%s\n</%s>", k, v, k)
		}

		// Token blocking check (simplified)
		if l.Config.AutoCompactEnabled && l.isAtBlockingLimit(messagesForQuery) {
			errMsg := createAssistantErrorMessage("Context window is at the blocking limit. Please use /compact or start a new session.")
			state.messages = append(state.messages, errMsg)
			return Terminal{Reason: TerminalReasonBlockingLimit, Message: &errMsg}
		}

		// Determine model
		model := params.ToolUseContext.Options.MainLoopModel
		if model == "" {
			model = "anthropic/claude-3.5-sonnet" // OpenRouter default
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
		l.UI.SpinnerStart("thinking")
		llmEvents, err := l.Client.Stream(ctx, req)
		if err != nil {
			l.UI.SpinnerStop()
			return Terminal{Reason: TerminalReasonError, Error: err}
		}

		assistantMsg, toolUses, streamErr := l.consumeStream(ctx, llmEvents, out)
		l.UI.SpinnerStop()
		
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
				l.UI.Status("→", fmt.Sprintf("executing %s...", tu.Name))
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

						// 2. Success/Error Status
						if sm.Message.APIError != "" {
							l.UI.Status("✗", fmt.Sprintf("tool failed: %s", sm.Message.APIError))
						} else {
							l.UI.Status("✓", "tool executed successfully")
						}
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
	var toolUses []types.ToolUseBlock

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if pendingToolUse != nil {
					msg.Content = append(msg.Content, *pendingToolUse)
					toolUses = append(toolUses, *pendingToolUse)
				}
				if currentText != "" {
					msg.Content = append(msg.Content, types.TextBlock{Text: currentText})
				}
				if len(msg.Content) == 0 {
					return nil, nil, nil
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
				}
				// In a real implementation, we'd incrementally parse JSON.
				// For the pattern, we accumulate and parse on stop.
			case types.LLMMessageStop:
				msg.StopReason = e.StopReason
				msg.Model = e.Model
				if pendingToolUse != nil {
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
	// Simplified token estimation
	// Real implementation would use a tokenizer
	return false
}

func createAssistantErrorMessage(content string) types.Message {
	return types.Message{
		UUID:      uuid.New().String(),
		Role:      types.RoleAssistant,
		Content:   []types.ContentBlock{types.TextBlock{Text: content}},
		Timestamp: time.Now(),
		APIError:  "invalid_request",
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
		model = "anthropic/claude-3.5-sonnet"
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
