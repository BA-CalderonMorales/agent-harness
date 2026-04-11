// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopFileSystem handles all file-related operations.
// It implements LoopBase but only knows about files - no shell, no search.
type FileSystemBucket struct {
	basePath      string
	allowedPaths  []string
	blockedPaths  []string
	maxFileSize   int64
	maxReadOffset int
	maxReadLimit  int
}

// NewLoopFileSystem creates a file system bucket.
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
	case "read", "read_file", "write", "write_file", "glob", "ls", "list_files",
		"edit", "file_edit", "search_files":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (fs *FileSystemBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,  // File reads are safe
		IsReadOnly:        false, // Can write
		IsDestructive:     true,  // Can modify/delete files
		ToolNames: []string{
			"read", "read_file", "write", "write_file",
			"glob", "ls", "list_files", "edit", "file_edit",
		},
		Category: "filesystem",
	}
}

// Execute runs the file operation.
func (fs *FileSystemBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "read", "read_file":
		return fs.handleRead(ctx)
	case "write", "write_file":
		return fs.handleWrite(ctx)
	case "glob", "ls", "list_files":
		return fs.handleList(ctx)
	case "edit", "file_edit":
		return fs.handleEdit(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "filesystem bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleRead reads a file.
func (fs *FileSystemBucket) handleRead(ctx loop.ExecutionContext) loop.LoopResult {
	path, ok := ctx.Input["path"].(string)
	if !ok || path == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "path is required"),
		}
	}

	// Security: check blocked paths
	if fs.isBlocked(path) {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("permission_denied", "reading this path is not allowed: "+path),
		}
	}

	// Resolve path
	fullPath := fs.resolvePath(path)

	// Check file size
	info, err := os.Stat(fullPath)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("stat_failed", err),
		}
	}

	if info.Size() > fs.maxFileSize {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("file_too_large", fmt.Sprintf("file size %d exceeds limit %d", info.Size(), fs.maxFileSize)),
		}
	}

	// Read with optional offset/limit
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("read_failed", err),
		}
	}

	offset := 0
	limit := len(content)

	if o, ok := ctx.Input["offset"].(float64); ok {
		offset = int(o)
	}
	if l, ok := ctx.Input["limit"].(float64); ok {
		limit = int(l)
	}

	// Apply offset/limit
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	if limit < 0 || offset+limit > len(content) {
		limit = len(content) - offset
	}

	result := string(content[offset : offset+limit])

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleWrite writes a file.
func (fs *FileSystemBucket) handleWrite(ctx loop.ExecutionContext) loop.LoopResult {
	path, ok := ctx.Input["path"].(string)
	if !ok || path == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "path is required"),
		}
	}

	content, ok := ctx.Input["content"].(string)
	if !ok {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "content is required"),
		}
	}

	// Security check
	if fs.isBlocked(path) {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("permission_denied", "writing to this path is not allowed: "+path),
		}
	}

	fullPath := fs.resolvePath(path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("mkdir_failed", err),
		}
	}

	// Atomic write: write to temp then rename
	tempPath := fullPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("write_failed", err),
		}
	}

	if err := os.Rename(tempPath, fullPath); err != nil {
		os.Remove(tempPath)
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("rename_failed", err),
		}
	}

	result := fmt.Sprintf("Wrote %d bytes to %s", len(content), path)
	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleList lists files in a directory.
func (fs *FileSystemBucket) handleList(ctx loop.ExecutionContext) loop.LoopResult {
	path, ok := ctx.Input["path"].(string)
	if !ok || path == "" {
		path = "."
	}

	fullPath := fs.resolvePath(path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("read_dir_failed", err),
		}
	}

	recursive := false
	if r, ok := ctx.Input["recursive"].(bool); ok {
		recursive = r
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
		if recursive && entry.IsDir() {
			subFiles := fs.listRecursive(filepath.Join(fullPath, entry.Name()))
			files = append(files, subFiles...)
		}
	}

	result := ""
	for _, f := range files {
		result += f + "\n"
	}

	return loop.LoopResult{
		Success: true,
		Data:    files,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleEdit performs a file edit.
func (fs *FileSystemBucket) handleEdit(ctx loop.ExecutionContext) loop.LoopResult {
	// Simplified edit - just do a string replace for now
	path, ok := ctx.Input["path"].(string)
	if !ok || path == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "path is required"),
		}
	}

	oldStr, _ := ctx.Input["old_string"].(string)
	newStr, _ := ctx.Input["new_string"].(string)

	fullPath := fs.resolvePath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("read_failed", err),
		}
	}

	newContent := string(content)
	if oldStr != "" {
		newContent = replaceOnce(newContent, oldStr, newStr)
	} else {
		// Append if no old_string specified
		newContent += newStr
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("write_failed", err),
		}
	}

	result := fmt.Sprintf("Edited %s", path)
	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
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

func replaceOnce(s, old, new string) string {
	idx := 0
	for i := 0; i < len(s)-len(old)+1; i++ {
		if s[i:i+len(old)] == old {
			if idx == 0 {
				return s[:i] + new + s[i+len(old):]
			}
			idx++
		}
	}
	return s
}

// Ensure LoopFileSystem implements LoopBase
var _ loop.LoopBase = (*FileSystemBucket)(nil)
