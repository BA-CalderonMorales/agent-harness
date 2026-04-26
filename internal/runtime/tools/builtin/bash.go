package builtin

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// BashTool executes shell commands.
var BashTool = tools.NewTool(tools.Tool{
	Name:        "bash",
	Description: "Execute a bash command in the current working directory.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "The bash command to execute"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in milliseconds"},
			},
			"required": []string{"command"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(input map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return false },
		IsDestructive:     detectDestructive,
		InterruptBehavior: func() string { return "cancel" },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		cmd, ok := input["command"].(string)
		if !ok || cmd == "" {
			return tools.ValidationResult{Valid: false, Message: "command is required"}
		}
		if isDangerousCommand(cmd) {
			return tools.ValidationResult{Valid: false, Message: "Potentially dangerous command detected"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		cmd, _ := input["command"].(string)
		if detectDestructive(input) {
			return tools.PermissionDecision{
				Behavior:     tools.Ask,
				Message:      "Shell command '" + cmd + "' is potentially destructive. Do you want to continue?",
				UpdatedInput: input,
			}
		}
		// Default to allow if not explicitly dangerous, but the engine will still wrap this.
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		cmdStr := input["command"].(string)
		timeoutMs := 60000
		if t, ok := getNumber(input, "timeout"); ok {
			timeoutMs = int(t)
		}

		// Import exec here to avoid top-level heavy deps pattern
		return runBashCommand(ctx.AbortController, cmdStr, timeoutMs, onProgress)
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		if content == "" {
			content = " "
		}
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "Shell" },
	GetActivityDescription: func(input map[string]any) string {
		if c, ok := input["command"].(string); ok {
			return "Running " + c
		}
		return "Running bash command"
	},
})

func detectDestructive(input map[string]any) bool {
	cmd, _ := input["command"].(string)
	dangerous := []string{"rm -rf", "dd if=", "mkfs", "> /dev/"}
	for _, d := range dangerous {
		if strings.Contains(cmd, d) {
			return true
		}
	}
	return false
}

func isDangerousCommand(cmd string) bool {
	forbidden := []string{
		"curl | sh",
		"wget | sh",
		"rm -rf /",
	}
	for _, f := range forbidden {
		if strings.Contains(cmd, f) {
			return true
		}
	}
	return false
}
