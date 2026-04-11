package builtin

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// WebFetchTool fetches content from a URL.
var WebFetchTool = tools.NewTool(tools.Tool{
	Name:        "web_fetch",
	Description: "Fetch the contents of a web page.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string"},
			},
			"required": []string{"url"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		url := getString(input, "url")
		if url == "" || !strings.HasPrefix(url, "http") {
			return tools.ValidationResult{Valid: false, Message: "valid http/https URL is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		url := getString(input, "url")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return tools.ToolResult{}, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return tools.ToolResult{}, err
		}

		content := string(body)
		if len(content) > 10000 {
			content = content[:10000] + "\n... (truncated)"
		}
		return tools.ToolResult{Data: fmt.Sprintf("Status: %s\n\n%s", resp.Status, content)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "web_fetch" },
})
