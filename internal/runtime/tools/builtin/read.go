package builtin

import (
	"fmt"
	"io"
	"os"
	stdpath "path"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// defaultMaxReadFileBytes caps raw file reads so an accidental read of a
// multi-gigabyte binary or log file cannot OOM the process.
const defaultMaxReadFileBytes int64 = 10 << 20

// blockedDevicePaths prevents reading blocking or infinite devices.
var blockedDevicePaths = map[string]bool{
	"/dev/zero": true, "/dev/random": true, "/dev/urandom": true,
	"/dev/full": true, "/dev/stdin": true, "/dev/tty": true,
	"/dev/console": true, "/dev/stdout": true, "/dev/stderr": true,
	"/dev/fd/0": true, "/dev/fd/1": true, "/dev/fd/2": true,
}

// FileReadTool reads files from disk with caching support.
var FileReadTool = tools.NewTool(tools.Tool{
	Name:        "read",
	Description: "Read the contents of a file. Supports text, images, and PDFs. Uses caching to preserve prompt cache tokens.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{"type": "string"},
				"offset":    map[string]any{"type": "integer", "description": "Line offset to start reading"},
				"limit":     map[string]any{"type": "integer", "description": "Maximum lines to read"},
			},
			"required": []string{"file_path"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:             func() bool { return true },
		IsConcurrencySafe:     func(map[string]any) bool { return true },
		IsReadOnly:            func(map[string]any) bool { return true },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags { return tools.SearchReadFlags{IsRead: true} },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		path := getString(input, "file_path")
		if path == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path is required"}
		}
		// UNC path security: reject SMB/NTLM credential leak vectors before Clean.
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
			return tools.ValidationResult{Valid: false, Message: "UNC paths are not supported for security reasons"}
		}
		cleanPath := stdpath.Clean(filepath.ToSlash(path))
		if blockedDevicePaths[cleanPath] {
			return tools.ValidationResult{Valid: false, Message: "Reading this device path is not allowed"}
		}
		// Block /proc/*/fd paths
		if strings.HasPrefix(cleanPath, "/proc/") && strings.Contains(cleanPath, "/fd/") {
			return tools.ValidationResult{Valid: false, Message: "Reading process file descriptors is not allowed"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		path = resolveScreenshotPath(path)

		offset := getInt(input, "offset")
		limit := getInt(input, "limit")

		// Get file info for cache key and stale tracking
		info, err := os.Stat(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to stat file: %w", err)
		}
		if info.IsDir() {
			return tools.ToolResult{}, fmt.Errorf("%s is a directory; use ls or ls_recursive to inspect directories", path)
		}
		if !info.Mode().IsRegular() {
			return tools.ToolResult{}, fmt.Errorf("cannot read non-regular file %s", path)
		}
		maxBytes := ctx.FileReadingLimits.MaxSizeBytes
		if maxBytes <= 0 {
			maxBytes = defaultMaxReadFileBytes
		}
		if info.Size() > maxBytes {
			return tools.ToolResult{}, fmt.Errorf("file %s is too large (%d bytes, limit %d bytes)", path, info.Size(), maxBytes)
		}

		// Check cache first. Do not overwrite DefaultStaleTracker with the
		// cached slice/truncated text: the original read already recorded the
		// full file hash, and replacing it with a partial slice causes false
		// stale-write errors on subsequent edits.
		cacheKey := fs.MakeKey(path, offset, limit, info)
		if cached, ok := fs.DefaultCache.Get(cacheKey); ok {
			return tools.ToolResult{Data: cached, ContextModifier: func(ctx tools.Context) tools.Context {
				// Mark as cached read in context
				return ctx
			}}, nil
		}

		data, readInfo, err := readFileBounded(path, maxBytes)
		if err != nil {
			return tools.ToolResult{}, err
		}

		content := string(data)

		if offset > 0 || limit > 0 {
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
				lines = lines[start:end]
				content = strings.Join(lines, "\n")
			} else {
				content = ""
			}
		}

		// Record read for stale-write protection using the descriptor we read.
		fs.DefaultStaleTracker.RecordRead(path, data, readInfo)

		// Truncate large outputs to prevent context overflow
		content = truncateReadOutput(content)

		// Cache the result
		fs.DefaultCache.Set(cacheKey, content)

		return tools.ToolResult{Data: content}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "Read" },
	GetActivityDescription: func(input map[string]any) string {
		path := getString(input, "file_path")
		if path == "" {
			return "Reading file"
		}
		// Show just the filename for brevity
		parts := strings.Split(path, "/")
		filename := parts[len(parts)-1]
		offset := getInt(input, "offset")
		limit := getInt(input, "limit")
		if offset > 0 || limit > 0 {
			return fmt.Sprintf("Reading %s (lines %d-%d)", filename, offset, offset+limit)
		}
		return fmt.Sprintf("Reading %s", filename)
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

// readFileBounded opens path and reads at most maxBytes bytes, rejecting files
// that grow past maxBytes after an earlier os.Stat before converting, hashing,
// or caching them. Files of exactly maxBytes bytes are accepted.
func readFileBounded(path string, maxBytes int64) ([]byte, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("%s is a directory; use ls or ls_recursive to inspect directories", path)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("cannot read non-regular file %s", path)
	}
	if info.Size() > maxBytes {
		return nil, nil, fmt.Errorf("file %s is too large (%d bytes, limit %d bytes)", path, info.Size(), maxBytes)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, nil, fmt.Errorf("file %s is too large (>%d bytes, limit %d bytes)", path, maxBytes, maxBytes)
	}
	return data, info, nil
}

func resolveScreenshotPath(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Try macOS screenshot thin-space variant (U+202F)
	filename := filepath.Base(path)
	thinSpace := string(rune(0x202F))
	if strings.Contains(filename, " ") {
		alt := strings.ReplaceAll(filename, " ", thinSpace)
		altPath := filepath.Join(filepath.Dir(path), alt)
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}
	if strings.Contains(filename, thinSpace) {
		alt := strings.ReplaceAll(filename, thinSpace, " ")
		altPath := filepath.Join(filepath.Dir(path), alt)
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}
	return path
}
