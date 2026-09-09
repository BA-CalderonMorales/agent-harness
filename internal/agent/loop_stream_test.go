package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestConsumeStreamRejectsMalformedToolInputAtMessageStop(t *testing.T) {
	err := consumeStreamError(t, []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"command":`},
		types.LLMMessageStop{StopReason: "tool_use"},
	})

	assertMalformedToolInput(t, err, "call-1", "bash")
}

func TestConsumeStreamRejectsMalformedToolInputAtToolIDTransition(t *testing.T) {
	err := consumeStreamError(t, []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"command":`},
		types.LLMToolUseDelta{ID: "call-2", Name: "read", Delta: `{"path":"ok"}`},
	})

	assertMalformedToolInput(t, err, "call-1", "bash")
}

func TestConsumeStreamRejectsMalformedToolInputWhenStreamCloses(t *testing.T) {
	err := consumeStreamError(t, []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"command":`},
	})

	assertMalformedToolInput(t, err, "call-1", "bash")
}

func TestConsumeStreamRejectsEmptyToolInput(t *testing.T) {
	err := consumeStreamError(t, []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: ""},
		types.LLMMessageStop{StopReason: "tool_use"},
	})

	assertMalformedToolInput(t, err, "call-1", "bash")
}

func TestConsumeStreamPreservesValidFragmentedToolInput(t *testing.T) {
	loop := &Loop{}
	events := make(chan types.LLMEvent, 4)
	events <- types.LLMMessageStart{ID: "message-1"}
	events <- types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"com`}
	events <- types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `mand":"ls"}`}
	events <- types.LLMMessageStop{StopReason: "tool_use"}
	close(events)

	_, toolUses, err := loop.consumeStream(context.Background(), events, make(chan types.StreamEvent, 1))
	if err != nil {
		t.Fatalf("consumeStream() error = %v", err)
	}
	if len(toolUses) != 1 {
		t.Fatalf("got %d tool uses, want 1", len(toolUses))
	}
	if got := toolUses[0].Input["command"]; got != "ls" {
		t.Fatalf("tool input command = %#v, want %q", got, "ls")
	}
}

func TestLoopDoesNotExecuteToolAfterMalformedInput(t *testing.T) {
	var executions atomic.Int32
	tool := tools.NewTool(tools.Tool{
		Name: "bash",
		Call: func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
			executions.Add(1)
			return tools.ToolResult{Data: "unexpected execution"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
	loop := NewLoop(&llm.MockClient{Events: []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"command":`},
		types.LLMMessageStop{StopReason: "tool_use"},
	}})

	stream, err := loop.Query(context.Background(), QueryParams{
		ToolUseContext: tools.Context{Options: tools.Options{Tools: []tools.Tool{tool}}},
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	var streamErr error
	for event := range stream {
		if failure, ok := event.(types.StreamError); ok {
			streamErr = failure.Error
		}
	}
	assertMalformedToolInput(t, streamErr, "call-1", "bash")
	if got := executions.Load(); got != 0 {
		t.Fatalf("malformed tool input executed %d tools, want 0", got)
	}
}

func consumeStreamError(t *testing.T, input []types.LLMEvent) error {
	t.Helper()
	events := make(chan types.LLMEvent, len(input))
	for _, event := range input {
		events <- event
	}
	close(events)

	_, _, err := (&Loop{}).consumeStream(context.Background(), events, make(chan types.StreamEvent, 1))
	if err == nil {
		t.Fatal("consumeStream() error = nil, want malformed tool input error")
	}
	return err
}

func assertMalformedToolInput(t *testing.T, err error, wantID, wantName string) {
	t.Helper()
	var malformed *malformedToolInputError
	if !errors.As(err, &malformed) {
		t.Fatalf("error type = %T, want *malformedToolInputError: %v", err, err)
	}
	if malformed.ToolID != wantID || malformed.ToolName != wantName {
		t.Fatalf("malformed tool = %q/%q, want %q/%q", malformed.ToolID, malformed.ToolName, wantID, wantName)
	}
	if !strings.Contains(err.Error(), wantID) {
		t.Fatalf("error is not actionable for tool %q: %v", wantID, err)
	}
}

func TestConsumeStreamPreservesProseAroundToolCalls(t *testing.T) {
	input := []types.LLMEvent{
		types.LLMTextDelta{Delta: "I will inspect the workspace."},
		types.LLMToolUseDelta{ID: "first", Name: "bash", Delta: `{"command":"pwd"}`},
		types.LLMTextDelta{Delta: "Next, read the configuration."},
		types.LLMToolUseDelta{ID: "second", Name: "read", Delta: `{"path":"config.yml"}`},
		types.LLMTextDelta{Delta: "Then I will summarize."},
		types.LLMMessageStop{StopReason: "tool_use"},
	}
	events := make(chan types.LLMEvent, len(input))
	for _, event := range input {
		events <- event
	}
	close(events)
	out := make(chan types.StreamEvent, 1)
	msg, calls, err := (&Loop{}).consumeStream(context.Background(), events, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Content) != 5 || len(calls) != 2 {
		t.Fatalf("content = %#v; calls = %#v", msg.Content, calls)
	}
	for i, want := range []string{"I will inspect the workspace.", "Next, read the configuration.", "Then I will summarize."} {
		block, ok := msg.Content[i*2].(types.TextBlock)
		if !ok || block.Text != want {
			t.Fatalf("block %d = %#v, want prose %q", i*2, msg.Content[i*2], want)
		}
	}
	for i, want := range []string{"first", "second"} {
		block, ok := msg.Content[i*2+1].(types.ToolUseBlock)
		if !ok || block.ID != want {
			t.Fatalf("block %d = %#v, want tool %q", i*2+1, msg.Content[i*2+1], want)
		}
	}
	emitted := (<-out).(types.StreamMessage)
	if len(emitted.Message.Content) != len(msg.Content) {
		t.Fatal("streamed content differs from durable message")
	}
}
