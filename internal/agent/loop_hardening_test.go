package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestBuildLLMRequestSortsSystemContextAndPreservesReasoningEffort(t *testing.T) {
	params := QueryParams{
		SystemPrompt: "base",
		SystemContext: map[string]string{
			"zeta":  "last",
			"alpha": "first",
			"mid":   "middle",
		},
		ReasoningEffort: "high",
		ToolUseContext: tools.Context{
			Options: tools.Options{MainLoopModel: "test-model"},
		},
	}
	state := &loopState{}

	first := buildLLMRequest(params, state)
	if first.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", first.ReasoningEffort)
	}
	alphaIdx := strings.Index(first.SystemPrompt, "<alpha>")
	midIdx := strings.Index(first.SystemPrompt, "<mid>")
	zetaIdx := strings.Index(first.SystemPrompt, "<zeta>")
	if !(alphaIdx > 0 && alphaIdx < midIdx && midIdx < zetaIdx) {
		t.Fatalf("SystemContext blocks not sorted deterministically: %q", first.SystemPrompt)
	}
	for i := 0; i < 20; i++ {
		if got := buildLLMRequest(params, state).SystemPrompt; got != first.SystemPrompt {
			t.Fatalf("iteration %d produced non-deterministic SystemPrompt", i)
		}
	}
}

func TestLoopAccumulatesUsageAcrossMultiToolTurn(t *testing.T) {
	client := &recordingLoopClient{
		streams: [][]types.LLMEvent{
			{
				types.LLMMessageStart{ID: "m1"},
				types.LLMToolUseDelta{ID: "t1", Name: "echo", Delta: `{"msg":"hi"}`},
				types.LLMMessageStop{
					StopReason: "tool_use",
					Model:      "test-model",
					Usage:      types.TokenUsage{InputTokens: 100, OutputTokens: 25, CacheReadInputTokens: 10},
				},
			},
			{
				types.LLMMessageStart{ID: "m2"},
				types.LLMTextDelta{Delta: "done"},
				types.LLMMessageStop{
					StopReason: "stop",
					Model:      "test-model",
					Usage:      types.TokenUsage{InputTokens: 150, OutputTokens: 40, CacheReadInputTokens: 20},
				},
			},
		},
	}
	echoTool := tools.NewTool(tools.Tool{
		Name: "echo",
		Call: func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "ok"}, nil
		},
		MapResult: func(result any, id string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: id, Content: fmt.Sprint(result)}
		},
	})
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	stream, err := loop.Query(context.Background(), QueryParams{
		SystemPrompt:    "sys",
		ReasoningEffort: "medium",
		ToolUseContext: tools.Context{
			AbortController: context.Background(),
			Options:         tools.Options{MainLoopModel: "test-model", Tools: []tools.Tool{echoTool}},
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	for range stream {
	}

	got := loop.LastTokenUsage()
	if got.InputTokens != 250 || got.OutputTokens != 65 || got.CacheReadInputTokens != 30 {
		t.Fatalf("LastTokenUsage() = %+v, want {Input:250 Output:65 CacheRead:30}", got)
	}
}

func TestRecoveryRetryPreservesReasoningEffort(t *testing.T) {
	client := &recordingLoopClient{
		streams: [][]types.LLMEvent{
			{
				types.LLMError{Error: fmt.Errorf("max_output_tokens exceeded")},
			},
			{
				types.LLMMessageStart{ID: "m2"},
				types.LLMTextDelta{Delta: "recovered"},
				types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
			},
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	stream, err := loop.Query(context.Background(), QueryParams{
		SystemPrompt:    "sys",
		ReasoningEffort: "high",
		ToolUseContext: tools.Context{
			AbortController: context.Background(),
			Options:         tools.Options{MainLoopModel: "test-model"},
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	for range stream {
	}
	if len(client.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(client.requests))
	}
	if client.requests[1].ReasoningEffort != "high" {
		t.Fatalf("retry ReasoningEffort = %q, want high", client.requests[1].ReasoningEffort)
	}
}

func TestRequestSummaryHonorsIdleWindow(t *testing.T) {
	client := &hangingSummaryClient{}
	loop := NewLoop(client)
	loop.Config.StreamIdleTimeout = 25 * time.Millisecond

	start := time.Now()
	_, err := loop.requestSummary(context.Background(), "summarize", "test-model")
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "went idle") {
		t.Fatalf("requestSummary() err = %v, want idle timeout error", err)
	}
	if elapsed > time.Second {
		t.Fatalf("requestSummary() took %v, expected bounded idle timeout", elapsed)
	}
}

func TestLoopIsolatesContentBudgetFromGlobalState(t *testing.T) {
	tools.ResetBudgetForNewTurn()
	defer tools.ResetBudgetForNewTurn()
	if err := tools.GetCurrentBudget().RecordUsage("custom", 1234, 10000); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	var observedUsed int
	checkTool := tools.NewTool(tools.Tool{
		Name: "check_budget",
		Call: func(_ map[string]any, ctx tools.Context, _ tools.CanUseToolFn, _ tools.OnProgress) (tools.ToolResult, error) {
			if ctx.ContentBudget == nil {
				return tools.ToolResult{}, fmt.Errorf("expected per-turn isolated ContentBudget")
			}
			observedUsed, _ = ctx.ContentBudget.CurrentUsage()
			return tools.ToolResult{Data: "ok"}, nil
		},
		MapResult: func(result any, id string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: id, Content: fmt.Sprint(result)}
		},
	})
	client := &recordingLoopClient{
		streams: [][]types.LLMEvent{
			{
				types.LLMMessageStart{ID: "m1"},
				types.LLMToolUseDelta{ID: "t1", Name: "check_budget", Delta: `{}`},
				types.LLMMessageStop{StopReason: "tool_use", Model: "test-model"},
			},
			{
				types.LLMMessageStart{ID: "m2"},
				types.LLMTextDelta{Delta: "done"},
				types.LLMMessageStop{StopReason: "stop", Model: "test-model"},
			},
		},
	}
	loop := NewLoop(client)
	loop.Config.AutoCompactEnabled = false
	stream, err := loop.Query(context.Background(), QueryParams{
		SystemPrompt: "sys",
		ToolUseContext: tools.Context{
			AbortController: context.Background(),
			Options:         tools.Options{MainLoopModel: "test-model", Tools: []tools.Tool{checkTool}},
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	for range stream {
	}

	if observedUsed != 0 {
		t.Fatalf("isolated turn budget started at %d, want 0", observedUsed)
	}
	if globalUsed, _ := tools.GetCurrentBudget().CurrentUsage(); globalUsed != 1234 {
		t.Fatalf("loop clobbered global budget: got %d, want 1234", globalUsed)
	}
}

type recordingLoopClient struct {
	streams  [][]types.LLMEvent
	requests []llm.Request
}

func (c *recordingLoopClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	idx := len(c.requests)
	c.requests = append(c.requests, req)
	var events []types.LLMEvent
	if idx < len(c.streams) {
		events = c.streams[idx]
	}
	out := make(chan types.LLMEvent, len(events))
	for _, ev := range events {
		out <- ev
	}
	close(out)
	return out, nil
}

type hangingSummaryClient struct{}

func (c *hangingSummaryClient) Stream(ctx context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	out := make(chan types.LLMEvent)
	go func() {
		<-ctx.Done()
		close(out)
	}()
	return out, nil
}
