package agent

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
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
