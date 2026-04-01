package builtin

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// PlanState tracks whether the agent is in plan mode.
var PlanState = struct {
	Active   bool
	Steps    []string
	Original string
}{}

// EnterPlanModeTool switches the agent into plan mode.
var EnterPlanModeTool = tools.NewTool(tools.Tool{
	Name:        "enter_plan_mode",
	Description: "Enter plan mode. In this mode, the agent must outline its approach before taking action.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false },
		IsReadOnly:        func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		PlanState.Active = true
		return tools.ToolResult{Data: "Entered plan mode. Please outline your approach before executing tools."}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "plan mode" },
})

// ExitPlanModeTool switches the agent out of plan mode.
var ExitPlanModeTool = tools.NewTool(tools.Tool{
	Name:        "exit_plan_mode",
	Description: "Exit plan mode and resume normal execution.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false },
		IsReadOnly:        func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		PlanState.Active = false
		return tools.ToolResult{Data: "Exited plan mode. Resuming normal execution."}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "exit plan" },
})
