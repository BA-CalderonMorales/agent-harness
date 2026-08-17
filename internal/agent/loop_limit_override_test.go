package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// toolBatchClient serves one assistant message per request: the first
// request yields batchSize tool calls, the follow-up yields a text reply.
type toolBatchClient struct {
	llm.Client
	first     bool
	batchSize int
}

func (c *toolBatchClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	first := c.first
	c.first = false // synchronous: no goroutine race on the flag
	out := make(chan types.LLMEvent, 16)
	go func() {
		defer close(out)
		if first {
			out <- types.LLMMessageStart{ID: "m1"}
			for i := 0; i < c.batchSize; i++ {
				out <- types.LLMToolUseDelta{ID: fmt.Sprintf("t%d", i), Name: "echo", Delta: fmt.Sprintf(`{"text":"%d"}`, i)}
			}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
			return
		}
		out <- types.LLMMessageStart{ID: "m2"}
		out <- types.LLMTextDelta{Delta: "done"}
		out <- types.LLMMessageStop{StopReason: "stop"}
	}()
	return out, nil
}

func TestQueryParamsMaxToolCallsOverridesLoopDefault(t *testing.T) {
	client := &toolBatchClient{first: true, batchSize: 4}
	loop := NewLoop(client)
	loop.Config.MaxToolCalls = 2 // the default ceiling would stop at 2

	params := QueryParams{
		Messages: []types.Message{{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "go"}}}},
		CanUseTool: func(name string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options:         tools.Options{Tools: []tools.Tool{echoTool()}},
			AbortController: context.Background(),
		},
		MaxToolCalls: 4, // the session /limit knob
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	sawLimit := false
	for ev := range stream {
		if sm, ok := ev.(types.StreamMessage); ok && sm.Message.Role == types.RoleSystem {
			if containsText(sm.Message, "Tool call limit reached") {
				sawLimit = true
			}
		}
	}
	if sawLimit {
		t.Fatal("session override 4 must not trip the default ceiling of 2")
	}
}

func echoTool() tools.Tool {
	return tools.Tool{
		Name:        "echo",
		Description: "echo",
		InputSchema: func() map[string]any {
			return map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}}
		},
		ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
			return tools.ValidationResult{Valid: true}
		},
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "ok"}, nil
		},
		MapResult: func(data any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: fmt.Sprintf("%v", data)}
		},
		Capabilities: tools.CapabilityFlags{IsReadOnly: func(map[string]any) bool { return true }},
	}
}

func containsText(m types.Message, substr string) bool {
	for _, b := range m.Content {
		if tb, ok := b.(types.TextBlock); ok && contains(tb.Text, substr) {
			return true
		}
	}
	return false
}
