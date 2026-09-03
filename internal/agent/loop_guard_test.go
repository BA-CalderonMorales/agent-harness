package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// loopingClient models a model that cannot converge: it keeps issuing the
// exact same ping tool call no matter how many results it has seen.
type loopingClient struct{}

func (loopingClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	ev := make(chan types.LLMEvent, 4)
	go func() {
		defer close(ev)
		ev <- types.LLMMessageStart{ID: "m1"}
		ev <- types.LLMToolUseDelta{ID: "t1", Name: "branch", Delta: `{"name":"pp"}`}
		ev <- types.LLMToolUseDelta{ID: "t1", Name: "branch", Delta: ""}
		ev <- types.LLMMessageStop{}
	}()
	return ev, nil
}

func TestLoop_RepeatingIdenticalToolCallAbortsTurn(t *testing.T) {
	loop := NewLoop(loopingClient{})
	loop.Config.MaxIdenticalToolUses = 2 // exercise the two-strike mechanism
	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "You are a test assistant.",
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options:         tools.Options{MainLoopModel: "test-model"},
			AbortController: context.Background(),
		},
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var loopMsg string
	var reasons []string
	executions, aborted := 0, 0
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if tb, ok := block.(types.TextBlock); ok {
					if strings.Contains(tb.Text, "Tool loop detected") {
						loopMsg = tb.Text
					}
				}
				if blk, ok := block.(types.ToolResultBlock); ok && blk.IsError {
					executions++
				}
			}
			if blk, ok := e.Message.Content[0].(types.ToolUseBlock); ok && blk.Name == "branch" {
				aborted++
			}
		case types.StreamTerminal:
			reasons = append(reasons, string(e.Reason))
		}
	}

	if !strings.Contains(loopMsg, "branch was called 3 times") {
		t.Fatalf("expected a tool-loop message, got %q", loopMsg)
	}
	if executions != 2 {
		t.Fatalf("expected exactly 2 identical executions before abort (two-strike config), got %d", executions)
	}
	if aborted != 3 {
		t.Fatalf("expected 3 identical tool-use messages (2 executed, 1 aborted), got %d", aborted)
	}
	if len(reasons) == 0 || reasons[len(reasons)-1] != string(TerminalReasonBlockingLimit) {
		t.Fatalf("expected blocking-limit terminal, got %v", reasons)
	}
}

// TestLoop_DefaultAbortsOnTheVeryFirstIdenticalRepeat pins the default:
// one clean execution, and the next identical call ends the turn.
func TestLoop_DefaultAbortsOnFirstIdenticalRepeat(t *testing.T) {
	loop := NewLoop(loopingClient{})
	if loop.Config.MaxIdenticalToolUses != 1 {
		t.Fatalf("expected default MaxIdenticalToolUses=1, got %d", loop.Config.MaxIdenticalToolUses)
	}
	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "You are a test assistant.",
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options:         tools.Options{MainLoopModel: "test-model"},
			AbortController: context.Background(),
		},
	}
	stream, _ := loop.Query(context.Background(), params)
	executions, loopMsg := 0, ""
	for ev := range stream {
		if sm, ok := ev.(types.StreamMessage); ok {
			for _, blk := range sm.Message.Content {
				if tb, ok := blk.(types.TextBlock); ok && strings.Contains(tb.Text, "Tool loop detected") {
					loopMsg = tb.Text
				}
				if blk, ok := blk.(types.ToolResultBlock); ok && blk.IsError {
					executions++
				}
			}
		}
	}
	if executions != 1 {
		t.Fatalf("expected exactly 1 execution before the default abort, got %d", executions)
	}
	if !strings.Contains(loopMsg, "called 2 times") {
		t.Fatalf("expected the abort message at the second call, got %q", loopMsg)
	}
}
