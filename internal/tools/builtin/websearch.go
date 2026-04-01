package builtin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// WebSearchTool performs a web search via DuckDuckGo instant answer API.
// In production, this would call a real search API.
var WebSearchTool = tools.NewTool(tools.Tool{
	Name:        "web_search",
	Description: "Search the web for information.",
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
		query := getString(input, "query")

		// Use DuckDuckGo HTML version as a simple search fallback
		// In production, integrate with a proper search API
		client := &http.Client{Timeout: 15 * time.Second}
		searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
		resp, err := client.Get(searchURL)
		if err != nil {
			return tools.ToolResult{}, err
		}
		defer resp.Body.Close()

		// For the pattern demonstration, we return a structured result
		result := map[string]any{
			"query":   query,
			"results": []string{"(search results would be parsed here)"},
			"note":    "In production, integrate with a search API like Serper, Bing, or Google Custom Search",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return tools.ToolResult{Data: string(data)}, nil
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
