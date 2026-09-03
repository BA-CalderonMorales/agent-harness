package tui

import (
	"fmt"
	"strings"
	"time"
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

	// Build content with status indicator
	content := m.formatToolContent(toolDisplayName, command, status)

	// Look for existing message with same ID
	for i := range m.messages {
		if m.messages[i].ID == id && m.messages[i].IsTool {
			// Update existing message
			m.messages[i].Content = content
			m.messages[i].ToolStatus = status
			m.messages[i].Timestamp = time.Now()
			m.refreshViewport()
			return
		}
	}

	// Add new message
	msg := ChatMessage{
		ID:              id,
		Role:            "tool",
		Content:         content,
		Timestamp:       time.Now(),
		IsTool:          true,
		ToolName:        toolName,
		ToolDisplayName: toolDisplayName,
		ToolStatus:      status,
	}
	m.appendToolMessage(msg)
	m.refreshViewport()
}

// formatToolContent formats tool message content based on status.
// Command previews are truncated dynamically to fit the terminal width so
// phone users (Termux) can still see the tool name and status even when
// the command itself is very long.
func (m *ChatModel) formatToolContent(toolDisplayName, command string, status ToolStatus) string {
	var statusIndicator string
	switch status {
	case ToolStatusRunning:
		statusIndicator = "→"
	case ToolStatusSuccess:
		statusIndicator = "✓"
	case ToolStatusError:
		statusIndicator = "✗"
	case ToolStatusComplete:
		statusIndicator = "✓"
	default:
		statusIndicator = "→"
	}

	if command != "" {
		command = m.truncateCommandForWidth(toolDisplayName, command)
		return fmt.Sprintf("%s %s %s", statusIndicator, toolDisplayName, ToolCommandPreviewStyle.Render(command))
	}
	return fmt.Sprintf("%s %s", statusIndicator, toolDisplayName)
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
