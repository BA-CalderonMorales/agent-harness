package loop

import (
	"context"
	"fmt"
	"sync"

	"github.com/BA-CalderonMorales/agent-harness/internal/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Orchestrator implements the core agent loop using the bucket architecture.
// It coordinates multiple LoopBase implementations without knowing their internals.
type Orchestrator struct {
	config        LoopConfig
	buckets       []LoopBase
	toolManager   *LoopTool
	prompts       LoopSystemPrompts
	strategies    LoopExecute
	llmClient     llm.Client

	// Event streaming
	eventsMu sync.RWMutex
	events   chan<- types.StreamEvent
}

// NewOrchestrator creates a new loop orchestrator with the given buckets.
func NewOrchestrator(config LoopConfig, client llm.Client, bucketsList ...LoopBase) *Orchestrator {
	return &Orchestrator{
		config:      config,
		buckets:     bucketsList,
		toolManager: NewLoopTool(),
		prompts:     NewLoopSystemPrompts(),
		strategies:  NewLoopExecute(),
		llmClient:   client,
	}
}

// RegisterBucket adds a bucket to the orchestrator.
func (o *Orchestrator) RegisterBucket(bucket LoopBase) {
	o.buckets = append(o.buckets, bucket)
}

// SetEventChannel sets the channel for streaming events.
func (o *Orchestrator) SetEventChannel(ch chan<- types.StreamEvent) {
	o.eventsMu.Lock()
	defer o.eventsMu.Unlock()
	o.events = ch
}

// emit sends an event to the event channel.
func (o *Orchestrator) emit(event types.StreamEvent) {
	o.eventsMu.RLock()
	ch := o.events
	o.eventsMu.RUnlock()

	if ch != nil {
		select {
		case ch <- event:
		default:
			// Channel full, drop event
		}
	}
}

// Run executes the agent loop until completion or max turns.
// This is the main entry point for running the loop.
func (o *Orchestrator) Run(ctx context.Context, params QueryParams) (*LoopState, error) {
	state := NewLoopState()
	state.Messages = params.Messages
	state.ToolUseContext = params.ToolUseContext

	o.prompts.SetBase(params.SystemPrompt)
	for k, v := range params.SystemContext {
		o.prompts.AddContextBlock(k, v)
	}

	maxTurns := params.MaxTurns
	if maxTurns == 0 {
		maxTurns = o.config.MaxTurns
	}

	for state.TurnCount <= maxTurns {
		state.TurnCount++

		// Check for context limit
		if o.config.AutoCompactEnabled && o.isAtBlockingLimit(state.Messages) {
			return state, fmt.Errorf("context window at blocking limit")
		}

		// Call LLM
		o.emit(types.StreamRequestStart{})
		
		response, err := o.callLLM(ctx, state, params)
		if err != nil {
			if rec, ok := IsRecoverable(err); ok && o.config.EnableRecovery {
				recovered := o.attemptRecovery(ctx, state, rec)
				if recovered {
					continue
				}
			}
			return state, err
		}

		// Check if text-only (done)
		toolUses := o.extractToolUses(response)
		if len(toolUses) == 0 {
			// Done - add final assistant message
			state.Messages = append(state.Messages, response)
			o.emit(types.StreamMessage{Message: response})
			return state, nil
		}

		// Execute tools
		state.Messages = append(state.Messages, response)
		o.emit(types.StreamMessage{Message: response})

		results := o.executeTools(ctx, toolUses, state, params.CanUseTool)
		for _, result := range results {
			state.Messages = append(state.Messages, result.Messages...)
		}
	}

	return state, fmt.Errorf("max turns (%d) exceeded", maxTurns)
}

// callLLM makes a request to the language model.
func (o *Orchestrator) callLLM(ctx context.Context, state *LoopState, params QueryParams) (types.Message, error) {
	sysPrompt := o.prompts.Compose()

	// Get enabled tools from tool manager
	enabledTools := o.toolManager.GetEnabledTools()

	req := llm.Request{
		Messages:     state.Messages,
		SystemPrompt: sysPrompt,
		Tools:        enabledTools,
		Model:        params.ToolUseContext.Options.MainLoopModel,
		MaxTokens:    o.config.MaxOutputTokens,
	}

	if state.MaxOutputTokensOverride > 0 {
		req.MaxTokens = state.MaxOutputTokensOverride
	}

	events, err := o.llmClient.Stream(ctx, req)
	if err != nil {
		return types.Message{}, err
	}

	return o.consumeStream(ctx, events)
}

// consumeStream reads LLM events and builds a message.
func (o *Orchestrator) consumeStream(ctx context.Context, events <-chan types.LLMEvent) (types.Message, error) {
	var msg types.Message
	msg.Role = types.RoleAssistant

	var currentText string
	var pendingToolUse *types.ToolUseBlock
	var toolInputBuffer string

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// Stream ended
				if pendingToolUse != nil && toolInputBuffer != "" {
					pendingToolUse.Input = parseToolInput(toolInputBuffer)
					msg.Content = append(msg.Content, *pendingToolUse)
				}
				if currentText != "" {
					msg.Content = append(msg.Content, types.TextBlock{Text: currentText})
				}
				return msg, nil
			}

			switch e := ev.(type) {
			case types.LLMMessageStart:
				msg.UUID = e.ID

			case types.LLMTextDelta:
				currentText += e.Delta

			case types.LLMToolUseDelta:
				if pendingToolUse == nil || pendingToolUse.ID != e.ID {
					// New tool use started
					if pendingToolUse != nil && toolInputBuffer != "" {
						pendingToolUse.Input = parseToolInput(toolInputBuffer)
						msg.Content = append(msg.Content, *pendingToolUse)
					}
					pendingToolUse = &types.ToolUseBlock{
						ID:   e.ID,
						Name: e.Name,
					}
					toolInputBuffer = ""
				}
				toolInputBuffer += e.Delta

			case types.LLMMessageStop:
				msg.StopReason = e.StopReason
				if pendingToolUse != nil && toolInputBuffer != "" {
					pendingToolUse.Input = parseToolInput(toolInputBuffer)
					msg.Content = append(msg.Content, *pendingToolUse)
				}
				if currentText != "" {
					msg.Content = append(msg.Content, types.TextBlock{Text: currentText})
				}
				return msg, nil

			case types.LLMError:
				return types.Message{}, e.Error
			}

		case <-ctx.Done():
			return types.Message{}, ctx.Err()
		}
	}
}

// extractToolUses gets all tool use blocks from a message.
func (o *Orchestrator) extractToolUses(msg types.Message) []ToolUse {
	return o.toolManager.ParseToolUses(msg.Content)
}

// executeTools routes and executes tool calls.
func (o *Orchestrator) executeTools(ctx context.Context, uses []ToolUse, state *LoopState, canUseTool tools.CanUseToolFn) []LoopResult {
	results := make([]LoopResult, len(uses))

	for i, use := range uses {
		bucket, found := o.findBucket(use.Name, use.Input)
		if !found {
			results[i] = LoopResult{
				Success: false,
				Error:   NewLoopError("no_bucket", "no bucket found for tool: "+use.Name),
				Messages: []types.Message{{
					Role: types.RoleUser,
					Content: []types.ContentBlock{types.ToolResultBlock{
						ToolUseID: use.ID,
						Content:   fmt.Sprintf("Error: No handler available for tool '%s'", use.Name),
						IsError:   true,
					}},
				}},
			}
			continue
		}

		execCtx := ExecutionContext{
			Context:    ctx,
			ToolName:   use.Name,
			Input:      use.Input,
			ToolUseID:  use.ID,
			Messages:   state.Messages,
			CanUseTool: canUseTool,
			OnProgress: func(data any) {
				o.emit(types.ProgressMessage{
					ToolUseID: use.ID,
					Type:      "progress",
					Data:      data,
				})
			},
		}

		results[i] = bucket.Execute(execCtx)
	}

	return results
}

// findBucket locates the appropriate bucket for a tool.
func (o *Orchestrator) findBucket(toolName string, input map[string]any) (LoopBase, bool) {
	for _, bucket := range o.buckets {
		if bucket.CanHandle(toolName, input) {
			return bucket, true
		}
	}
	return nil, false
}

// isAtBlockingLimit checks if we're near context limits.
func (o *Orchestrator) isAtBlockingLimit(messages []types.Message) bool {
	// Simplified check - real implementation would use tokenizer
	return false
}

// attemptRecovery tries to recover from recoverable errors.
func (o *Orchestrator) attemptRecovery(ctx context.Context, state *LoopState, rec *RecoverableError) bool {
	if state.MaxOutputTokensRecoveryCount >= o.config.MaxOutputTokensRecovery {
		return false
	}

	state.MaxOutputTokensRecoveryCount++

	switch rec.Reason {
	case "max_output_tokens":
		// Double the token limit
		if state.MaxOutputTokensOverride == 0 {
			state.MaxOutputTokensOverride = o.config.MaxOutputTokens * 2
		} else {
			state.MaxOutputTokensOverride *= 2
		}
		// Cap at reasonable maximum
		if state.MaxOutputTokensOverride > 64000 {
			state.MaxOutputTokensOverride = 64000
		}
		o.emit(types.StreamMessage{Message: types.Message{
			Role:    types.RoleSystem,
			Content: []types.ContentBlock{types.TextBlock{Text: fmt.Sprintf("[Recovering: increasing token limit to %d]", state.MaxOutputTokensOverride)}},
		}})
		return true

	case "prompt_too_long":
		if state.HasAttemptedReactiveCompact {
			return false
		}
		state.HasAttemptedReactiveCompact = true
		o.emit(types.StreamMessage{Message: types.Message{
			Role:    types.RoleSystem,
			Content: []types.ContentBlock{types.TextBlock{Text: "[Recovering: context compaction needed]"}},
		}})
		// In real implementation, trigger compaction here
		return true

	default:
		return false
	}
}

// parseToolInput parses JSON tool input.
func parseToolInput(raw string) map[string]any {
	// Simplified - real implementation would use proper JSON parsing
	result := make(map[string]any)
	// Implementation details omitted for brevity
	_ = raw
	return result
}
