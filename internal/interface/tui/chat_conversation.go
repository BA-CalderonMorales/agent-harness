package tui

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
	"time"
)

// AddMessage adds a message to the chat.
func (m *ChatModel) AddMessage(role, content string) {
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.refreshViewportFollow()
}

// PrependSystemNote inserts a system note as the first message of the
// conversation so session notices land under the chat header exactly once;
// the user can always scroll back up to it.
func (m *ChatModel) PrependSystemNote(content string) {
	note := ChatMessage{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append([]ChatMessage{note}, m.messages...)
	m.refreshViewport()
	m.refreshViewportFollow()
}

// SetMessages replaces the visible chat transcript from persisted session
// messages, preserving only user, assistant, system, and tool-result text.
func (m *ChatModel) SetMessages(messages []types.Message) {
	m.messages = make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMsg, ok := chatMessageFromSessionMessage(msg)
		if ok {
			m.messages = append(m.messages, chatMsg)
		}
	}
	m.refreshViewportFollow()
}

func chatMessageFromSessionMessage(msg types.Message) (ChatMessage, bool) {
	var content strings.Builder
	isTool := false
	toolName := ""
	status := ToolStatusComplete

	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			if b.Text != "" {
				content.WriteString(b.Text)
			}
		case types.ToolUseBlock:
			isTool = true
			toolName = b.Name
			input := fmt.Sprintf("%v", b.Input)
			if input != "" && input != "map[]" {
				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(fmt.Sprintf("→ %s %s", b.Name, input))
			} else {
				content.WriteString(fmt.Sprintf("→ %s", b.Name))
			}
		case types.ToolResultBlock:
			isTool = true
			if b.IsError {
				status = ToolStatusError
			}
			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(fmt.Sprintf("%v", b.Content))
		}
	}

	text := strings.TrimSpace(content.String())
	if text == "" {
		return ChatMessage{}, false
	}

	role := string(msg.Role)
	if isTool {
		role = "tool"
	}
	return ChatMessage{
		ID:         msg.UUID,
		Role:       role,
		Content:    text,
		Timestamp:  msg.Timestamp,
		IsTool:     isTool,
		ToolName:   toolName,
		ToolStatus: status,
	}, true
}

// AddToolMessage adds a tool message to the chat.
// If you need message replacement (for live updates), use AddOrUpdateToolMessage instead.
