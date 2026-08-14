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

// silentClient opens a stream that never emits an event and never closes.
type silentClient struct{}

func (silentClient) Stream(_ context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	return make(chan types.LLMEvent), nil
}

// TestLoop_StreamIdleTimeout verifies a provider that goes silent mid-turn
// fails the turn cleanly instead of hanging forever.
func TestLoop_StreamIdleTimeout(t *testing.T) {
	original := maxStreamIdle
	maxStreamIdle = 60 * time.Millisecond
	defer func() { maxStreamIdle = original }()

	loop := NewLoop(silentClient{})
	params := QueryParams{
		Messages:     []types.Message{},
		SystemPrompt: "You are a test assistant.",
		CanUseTool: func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		ToolUseContext: tools.Context{
			Options: tools.Options{MainLoopModel: "test-model"},
		},
	}

	stream, err := loop.Query(context.Background(), params)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var idleErr string
	for event := range stream {
		if se, ok := event.(types.StreamError); ok {
			idleErr = se.Error.Error()
		}
	}
	if !strings.Contains(idleErr, "went idle") {
		t.Fatalf("expected an idle-stream error, got %q", idleErr)
	}
}
