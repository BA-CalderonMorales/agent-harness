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

	_, toolUses, err := loop.consumeStream(context.Background(), events, make(chan types.StreamEvent, 8))
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
	out := make(chan types.StreamEvent, len(input)+2)
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
	var streamed strings.Builder
	var emitted *types.StreamMessage
	for len(out) > 0 {
		switch event := (<-out).(type) {
		case types.StreamTextDelta:
			streamed.WriteString(event.Text)
		case types.StreamMessage:
			message := event
			emitted = &message
		}
	}
	if emitted == nil {
		t.Fatal("no final StreamMessage emitted")
	}
	if len(emitted.Message.Content) != len(msg.Content) {
		t.Fatal("streamed content differs from durable message")
	}
	want := "I will inspect the workspace.Next, read the configuration.Then I will summarize."
	if streamed.String() != want {
		t.Fatalf("streamed prose = %q, want %q", streamed.String(), want)
	}
}

func TestConsumeStreamEmitsToolUseAtItsStreamPosition(t *testing.T) {
	input := []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMTextDelta{Delta: "before "},
		types.LLMToolUseDelta{ID: "call-1", Name: "bash", Delta: `{"command":"pwd"}`},
		types.LLMTextDelta{Delta: "after"},
		types.LLMMessageStop{StopReason: "tool_use"},
	}
	events := make(chan types.LLMEvent, len(input))
	for _, event := range input {
		events <- event
	}
	close(events)
	out := make(chan types.StreamEvent, len(input)+2)

	if _, _, err := (&Loop{}).consumeStream(context.Background(), events, out); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for len(out) > 0 {
		switch event := (<-out).(type) {
		case types.StreamTextDelta:
			kinds = append(kinds, "text:"+event.Text)
		case types.StreamToolUse:
			kinds = append(kinds, "tool:"+event.Block.ID)
		case types.StreamMessage:
			kinds = append(kinds, "message")
		}
	}
	want := []string{"text:before ", "tool:call-1", "text:after", "message"}
	if strings.Join(kinds, "|") != strings.Join(want, "|") {
		t.Fatalf("event order = %v, want %v", kinds, want)
	}
}

// TestConsumeStreamBoundsLiveReasoningPreview: a hosted reasoning model
// can stream for a minute. The preview only shows the tail, so forwarding
// the whole accumulated trace on every delta was quadratic copying.
func TestConsumeStreamBoundsLiveReasoningPreview(t *testing.T) {
	chunk := strings.Repeat("r", 1024)
	input := make([]types.LLMEvent, 0, 12)
	for i := 0; i < 10; i++ {
		input = append(input, types.LLMReasoningDelta{Delta: chunk})
	}
	input = append(input,
		types.LLMTextDelta{Delta: "answer"},
		types.LLMMessageStop{StopReason: "stop"},
	)
	events := make(chan types.LLMEvent, len(input))
	for _, event := range input {
		events <- event
	}
	close(events)
	out := make(chan types.StreamEvent, 32)

	if _, _, err := (&Loop{}).consumeStream(context.Background(), events, out); err != nil {
		t.Fatal(err)
	}

	var last string
	previews := 0
	for len(out) > 0 {
		switch event := (<-out).(type) {
		case types.StreamThinking:
			previews++
			if len(event.Text) > maxLiveReasoning {
				t.Fatalf("thinking preview = %d bytes, want at most %d", len(event.Text), maxLiveReasoning)
			}
			last = event.Text
		}
	}
	if previews == 0 {
		t.Fatal("no thinking previews emitted")
	}
	if want := strings.Repeat("r", maxLiveReasoning); last != want {
		t.Fatalf("final preview length = %d, want the bounded tail of length %d", len(last), len(want))
	}
}

func TestConsumeStreamEmitsTextDeltasBeforeTheFinalMessage(t *testing.T) {
	input := []types.LLMEvent{
		types.LLMMessageStart{ID: "message-1"},
		types.LLMTextDelta{Delta: "Hel"},
		types.LLMTextDelta{Delta: "lo"},
		types.LLMMessageStop{StopReason: "stop"},
	}
	events := make(chan types.LLMEvent, len(input))
	for _, event := range input {
		events <- event
	}
	close(events)
	out := make(chan types.StreamEvent, len(input))

	msg, _, err := (&Loop{}).consumeStream(context.Background(), events, out)
	if err != nil {
		t.Fatal(err)
	}
	var emitted []types.StreamEvent
	for len(out) > 0 {
		emitted = append(emitted, <-out)
	}
	if len(emitted) != 3 {
		t.Fatalf("got %d events, want two deltas and the final message", len(emitted))
	}
	for i, want := range []string{"Hel", "lo"} {
		delta, ok := emitted[i].(types.StreamTextDelta)
		if !ok || delta.Text != want {
			t.Fatalf("event %d = %#v, want delta %q", i, emitted[i], want)
		}
	}
	final, ok := emitted[2].(types.StreamMessage)
	if !ok {
		t.Fatalf("last event = %T, want StreamMessage", emitted[2])
	}
	if got := final.Message.Content[0].(types.TextBlock).Text; got != "Hello" {
		t.Fatalf("final message text = %q, want the assembled prose", got)
	}
	if got := msg.Content[0].(types.TextBlock).Text; got != "Hello" {
		t.Fatalf("durable message text = %q, want the assembled prose", got)
	}
}
