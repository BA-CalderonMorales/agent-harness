package tui

import (
	"encoding/json"
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
	// Immediate: user-visible follow semantics (submit yanks to
	// bottom). The per-frame hot paths (chunks, ticks, session load)
	// defer; appends arrive at human scale.
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
	m.refreshDeferred()
}

// SetMessages replaces the visible chat transcript from persisted session
// messages, preserving only user, assistant, system, and tool-result text.
func (m *ChatModel) SetMessages(messages []types.Message) {
	m.messages = make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMsg, ok := m.chatMessageFromSessionMessage(msg)
		if ok {
			m.messages = append(m.messages, chatMsg)
		}
	}
	m.refreshViewportFollow()
}

// chatMessageFromSessionMessage maps one persisted message onto its
// chat-render form. Tool calls must carry the same display fields the
// live path populates (getToolDisplayName / extractCommandFromToolInput):
// without them the collapse machinery renders bare carets — no tool
// name, no detail, no duration — after a session reload.
func (m ChatModel) chatMessageFromSessionMessage(msg types.Message) (ChatMessage, bool) {
	var content strings.Builder
	isTool := false
	toolName := ""
	status := ToolStatusComplete
	var parts []TurnPart

	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			if b.Text != "" {
				content.WriteString(b.Text)
				parts = append(parts, TurnPart{Text: b.Text})
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
			parts = append(parts, TurnPart{ToolID: b.ID})
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
	msgOut := ChatMessage{
		ID:         msg.UUID,
		Role:       role,
		Content:    text,
		Parts:      parts,
		Timestamp:  msg.Timestamp,
		IsTool:     isTool,
		ToolName:   toolName,
		ToolStatus: status,
	}
	if isTool && toolName != "" {
		msgOut.ToolDisplayName = getToolDisplayName(toolName)
		msgOut.ToolDetail = m.extractCommandFromToolInput(toolName, toolInputForSession(msg))
		if !msg.Timestamp.IsZero() {
			msgOut.ToolStartedAt = msg.Timestamp
		}
		if input := toolInputForSession(msg); len(input) > 0 {
			if raw, err := json.Marshal(input); err == nil {
				msgOut.ToolInputJSON = string(raw)
			}
		}
	}
	return msgOut, true
}

// toolInputForSession recovers the tool's input map from the message's
// tool-use block so the session-reload path can rebuild the same detail
// string the live path showed.
func toolInputForSession(msg types.Message) map[string]any {
	for _, block := range msg.Content {
		if b, ok := block.(types.ToolUseBlock); ok {
			return b.Input
		}
	}
	return nil
}

// AddToolMessage adds a tool message to the chat.
// If you need message replacement (for live updates), use AddOrUpdateToolMessage instead.
