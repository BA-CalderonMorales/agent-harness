package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
	"strings"
	"time"
)

// consumeStream reads LLM events and builds an assistant message + tool uses.
// Handles max_output_tokens recovery with error withholding.
//
// maxStreamIdle guards against a provider that goes silent mid-turn: with
// no events for this long, the turn fails cleanly instead of hanging and
// leaving the UI stuck on a running/thinking state. LoopConfig.StreamIdleTimeout
// overrides this per loop (local providers need multi-minute first-token
// windows; the package default stays tight for hosted APIs).
var maxStreamIdle = 90 * time.Second

// idleWindow returns the configured idle watchdog for the loop, falling
// back to the package default when the loop carries no override.
func (l *Loop) idleWindow() time.Duration {
	if l != nil && l.Config.StreamIdleTimeout > 0 {
		return l.Config.StreamIdleTimeout
	}
	return maxStreamIdle
}

func (l *Loop) consumeStream(ctx context.Context, events <-chan types.LLMEvent, out chan<- types.StreamEvent) (*types.Message, []types.ToolUseBlock, error) {
	var msg types.Message
	msg.UUID = uuid.New().String()
	msg.Role = types.RoleAssistant
	msg.Timestamp = time.Now()

	var currentText string
	var pendingToolUse *types.ToolUseBlock
	var toolInputBuffer string
	var toolUses []types.ToolUseBlock
	var currentReasoning strings.Builder

	idle := time.NewTimer(l.idleWindow())
	defer idle.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-idle.C:
			return nil, nil, fmt.Errorf("LLM stream went idle for %s; the provider stopped sending events", l.idleWindow())
		case ev, ok := <-events:
			_ = idle.Reset(l.idleWindow())
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
			case types.LLMReasoningDelta:
				// Reasoning is wait-state color, not transcript material:
				// forward the live text for the thinking badge and keep
				// it out of the durable message.
				currentReasoning.WriteString(e.Delta)
				select {
				case out <- types.StreamThinking{Text: currentReasoning.String()}:
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
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
