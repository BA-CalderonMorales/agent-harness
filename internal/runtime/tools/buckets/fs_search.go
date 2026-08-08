package buckets

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"path/filepath"
)

// handleGlob lists files matching a pattern.
func (fs *FileSystemBucket) handleGlob(ctx tools.ToolExecutionContext) tools.ToolResult {
	pattern := fs.getString(ctx.Input, "pattern")
	if pattern == "" {
		pattern = "*"
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("glob_failed", err),
		}
	}

	return tools.ToolResult{Data: matches}
}

// handleLsRecursive recursively lists files.
func (fs *FileSystemBucket) handleLsRecursive(ctx tools.ToolExecutionContext) tools.ToolResult {
	path := fs.getString(ctx.Input, "path")
	if path == "" {
		path = "."
	}

	fullPath := fs.resolvePath(path)
	files := fs.listRecursive(fullPath)

	return tools.ToolResult{Data: files}
}

// makeReadTool creates the read tool definition.

// makeGlobTool creates the glob tool definition.
func (fs *FileSystemBucket) makeGlobTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern.",
		InputSchema: func() map[string]any {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string"},
				},
				"required": []string{"pattern"},
			}
		},
		Capabilities: tools.CapabilityFlags{
			IsEnabled:         func() bool { return true },
			IsConcurrencySafe: func(map[string]any) bool { return true },
			IsReadOnly:        func(map[string]any) bool { return true },
		},
	})
}

// makeLsRecursiveTool creates the ls_recursive tool definition.
func (fs *FileSystemBucket) makeLsRecursiveTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name:        "ls_recursive",
		Description: "Recursively list all files in a directory.",
		InputSchema: func() map[string]any {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			}
		},
		Capabilities: tools.CapabilityFlags{
			IsEnabled:         func() bool { return true },
			IsConcurrencySafe: func(map[string]any) bool { return true },
			IsReadOnly:        func(map[string]any) bool { return true },
		},
	})
}

// Helpers
