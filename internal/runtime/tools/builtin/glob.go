package builtin

import (
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// GlobTool searches for files matching a pattern.
var GlobTool = tools.NewTool(tools.Tool{
	Name:        "glob",
	Description: "Find files matching a glob pattern in a single directory (e.g., '*.go', '*.ts'). Does not search recursively; use find or ls_recursive for recursive search.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string"},
				"path":    map[string]any{"type": "string", "description": "Directory to search in"},
			},
			"required": []string{"pattern"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:             func() bool { return true },
		IsConcurrencySafe:     func(map[string]any) bool { return true },
		IsReadOnly:            func(map[string]any) bool { return true },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags { return tools.SearchReadFlags{IsSearch: true} },
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
		searchPath := getString(input, "path")
		if searchPath == "" {
			searchPath = "."
		}

		matches, err := filepath.Glob(filepath.Join(searchPath, pattern))
		if err != nil {
			return tools.ToolResult{}, err
		}

		// Respect glob limits
		limit := ctx.GlobLimits.MaxResults
		if limit > 0 && len(matches) > limit {
			matches = matches[:limit]
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
	UserFacingName: func(map[string]any) string { return "glob" },
	GetActivityDescription: func(input map[string]any) string {
		if p, ok := input["pattern"].(string); ok {
			return "Searching for " + p
		}
		return "Searching files"
	},
})
