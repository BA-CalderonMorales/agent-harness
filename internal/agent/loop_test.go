package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestLoop_TextOnlyResponse(t *testing.T) {
	mock := &llm.MockClient{Events: llm.MockTextResponse("Hello, world!")}
	loop := NewLoop(mock)

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "You are a test assistant.",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options: tools.Options{
				MainLoopModel: "test-model",
				Tools:         nil,
			},
			AbortController: context.Background(),
		},
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var gotText bool
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if tb, ok := block.(types.TextBlock); ok {
					if tb.Text == "Hello, world!" {
						gotText = true
					}
				}
			}
		}
	}

	if !gotText {
		t.Error("expected to receive 'Hello, world!' text")
	}
}

func TestLoop_ToolUseResponse(t *testing.T) {
	mock := &llm.MockClient{Events: llm.MockToolUseResponse("bash", "ls")}
	loop := NewLoop(mock)

	bashTool := tools.NewTool(tools.Tool{
		Name:        "bash",
		Description: "Run bash",
		InputSchema: func() map[string]any { return map[string]any{"type": "object"} },
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "file.txt"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "You are a test assistant.",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options: tools.Options{
				MainLoopModel: "test-model",
				Tools:         []tools.Tool{bashTool},
			},
			AbortController: context.Background(),
		},
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var gotToolResult bool
	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if tr, ok := block.(types.ToolResultBlock); ok {
					if tr.Content == "file.txt" {
						gotToolResult = true
					}
				}
			}
		}
	}

	if !gotToolResult {
		t.Error("expected to receive tool result 'file.txt'")
	}
}

func TestLoop_MaxTurnsRespected(t *testing.T) {
	// Every response asks for the same tool, which would loop forever
	mock := &llm.MockClient{Events: llm.MockToolUseResponse("bash", "ls")}
	loop := NewLoop(mock)
	loop.Config.DefaultMaxTurns = 2

	bashTool := tools.NewTool(tools.Tool{
		Name: "bash",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: "done"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options:         tools.Options{Tools: []tools.Tool{bashTool}},
			AbortController: context.Background(),
		},
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	eventCount := 0
	for event := range stream {
		_ = event
		eventCount++
	}

	if eventCount == 0 {
		t.Error("expected some events")
	}
}

func TestLoop_ContextCancellation(t *testing.T) {
	// Slow mock that never yields
	mock := &llm.MockClient{Events: []types.LLMEvent{}}
	loop := NewLoop(mock)

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options:         tools.Options{},
			AbortController: context.Background(),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream, err := loop.Query(ctx, params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	done := make(chan bool)
	go func() {
		for range stream {
		}
		done <- true
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("stream did not close after context cancellation")
	}
}

func TestLoop_StreamReturnsError(t *testing.T) {
	mock := &llm.MockClient{Err: fmt.Errorf("stream failed: 401 Unauthorized")}
	loop := NewLoop(mock)

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
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

	var gotStreamError bool
	var streamErrMsg string
	for event := range stream {
		if se, ok := event.(types.StreamError); ok {
			gotStreamError = true
			streamErrMsg = se.Error.Error()
		}
	}

	if !gotStreamError {
		t.Fatal("expected a StreamError event when Stream returns an error")
	}
	if streamErrMsg != "stream failed: 401 Unauthorized" {
		t.Errorf("expected error message 'stream failed: 401 Unauthorized', got %q", streamErrMsg)
	}
}

func TestLoop_EmptyStreamYieldsError(t *testing.T) {
	mock := &llm.MockClient{Events: []types.LLMEvent{}}
	loop := NewLoop(mock)

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
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

	var gotStreamError bool
	for event := range stream {
		if _, ok := event.(types.StreamError); ok {
			gotStreamError = true
		}
	}

	if !gotStreamError {
		t.Fatal("expected a StreamError event when stream closes with no content")
	}
}

func TestLoop_ValidStreamYieldsStreamMessage(t *testing.T) {
	mock := &llm.MockClient{Events: llm.MockTextResponse("Hello, world!")}
	loop := NewLoop(mock)

	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "Test",
		CanUseTool: func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
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

	var gotRequestStart bool
	var gotStreamMessage bool
	for event := range stream {
		switch e := event.(type) {
		case types.StreamRequestStart:
			gotRequestStart = true
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				if tb, ok := block.(types.TextBlock); ok && tb.Text == "Hello, world!" {
					gotStreamMessage = true
				}
			}
		case types.StreamError:
			t.Fatalf("unexpected StreamError: %v", e.Error)
		}
	}

	if !gotRequestStart {
		t.Error("expected StreamRequestStart event")
	}
	if !gotStreamMessage {
		t.Error("expected StreamMessage with 'Hello, world!'")
	}
}
