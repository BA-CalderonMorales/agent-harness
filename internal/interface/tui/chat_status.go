package tui

import (
	"fmt"
	"strings"
	"time"
)

func (m *ChatModel) renderStatusLine() string {
	if !m.thinking {
		return ""
	}

	// Calculate elapsed time
	if m.timerRunning {
		m.elapsed = time.Since(m.startTime)
	}

	// Format elapsed time
	elapsedStr := formatElapsed(m.elapsed)

	// If we have an active tool with animation state, show animated tool display
	// This provides the "yolo mode" single-line animated tool view
	if m.toolAnimation != nil && m.currentTool != nil {
		// Update animation frame
		m.toolAnimation.Frame++

		spinner := ToolSpinnerRender(m.toolAnimation.Frame)
		toolName := m.toolAnimation.ToolName
		command := m.truncateCommandForWidth(toolName, m.toolAnimation.Command)

		// Build animated tool line: spinner + tool name + grey command preview
		var statusParts []string
		statusParts = append(statusParts, InfoStyle.Render(spinner))
		statusParts = append(statusParts, ToolCallStyle.Render(toolName))
		if command != "" {
			statusParts = append(statusParts, ToolCommandPreviewStyle.Render(command))
		}
		statusParts = append(statusParts, HelpDimStyle.Render(fmt.Sprintf("(%s)", elapsedStr)))

		return strings.Join(statusParts, " ")
	}

	// Determine status text based on state
	status := "thinking"
	if m.streaming && m.streamBuffer != "" {
		status = "streaming"
	} else if m.currentTool != nil {
		status = fmt.Sprintf("using %s", m.currentTool.Name)
	}

	// Build status line: spinner + status + elapsed + chunks
	spinner := SpinnerRender("")
	detail := elapsedStr
	if m.chunkCount > 0 {
		detail = fmt.Sprintf("%s | %d chunks", elapsedStr, m.chunkCount)
	}

	statusText := StreamingStyle.Render(fmt.Sprintf("%s %s (%s)", strings.TrimSpace(spinner), status, detail))
	return statusText
}

// formatElapsed formats a duration as human-readable string
func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}

// ConsumesTab returns whether this view consumes Tab key.
// When inline suggestions are showing, Tab is used for auto-completion.
func (m *ChatModel) updateOrCreateStreamingMessage(content string) {
	if m.currentStreamingAssistantIdx >= 0 && m.currentStreamingAssistantIdx < len(m.messages) {
		m.messages[m.currentStreamingAssistantIdx].Content = content
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:      "assistant",
			Content:   content,
			Timestamp: time.Now(),
		})
		m.currentStreamingAssistantIdx = len(m.messages) - 1
	}
	m.refreshViewport()
}

// finalizeStreamingMessage finalizes the streaming message for the current turn.
// Uses currentStreamingAssistantIdx to ensure the correct message is finalized
// even when other messages were added mid-stream.
func (m *ChatModel) finalizeStreamingMessage(content string) {
	if m.currentStreamingAssistantIdx >= 0 && m.currentStreamingAssistantIdx < len(m.messages) {
		m.messages[m.currentStreamingAssistantIdx].Content = content
		m.messages[m.currentStreamingAssistantIdx].Timestamp = time.Now()
		m.messages[m.currentStreamingAssistantIdx].ResponseTime = m.elapsed
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:         "assistant",
			Content:      content,
			Timestamp:    time.Now(),
			ResponseTime: m.elapsed,
		})
	}
	m.currentStreamingAssistantIdx = -1
	m.refreshViewport()
}

// refreshViewport refreshes the viewport content.
