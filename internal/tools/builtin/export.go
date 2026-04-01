package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ExportTool saves the conversation transcript to a file.
var ExportTool = tools.NewTool(tools.Tool{
	Name:        "export",
	Description: "Export the current conversation transcript to a JSON or Markdown file.",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to save the export",
				},
				"format": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "markdown"},
					"description": "Export format",
				},
			},
			"required": []string{"file_path", "format"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return true },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		format := getString(input, "format")
		if format != "json" && format != "markdown" {
			return tools.ValidationResult{Valid: false, Message: "format must be 'json' or 'markdown'"}
		}
		if getString(input, "file_path") == "" {
			return tools.ValidationResult{Valid: false, Message: "file_path is required"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		path := getString(input, "file_path")
		format := getString(input, "format")

		var content string
		if format == "json" {
			data, err := json.MarshalIndent(ctx.Messages, "", "  ")
			if err != nil {
				return tools.ToolResult{}, err
			}
			content = string(data)
		} else {
			content = exportMarkdown(ctx.Messages)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return tools.ToolResult{}, fmt.Errorf("failed to write export: %w", err)
		}

		return tools.ToolResult{Data: fmt.Sprintf("Exported %d messages to %s", len(ctx.Messages), path)}, nil
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "export" },
})

func exportMarkdown(msgs []types.Message) string {
	var b string
	b += "# Agent Harness Transcript\n\n"
	b += fmt.Sprintf("Exported: %s\n\n", time.Now().Format(time.RFC3339))
	for _, m := range msgs {
		b += fmt.Sprintf("## %s\n\n", m.Role)
		for _, block := range m.Content {
			if tb, ok := block.(types.TextBlock); ok {
				b += tb.Text + "\n\n"
			} else if tu, ok := block.(types.ToolUseBlock); ok {
				b += fmt.Sprintf("**Tool Use:** %s\n\n", tu.Name)
			} else if tr, ok := block.(types.ToolResultBlock); ok {
				b += fmt.Sprintf("**Tool Result:** %s\n\n", tr.Content)
			}
		}
	}
	return b
}
