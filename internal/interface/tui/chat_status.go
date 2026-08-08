package tui

import (
	"fmt"
	"time"
)

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
		m.messages[m.currentStreamingAssistantIdx].ResponseTime = m.elapsed
		m.messages[m.currentStreamingAssistantIdx].StreamedChunks = m.chunkCount
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:           "assistant",
			Content:        content,
			Timestamp:      time.Now(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
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
		m.messages[m.currentStreamingAssistantIdx].StreamedChunks = m.chunkCount
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:           "assistant",
			Content:        content,
			Timestamp:      time.Now(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
		})
	}
	m.currentStreamingAssistantIdx = -1
	m.refreshViewport()
}

// refreshViewport refreshes the viewport content.
