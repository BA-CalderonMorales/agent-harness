package builtin

import (
	"fmt"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// FileEditTool performs string-replace edits on files with stale-write protection.
var FileEditTool = tools.NewTool(tools.Tool{
	Name:        "edit",
	Description: "Edit a file by replacing one string with another. The old_string must match exactly. Protected against concurrent modifications.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path":  map[string]any{"type": "string"},
				"old_string": map[string]any{"type": "string"},
				"new_string": map[string]any{"type": "string"},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return false }, // File edits are serial
		IsReadOnly:        func(map[string]any) bool { return false },
		IsDestructive:     func(map[string]any) bool { return false },
		InterruptBehavior: func() string { return "cancel" }, // Edits can be cancelled
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		path := getString(input, "file_path")
		oldStr := getString(input, "old_string")
		if path == "" || oldStr == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path and old_string are required"}
		}
		// UNC path security
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
			return tools.ValidationResult{Valid: false, Message: "UNC paths are not supported for security reasons"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		oldStr := getString(input, "old_string")
		newStr := getString(input, "new_string")

		data, err := os.ReadFile(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to read file: %w", err)
		}

		content := string(data)
		if !strings.Contains(content, oldStr) {
			return tools.ToolResult{}, fmt.Errorf("old_string not found in file")
		}

		// Stale write protection: check if file was modified since last read
		if err := fs.DefaultStaleTracker.CheckStale(path); err != nil {
			return tools.ToolResult{}, fmt.Errorf("stale write detected: %w; the file may have been modified by another process - please re-read and try again", err)
		}

		updated := strings.Replace(content, oldStr, newStr, 1)

		// Write atomically: write to temp file then rename
		tempPath := path + ".tmp"
		if err := os.WriteFile(tempPath, []byte(updated), 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write temp file: %w", err)
		}

		if err := os.Rename(tempPath, path); err != nil {
			// Try to clean up temp file
			os.Remove(tempPath)
			return tools.ToolResult{}, fmt.Errorf("failed to rename file: %w", err)
		}

		// Update stale tracker with new content
		info, _ := os.Stat(path)
		if info != nil {
			fs.DefaultStaleTracker.RecordRead(path, []byte(updated), info)
		}

		// Invalidate read cache for this file
		fs.DefaultCache.InvalidatePath(path)

		return tools.ToolResult{Data: fmt.Sprintf("Edited %s successfully", path)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "edit" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["file_path"].(string); ok {
			return "Editing " + p
		}
		return "Editing file"
	},
})
