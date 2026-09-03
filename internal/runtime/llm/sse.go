package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"io"
	"sort"
	"strings"
)

func (c *HTTPClient) readSSE(ctx context.Context, body io.ReadCloser, out chan<- types.LLMEvent) {
	defer close(out)
	defer body.Close()

	readFinished := make(chan struct{})
	defer close(readFinished)
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-readFinished:
		}
	}()

	type toolCallDelta struct {
		Index    *int   `json:"index"`
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	type streamChoice struct {
		Delta struct {
			Content          string          `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			ToolCalls        []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}
	type streamUsage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		PromptDetails    struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	type streamChunk struct {
		ID      string         `json:"id"`
		Model   string         `json:"model"`
		Choices []streamChoice `json:"choices"`
		Usage   *streamUsage   `json:"usage"`
	}
	type accumulatedToolCall struct {
		id        string
		name      strings.Builder
		arguments strings.Builder
	}

	const (
		maxStreamedToolCalls     = 128
		maxStreamedToolNameBytes = 512
		maxStreamedArguments     = 1 << 20
	)

	reader := bufio.NewReader(body)
	var currentMessageID string
	var model string
	var finishReason string
	var usage types.TokenUsage
	toolCalls := make(map[int]*accumulatedToolCall)

	emit := func(event types.LLMEvent) bool {
		select {
		case out <- event:
			return true
		case <-ctx.Done():
			return false
		}
	}
	emitContextError := func() {
		select {
		case out <- types.LLMError{Error: ctx.Err()}:
		default:
		}
	}
	finalize := func() error {
		if currentMessageID == "" && finishReason == "" && len(toolCalls) == 0 {
			return nil
		}
		if finishReason == "" {
			return fmt.Errorf("SSE stream ended without a finish reason")
		}

		indices := make([]int, 0, len(toolCalls))
		for index := range toolCalls {
			indices = append(indices, index)
		}
		sort.Ints(indices)

		completed := make([]types.LLMToolUseDelta, 0, len(indices))
		for _, index := range indices {
			call := toolCalls[index]
			if call.id == "" {
				return fmt.Errorf("tool call index %d is missing a stable ID", index)
			}
			name := call.name.String()
			if name == "" {
				return fmt.Errorf("tool call index %d (%s) is missing a function name", index, call.id)
			}

			rawArguments := call.arguments.String()
			if rawArguments == "" {
				rawArguments = "{}"
			}
			var object map[string]any
			if err := json.Unmarshal([]byte(rawArguments), &object); err != nil {
				return fmt.Errorf("tool call index %d (%s) has invalid arguments JSON: %w", index, call.id, err)
			}
			if object == nil {
				return fmt.Errorf("tool call index %d (%s) has invalid arguments JSON: expected an object", index, call.id)
			}

			completed = append(completed, types.LLMToolUseDelta{
				ID:    call.id,
				Name:  name,
				Delta: rawArguments,
			})
		}

		for _, event := range completed {
			if !emit(event) {
				return ctx.Err()
			}
		}
		if !emit(types.LLMMessageStop{
			StopReason: finishReason,
			Model:      model,
			Usage:      usage,
		}) {
			return ctx.Err()
		}
		return nil
	}
	fail := func(err error) {
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			emitContextError()
			return
		}
		_ = emit(types.LLMError{Error: err})
	}

	for {
		if ctx.Err() != nil {
			emitContextError()
			return
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data == "[DONE]" {
					if finalErr := finalize(); finalErr != nil {
						fail(finalErr)
					}
					return
				}

				var chunk streamChunk
				if unmarshalErr := json.Unmarshal([]byte(data), &chunk); unmarshalErr != nil {
					fail(fmt.Errorf("invalid SSE JSON: %w", unmarshalErr))
					return
				}

				if chunk.ID != "" {
					if currentMessageID == "" {
						currentMessageID = chunk.ID
						if !emit(types.LLMMessageStart{ID: chunk.ID}) {
							emitContextError()
							return
						}
					} else if chunk.ID != currentMessageID {
						fail(fmt.Errorf("SSE message ID changed from %q to %q", currentMessageID, chunk.ID))
						return
					}
				}
				if chunk.Model != "" {
					model = chunk.Model
				}
				if chunk.Usage != nil {
					usage.InputTokens = chunk.Usage.PromptTokens
					usage.OutputTokens = chunk.Usage.CompletionTokens
					usage.CacheReadInputTokens = chunk.Usage.PromptDetails.CachedTokens
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]
					if choice.Delta.ReasoningContent != "" {
						if !emit(types.LLMReasoningDelta{Delta: choice.Delta.ReasoningContent}) {
							emitContextError()
							return
						}
					}
					if choice.Delta.Content != "" {
						if !emit(types.LLMTextDelta{Delta: choice.Delta.Content}) {
							emitContextError()
							return
						}
					}

					for _, delta := range choice.Delta.ToolCalls {
						if delta.Index == nil || *delta.Index < 0 {
							fail(fmt.Errorf("tool call delta is missing a valid index"))
							return
						}
						index := *delta.Index
						call, ok := toolCalls[index]
						if !ok {
							if len(toolCalls) >= maxStreamedToolCalls {
								fail(fmt.Errorf("too many streamed tool calls: limit is %d", maxStreamedToolCalls))
								return
							}
							call = &accumulatedToolCall{}
							toolCalls[index] = call
						}

						if delta.ID != "" {
							if call.id != "" && call.id != delta.ID {
								fail(fmt.Errorf("tool call index %d changed ID from %q to %q", index, call.id, delta.ID))
								return
							}
							call.id = delta.ID
						}
						if delta.Function.Name != "" && delta.Function.Name != call.name.String() {
							if call.name.Len()+len(delta.Function.Name) > maxStreamedToolNameBytes {
								fail(fmt.Errorf("tool call index %d function name exceeds %d bytes", index, maxStreamedToolNameBytes))
								return
							}
							call.name.WriteString(delta.Function.Name)
						}
						if delta.Function.Arguments != "" {
							if call.arguments.Len()+len(delta.Function.Arguments) > maxStreamedArguments {
								fail(fmt.Errorf("tool call index %d arguments exceed %d bytes", index, maxStreamedArguments))
								return
							}
							call.arguments.WriteString(delta.Function.Arguments)
						}
					}

					if choice.FinishReason != nil && *choice.FinishReason != "" {
						if finishReason != "" && finishReason != *choice.FinishReason {
							fail(fmt.Errorf("SSE finish reason changed from %q to %q", finishReason, *choice.FinishReason))
							return
						}
						finishReason = *choice.FinishReason
					}
				}
			}
		}

		if err != nil {
			if ctx.Err() != nil {
				emitContextError()
				return
			}
			if err == io.EOF {
				if finalErr := finalize(); finalErr != nil {
					fail(finalErr)
				}
				return
			}
			fail(fmt.Errorf("SSE read failed: %w", err))
			return
		}
	}
}

// modelCache caches fetched model lists per base URL.
