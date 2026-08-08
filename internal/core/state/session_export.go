package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SaveExportToFile saves a redacted session export without changing the raw
// session persistence format used for restore.
func (s *Session) SaveExportToFile(path, format string) error {
	var data []byte
	var err error

	switch strings.ToLower(format) {
	case "json":
		data, err = s.ExportToRedactedJSON()
	case "markdown", "md":
		data = []byte(s.ExportToMarkdown())
	case "text", "txt":
		data = []byte(s.ExportToText())
	default:
		return fmt.Errorf("unsupported export format: %q", format)
	}
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}
	return nil
}

// ExportToRedactedJSON exports the session as JSON with secrets and local paths
// removed for safe maintainer sharing.
func (s *Session) ExportToRedactedJSON() ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.redactedExportSession()); err != nil {
		return nil, fmt.Errorf("failed to marshal export: %w", err)
	}
	return buf.Bytes(), nil
}

// ExportToMarkdown exports the session as a human-readable Markdown transcript.
func (s *Session) ExportToMarkdown() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Session %s\n\n", exportIDPrefix(s.ID)))
	b.WriteString(fmt.Sprintf("- **Model**: %s\n", redactExportString(s.Model)))
	b.WriteString(fmt.Sprintf("- **Created**: %s\n", s.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Turns**: %d\n", s.Turns))
	b.WriteString(fmt.Sprintf("- **Messages**: %d\n\n", len(s.Messages)))

	for _, msg := range s.Messages {
		if msg.Role == "system" {
			continue
		}
		roleTitle := strings.ToUpper(string(msg.Role)[:1]) + string(msg.Role)[1:]
		b.WriteString(fmt.Sprintf("## %s\n\n", roleTitle))
		for _, block := range msg.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redactExportString(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("**Tool**: `%s`\n\n", v.Name))
				inputJSON, _ := json.MarshalIndent(redactExportValue(v.Input), "", "  ")
				b.WriteString("```json\n")
				b.WriteString(string(inputJSON))
				b.WriteString("\n```\n\n")
			case types.ToolResultBlock:
				b.WriteString("**Result**:\n\n```\n")
				b.WriteString(redactExportString(v.Content))
				b.WriteString("\n```\n\n")
			}
		}
	}
	return b.String()
}

// ExportToText exports a plain-text support log that works well in Termux/tmux
// and can be attached directly to maintainer reports.
func (s *Session) ExportToText() string {
	var b strings.Builder
	b.WriteString("Agent Harness Session Export\n")
	b.WriteString(fmt.Sprintf("Session: %s\n", exportIDPrefix(s.ID)))
	b.WriteString(fmt.Sprintf("Model: %s\n", redactExportString(s.Model)))
	b.WriteString(fmt.Sprintf("Created: %s\n", s.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Updated: %s\n", s.UpdatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Turns: %d\n", s.Turns))
	b.WriteString(fmt.Sprintf("Messages: %d\n\n", len(s.Messages)))

	for _, msg := range s.Messages {
		if msg.Role == types.RoleSystem {
			continue
		}
		b.WriteString(fmt.Sprintf("== %s ==\n", strings.ToUpper(string(msg.Role))))
		if msg.APIError != "" {
			b.WriteString("API error: ")
			b.WriteString(redactExportString(msg.APIError))
			b.WriteString("\n")
		}
		for _, block := range msg.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redactExportString(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("[tool use: %s]\n", v.Name))
				inputJSON, _ := json.MarshalIndent(redactExportValue(v.Input), "", "  ")
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

func (s *Session) redactedExportSession() *Session {
	cp := *s
	cp.Model = redactExportString(cp.Model)
	cp.Messages = make([]types.Message, 0, len(s.Messages))
	for _, msg := range s.Messages {
		msg.APIError = redactExportString(msg.APIError)
		msg.StopReason = redactExportString(msg.StopReason)
		msg.Model = redactExportString(msg.Model)
		originalContent := msg.Content
		msg.Content = make([]types.ContentBlock, 0, len(originalContent))
		for _, block := range originalContent {
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
		cp.Messages = append(cp.Messages, msg)
	}
	return &cp
}

func redactExportMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if isSensitiveExportKey(k) {
			out[k] = "<redacted>"
			continue
		}
		out[k] = redactExportValue(v)
	}
	return out
}

func redactExportValue(v any) any {
	switch value := v.(type) {
	case string:
		return redactExportString(value)
	case map[string]any:
		return redactExportMap(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = redactExportValue(item)
		}
		return out
	default:
		return value
	}
}

func isSensitiveExportKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password")
}

var exportRedactions = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*)\s*[:=]\s*["']?[^"',\s}\]]+["']?`), `$1=<redacted>`},
	{regexp.MustCompile(`\b(sk-or-v1-[A-Za-z0-9._-]+|sk-ant-[A-Za-z0-9._-]+|sk-proj-[A-Za-z0-9._-]+|sk-[A-Za-z0-9._-]{20,}|github_pat_[A-Za-z0-9_]+|ghp_[A-Za-z0-9_]+|AKIA[0-9A-Z]{16})\b`), `<redacted>`},
	{regexp.MustCompile(`/data/data/com\.termux/files/home(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<termux-home>`},
	{regexp.MustCompile(`/data/data/com\.termux/files/usr(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<termux-prefix>`},
	{regexp.MustCompile(`/(?:home|Users)/[A-Za-z0-9._-]+(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<user-home>`},
	{regexp.MustCompile(`/root(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<user-home>`},
}

func redactExportString(value string) string {
	out := value
	for _, redaction := range exportRedactions {
		out = redaction.re.ReplaceAllString(out, redaction.repl)
	}
	return out
}

func exportIDPrefix(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// LoadSession loads a session from a file
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// Compact compacts the session based on the provided config
