package tui

import (
	"fmt"
	"strings"
	"time"
)

// streamingAssistant returns the in-progress assistant message by its
// stable ID. The streaming index drifts when a mid-turn prepend inserts
// at position 0, so lookups must never go by index alone.
func (m *ChatModel) streamingAssistant() *ChatMessage {
	if m.currentStreamingAssistantID == "" {
		return nil
	}
	for i := range m.messages {
		if m.messages[i].Role == "assistant" && m.messages[i].ID == m.currentStreamingAssistantID {
			m.currentStreamingAssistantIdx = i
			return &m.messages[i]
		}
	}
	return nil
}

// dropPlaceholderIfEmpty removes the in-progress assistant message when it
// never received content (cancelled or failed before the first token), so a
// dead turn leaves no dangling thinking message behind.
func (m *ChatModel) dropPlaceholderIfEmpty() {
	if msg := m.streamingAssistant(); msg != nil && strings.TrimSpace(msg.Content) == "" {
		for i := range m.messages {
			if m.messages[i].ID == m.currentStreamingAssistantID {
				m.messages = append(m.messages[:i], m.messages[i+1:]...)
				break
			}
		}
	}
	m.currentStreamingAssistantIdx = -1
	m.currentStreamingAssistantID = ""
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
	if msg := m.streamingAssistant(); msg != nil {
		msg.Content = content
		msg.ResponseTime = m.elapsed
		msg.StreamedChunks = m.chunkCount
		msg.Thinking = true
	} else {
		id := fmt.Sprintf("assistant-%d", time.Now().UnixNano())
		m.messages = append(m.messages, ChatMessage{
			ID:             id,
			Role:           "assistant",
			Content:        content,
			Timestamp:      time.Now(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
			Thinking:       true,
		})
		m.currentStreamingAssistantID = id
		m.currentStreamingAssistantIdx = len(m.messages) - 1
	}
	m.refreshViewport()
}

// finalizeStreamingMessage finalizes the streaming message for the current turn.
// The lookup goes by stable ID: a mid-turn prepend (provider probe,
// auto-save notice) shifts every index, and an index-based finalize used to
// write the reply into the wrong message - the user's bubble stayed empty
// while a system note absorbed the content.
func (m *ChatModel) finalizeStreamingMessage(content string) {
	if msg := m.streamingAssistant(); msg != nil {
		msg.Content = content
		msg.Timestamp = time.Now()
		msg.ResponseTime = m.elapsed
		msg.StreamedChunks = m.chunkCount
		msg.Thinking = false
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:           "assistant",
			Content:        content,
			Timestamp:      time.Now(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
			Thinking:       false,
		})
	}
	m.currentStreamingAssistantIdx = -1
	m.currentStreamingAssistantID = ""
	m.refreshViewport()
}

// refreshViewport refreshes the viewport content.
