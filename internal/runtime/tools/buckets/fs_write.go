package buckets

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"os"
	"path/filepath"
)

// handleWrite writes a file.
func (fs *FileSystemBucket) handleWrite(ctx tools.ToolExecutionContext) tools.ToolResult {
	path := fs.getString(ctx.Input, "file_path")
	if path == "" {
		return tools.ToolResult{
			Data: tools.NewToolError("invalid_input", "file_path is required"),
		}
	}

	content := fs.getString(ctx.Input, "content")

	// Security check
	if fs.isBlocked(path) {
		return tools.ToolResult{
			Data: tools.NewToolError("permission_denied", "writing to this path is not allowed: "+path),
		}
	}

	fullPath := fs.resolvePath(path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("mkdir_failed", err),
		}
	}

	// Atomic write: write to temp then rename
	tempPath := fullPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("write_failed", err),
		}
	}

	if err := os.Rename(tempPath, fullPath); err != nil {
		os.Remove(tempPath)
		return tools.ToolResult{
			Data: tools.WrapToolError("rename_failed", err),
		}
	}

	return tools.ToolResult{
		Data: fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
	}
}

// handleEdit performs a file edit.

// makeWriteTool creates the write tool definition.
func (fs *FileSystemBucket) makeWriteTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name:        "write",
		Description: "Write content to a file.",
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
			IsEnabled:     func() bool { return true },
			IsDestructive: func(map[string]any) bool { return true },
		},
	})
}

// makeEditTool creates the edit tool definition.
