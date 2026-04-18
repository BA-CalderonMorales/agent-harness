package builtin

import (
	"fmt"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// AgentTool spawns a sub-agent with its own message context.
var AgentTool = tools.NewTool(tools.Tool{
	Name:        "agent",
	Description: "Spawn a sub-agent to work on a specific task with clean context.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "The task to delegate to the sub-agent",
				},
				"agent_type": map[string]any{
					"type":        "string",
					"description": "Optional agent specialization (e.g., 'reviewer', 'tester')",
				},
			},
			"required": []string{"prompt"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return false },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "prompt") == "" {
			return tools.ValidationResult{Valid: false, Message: "prompt is required"}
		}
		// Prevent infinite recursion
		if ctx.QueryTracking.Depth >= 5 {
			return tools.ValidationResult{Valid: false, Message: "agent recursion limit reached"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		prompt := getString(input, "prompt")
		agentType := getString(input, "agent_type")
		if agentType == "" {
			agentType = "default"
		}

		// If SubAgentQuery is wired, use it for real execution
		if ctx.SubAgentQuery != nil {
			result, err := ctx.SubAgentQuery(prompt)
			if err != nil {
				return tools.ToolResult{}, fmt.Errorf("sub-agent failed: %w", err)
			}
			return tools.ToolResult{Data: fmt.Sprintf("[Sub-agent %s]\n%s", agentType, result)}, nil
		}

		// Fallback: demonstrate the pattern without real execution
		_ = agent.NewLoop(nil)
		result := fmt.Sprintf("[Sub-agent %s completed]\nTask: %s\nResult: (sub-agent execution would run here)", agentType, prompt)
		return tools.ToolResult{Data: result}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "agent" },
	GetActivityDescription: func(input map[string]any) string {
		if t, ok := input["agent_type"].(string); ok && t != "" {
			return "Spawning " + t + " agent"
		}
		return "Spawning sub-agent"
	},
})
