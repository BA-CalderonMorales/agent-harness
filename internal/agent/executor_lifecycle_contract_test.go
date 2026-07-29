package agent

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestStreamingToolExecutorCloseDrainsAcceptedToolBeforeClosingEvents(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	executor := NewStreamingToolExecutor(
		[]tools.Tool{lifecycleGateTool("gated", true, started, release, nil)},
		nil,
		tools.Context{AbortController: context.Background()},
	)

	executor.AddTool(
		types.ToolUseBlock{ID: "tool-gated", Name: "gated", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	<-started

	closeReturned := make(chan struct{})
	go func() {
		executor.Close()
		close(closeReturned)
	}()

	closedBeforeToolFinished := false
	select {
	case <-closeReturned:
		closedBeforeToolFinished = true
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	results, err := executor.GetRemainingResults(context.Background())
	if err != nil {
		t.Fatalf("waiting for accepted tool: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("accepted tool results = %d, want 1", len(results))
	}

	select {
	case <-closeReturned:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after the accepted tool finished")
	}

	var finalEvents []types.StreamMessage
	for event := range executor.Events() {
		if final, ok := event.(types.StreamMessage); ok {
			finalEvents = append(finalEvents, final)
		}
	}

	if closedBeforeToolFinished {
		t.Error("Close returned while an accepted tool was still running")
	}
	if len(finalEvents) != 1 {
		t.Fatalf("final result events = %d, want exactly 1", len(finalEvents))
	}
	if got := lifecycleToolResult(t, finalEvents[0].Message).ToolUseID; got != "tool-gated" {
		t.Fatalf("final result tool ID = %q, want %q", got, "tool-gated")
	}
}

func TestStreamingToolExecutorCancellationWakesEveryResultWaiter(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	executor := NewStreamingToolExecutor(
		[]tools.Tool{lifecycleGateTool("blocked", true, started, release, nil)},
		nil,
		tools.Context{AbortController: context.Background()},
	)

	executor.AddTool(
		types.ToolUseBlock{ID: "tool-blocked", Name: "blocked", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	<-started

	waitCtx, cancel := context.WithCancel(context.Background())
	const waiterCount = 3
	waiters := make(chan error, waiterCount)
	for range waiterCount {
		go func() {
			_, err := executor.GetRemainingResults(waitCtx)
			waiters <- err
		}()
	}
	cancel()

	var waiterErrors []error
	timer := time.NewTimer(250 * time.Millisecond)
	for len(waiterErrors) < waiterCount {
		select {
		case err := <-waiters:
			waiterErrors = append(waiterErrors, err)
		case <-timer.C:
			wokeOnCancellation := len(waiterErrors)
			close(release)
			for len(waiterErrors) < waiterCount {
				select {
				case err := <-waiters:
					waiterErrors = append(waiterErrors, err)
				case <-time.After(time.Second):
					t.Fatal("result waiters remained blocked after tool cleanup")
				}
			}
			_ = waitLifecycleFinalEvent(t, executor.Events())
			executor.Close()
			t.Fatalf("only %d of %d result waiters woke on cancellation", wokeOnCancellation, waiterCount)
		}
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	for i, err := range waiterErrors {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("waiter %d error = %v, want context.Canceled", i, err)
		}
	}

	close(release)
	if _, err := executor.GetRemainingResults(context.Background()); err != nil {
		t.Fatalf("waiting for tool cleanup: %v", err)
	}
	_ = waitLifecycleFinalEvent(t, executor.Events())
	executor.Close()
}

func TestStreamingToolExecutorReturnsResultsInSubmissionOrder(t *testing.T) {
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	secondMapped := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	defs := []tools.Tool{
		lifecycleGateTool("first", true, firstStarted, releaseFirst, nil),
		lifecycleMappedGateTool("second", secondStarted, releaseSecond, secondMapped),
	}
	executor := NewStreamingToolExecutor(defs, nil, tools.Context{AbortController: context.Background()})

	// Keep worker pointers stable so this test isolates result ordering from the
	// separate late-append ownership defect in the executor's value slice.
	executor.tools = make([]trackedTool, 0, len(defs))

	executor.AddTool(
		types.ToolUseBlock{ID: "tool-first", Name: "first", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	executor.AddTool(
		types.ToolUseBlock{ID: "tool-second", Name: "second", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	<-firstStarted
	<-secondStarted

	close(releaseSecond)
	<-secondMapped
	close(releaseFirst)
	var eventIDs []string
	for range 2 {
		event := waitLifecycleFinalEvent(t, executor.Events())
		eventIDs = append(eventIDs, lifecycleToolResult(t, event.Message).ToolUseID)
	}
	wantIDs := []string{"tool-first", "tool-second"}
	for i := range wantIDs {
		if eventIDs[i] != wantIDs[i] {
			t.Fatalf("final event order = %v, want %v", eventIDs, wantIDs)
		}
	}

	results, err := executor.GetRemainingResults(context.Background())
	if err != nil {
		t.Fatalf("waiting for results: %v", err)
	}
	executor.Close()

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	gotIDs := []string{
		lifecycleToolResult(t, results[0]).ToolUseID,
		lifecycleToolResult(t, results[1]).ToolUseID,
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("result order = %v, want %v", gotIDs, wantIDs)
		}
	}
}

func TestStreamingToolExecutorRetainsTrackedStateWhenQueueGrowsAfterWorkerStart(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	defs := []tools.Tool{
		lifecycleGateTool("first", true, firstStarted, releaseFirst, nil),
		lifecycleGateTool("late", true, nil, nil, nil),
	}
	executor := NewStreamingToolExecutor(defs, nil, tools.Context{AbortController: context.Background()})

	// Force the second submission to move the value slice after executeTool has
	// retained a pointer to the first element.
	executor.tools = make([]trackedTool, 0, 1)
	executor.AddTool(
		types.ToolUseBlock{ID: "tool-first", Name: "first", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	<-firstStarted

	executor.AddTool(
		types.ToolUseBlock{ID: "tool-late", Name: "late", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)
	close(releaseFirst)

	eventIDs := make(map[string]bool, 2)
	for range 2 {
		event := waitLifecycleFinalEvent(t, executor.Events())
		eventIDs[lifecycleToolResult(t, event.Message).ToolUseID] = true
	}

	executor.mu.Lock()
	firstStatus := executor.tools[0].status
	firstResults := len(executor.tools[0].results)
	executor.mu.Unlock()
	executor.Close()

	if !eventIDs["tool-first"] || !eventIDs["tool-late"] {
		t.Errorf("final event IDs = %v, want both submitted tools", eventIDs)
	}
	if firstStatus != statusCompleted {
		t.Errorf("first tracked status after completion = %q, want %q", firstStatus, statusCompleted)
	}
	if firstResults != 1 {
		t.Errorf("first tracked result count = %d, want 1", firstResults)
	}
}

func TestStreamingToolExecutorStreamsStructuredUnknownToolFailure(t *testing.T) {
	executor := NewStreamingToolExecutor(nil, nil, tools.Context{AbortController: context.Background()})
	executor.AddTool(
		types.ToolUseBlock{ID: "tool-missing", Name: "missing", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)

	results, err := executor.GetRemainingResults(context.Background())
	if err != nil {
		t.Fatalf("waiting for unknown-tool result: %v", err)
	}
	executor.Close()

	if len(results) != 1 {
		t.Fatalf("unknown-tool results = %d, want 1", len(results))
	}
	resultBlock := lifecycleToolResult(t, results[0])
	if resultBlock.ToolUseID != "tool-missing" || !resultBlock.IsError {
		t.Fatalf("unknown-tool result = %#v, want correlated structured failure", resultBlock)
	}

	var finalEvents []types.StreamMessage
	for event := range executor.Events() {
		if final, ok := event.(types.StreamMessage); ok {
			finalEvents = append(finalEvents, final)
		}
	}
	if len(finalEvents) != 1 {
		t.Fatalf("unknown-tool final events = %d, want exactly 1", len(finalEvents))
	}
	eventBlock := lifecycleToolResult(t, finalEvents[0].Message)
	if eventBlock.ToolUseID != "tool-missing" || !eventBlock.IsError {
		t.Fatalf("unknown-tool event = %#v, want correlated structured failure", eventBlock)
	}
}

func TestStreamingToolExecutorStreamsStructuredToolFailure(t *testing.T) {
	executor := NewStreamingToolExecutor(
		[]tools.Tool{lifecycleGateTool("failing", true, nil, nil, errors.New("deterministic failure"))},
		nil,
		tools.Context{AbortController: context.Background()},
	)
	executor.AddTool(
		types.ToolUseBlock{ID: "tool-failing", Name: "failing", Input: map[string]any{}},
		types.Message{UUID: "assistant"},
	)

	finalEvent := waitLifecycleFinalEvent(t, executor.Events())
	results, err := executor.GetRemainingResults(context.Background())
	if err != nil {
		t.Fatalf("waiting for failed tool result: %v", err)
	}
	executor.Close()

	if len(results) != 1 {
		t.Fatalf("failed-tool results = %d, want 1", len(results))
	}
	for label, message := range map[string]types.Message{
		"result": results[0],
		"event":  finalEvent.Message,
	} {
		block := lifecycleToolResult(t, message)
		if block.ToolUseID != "tool-failing" || !block.IsError {
			t.Errorf("%s failure = %#v, want correlated structured failure", label, block)
		}
		if block.Content != "deterministic failure" {
			t.Errorf("%s failure content = %q, want %q", label, block.Content, "deterministic failure")
		}
	}
}

func TestLoopQueryStreamsStructuredToolFailureBeforeTerminalReason(t *testing.T) {
	client := &lifecycleScriptedClient{
		streams: [][]types.LLMEvent{
			{
				types.LLMMessageStart{ID: "assistant-tool"},
				types.LLMToolUseDelta{ID: "tool-failing", Name: "failing", Delta: `{}`},
				types.LLMMessageStop{StopReason: "tool_use", Model: "test-model"},
			},
			{
				types.LLMMessageStart{ID: "assistant-final"},
				types.LLMTextDelta{Delta: "finished after tool failure"},
				types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
			},
		},
	}
	failingTool := lifecycleGateTool("failing", true, nil, nil, errors.New("deterministic failure"))
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	// Keep this contract focused on the public stream's terminal visibility.
	// Structured batch failures use the same ToolResultBlock caller contract
	// without also exercising the known executor close/send race.
	loop.Config.StreamingToolExecution = false

	stream, err := loop.Query(context.Background(), QueryParams{
		SystemPrompt: "test",
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options: tools.Options{
				MainLoopModel: "test-model",
				Tools:         []tools.Tool{failingTool},
			},
			AbortController: context.Background(),
		},
		MaxTurns: 3,
	})
	if err != nil {
		t.Fatalf("starting public query: %v", err)
	}

	var (
		failureSeen      bool
		terminalSeen     bool
		terminalCount    int
		terminalReason   string
		failurePosition  = -1
		terminalPosition = -1
		eventPosition    int
	)
	for event := range stream {
		if message, ok := event.(types.StreamMessage); ok {
			for _, content := range message.Message.Content {
				result, ok := content.(types.ToolResultBlock)
				if !ok || result.ToolUseID != "tool-failing" {
					continue
				}
				if !result.IsError || result.Content != "deterministic failure" {
					t.Fatalf("tool failure = %#v, want correlated structured failure", result)
				}
				failureSeen = true
				failurePosition = eventPosition
			}
		}
		if reason, ok := reflectedTerminalReason(event); ok {
			terminalSeen = true
			terminalCount++
			terminalReason = reason
			terminalPosition = eventPosition
		}
		eventPosition++
	}

	if !failureSeen {
		t.Error("public query stream did not expose the correlated failed-tool result")
	}
	if !terminalSeen {
		t.Fatal("public query stream closed without exposing its TerminalReason")
	}
	if terminalReason != string(TerminalReasonComplete) {
		t.Fatalf("public terminal reason = %q, want %q", terminalReason, TerminalReasonComplete)
	}
	if terminalCount != 1 {
		t.Fatalf("public terminal event count = %d, want exactly 1", terminalCount)
	}
	if failurePosition >= terminalPosition || terminalPosition != eventPosition-1 {
		t.Fatalf("failed-tool result position = %d, terminal position = %d, event count = %d; terminal must be last", failurePosition, terminalPosition, eventPosition)
	}
}

type lifecycleScriptedClient struct {
	streams [][]types.LLMEvent
	call    int
}

func (c *lifecycleScriptedClient) Stream(ctx context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var events []types.LLMEvent
	if c.call < len(c.streams) {
		events = c.streams[c.call]
	}
	c.call++

	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func reflectedTerminalReason(event types.StreamEvent) (string, bool) {
	value := reflect.ValueOf(event)
	if !value.IsValid() {
		return "", false
	}
	eventType := value.Type()
	if eventType.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		eventType = eventType.Elem()
		value = value.Elem()
	}
	if !strings.Contains(strings.ToLower(eventType.Name()), "terminal") {
		return "", false
	}
	if value.Kind() != reflect.Struct {
		return "", false
	}

	reason := value.FieldByName("Reason")
	if !reason.IsValid() {
		terminal := value.FieldByName("Terminal")
		if terminal.IsValid() {
			if terminal.Kind() == reflect.Pointer {
				if terminal.IsNil() {
					return "", false
				}
				terminal = terminal.Elem()
			}
			if terminal.Kind() == reflect.Struct {
				reason = terminal.FieldByName("Reason")
			}
		}
	}
	if !reason.IsValid() || reason.Kind() != reflect.String {
		return "", false
	}
	return reason.String(), true
}

func lifecycleGateTool(name string, safe bool, started chan<- struct{}, release <-chan struct{}, callErr error) tools.Tool {
	return tools.NewTool(tools.Tool{
		Name: name,
		Capabilities: tools.CapabilityFlags{
			IsConcurrencySafe: func(map[string]any) bool { return safe },
		},
		Call: func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
			if started != nil {
				close(started)
			}
			if release != nil {
				<-release
			}
			if callErr != nil {
				return tools.ToolResult{}, callErr
			}
			return tools.ToolResult{Data: name + "-result"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{
				ToolUseID: toolUseID,
				Content:   fmt.Sprintf("%v", result),
			}
		},
	})
}

func lifecycleMappedGateTool(name string, started chan<- struct{}, release <-chan struct{}, mapped chan<- struct{}) tools.Tool {
	tool := lifecycleGateTool(name, true, started, release, nil)
	mapResult := tool.MapResult
	tool.MapResult = func(result any, toolUseID string) types.ToolResultBlock {
		block := mapResult(result, toolUseID)
		close(mapped)
		return block
	}
	return tool
}

func waitLifecycleFinalEvent(t *testing.T, events <-chan types.StreamEvent) types.StreamMessage {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before a final tool result")
		}
		final, ok := event.(types.StreamMessage)
		if !ok {
			t.Fatalf("event type = %T, want types.StreamMessage", event)
		}
		return final
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for final tool result event")
		return types.StreamMessage{}
	}
}

func lifecycleToolResult(t *testing.T, message types.Message) types.ToolResultBlock {
	t.Helper()
	if len(message.Content) != 1 {
		t.Fatalf("message content blocks = %d, want 1", len(message.Content))
	}
	block, ok := message.Content[0].(types.ToolResultBlock)
	if !ok {
		t.Fatalf("message content type = %T, want types.ToolResultBlock", message.Content[0])
	}
	return block
}
