package agent

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

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
