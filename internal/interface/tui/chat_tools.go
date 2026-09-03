package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m *ChatModel) AddToolMessage(toolName, toolDisplayName, content string) {
	if toolDisplayName == "" {
		toolDisplayName = toolName
	}
	msg := ChatMessage{
		ID:              fmt.Sprintf("%s-%d", toolName, time.Now().UnixNano()),
		Role:            "tool",
		Content:         content,
		Timestamp:       time.Now(),
		IsTool:          true,
		ToolName:        toolName,
		ToolDisplayName: toolDisplayName,
		ToolStatus:      ToolStatusComplete,
	}
	m.messages = append(m.messages, msg)
	// Background tool activity must never yank a scrolled-up user back
	// to the bottom: follow only when the view was already there.
	m.refreshViewport()
}

// appendToolMessage adds a tool message to the transcript. While a turn
// streams, the assistant bubble is pinned to the bottom of the message
// list: tool calls that arrive after it materialized (multi-step tool
// chains) insert BEFORE it — chronologically the work happened first,
// and appending below rendered trailing tool calls after the final
// response.
func (m *ChatModel) appendToolMessage(msg ChatMessage) {
	assistantIdx := -1
	if m.streamingAssistant() != nil {
		assistantIdx = m.currentStreamingAssistantIdx
	}
	if assistantIdx < 0 {
		m.messages = append(m.messages, msg)
		return
	}
	m.messages = append(m.messages, ChatMessage{})
	copy(m.messages[assistantIdx+1:], m.messages[assistantIdx:])
	m.messages[assistantIdx] = msg
}

// AddOrUpdateToolMessage adds a tool message or updates existing one by ID.
// This prevents duplicate tool messages - instead updates in place.
func (m *ChatModel) AddOrUpdateToolMessage(id, toolName, toolDisplayName, command string, status ToolStatus) {
	if toolDisplayName == "" {
		toolDisplayName = toolName
	}

	// Look for existing message with same ID
	for i := range m.messages {
		if m.messages[i].ID == id && m.messages[i].IsTool {
			// Update existing message in place: the start time set at
			// creation is the log timestamp; the terminal status fills
			// in the elapsed time. The detail column keeps the original
			// target when the update carries none.
			detail := command
			if detail == "" {
				detail = m.messages[i].ToolDetail
			} else {
				m.messages[i].ToolDetail = detail
			}
			if m.messages[i].ToolStatus == ToolStatusRunning && status != ToolStatusRunning {
				m.messages[i].ToolElapsed = time.Since(m.messages[i].ToolStartedAt)
			}
			m.messages[i].ToolStatus = status
			m.messages[i].Content = m.formatToolContent(toolDisplayName, detail, status, m.messages[i].ToolStartedAt, m.messages[i].ToolElapsed)
			m.refreshViewport()
			return
		}
	}

	// Add new message
	started := time.Now()
	msg := ChatMessage{
		ID:              id,
		Role:            "tool",
		Content:         m.formatToolContent(toolDisplayName, command, status, started, 0),
		Timestamp:       started,
		IsTool:          true,
		ToolName:        toolName,
		ToolDisplayName: toolDisplayName,
		ToolStatus:      status,
		ToolStartedAt:   started,
		ToolDetail:      command,
	}
	m.appendToolMessage(msg)
	m.refreshViewport()
}

// toolNameColumn is the fixed width of the tool-name column in the
// structured tool line, so the detail column aligns down a turn.
const toolNameColumn = 8

// formatToolContent renders one tool event as a structured log record,
// Splunk-shaped but readable at a glance:
//
//	01:20:03 ✓ bash     git log --oneline -8                          0.4s
//	01:20:05 ✓ read     pkg/format/format.go                          0.1s
//	01:20:07 → grep     "ToolStatus" in internal/                        …
//
// Time · status glyph · tool name (padded) · target detail · right-
// aligned duration (live calls show a running ellipsis instead).
func (m *ChatModel) formatToolContent(toolDisplayName, command string, status ToolStatus, started time.Time, elapsed time.Duration) string {
	var glyph string
	switch status {
	case ToolStatusRunning:
		glyph = "→"
	case ToolStatusError:
		glyph = "✗"
	default:
		glyph = "✓"
	}

	detail := command
	if detail != "" {
		detail = m.truncateCommandForWidth(toolDisplayName, detail)
	}

	timeStr := started.Format("15:04:05")
	name := toolDisplayName
	if pad := toolNameColumn - len(name); pad > 0 {
		name += strings.Repeat(" ", pad)
	}

	glyphAndName := glyph + " " + name
	line := fmt.Sprintf("%s %s %s",
		ToolTimeStyle.Render(timeStr),
		glyph+" "+name,
		detail,
	)

	if status == ToolStatusRunning {
		return line + ToolTimeStyle.Render("  …")
	}
	dur := formatElapsed(elapsed)
	// Display-width padding: glyph and dot runes are multi-byte, so
	// len() would over-count and shove the duration off the edge. Two
	// columns are reserved for the expand caret the render path
	// prepends (▸ folded / ▾ open) so the line stays exact-width.
	pad := m.width - lipgloss.Width(timeStr) - lipgloss.Width(glyphAndName) - lipgloss.Width(detail) - lipgloss.Width(dur) - 6
	if pad < 2 {
		pad = 2
	}
	return line + strings.Repeat(" ", pad) + ToolTimeStyle.Render(dur)
}

// ExpandLatestRecord expands the most recent expandable record — a
// tool call or the model's reasoning (keyboard path). It reports
// whether anything was expanded.
func (m *ChatModel) ExpandLatestRecord() bool {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.IsTool || (msg.Role == "assistant" && strings.TrimSpace(msg.ReasoningText) != "") {
			m.expandedMessageID = msg.ID
			m.refreshViewport()
			return true
		}
	}
	return false
}

// CollapseRecordExpansion folds an expanded record back to its summary
// line. It reports whether something was collapsed, so Esc can fall
// through to other behavior when nothing is open.
func (m *ChatModel) CollapseRecordExpansion() bool {
	if m.expandedMessageID == "" {
		return false
	}
	m.expandedMessageID = ""
	m.refreshViewport()
	return true
}

// AddToolMessageWithPreview adds a tool message with command preview.
// DEPRECATED: Use AddOrUpdateToolMessage instead for proper message replacement.
func (m *ChatModel) AddToolMessageWithPreview(toolName, toolDisplayName, command string) {
	m.AddOrUpdateToolMessage(toolName, toolName, toolDisplayName, command, ToolStatusRunning)
}

// extractCommandFromToolInput extracts the human-readable command from tool input
func (m *ChatModel) extractCommandFromToolInput(toolName string, input map[string]any) string {
	switch toolName {
	case "bash", "BashTool":
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			// Truncate long commands for display
			return m.truncateCommand(cmd, 60)
		}
	case "read", "ReadTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("cat %s", path)
		}
	case "write", "WriteTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("write %s", path)
		}
	case "edit", "EditTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("edit %s", path)
		}
	case "glob", "GlobTool":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return fmt.Sprintf("find %s", pattern)
		}
	case "grep", "GrepTool":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			return fmt.Sprintf("grep '%s'", pattern)
		}
	case "webfetch", "WebFetchTool":
		if url, ok := input["url"].(string); ok && url != "" {
			return fmt.Sprintf("fetch %s", m.truncateCommand(url, 40))
		}
	case "websearch", "WebSearchTool":
		if query, ok := input["query"].(string); ok && query != "" {
			return fmt.Sprintf("search '%s'", m.truncateCommand(query, 40))
		}
	}
	return ""
}

// truncateCommand truncates a command for display with ellipsis.
// Deprecated: use truncateCommandForWidth for responsive width-aware truncation.
func (m *ChatModel) truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}

// truncateCommandForWidth truncates a command so the entire tool line fits
// within the current terminal width, preserving space for the status indicator
// and tool display name.
func (m *ChatModel) truncateCommandForWidth(toolDisplayName, cmd string) string {
	// Reserve space for indicator (2), spaces (2), tool name, and padding (4)
	maxCmdLen := m.width - len(toolDisplayName) - 8
	if maxCmdLen < 12 {
		maxCmdLen = 12 // absolute minimum so something is visible
	}
	if len(cmd) > 40 && strings.Contains(cmd, "/") {
		compact := compactCommandForWidth(cmd, maxCmdLen)
		if len(compact) < len(cmd) {
			return compact
		}
	}
	if len(cmd) <= maxCmdLen {
		return cmd
	}
	return compactCommandForWidth(cmd, maxCmdLen)
}

func compactCommandForWidth(cmd string, maxLen int) string {
	if maxLen <= 0 || len(cmd) <= maxLen {
		return cmd
	}
	if maxLen <= 3 {
		return cmd[:maxLen]
	}
	fields := strings.Fields(cmd)
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if strings.Contains(last, "/") {
			name := pathBase(last)
			prefix := strings.Join(fields[:len(fields)-1], " ")
			candidate := ".../" + name
			if prefix != "" {
				candidate = prefix + " " + candidate
			}
			if len(candidate) <= maxLen {
				return candidate
			}
			if len(name)+4 <= maxLen {
				return ".../" + name
			}
			return "..." + name[len(name)-(maxLen-3):]
		}
	}
	return cmd[:maxLen-3] + "..."
}

func pathBase(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return path
	}
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

// SetInput sets the input text.
