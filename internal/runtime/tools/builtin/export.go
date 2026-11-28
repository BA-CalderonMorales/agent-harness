package builtin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/redact"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// ExportTool saves the conversation transcript to a file.
var ExportTool = tools.NewTool(tools.Tool{
	Name:        "export",
	Description: "Export the current conversation transcript to a redacted JSON, Markdown, or text file.",
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
					"enum":        []string{"json", "markdown", "md", "txt", "text"},
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
		switch strings.ToLower(format) {
		case "json", "markdown", "md", "txt", "text":
		default:
			return tools.ValidationResult{Valid: false, Message: "format must be 'json', 'markdown', 'md', 'txt', or 'text'"}
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
		switch strings.ToLower(format) {
		case "json":
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(redactExportMessages(ctx.Messages)); err != nil {
				return tools.ToolResult{}, err
			}
			content = buf.String()
		case "markdown", "md":
			content = exportMarkdown(ctx.Messages)
		case "txt", "text":
			content = exportText(ctx.Messages)
		}

		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
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
	var b strings.Builder
	b.WriteString("# Agent Harness Transcript\n\n")
	fmt.Fprintf(&b, "Exported: %s\n\n", time.Now().Format(time.RFC3339))
	for _, m := range msgs {
		fmt.Fprintf(&b, "## %s\n\n", m.Role)
		for _, block := range m.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redact.String(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				fmt.Fprintf(&b, "**Tool Use:** %s\n\n", v.Name)
				inputJSON, _ := json.MarshalIndent(redact.Map(v.Input), "", "  ")
				b.WriteString("```json\n")
				b.WriteString(redact.String(string(inputJSON)))
				b.WriteString("\n```\n\n")
			case types.ToolResultBlock:
				fmt.Fprintf(&b, "**Tool Result:** %s\n\n", redact.String(v.Content))
			}
		}
	}
	return b.String()
}

func exportText(msgs []types.Message) string {
	var b strings.Builder
	b.WriteString("Agent Harness Transcript\n\n")
	fmt.Fprintf(&b, "Exported: %s\n\n", time.Now().Format(time.RFC3339))
	for _, m := range msgs {
		if m.Role == types.RoleSystem {
			continue
		}
		fmt.Fprintf(&b, "== %s ==\n", strings.ToUpper(string(m.Role)))
		for _, block := range m.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redact.String(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				fmt.Fprintf(&b, "[tool use: %s]\n", v.Name)
				inputJSON, _ := json.MarshalIndent(redact.Map(v.Input), "", "  ")
				b.WriteString(redact.String(string(inputJSON)))
				b.WriteString("\n\n")
			case types.ToolResultBlock:
				b.WriteString("[tool result]\n")
				b.WriteString(redact.String(v.Content))
				b.WriteString("\n\n")
			}
		}
	}
	return b.String()
}

func redactExportMessages(msgs []types.Message) []types.Message {
	return redact.Messages(msgs)
}
