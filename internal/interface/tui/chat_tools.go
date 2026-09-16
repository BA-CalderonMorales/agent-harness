package tui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
			m.messages[i].bumpRev()
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

// shortToolTag derives the stable short identifier that joins a tool
// call to its full record (goal 0.3.29 Task 3c, Slice 1): the first
// four hex characters of the FNV-1a 64 hash of the tool-use ID — the
// same ID the audit log records as ToolCallID, so a tag typed into the
// Logs filter (Slice 2) joins the chat record to the full command,
// permission decision, and audit trail. Always derived, never stored: a
// scheme change never migrates data. An empty ID yields an empty tag
// (records that have no tool-use ID — legacy data — render without
// one).
//
// It renders on the record, not on the summary row: beside a duration a
// `#d82b` reads as a color code (live dogfood finding), so the row the
// user scans carries no tag and the expanded record does.
func shortToolTag(toolID string) string {
	if toolID == "" {
		return ""
	}
	h := fnv.New64a()
	h.Write([]byte(toolID))
	return fmt.Sprintf("#%04x", h.Sum64()&0xffff)
}

// formatToolContent renders one tool event as a structured log record,
// Splunk-shaped but readable at a glance:
//
//	01:20:03 ✓ bash     git log --oneline -8                    0.4s
//	01:20:05 ✓ read     pkg/format/format.go                    0.1s
//	01:20:07 → grep     "ToolStatus" in internal/                 …
//
// Time · status glyph · tool name (padded) · target detail, then the
// duration right-aligned against the row's right edge so a column of
// calls reads as a settled list. Running calls put an ellipsis in the
// duration column instead, so the list does not reflow when a call
// settles.
func (m *ChatModel) formatToolContent(toolDisplayName, command string, status ToolStatus, started time.Time, elapsed time.Duration) string {
	return m.formatToolContentAt(m.width, toolDisplayName, command, status, started, elapsed)
}

// formatToolContentAt renders the record for a width budget — nested
// rows live inside the response bubble, which is narrower than the
// pane, so the budget is the caller's actual space and a full-width row
// would wrap its duration onto its own line.
//
// The layout is a fixed head (timestamp, glyph, name column and their
// separators), one elastic detail column, and a fixed duration. The
// duration is padded out to the row's right edge, so it is the column
// that lines up turn after turn; the detail yields first. Nothing here
// double-counts: the old reservation added the tag width twice and an
// 8-column fudge on top, which left the duration stranded ~13 columns
// short of the edge (live dogfood: the gap was the loudest thing on the
// row). Display width, never len(): glyphs are multi-byte.
func (m *ChatModel) formatToolContentAt(width int, toolDisplayName, command string, status ToolStatus, started time.Time, elapsed time.Duration) string {
	glyph := "✓"
	switch status {
	case ToolStatusRunning:
		glyph = "→"
	case ToolStatusError:
		glyph = "✗"
	}

	timeStr := started.Format("15:04:05")
	name := toolDisplayName
	if pad := toolNameColumn - lipgloss.Width(name); pad > 0 {
		name += strings.Repeat(" ", pad)
	}

	dur := "…"
	if status != ToolStatusRunning {
		dur = formatElapsed(elapsed)
	}

	// Head: timestamp, space, glyph, space, padded name column. Its
	// width is fixed for every row of a turn, which is what lets the
	// duration land in the same column each time.
	head := ToolTimeStyle.Render(timeStr) + " " + glyph + " " + name
	headW := lipgloss.Width(timeStr) + 3 + lipgloss.Width(name)
	durW := lipgloss.Width(dur)

	// The detail takes what is left after the head, one separating
	// space, the duration, and one column of minimum pad.
	detail := command
	budget := width - headW - 1 - durW - 1
	if budget < 1 {
		budget = 1
	}
	if lipgloss.Width(detail) > budget {
		detail = compactCommandForWidth(detail, budget)
	}

	pad := width - headW - 1 - lipgloss.Width(detail) - durW
	if pad < 1 {
		pad = 1
	}
	return head + " " + detail + strings.Repeat(" ", pad) + ToolTimeStyle.Render(dur)
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
	case "ls", "ls_recursive", "list_files", "LsTool":
		if path, ok := input["path"].(string); ok && path != "" {
			return fmt.Sprintf("ls %s", path)
		}
		if cmd, ok := input["command"].(string); ok && cmd != "" {
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
			return fmt.Sprintf("update %s", path)
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
// Deprecated: the tool row truncates to its own measured budget with
// compactCommandForWidth; this byte-based form survives only for the
// input-map previews in extractCommandFromToolInput.
func (m *ChatModel) truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}

func compactCommandForWidth(cmd string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(cmd) <= maxLen {
		return cmd
	}
	if maxLen <= 3 {
		return truncateDisplayWidth(cmd, maxLen)
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
			if lipgloss.Width(candidate) <= maxLen {
				return candidate
			}
			if lipgloss.Width(name)+4 <= maxLen {
				return ".../" + name
			}
			return "..." + truncateDisplayWidth(name, maxLen-3)
		}
	}
	return truncateDisplayWidth(cmd, maxLen)
}

// truncateDisplayWidth keeps terminal rows valid for Unicode text and
// measures what the terminal displays rather than UTF-8 bytes.
func truncateDisplayWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return ansi.Truncate(text, maxWidth, "")
	}
	return ansi.Truncate(text, maxWidth, "...")
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
