package buckets

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"os"
)

// handleEdit performs a file edit.
func (fs *FileSystemBucket) handleEdit(ctx tools.ToolExecutionContext) tools.ToolResult {
	path := fs.getString(ctx.Input, "file_path")
	if path == "" {
		return tools.ToolResult{
			Data: tools.NewToolError("invalid_input", "file_path is required"),
		}
	}

	oldStr := fs.getString(ctx.Input, "old_string")
	newStr := fs.getString(ctx.Input, "new_string")

	fullPath := fs.resolvePath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("read_failed", err),
		}
	}

	newContent := string(content)
	if oldStr != "" {
		newContent = fs.replaceOnce(newContent, oldStr, newStr)
	} else {
		newContent += newStr
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("write_failed", err),
		}
	}

	return tools.ToolResult{
		Data: fmt.Sprintf("Edited %s", path),
	}
}

// handleGlob lists files matching a pattern.

// makeEditTool creates the edit tool definition.
func (fs *FileSystemBucket) makeEditTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name:        "edit",
		Description: "Edit a file by replacing old_string with new_string.",
		InputSchema: func() map[string]any {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path":  map[string]any{"type": "string"},
					"old_string": map[string]any{"type": "string"},
					"new_string": map[string]any{"type": "string"},
				},
				"required": []string{"file_path"},
			}
		},
		Capabilities: tools.CapabilityFlags{
			IsEnabled:     func() bool { return true },
			IsDestructive: func(map[string]any) bool { return true },
		},
	})
}

// makeGlobTool creates the glob tool definition.
