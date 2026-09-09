package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type delegatedSequenceClient struct {
	mu    sync.Mutex
	turn  int
	tools [][]tools.Tool
	path  string
}

func (c *delegatedSequenceClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.turn++
	turn := c.turn
	c.tools = append(c.tools, req.Tools)
	c.mu.Unlock()

	var events []types.LLMEvent
	switch turn {
	case 1:
		events = llm.MockToolUseResponse("write", `{"file_path":"`+c.path+`","content":"delegated"}`)
	case 2:
		events = llm.MockToolUseResponse("read", `{"file_path":"`+c.path+`"}`)
	default:
		events = llm.MockTextResponse("delegated child complete")
	}
	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func TestDelegatedAgentRunsMultipleToolsWithParentPermission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.txt")
	client := &delegatedSequenceClient{path: path}
	app := &App{
		config:  &config.LayeredConfig{MaxTokens: 256, PermissionMode: config.PermissionDangerFullAccess},
		session: state.NewSession("test-model"),
		client:  client,
		cwd:     t.TempDir(),
	}
	var permissionCalls []string
	canUse := func(name string, _ map[string]any, _ tools.Context) (tools.PermissionDecision, error) {
		permissionCalls = append(permissionCalls, name)
		return tools.PermissionDecision{Behavior: tools.Allow}, nil
	}
	parentCtx := tools.Context{
		AbortController: context.Background(),
		Options:         tools.Options{MainLoopModel: "test-model", Tools: []tools.Tool{builtin.AgentTool, builtin.FileWriteTool, builtin.FileReadTool}},
	}
	result, err := app.runDelegatedAgent(context.Background(), "write then read the result", parentCtx, canUse)
	if err != nil {
		t.Fatalf("delegation failed: %v", err)
	}
	if !strings.Contains(result, "delegated child complete") {
		t.Fatalf("child result = %q, want complete child output", result)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "delegated" {
		t.Fatalf("delegated write = %q, err=%v; want disposable file content", data, err)
	}
	if len(permissionCalls) != 2 || permissionCalls[0] != "write" || permissionCalls[1] != "read" {
		t.Fatalf("permission calls = %v, want write then read", permissionCalls)
	}
	for _, defs := range client.tools {
		for _, def := range defs {
			if def.Name == builtin.AgentTool.Name {
				t.Fatal("delegated child unexpectedly received recursive agent tool")
			}
		}
	}
}

func TestDelegatedAgentReportsNonCompleteTerminal(t *testing.T) {
	app := &App{config: &config.LayeredConfig{MaxTokens: 256}, session: state.NewSession("test-model"), client: &terminalClient{}, cwd: t.TempDir()}
	_, err := app.runDelegatedAgent(context.Background(), "stop", tools.Context{}, func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
		return tools.PermissionDecision{Behavior: tools.Allow}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "delegated worker failed") {
		t.Fatalf("error = %v, want delegated failure", err)
	}
}

type terminalClient struct{}

func (*terminalClient) Stream(ctx context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	return nil, context.Canceled
}
