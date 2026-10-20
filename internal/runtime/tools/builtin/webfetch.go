package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"

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
		if url == "" {
			return tools.ValidationResult{Valid: false, Message: "valid http/https URL is required"}
		}
		if err := validateFetchURL(url); err != nil {
			return tools.ValidationResult{Valid: false, Message: err.Error()}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		url := getString(input, "url")
		status, content, err := fetchWebContent(ctx.AbortController, url)
		if err != nil {
			return tools.ToolResult{}, err
		}
		return tools.ToolResult{Data: fmt.Sprintf("Status: %s\n\n%s", status, content)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "web_fetch" },
	GetActivityDescription: func(input map[string]any) string {
		if u, ok := input["url"].(string); ok && u != "" {
			return "Fetching " + u
		}
		return "Fetching URL"
	},
})

// fetchWebContent performs the guarded request and returns the status
// line plus a bounded amount of body text. The body read is capped so a
// hostile or misconfigured server cannot exhaust memory before the
// truncation check runs.
func fetchWebContent(parent context.Context, rawURL string) (string, string, error) {
	if err := validateFetchURL(rawURL); err != nil {
		return "", "", err
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", webFetchUserAgent)

	allowPrivate := allowPrivateFetch()
	if !allowPrivate {
		if err := refusePrivateHost(ctx, req.URL.Hostname()); err != nil {
			return "", "", err
		}
	}

	client := newWebFetchClient(allowPrivate)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBodyBytes+1))
	if err != nil {
		return "", "", err
	}
	truncated := len(body) > webFetchMaxBodyBytes
	if truncated {
		body = body[:webFetchMaxBodyBytes]
	}

	content := string(body)
	if len(content) > webFetchMaxOutput {
		content = content[:webFetchMaxOutput] + "\n... (truncated)"
	} else if truncated {
		content += "\n... (truncated at 1 MiB)"
	}
	return resp.Status, content, nil
}
