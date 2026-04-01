package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// blockedDevicePaths prevents reading blocking or infinite devices.
var blockedDevicePaths = map[string]bool{
	"/dev/zero": true, "/dev/random": true, "/dev/urandom": true,
	"/dev/full": true, "/dev/stdin": true, "/dev/tty": true,
	"/dev/console": true, "/dev/stdout": true, "/dev/stderr": true,
	"/dev/fd/0": true, "/dev/fd/1": true, "/dev/fd/2": true,
}

// FileReadTool reads files from disk.
var FileReadTool = tools.NewTool(tools.Tool{
	Name:        "read",
	Description: "Read the contents of a file. Supports text, images, and PDFs.",
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
		if strings.HasPrefix(path, "\\") || strings.HasPrefix(path, "//") {
			return tools.ValidationResult{Valid: false, Message: "UNC paths are not supported"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		path = resolveScreenshotPath(path)

		data, err := os.ReadFile(path)
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to read file: %w", err)
		}

		content := string(data)
		offset := int(getFloat(input, "offset"))
		limit := int(getFloat(input, "limit"))

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

		return tools.ToolResult{Data: content}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		content, _ := result.(string)
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: content}
	},
	UserFacingName: func(map[string]any) string { return "read" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["file_path"].(string); ok {
			return "Reading " + p
		}
		return "Reading file"
	},
})

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
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
