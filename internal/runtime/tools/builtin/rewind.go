package builtin

import (
	"fmt"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// RewindTool restores a file to its state before the last edit.
// This demonstrates the file history / undo pattern.
var RewindTool = tools.NewTool(tools.Tool{
	Name:        "rewind",
	Description: "Undo the last edit to a file by restoring its previous content.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{"type": "string"},
			},
			"required": []string{"file_path"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false },
		IsReadOnly:        func(map[string]any) bool { return false },
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

		// In a full implementation, this looks up the file in AppState.FileHistory
		// For the pattern demonstration, we check a simple backup file
		backupPath := path + ".agent-harness-backup"
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("no backup found for %s", path)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to restore file: %w", err)
		}

		return tools.ToolResult{Data: fmt.Sprintf("Restored %s from backup", path)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "rewind" },
})
