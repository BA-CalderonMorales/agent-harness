package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
			data, err := json.MarshalIndent(redactExportMessages(ctx.Messages), "", "  ")
			if err != nil {
				return tools.ToolResult{}, err
			}
			content = string(data)
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
	var b string
	b += "# Agent Harness Transcript\n\n"
	b += fmt.Sprintf("Exported: %s\n\n", time.Now().Format(time.RFC3339))
	for _, m := range msgs {
		b += fmt.Sprintf("## %s\n\n", m.Role)
		for _, block := range m.Content {
			if tb, ok := block.(types.TextBlock); ok {
				b += redactExportString(tb.Text) + "\n\n"
			} else if tu, ok := block.(types.ToolUseBlock); ok {
				b += fmt.Sprintf("**Tool Use:** %s\n\n", tu.Name)
				inputJSON, _ := json.MarshalIndent(redactExportMap(tu.Input), "", "  ")
				b += "```json\n" + redactExportString(string(inputJSON)) + "\n```\n\n"
			} else if tr, ok := block.(types.ToolResultBlock); ok {
				b += fmt.Sprintf("**Tool Result:** %s\n\n", redactExportString(tr.Content))
			}
		}
	}
	return b
}

func exportText(msgs []types.Message) string {
	var b strings.Builder
	b.WriteString("Agent Harness Transcript\n\n")
	b.WriteString(fmt.Sprintf("Exported: %s\n\n", time.Now().Format(time.RFC3339)))
	for _, m := range msgs {
		if m.Role == types.RoleSystem {
			continue
		}
		b.WriteString(fmt.Sprintf("== %s ==\n", strings.ToUpper(string(m.Role))))
		for _, block := range m.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redactExportString(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("[tool use: %s]\n", v.Name))
				inputJSON, _ := json.MarshalIndent(redactExportMap(v.Input), "", "  ")
				b.WriteString(redactExportString(string(inputJSON)))
				b.WriteString("\n\n")
			case types.ToolResultBlock:
				b.WriteString("[tool result]\n")
				b.WriteString(redactExportString(v.Content))
				b.WriteString("\n\n")
			}
		}
	}
	return b.String()
}

func redactExportMessages(msgs []types.Message) []types.Message {
	out := make([]types.Message, 0, len(msgs))
	for _, msg := range msgs {
		msg.APIError = redactExportString(msg.APIError)
		msg.StopReason = redactExportString(msg.StopReason)
		msg.Model = redactExportString(msg.Model)
		msg.Content = make([]types.ContentBlock, 0, len(msg.Content))
		for _, block := range msg.Content {
			switch v := block.(type) {
			case types.TextBlock:
				msg.Content = append(msg.Content, types.TextBlock{Text: redactExportString(v.Text)})
			case types.ToolUseBlock:
				msg.Content = append(msg.Content, types.ToolUseBlock{ID: v.ID, Name: v.Name, Input: redactExportMap(v.Input)})
			case types.ToolResultBlock:
				msg.Content = append(msg.Content, types.ToolResultBlock{ToolUseID: v.ToolUseID, Content: redactExportString(v.Content), IsError: v.IsError})
			case types.ThinkingBlock:
				msg.Content = append(msg.Content, types.ThinkingBlock{Thinking: "<redacted>", Signature: "<redacted>"})
			default:
				msg.Content = append(msg.Content, block)
			}
		}
		out = append(out, msg)
	}
	return out
}
