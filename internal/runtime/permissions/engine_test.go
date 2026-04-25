package permissions

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
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
		Mode:                         ModeBypassPermissions,
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

func TestEvaluate_ModeHappyPaths(t *testing.T) {
	makeTool := func(name string, destructive bool) tools.Tool {
		return tools.NewTool(tools.Tool{
			Name: name,
			Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
				return tools.ToolResult{}, nil
			},
			MapResult: func(result any, toolUseID string) types.ToolResultBlock {
				return types.ToolResultBlock{}
			},
			Capabilities: tools.CapabilityFlags{
				IsDestructive: func(map[string]any) bool { return destructive },
			},
			CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
				return tools.PermissionDecision{Behavior: tools.Allow}
			},
		})
	}

	tests := []struct {
		name string
		ctx  Context
		tool tools.Tool
		want tools.DecisionBehavior
	}{
		{
			name: "dontAsk allows read-only tool",
			ctx:  Context{Mode: ModeDontAsk},
			tool: makeTool("read", false),
			want: tools.Allow,
		},
		{
			name: "dontAsk asks for destructive tool",
			ctx:  Context{Mode: ModeDontAsk},
			tool: makeTool("bash", true),
			want: tools.Ask,
		},
		{
			name: "bypass without availability falls back to ask",
			ctx:  Context{Mode: ModeBypassPermissions, IsBypassPermissionsAvailable: false},
			tool: makeTool("edit", false),
			want: tools.Ask,
		},
		{
			name: "auto mode asks",
			ctx:  Context{Mode: ModeAuto},
			tool: makeTool("read", false),
			want: tools.Ask,
		},
		{
			name: "default mode with tool checkPermission allows",
			ctx:  EmptyContext(),
			tool: makeTool("read", false),
			want: tools.Allow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.tool, map[string]any{}, tt.ctx)
			if got.Behavior != tt.want {
				t.Errorf("got %s, want %s", got.Behavior, tt.want)
			}
		})
	}
}

func TestEvaluate_AliasMatching(t *testing.T) {
	tool := tools.NewTool(tools.Tool{
		Name:    "bash",
		Aliases: []string{"shell", "sh"},
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock { return types.ToolResultBlock{} },
	})

	ctx := Context{
		Mode: ModeDefault,
		AlwaysAllowRules: map[RuleSource][]PermissionRule{
			SourceUserSettings: {{ToolName: "shell", Behavior: tools.Allow}},
		},
	}

	got := Evaluate(tool, map[string]any{"command": "ls"}, ctx)
	if got.Behavior != tools.Allow {
		t.Errorf("expected allow via alias, got %s", got.Behavior)
	}
}

func TestEvaluate_RulePrecedence(t *testing.T) {
	tool := tools.NewTool(tools.Tool{
		Name: "bash",
		Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock { return types.ToolResultBlock{} },
	})

	// Deny should win over allow when both match
	ctx := Context{
		Mode: ModeDefault,
		AlwaysDenyRules: map[RuleSource][]PermissionRule{
			SourceUserSettings: {{ToolName: "bash", Behavior: tools.Deny}},
		},
		AlwaysAllowRules: map[RuleSource][]PermissionRule{
			SourceProjectSettings: {{ToolName: "bash", Behavior: tools.Allow}},
		},
	}

	got := Evaluate(tool, map[string]any{"command": "ls"}, ctx)
	if got.Behavior != tools.Deny {
		t.Errorf("expected deny to win over allow, got %s", got.Behavior)
	}
}
