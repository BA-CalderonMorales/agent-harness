// Package buckets provides domain-specific ToolBase implementations.
package buckets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/defaults"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
)

// FileSystemBucket handles all file-related tool operations.
// It implements ToolBase but only knows about files - no shell, no search.
type FileSystemBucket struct {
	basePath      string
	allowedPaths  []string
	blockedPaths  []string
	maxFileSize   int64
	maxReadOffset int
	maxReadLimit  int
}

// FileSystem creates a file system bucket with defaults.
func FileSystem(basePath string) *FileSystemBucket {
	return &FileSystemBucket{
		basePath:      basePath,
		allowedPaths:  []string{},
		blockedPaths:  defaults.FSDangerousPaths,
		maxFileSize:   defaults.FSMaxFileSize,
		maxReadOffset: defaults.FSMaxReadOffset,
		maxReadLimit:  defaults.FSMaxReadLimit,
	}
}

// WithAllowedPaths restricts operations to specific paths.
func (fs *FileSystemBucket) WithAllowedPaths(paths ...string) *FileSystemBucket {
	fs.allowedPaths = paths
	return fs
}

// WithBlockedPaths adds additional blocked paths.
func (fs *FileSystemBucket) WithBlockedPaths(paths ...string) *FileSystemBucket {
	fs.blockedPaths = append(fs.blockedPaths, paths...)
	return fs
}

// Name returns the bucket identifier.
func (fs *FileSystemBucket) Name() string {
	return "filesystem"
}

// CanHandle determines if this bucket handles the tool.
func (fs *FileSystemBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "read", "write", "edit", "glob", "ls_recursive":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (fs *FileSystemBucket) Capabilities() tools.ToolBucketCapabilities {
	return tools.ToolBucketCapabilities{
		IsConcurrencySafe: true,  // File reads are safe
		IsReadOnly:        false, // Can write
		IsDestructive:     true,  // Can modify/delete files
		ToolNames: []string{
			"read", "write", "edit", "glob", "ls_recursive",
		},
		Category: "filesystem",
	}
}

// GetTools returns the tool definitions for this bucket.
func (fs *FileSystemBucket) GetTools() []tools.Tool {
	return []tools.Tool{
		fs.makeReadTool(),
		fs.makeWriteTool(),
		fs.makeEditTool(),
		fs.makeGlobTool(),
		fs.makeLsRecursiveTool(),
	}
}

// Execute runs the file operation.
func (fs *FileSystemBucket) Execute(ctx tools.ToolExecutionContext) tools.ToolResult {
	switch ctx.ToolName {
	case "read":
		return fs.handleRead(ctx)
	case "write":
		return fs.handleWrite(ctx)
	case "edit":
		return fs.handleEdit(ctx)
	case "glob":
		return fs.handleGlob(ctx)
	case "ls_recursive":
		return fs.handleLsRecursive(ctx)
	default:
		return tools.ToolResult{
			Data: tools.NewToolError("unknown_tool", "filesystem bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

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

func (fs *FileSystemBucket) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(fs.basePath, path)
}

func (fs *FileSystemBucket) isBlocked(path string) bool {
	for _, blocked := range fs.blockedPaths {
		if path == blocked || strings.HasPrefix(path, blocked+"/") {
			return true
		}
	}
	return false
}

func (fs *FileSystemBucket) listRecursive(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		relPath, _ := filepath.Rel(fs.basePath, filepath.Join(dir, entry.Name()))
		files = append(files, relPath)
		if entry.IsDir() {
			subFiles := fs.listRecursive(filepath.Join(dir, entry.Name()))
			files = append(files, subFiles...)
		}
	}
	return files
}

func (fs *FileSystemBucket) applyOffsetLimit(content string, offset, limit int) string {
	lines := strings.Split(content, "\n")
	start := offset
	if start < 0 {
		start = 0
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	if start < end {
		return strings.Join(lines[start:end], "\n")
	}
	return ""
}

func (fs *FileSystemBucket) replaceOnce(s, old, new string) string {
	idx := strings.Index(s, old)
	if idx == -1 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func (fs *FileSystemBucket) getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (fs *FileSystemBucket) getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// MakeCacheKey creates a cache key for a file read.
func (b *FileSystemBucket) MakeCacheKey(path string, offset, limit int, info os.FileInfo) fs.ReadCacheKey {
	return fs.MakeKey(path, offset, limit, info)
}

// GetCache returns the global cache.
func (b *FileSystemBucket) GetCache() *fs.ReadCache {
	return fs.DefaultCache
}

// GetStaleTracker returns the global stale tracker.
func (b *FileSystemBucket) GetStaleTracker() *fs.StaleWriteTracker {
	return fs.DefaultStaleTracker
}

// Ensure FileSystemBucket implements ToolBase
var _ tools.ToolBase = (*FileSystemBucket)(nil)
