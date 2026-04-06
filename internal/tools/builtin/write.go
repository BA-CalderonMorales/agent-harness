package builtin

import (
	"fmt"
	"os"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/fs"
	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// FileWriteTool creates or overwrites a file with stale-write protection.
var FileWriteTool = tools.NewTool(tools.Tool{
	Name:        "write",
	Description: "Write content to a file. Creates the file if it does not exist. Protected against concurrent modifications.",
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
		InterruptBehavior: func() string { return "cancel" },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		path := getString(input, "file_path")
		if path == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path is required"}
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
		content := getString(input, "content")

		// Check if file exists - if so, apply stale-write protection
		_, err := os.Stat(path)
		if err == nil {
			if err := fs.DefaultStaleTracker.CheckStale(path); err != nil {
				return tools.ToolResult{}, fmt.Errorf("stale write detected: %w; the file may have been modified by another process - please re-read and try again", err)
			}
		}

		// Atomic write: temp file then rename
		tempPath := path + ".tmp"
		if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write temp file: %w", err)
		}

		if err := os.Rename(tempPath, path); err != nil {
			os.Remove(tempPath)
			return tools.ToolResult{}, fmt.Errorf("failed to rename file: %w", err)
		}

		// Update stale tracker and invalidate cache
		info, _ := os.Stat(path)
		if info != nil {
			fs.DefaultStaleTracker.RecordRead(path, []byte(content), info)
		}
		fs.DefaultCache.InvalidatePath(path)

		return tools.ToolResult{Data: fmt.Sprintf("Wrote %s (%d bytes)", path, len(content))}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "Write" },
	GetActivityDescription: func(input map[string]any) string {
		path := getString(input, "file_path")
		if path == "" {
			return "Writing file"
		}
		parts := strings.Split(path, "/")
		filename := parts[len(parts)-1]
		content := getString(input, "content")
		// Determine if creating or updating
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Sprintf("Creating %s (%d bytes)", filename, len(content))
		}
		return fmt.Sprintf("Updating %s (%d bytes)", filename, len(content))
	},
	GetToolUseSummary: func(input map[string]any) string {
		path := getString(input, "file_path")
		if path == "" {
			return ""
		}
		parts := strings.Split(path, "/")
		return parts[len(parts)-1]
	},
})
