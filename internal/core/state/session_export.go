package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/redact"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
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
	if err := os.MkdirAll(dir, 0700); err != nil {
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
	fmt.Fprintf(&b, "# Session %s\n\n", exportIDPrefix(s.ID))
	fmt.Fprintf(&b, "- **Model**: %s\n", redactExportString(s.Model))
	fmt.Fprintf(&b, "- **Created**: %s\n", s.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Turns**: %d\n", s.Turns)
	fmt.Fprintf(&b, "- **Messages**: %d\n\n", len(s.Messages))

	for _, msg := range s.Messages {
		if msg.Role == "system" {
			continue
		}
		role := string(msg.Role)
		roleTitle := "Unknown"
		if len(role) > 0 {
			roleTitle = strings.ToUpper(role[:1]) + role[1:]
		}
		fmt.Fprintf(&b, "## %s\n\n", roleTitle)
		for _, block := range msg.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(redactExportString(v.Text))
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				fmt.Fprintf(&b, "**Tool**: `%s`\n\n", v.Name)
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
	fmt.Fprintf(&b, "Session: %s\n", exportIDPrefix(s.ID))
	fmt.Fprintf(&b, "Model: %s\n", redactExportString(s.Model))
	fmt.Fprintf(&b, "Created: %s\n", s.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Updated: %s\n", s.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Turns: %d\n", s.Turns)
	fmt.Fprintf(&b, "Messages: %d\n\n", len(s.Messages))

	for _, msg := range s.Messages {
		if msg.Role == types.RoleSystem {
			continue
		}
		fmt.Fprintf(&b, "== %s ==\n", strings.ToUpper(string(msg.Role)))
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
				fmt.Fprintf(&b, "[tool use: %s]\n", v.Name)
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
	cp.Model = redact.String(cp.Model)
	cp.Messages = redact.Messages(s.Messages)
	return &cp
}

func redactExportMap(input map[string]any) map[string]any {
	return redact.Map(input)
}

func redactExportValue(v any) any {
	return redact.Value(v)
}

// SetExportRedactSecret registers the active API key for exact redaction
// in session exports. An empty value clears it.
func SetExportRedactSecret(secret string) {
	redact.SetSecret(secret)
}

func redactExportString(value string) string {
	return redact.String(value)
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
