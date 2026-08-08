// Package buckets provides domain-specific ToolBase implementations.

package buckets

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/defaults"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
	"os"
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
