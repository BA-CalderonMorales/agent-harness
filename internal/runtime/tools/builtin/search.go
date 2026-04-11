package builtin

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// SearchTranscriptTool searches the conversation history.
var SearchTranscriptTool = tools.NewTool(tools.Tool{
	Name:        "search_transcript",
	Description: "Search the conversation transcript for a keyword or phrase.",
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
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
		IsSearchOrReadCommand: func(map[string]any) tools.SearchReadFlags {
			return tools.SearchReadFlags{IsSearch: true}
		},
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
		query := strings.ToLower(getString(input, "query"))
		var matches []string

		for i, m := range ctx.Messages {
			for _, block := range m.Content {
				if tb, ok := block.(types.TextBlock); ok {
					if strings.Contains(strings.ToLower(tb.Text), query) {
						matches = append(matches, fmt.Sprintf("Message %d (%s): %s", i+1, m.Role, truncate(tb.Text, 100)))
					}
				}
			}
		}

		result := strings.Join(matches, "\n")
		if result == "" {
			result = "(no matches found in transcript)"
		}
		return tools.ToolResult{Data: result}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "search" },
	GetActivityDescription: func(input map[string]any) string {
		if q, ok := input["query"].(string); ok {
			return "Searching transcript for: " + q
		}
		return "Searching transcript"
	},
})

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
