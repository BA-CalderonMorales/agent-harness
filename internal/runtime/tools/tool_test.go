package tools

import (
	"errors"
	"fmt"
	"strings"
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

type fakeBucket struct {
	name    string
	handles string
	calls   int
}

func (b *fakeBucket) Name() string { return b.name }

func (b *fakeBucket) CanHandle(toolName string, input map[string]any) bool {
	return toolName == b.handles && input["deny"] != true
}

func (b *fakeBucket) Execute(ctx ToolExecutionContext) ToolResult {
	b.calls++
	return ToolResult{Data: fmt.Sprintf("%s:%v", ctx.ToolName, ctx.Input["value"])}
}

func (b *fakeBucket) Capabilities() ToolBucketCapabilities {
	return ToolBucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        true,
		ToolNames:         []string{b.handles},
		Category:          "test",
	}
}

func (b *fakeBucket) GetTools() []Tool {
	return []Tool{NewTool(Tool{Name: b.handles, Description: "fake tool"})}
}

func TestToolOrchestratorRoutesToRegisteredBucket(t *testing.T) {
	bucket := &fakeBucket{name: "fake", handles: "fake_tool"}
	orchestrator := NewToolOrchestrator(bucket)

	found, ok := orchestrator.FindBucket("fake_tool", map[string]any{"value": "ok"})
	if !ok {
		t.Fatal("expected registered bucket to be found")
	}
	if found.Name() != "fake" {
		t.Fatalf("bucket = %q, want fake", found.Name())
	}

	result := orchestrator.ExecuteTool(ToolExecutionContext{
		ToolName:  "fake_tool",
		Input:     map[string]any{"value": "ok"},
		ToolUseID: "toolu_123",
	})
	if result.Data != "fake_tool:ok" {
		t.Fatalf("result = %#v, want routed fake bucket output", result.Data)
	}
	if bucket.calls != 1 {
		t.Fatalf("calls = %d, want 1", bucket.calls)
	}
}

func TestToolOrchestratorRegisterBucketUpdatesRegistry(t *testing.T) {
	orchestrator := NewToolOrchestrator()
	orchestrator.RegisterBucket(&fakeBucket{name: "later", handles: "later_tool"})

	if _, ok := orchestrator.FindBucket("later_tool", map[string]any{}); !ok {
		t.Fatal("expected registered bucket lookup to succeed")
	}
	if _, ok := orchestrator.GetRegistry().FindToolByName("later_tool"); !ok {
		t.Fatal("expected registry to include tools from registered bucket")
	}
}

func TestToolOrchestratorUnknownToolReturnsErrorData(t *testing.T) {
	orchestrator := NewToolOrchestrator(&fakeBucket{name: "fake", handles: "fake_tool"})

	result := orchestrator.ExecuteTool(ToolExecutionContext{
		ToolName: "missing_tool",
		Input:    map[string]any{},
	})

	err, ok := result.Data.(error)
	if !ok {
		t.Fatalf("result data = %#v, want error", result.Data)
	}
	if !strings.Contains(err.Error(), "no bucket found for tool: missing_tool") {
		t.Fatalf("error = %q, want missing tool context", err)
	}
}

func TestWrapToolErrorPreservesCodeMessageAndCause(t *testing.T) {
	cause := errors.New("permission denied")

	err := WrapToolError("permission_denied", cause)

	if err.Code != "permission_denied" {
		t.Fatalf("code = %q", err.Code)
	}
	if err.Message != cause.Error() {
		t.Fatalf("message = %q, want %q", err.Message, cause.Error())
	}
	if !errors.Is(err.Cause, cause) {
		t.Fatalf("cause = %#v, want wrapped cause", err.Cause)
	}
	if !strings.Contains(err.Error(), "permission_denied") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error string = %q, want code and cause", err.Error())
	}
}
