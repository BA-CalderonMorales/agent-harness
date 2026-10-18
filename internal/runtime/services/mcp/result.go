package mcp

import (
	"encoding/json"
	"errors"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Result keeps outcome truth separate from display content, so a receipt for
// oversized evidence cannot accidentally turn a tool failure into success.
type Result struct {
	Content string
	IsError bool
}

func renderToolResult(result *CallToolResult) (Result, error) {
	if result == nil {
		return Result{}, errors.New("MCP server returned no tool result")
	}
	var text []string
	plain := result.StructuredContent == nil && len(result.Meta) == 0 && !result.IsError && !result.NeedsInput()
	for _, content := range result.Content {
		if item, ok := content.(*sdk.TextContent); ok {
			text = append(text, item.Text)
			if len(item.Meta) > 0 || item.Annotations != nil {
				plain = false
			}
		} else {
			plain = false
		}
	}
	if plain {
		return Result{Content: strings.Join(text, "\n")}, nil
	}
	// Preserve the complete MCP envelope when reducing it to text would lose
	// structured data, provenance, resources, media, or a tool-error boundary.
	data, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	out := Result{Content: string(data), IsError: result.IsError}
	if result.NeedsInput() {
		out.IsError = true
		out.Content = "MCP server requires a client interaction this harness does not support.\n" + out.Content
	}
	return out, nil
}
