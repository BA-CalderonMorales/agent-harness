package builtin

import (
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// FindTool searches for files by name pattern recursively.
var FindTool = tools.NewTool(tools.Tool{
	Name:        "find",
	Aliases:     []string{"search_files"},
	Description: "Search for files by name pattern recursively (e.g. '*.go', '*_test.js').",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern to match file names"},
				"path":    map[string]any{"type": "string", "description": "Directory to search in", "default": "."},
			},
			"required": []string{"pattern"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
		IsDestructive:     func(map[string]any) bool { return false },
		InterruptBehavior: func() string { return "cancel" },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags {
			return tools.SearchReadFlags{IsSearch: true}
		},
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "pattern") == "" {
			return tools.ValidationResult{Valid: false, Message: "pattern is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		pattern := getString(input, "pattern")
		root := getString(input, "path")
		if root == "" {
			root = "."
		}

		var matches []string
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip paths we can't access
			}

			// Skip common ignored directories for performance
			if info.IsDir() && isCommonIgnoredDir(info.Name()) {
				return filepath.SkipDir
			}

			// Match file name against pattern
			matched, _ := filepath.Match(pattern, info.Name())
			if matched && !info.IsDir() {
				matches = append(matches, path)
				if onProgress != nil {
					onProgress("found: " + path)
				}
			}
			return nil
		})

		if err != nil {
			return tools.ToolResult{Data: "error: " + err.Error()}, nil
		}

		result := ""
		for _, m := range matches {
			result += m + "\n"
		}
		if result == "" {
			result = "(no files found)"
		}
		return tools.ToolResult{Data: result}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "find" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["pattern"].(string); ok {
			return "Searching for " + p
		}
		return "Searching files"
	},
})
