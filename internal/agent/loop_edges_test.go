package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type scriptedLoopClient struct {
	mu        sync.Mutex
	streams   [][]types.LLMEvent
	requests  []llm.Request
	onRequest func(context.Context, int, llm.Request)
}

func (c *scriptedLoopClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	call := len(c.requests)
	c.requests = append(c.requests, req)
	var events []types.LLMEvent
	if call < len(c.streams) {
		events = c.streams[call]
	}
	onRequest := c.onRequest
	c.mu.Unlock()

	if onRequest != nil {
		onRequest(ctx, call, req)
	}
	if events == nil {
		return make(chan types.LLMEvent), nil
	}

	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func (c *scriptedLoopClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

func (c *scriptedLoopClient) requestAt(index int) llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests[index]
}

func TestLoopRetriesMaxOutputTokenErrorWithHigherLimit(t *testing.T) {
	client := &scriptedLoopClient{
		streams: [][]types.LLMEvent{
			{types.LLMError{Error: errors.New("max_output_tokens exceeded")}},
			textEvents("recovered"),
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false

	terminal, events := runLoopForEdgeTest(t, loop, edgeParams(nil), 64)

	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("expected terminal reason %q, got %q with error %v", TerminalReasonComplete, terminal.Reason, terminal.Error)
	}
	if client.callCount() != 2 {
		t.Fatalf("expected retry to call LLM twice, got %d", client.callCount())
	}
	if got := client.requestAt(1).MaxTokens; got != 16384 {
		t.Fatalf("expected retry max tokens to increase to 16384, got %d", got)
	}
	if !streamMessagesContain(events, "[Recovering: increasing output token limit to 16384]") {
		t.Fatalf("expected recovery notice in stream events, got %#v", events)
	}
	if !streamMessagesContain(events, "recovered") {
		t.Fatalf("expected recovered assistant response in stream events, got %#v", events)
	}
}

func TestLoopPassesConfiguredGenerationOptions(t *testing.T) {
	client := &scriptedLoopClient{
		streams: [][]types.LLMEvent{textEvents("ok")},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false

	params := edgeParams(nil)
	params.MaxOutputTokens = 1234
	params.Temperature = 0.25
	terminal, _ := runLoopForEdgeTest(t, loop, params, 64)

	if terminal.Reason != TerminalReasonComplete {
		t.Fatalf("expected terminal reason %q, got %q", TerminalReasonComplete, terminal.Reason)
	}
	req := client.requestAt(0)
	if req.MaxTokens != 1234 {
		t.Fatalf("MaxTokens = %d, want 1234", req.MaxTokens)
	}
	if req.Temperature != 0.25 {
		t.Fatalf("Temperature = %v, want 0.25", req.Temperature)
	}
}

func TestLoopStopsAfterRecoveryAttemptsExhausted(t *testing.T) {
	client := &scriptedLoopClient{
		streams: [][]types.LLMEvent{
			{types.LLMError{Error: errors.New("max_output_tokens exceeded")}},
			{types.LLMError{Error: errors.New("max_output_tokens exceeded again")}},
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	loop.Config.MaxOutputTokensRecovery = 1

	terminal, events := runLoopForEdgeTest(t, loop, edgeParams(nil), 64)

	if terminal.Reason != TerminalReasonError {
		t.Fatalf("expected terminal reason %q, got %q", TerminalReasonError, terminal.Reason)
	}
	if client.callCount() != 2 {
		t.Fatalf("expected one recovery retry before failing, got %d calls", client.callCount())
	}
	if terminal.Error == nil || !strings.Contains(terminal.Error.Error(), "recovery failed after 1 attempts") {
		t.Fatalf("expected recovery exhaustion error, got %v", terminal.Error)
	}
	if !streamErrorsContain(events, "recovery failed after 1 attempts") {
		t.Fatalf("expected stream error to expose recovery exhaustion, got %#v", events)
	}
}

func TestLoopReturnsCancellationErrorWhenContextCancelsDuringStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &scriptedLoopClient{
		onRequest: func(context.Context, int, llm.Request) {
			cancel()
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false

	out := make(chan types.StreamEvent, 64)
	state := edgeLoopState(nil)
	terminal := loop.queryLoop(ctx, edgeParams(nil), state, out)
	events := drainEdgeEvents(out)

	if terminal.Reason != TerminalReasonError {
		t.Fatalf("expected terminal reason %q, got %q", TerminalReasonError, terminal.Reason)
	}
	if !errors.Is(terminal.Error, context.Canceled) {
		t.Fatalf("expected terminal error context.Canceled, got %v", terminal.Error)
	}
	if !streamErrorsContain(events, context.Canceled.Error()) {
		t.Fatalf("expected cancellation stream error, got %#v", events)
	}
}

func TestLoopReturnsMaxTurnsAfterToolResultWhenTurnBudgetExpires(t *testing.T) {
	var toolCalls int
	tool := edgeTool(&toolCalls)
	client := &scriptedLoopClient{
		streams: [][]types.LLMEvent{
			toolEvents("tool_1", tool.Name, `{"cmd":"first"}`),
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false

	params := edgeParams([]tools.Tool{tool})
	params.MaxTurns = 1
	terminal, _ := runLoopForEdgeTest(t, loop, params, 64)

	if terminal.Reason != TerminalReasonMaxTurns {
		t.Fatalf("expected terminal reason %q, got %q", TerminalReasonMaxTurns, terminal.Reason)
	}
	if client.callCount() != 1 {
		t.Fatalf("expected one LLM turn before max-turn stop, got %d", client.callCount())
	}
	if toolCalls != 1 {
		t.Fatalf("expected tool to execute before max-turn stop, got %d calls", toolCalls)
	}
}

func TestLoopAllowsToolAtBudgetThenBlocksNextToolRequest(t *testing.T) {
	var toolCalls int
	tool := edgeTool(&toolCalls)
	client := &scriptedLoopClient{
		streams: [][]types.LLMEvent{
			toolEvents("tool_1", tool.Name, `{"cmd":"first"}`),
			toolEvents("tool_2", tool.Name, `{"cmd":"second"}`),
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	loop.Config.MaxToolCalls = 1
	loop.Config.DefaultMaxTurns = 5

	terminal, events := runLoopForEdgeTest(t, loop, edgeParams([]tools.Tool{tool}), 64)

	if terminal.Reason != TerminalReasonBlockingLimit {
		t.Fatalf("expected terminal reason %q, got %q", TerminalReasonBlockingLimit, terminal.Reason)
	}
	if client.callCount() != 2 {
		t.Fatalf("expected second LLM turn to hit tool budget, got %d calls", client.callCount())
	}
	if toolCalls != 1 {
		t.Fatalf("expected only first in-budget tool call to execute, got %d", toolCalls)
	}
	if !streamMessagesContain(events, "Tool call limit reached (1 tools). Runaway-loop protection stopped this turn. Type /limit 2 to continue") {
		t.Fatalf("expected tool budget warning in stream events, got %#v", events)
	}
}

func runLoopForEdgeTest(t *testing.T, loop *Loop, params QueryParams, buffer int) (Terminal, []types.StreamEvent) {
	t.Helper()
	out := make(chan types.StreamEvent, buffer)
	state := edgeLoopState(params.Messages)
	terminal := loop.queryLoop(context.Background(), params, state, out)
	return terminal, drainEdgeEvents(out)
}

func edgeLoopState(messages []types.Message) *loopState {
	return &loopState{
		messages:       messages,
		toolUseContext: edgeToolContext(nil),
		turnCount:      1,
	}
}

func edgeParams(toolDefs []tools.Tool) QueryParams {
	return QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: edgeToolContext(toolDefs),
	}
}

func edgeToolContext(toolDefs []tools.Tool) tools.Context {
	return tools.Context{
		Options: tools.Options{
			MainLoopModel: "test-model",
			Tools:         toolDefs,
		},
		AbortController: context.Background(),
	}
}

func edgeTool(calls *int) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: "edge_tool",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			*calls++
			return tools.ToolResult{Data: fmt.Sprintf("ran %v", input["cmd"])}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})
}

func textEvents(text string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "msg_text"},
		types.LLMTextDelta{Delta: text},
		types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
	}
}

func toolEvents(id, name, input string) []types.LLMEvent {
	return []types.LLMEvent{
		types.LLMMessageStart{ID: "msg_" + id},
		types.LLMToolUseDelta{ID: id, Name: name, Delta: input},
		types.LLMMessageStop{StopReason: "tool_use", Model: "test-model"},
	}
}

func drainEdgeEvents(out <-chan types.StreamEvent) []types.StreamEvent {
	var events []types.StreamEvent
	for {
		select {
		case event := <-out:
			events = append(events, event)
		default:
			return events
		}
	}
}

func streamMessagesContain(events []types.StreamEvent, want string) bool {
	for _, event := range events {
		streamMessage, ok := event.(types.StreamMessage)
		if !ok {
			continue
		}
		for _, block := range streamMessage.Message.Content {
			textBlock, ok := block.(types.TextBlock)
			if ok && strings.Contains(textBlock.Text, want) {
				return true
			}
		}
	}
	return false
}

func streamErrorsContain(events []types.StreamEvent, want string) bool {
	for _, event := range events {
		streamError, ok := event.(types.StreamError)
		if ok && streamError.Error != nil && strings.Contains(streamError.Error.Error(), want) {
			return true
		}
	}
	return false
}
