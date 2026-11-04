package builtin

import (
	"errors"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// WebSearchTool remains registered for compatibility with saved tool calls.
// No search backend is implemented, so it must not be advertised as usable.
var WebSearchTool = tools.NewTool(tools.Tool{
	Name:        "web_search",
	Description: "Web search is unavailable: no search backend is configured.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return false },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
		IsOpenWorld:       func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		if getString(input, "query") == "" {
			return tools.ValidationResult{Valid: false, Message: "query is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		return tools.ToolResult{}, errors.New("web_search is unavailable: no search backend is implemented. No search was performed. If a source URL is known, web_fetch can retrieve it; do not invent current facts or sources")
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "web_search" },
	GetActivityDescription: func(input map[string]any) string {
		if q, ok := input["query"].(string); ok {
			return "Searching web for: " + q
		}
		return "Searching web"
	},
})
