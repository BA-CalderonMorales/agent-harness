package builtin

import (
	"fmt"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// FileWriteTool creates or overwrites a file.
var FileWriteTool = tools.NewTool(tools.Tool{
	Name:        "write",
	Description: "Write content to a file. Creates the file if it does not exist.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{"type": "string"},
				"content":   map[string]any{"type": "string"},
			},
			"required": []string{"file_path", "content"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false },
		IsReadOnly:        func(map[string]any) bool { return false },
		IsDestructive:     func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "file_path") == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		content := getString(input, "content")

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write file: %w", err)
		}
		return tools.ToolResult{Data: fmt.Sprintf("Wrote %s (%d bytes)", path, len(content))}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "write" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["file_path"].(string); ok {
			return "Writing " + p
		}
		return "Writing file"
	},
})
