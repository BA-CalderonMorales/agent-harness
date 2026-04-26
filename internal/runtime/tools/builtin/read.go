package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

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
		if blockedDevicePaths[path] {
			return tools.ValidationResult{Valid: false, Message: "Reading this device path is not allowed"}
		}
		// UNC path security: reject SMB/NTLM credential leak vectors
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
			return tools.ValidationResult{Valid: false, Message: "UNC paths are not supported for security reasons"}
		}
		// Block /proc/*/fd paths
		if strings.HasPrefix(path, "/proc/") && strings.Contains(path, "/fd/") {
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

		// Check cache first
		cacheKey := fs.MakeKey(path, offset, limit, info)
		if cached, ok := fs.DefaultCache.Get(cacheKey); ok {
			// Record this read for stale-write protection
			fs.DefaultStaleTracker.RecordRead(path, []byte(cached), info)
			return tools.ToolResult{Data: cached, ContextModifier: func(ctx tools.Context) tools.Context {
				// Mark as cached read in context
				return ctx
			}}, nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to read file: %w", err)
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

		// Record read for stale-write protection
		fs.DefaultStaleTracker.RecordRead(path, data, info)

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
