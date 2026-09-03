package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// TestLoop_ConfiguredStreamIdleTimeout verifies a per-loop configured idle
// watchdog wins over the package default, so local providers can opt into
// multi-minute first-token windows without disturbing the default guard.
func TestLoop_ConfiguredStreamIdleTimeout(t *testing.T) {
	loop := NewLoop(silentClient{})
	loop.Config.StreamIdleTimeout = 60 * time.Millisecond

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
	if !strings.Contains(idleErr, "60ms") {
		t.Fatalf("idle error should report the configured window, got %q", idleErr)
	}
}
