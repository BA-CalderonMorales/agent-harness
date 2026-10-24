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

// recordingClient captures what a delegated child was asked to do.
type recordingClient struct {
	mu      sync.Mutex
	system  string
	tools   []tools.Tool
	model   string
	respond string
}

func (c *recordingClient) Stream(_ context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.system = req.SystemPrompt
	c.tools = req.Tools
	c.model = req.Model
	c.mu.Unlock()

	events := llm.MockTextResponse(c.respond)
	out := make(chan types.LLMEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out, nil
}

func writeAgentDefinition(t *testing.T, workspace, name, body string) {
	t.Helper()
	dir := filepath.Join(workspace, ".agent-harness", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write definition: %v", err)
	}
}

func TestDelegationUsesTheNamedSubAgent(t *testing.T) {
	workspace := t.TempDir()
	writeAgentDefinition(t, workspace, "reviewer", `---
name: reviewer
description: Reviews a finished change.
model: reviewer-model
tools: [read]
---
You are a strict reviewer. Report findings with evidence.
`)

	client := &recordingClient{respond: "review complete"}
	app := &App{
		config:  &config.LayeredConfig{MaxTokens: 256, PermissionMode: config.PermissionDangerFullAccess},
		session: state.NewSession("parent-model"),
		client:  client,
		cwd:     workspace,
	}

	parentCtx := tools.Context{
		AbortController: context.Background(),
		Options:         tools.Options{MainLoopModel: "parent-model", Tools: []tools.Tool{builtin.AgentTool, builtin.FileReadTool, builtin.FileWriteTool}},
	}
	canUse := func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
		return tools.PermissionDecision{Behavior: tools.Allow}, nil
	}

	if _, err := app.runDelegatedAgent(context.Background(), "review the change", "reviewer", parentCtx, canUse); err != nil {
		t.Fatalf("delegation failed: %v", err)
	}

	if !strings.Contains(client.system, "You are a strict reviewer.") {
		t.Fatalf("child system prompt does not carry the definition:\n%s", client.system)
	}
	if client.model != "reviewer-model" {
		t.Fatalf("child model = %q, want the definition's model", client.model)
	}
	names := map[string]bool{}
	for _, tool := range client.tools {
		names[tool.Name] = true
	}
	if !names["read"] {
		t.Fatalf("child tools = %v, want read", names)
	}
	if names["write"] {
		t.Fatalf("child tools = %v, want the definition's allowlist to exclude write", names)
	}
}

func TestDelegationRejectsUnknownSubAgent(t *testing.T) {
	workspace := t.TempDir()
	writeAgentDefinition(t, workspace, "reviewer", "---\nname: reviewer\n---\nreview things\n")

	app := &App{
		config:  &config.LayeredConfig{MaxTokens: 256},
		session: state.NewSession("parent-model"),
		client:  &recordingClient{respond: "unused"},
		cwd:     workspace,
	}
	parentCtx := tools.Context{AbortController: context.Background()}
	canUse := func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
		return tools.PermissionDecision{Behavior: tools.Allow}, nil
	}

	_, err := app.runDelegatedAgent(context.Background(), "do a thing", "tester", parentCtx, canUse)
	if err == nil {
		t.Fatal("delegation with an unknown type succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("error = %v, want the available names", err)
	}
}

func TestFormatAgentDefinitionsListsInstalledAgents(t *testing.T) {
	workspace := t.TempDir()
	writeAgentDefinition(t, workspace, "reviewer", "---\nname: reviewer\ndescription: Reviews changes.\ntools: [read, grep]\n---\nreview\n")

	app := &App{cwd: workspace}
	out := app.formatAgentDefinitions()
	if !strings.Contains(out, "reviewer") || !strings.Contains(out, "Reviews changes.") {
		t.Fatalf("listing = %q, want the definition and its description", out)
	}
	if !strings.Contains(out, "read, grep") {
		t.Fatalf("listing = %q, want the tool allowlist", out)
	}
}
