package buckets

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"os"
)

// handleRead reads a file.
func (fs *FileSystemBucket) handleRead(ctx tools.ToolExecutionContext) tools.ToolResult {
	path := fs.getString(ctx.Input, "file_path")
	if path == "" {
		return tools.ToolResult{
			Data: tools.NewToolError("invalid_input", "file_path is required"),
		}
	}

	// Security: check blocked paths
	if fs.isBlocked(path) {
		return tools.ToolResult{
			Data: tools.NewToolError("permission_denied", "reading this path is not allowed: "+path),
		}
	}

	path = fs.resolvePath(path)
	offset := int(fs.getFloat(ctx.Input, "offset"))
	limit := int(fs.getFloat(ctx.Input, "limit"))

	// Get file info for cache key
	info, err := os.Stat(path)
	if err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("stat_failed", err),
		}
	}

	// Check file size
	if info.Size() > fs.maxFileSize {
		return tools.ToolResult{
			Data: tools.NewToolError("file_too_large", fmt.Sprintf("file size %d exceeds limit %d", info.Size(), fs.maxFileSize)),
		}
	}

	// Check cache first
	cacheKey := fs.MakeCacheKey(path, offset, limit, info)
	if cached, ok := fs.GetCache().Get(cacheKey); ok {
		fs.GetStaleTracker().RecordRead(path, []byte(cached), info)
		return tools.ToolResult{Data: cached}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return tools.ToolResult{
			Data: tools.WrapToolError("read_failed", err),
		}
	}

	content := string(data)
	if offset > 0 || limit > 0 {
		content = fs.applyOffsetLimit(content, offset, limit)
	}

	// Record for stale-write protection and cache
	fs.GetStaleTracker().RecordRead(path, data, info)
	fs.GetCache().Set(cacheKey, content)

	return tools.ToolResult{Data: content}
}

// handleWrite writes a file.

// makeReadTool creates the read tool definition.
func (fs *FileSystemBucket) makeReadTool() tools.Tool {
	return tools.NewTool(tools.Tool{
		Name:        "read",
		Description: "Read the contents of a file. Supports text, images, and PDFs.",
		InputSchema: func() map[string]any {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
					"offset":    map[string]any{"type": "integer"},
					"limit":     map[string]any{"type": "integer"},
				},
				"required": []string{"file_path"},
			}
		},
		Capabilities: tools.CapabilityFlags{
			IsEnabled:         func() bool { return true },
			IsConcurrencySafe: func(map[string]any) bool { return true },
			IsReadOnly:        func(map[string]any) bool { return true },
		},
	})
}

// makeWriteTool creates the write tool definition.
