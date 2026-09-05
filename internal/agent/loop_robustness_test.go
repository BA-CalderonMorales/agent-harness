package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Cancellation and stream-robustness contracts: the loop must settle
// — every started tool carries a final record, a dead provider becomes
// one clean error, and a cancelled turn leaves no stuck state.

// hangingClient streams a tool call, then hangs forever: the model
// died mid-turn. The loop's idle watchdog must convert the hang into
// one clean error instead of waiting out eternity.
type hangingClient struct {
	llm.Client
}

func (c *hangingClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	out := make(chan types.LLMEvent, 8)
	go func() {
		defer close(out)
		out <- types.LLMMessageStart{ID: "m1"}
		out <- types.LLMTextDelta{Delta: "I am about to hang mid-sen"}
		<-time.After(10 * time.Second) // outlive the idle window: no stop, no close
	}()
	return out, nil
}

// disconnectClient opens the channel and slams it shut mid-turn: the
// provider vanished. No MessageStop, no tool results — just silence.
type disconnectClient struct {
	llm.Client
}

func (c *disconnectClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	out := make(chan types.LLMEvent, 8)
	go func() {
		defer close(out)
		out <- types.LLMMessageStart{ID: "m1"}
		out <- types.LLMTextDelta{Delta: "I was about to say some"}
		// The channel closes with no stop event: the provider vanished
		// mid-sentence. consumeStream must name it, not pass the
		// truncated text off as a complete answer.
	}()
	return out, nil
}

// duplicateClient re-runs the identical tool with identical input —
// the runaway-loop signature. The convergence guard must abort the
// turn, not clone the call.
type duplicateClient struct {
	llm.Client
	calls int
}

func (c *duplicateClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.calls++
	out := make(chan types.LLMEvent, 8)
	go func() {
		defer close(out)
		out <- types.LLMMessageStart{ID: "m" + string(rune('0'+c.calls))}
		out <- types.LLMToolUseDelta{ID: "t-dup-" + string(rune('0'+c.calls)), Name: "echo", Delta: `{"text":"same"}`}
		out <- types.LLMMessageStop{StopReason: "tool_calls"}
	}()
	return out, nil
}

func robustnessParams() QueryParams {
	return QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			AbortController: context.Background(),
		},
	}
}

func TestIdleStreamBecomesOneCleanError(t *testing.T) {
	loop := NewLoop(&hangingClient{})
	loop.Config.StreamIdleTimeout = 150 * time.Millisecond
	loop.Config.MaxToolCalls = 5
	loop.Config.MaxIdenticalToolUses = 10

	stream, err := loop.Query(context.Background(), robustnessParams())
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	errCount := 0
	var lastErr error
	for event := range stream {
		if terminal, ok := event.(types.StreamTerminal); ok && terminal.Error != nil {
			errCount++
			lastErr = terminal.Error
		}
	}
	if errCount != 1 {
		t.Fatalf("idle stream produced %d error terminals, want exactly 1 clean error (last: %v)", errCount, lastErr)
	}
	if !strings.Contains(lastErr.Error(), "idle") {
		t.Fatalf("idle error does not name the idle watchdog: %v", lastErr)
	}
}

func TestDisconnectMidTurnBecomesOneCleanError(t *testing.T) {
	loop := NewLoop(&disconnectClient{})
	loop.Config.MaxToolCalls = 5

	stream, err := loop.Query(context.Background(), robustnessParams())
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	errCount := 0
	partialText := ""
	for event := range stream {
		switch e := event.(type) {
		case types.StreamTerminal:
			if e.Error != nil {
				errCount++
			}
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if tb, ok := block.(types.TextBlock); ok {
					partialText += tb.Text
				}
			}
		}
	}
	if errCount != 1 {
		t.Fatalf("disconnect produced %d error terminals, want exactly 1 clean error", errCount)
	}
	if !strings.Contains(partialText, "I was about to say some") {
		t.Fatalf("partial stream text was dropped before the error: %q", partialText)
	}
}

func TestDuplicateToolCallsAbortTheTurn(t *testing.T) {
	loop := NewLoop(&duplicateClient{})
	loop.Config.MaxToolCalls = 100
	loop.Config.MaxIdenticalToolUses = 1
	loop.Config.DefaultMaxTurns = 100

	stream, err := loop.Query(context.Background(), robustnessParams())
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	sawLoopGuard := false
	events := 0
	for event := range stream {
		events++
		if msg, ok := event.(types.StreamMessage); ok {
			for _, block := range msg.Message.Content {
				if tb, ok := block.(types.TextBlock); ok && strings.Contains(tb.Text, "Tool loop detected") {
					sawLoopGuard = true
				}
			}
		}
		if events > 500 {
			t.Fatal("duplicate tool calls did not abort the turn — runaway loop")
		}
	}
	if !sawLoopGuard {
		t.Fatal("the convergence guard never announced the loop")
	}
}
