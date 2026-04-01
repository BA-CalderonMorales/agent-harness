package tools

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestNewToolAppliesDefaults(t *testing.T) {
	tool := NewTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Call: func(input map[string]any, ctx Context, canUse CanUseToolFn, onProgress OnProgress) (ToolResult, error) {
			return ToolResult{Data: "ok"}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})

	if tool.Name != "test_tool" {
		t.Errorf("expected name test_tool, got %s", tool.Name)
	}

	// Defaults should be applied
	if tool.Capabilities.IsEnabled == nil || !tool.Capabilities.IsEnabled() {
		t.Error("expected IsEnabled default to be true")
	}
	if tool.Capabilities.IsConcurrencySafe == nil || tool.Capabilities.IsConcurrencySafe(nil) {
		t.Error("expected IsConcurrencySafe default to be false")
	}
	if tool.CheckPermissions == nil {
		t.Fatal("expected CheckPermissions default to be set")
	}
	decision := tool.CheckPermissions(nil, Context{})
	if decision.Behavior != Allow {
		t.Errorf("expected default allow, got %s", decision.Behavior)
	}
}

func TestToolRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterBuiltIn(NewTool(Tool{
		Name: "tool_a",
		Call: func(input map[string]any, ctx Context, canUse CanUseToolFn, onProgress OnProgress) (ToolResult, error) {
			return ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{}
		},
	}))

	found, ok := reg.FindToolByName("tool_a")
	if !ok {
		t.Fatal("expected to find tool_a")
	}
	if found.Name != "tool_a" {
		t.Errorf("expected tool_a, got %s", found.Name)
	}

	_, ok = reg.FindToolByName("tool_b")
	if ok {
		t.Error("expected not to find tool_b")
	}
}

func TestToolRegistryAlias(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterBuiltIn(NewTool(Tool{
		Name:    "tool_a",
		Aliases: []string{"legacy_tool_a"},
		Call: func(input map[string]any, ctx Context, canUse CanUseToolFn, onProgress OnProgress) (ToolResult, error) {
			return ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{}
		},
	}))

	found, ok := reg.FindToolByName("legacy_tool_a")
	if !ok {
		t.Fatal("expected to find tool by alias")
	}
	if found.Name != "tool_a" {
		t.Errorf("expected tool_a, got %s", found.Name)
	}
}
