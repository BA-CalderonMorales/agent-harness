package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func TestAgentToolWithoutExecutorFailsTruthfully(t *testing.T) {
	_, err := AgentTool.Call(map[string]any{"prompt": "do work"}, tools.Context{AbortController: context.Background()}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "delegated execution is not configured") {
		t.Fatalf("error = %v, want truthful unavailable-executor error", err)
	}
}
