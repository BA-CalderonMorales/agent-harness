package llm

import (
	"context"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// MockClient is a test double for LLMClient.
type MockClient struct {
	Events []types.LLMEvent
	Err    error
}

// Stream implements Client by yielding pre-configured events.
func (m *MockClient) Stream(ctx context.Context, req Request) (<-chan types.LLMEvent, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := make(chan types.LLMEvent, len(m.Events))
	for _, ev := range m.Events {
		out <- ev
	}
	close(out)
	return out, nil
}

// MockTextResponse creates a simple text-only response.
func MockTextResponse(text string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "msg_1"},
		types.LLMTextDelta{Delta: text},
		types.LLMMessageStop{StopReason: "stop", Usage: types.TokenUsage{InputTokens: 10, OutputTokens: 5}},
	}
}

// MockToolUseResponse creates a response requesting a tool.
func MockToolUseResponse(toolName, toolInput string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "msg_1"},
		types.LLMToolUseDelta{ID: "tu_1", Name: toolName, Delta: toolInput},
		types.LLMMessageStop{StopReason: "tool_use", Usage: types.TokenUsage{InputTokens: 10, OutputTokens: 5}},
	}
}
