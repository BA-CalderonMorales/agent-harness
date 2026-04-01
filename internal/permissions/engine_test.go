package permissions

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestEvaluateDenyRule(t *testing.T) {
	ctx := Context{
		Mode: ModeDefault,
		AlwaysDenyRules: map[RuleSource][]PermissionRule{
			SourceUserSettings: {
				{ToolName: "bash", Behavior: tools.Deny},
			},
		},
	}

	testTool := tools.NewTool(tools.Tool{
		Name: "bash",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{}
		},
	})

	decision := Evaluate(testTool, map[string]any{"command": "ls"}, ctx)
	if decision.Behavior != tools.Deny {
		t.Errorf("expected deny, got %s", decision.Behavior)
	}
}

func TestEvaluateAllowRule(t *testing.T) {
	ctx := Context{
		Mode: ModeDefault,
		AlwaysAllowRules: map[RuleSource][]PermissionRule{
			SourceUserSettings: {
				{ToolName: "read", Behavior: tools.Allow},
			},
		},
	}

	testTool := tools.NewTool(tools.Tool{
		Name: "read",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{}
		},
	})

	decision := Evaluate(testTool, map[string]any{"file_path": "x.go"}, ctx)
	if decision.Behavior != tools.Allow {
		t.Errorf("expected allow, got %s", decision.Behavior)
	}
}

func TestEvaluateModeBypass(t *testing.T) {
	ctx := Context{
		Mode:                        ModeBypassPermissions,
		IsBypassPermissionsAvailable: true,
	}

	testTool := tools.NewTool(tools.Tool{
		Name: "edit",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{}
		},
	})

	decision := Evaluate(testTool, map[string]any{"file_path": "x.go"}, ctx)
	if decision.Behavior != tools.Allow {
		t.Errorf("expected allow in bypass mode, got %s", decision.Behavior)
	}
}

func TestDenialTracking(t *testing.T) {
	dt := &DenialTrackingState{}
	dt.RecordDenial()
	dt.RecordDenial()
	dt.RecordDenial()
	if !dt.ShouldFallbackToPrompting {
		t.Error("expected fallback to prompting after 3 denials")
	}
	dt.RecordAllow()
	if dt.ShouldFallbackToPrompting {
		t.Error("expected fallback to reset after allow")
	}
}
