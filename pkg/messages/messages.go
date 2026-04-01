package messages

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// CreateUserMessage builds a user role message.
func CreateUserMessage(text string) types.Message {
	return types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.TextBlock{Text: text}},
	}
}

// CreateAssistantMessage builds an assistant role message.
func CreateAssistantMessage(text string) types.Message {
	return types.Message{
		Role:    types.RoleAssistant,
		Content: []types.ContentBlock{types.TextBlock{Text: text}},
	}
}

// CreateSystemMessage builds a system role message.
func CreateSystemMessage(text string) types.Message {
	return types.Message{
		Role:    types.RoleSystem,
		Content: []types.ContentBlock{types.TextBlock{Text: text}},
	}
}

// CreateToolResultMessage builds a tool result message.
func CreateToolResultMessage(toolUseID string, content string, isError bool) types.Message {
	return types.Message{
		Role:    types.RoleUser,
		Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: toolUseID, Content: content, IsError: isError}},
	}
}

// NormalizeMessagesForAPI strips UI-only fields and ensures validity.
func NormalizeMessagesForAPI(messages []types.Message) []types.Message {
	out := make([]types.Message, 0, len(messages))
	for _, m := range messages {
		// Skip tombstones and UI-only system messages
		if shouldSkipMessage(m) {
			continue
		}
		out = append(out, sanitizeMessage(m))
	}
	return out
}

// GetMessagesAfterCompactBoundary returns messages after the last compact boundary.
func GetMessagesAfterCompactBoundary(messages []types.Message) []types.Message {
	boundary := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if isCompactBoundary(messages[i]) {
			boundary = i
			break
		}
	}
	if boundary == -1 {
		return messages
	}
	return messages[boundary:]
}

// AppendSystemContext appends key-value context to a system prompt.
func AppendSystemContext(prompt string, ctx map[string]string) string {
	if len(ctx) == 0 {
		return prompt
	}
	var sb strings.Builder
	sb.WriteString(prompt)
	for k, v := range ctx {
		sb.WriteString(fmt.Sprintf("\n\n<%s>\n%s\n</%s>", k, v, k))
	}
	return sb.String()
}

// StripSignatureBlocks removes thinking signatures to prevent cache invalidation.
func StripSignatureBlocks(messages []types.Message) []types.Message {
	for i := range messages {
		for j, block := range messages[i].Content {
			if tb, ok := block.(types.ThinkingBlock); ok {
				tb.Signature = ""
				messages[i].Content[j] = tb
			}
		}
	}
	return messages
}

func shouldSkipMessage(m types.Message) bool {
	// Skip empty messages or pure compact markers
	if len(m.Content) == 0 {
		return true
	}
	for _, b := range m.Content {
		if tb, ok := b.(types.TextBlock); ok {
			if tb.Text != "" && tb.Text != "(compact boundary)" {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func sanitizeMessage(m types.Message) types.Message {
	// Ensure thinking blocks are not the last block
	if len(m.Content) > 0 {
		if _, ok := m.Content[len(m.Content)-1].(types.ThinkingBlock); ok {
			// Append empty text block to satisfy API rules
			m.Content = append(m.Content, types.TextBlock{Text: " "})
		}
	}
	return m
}

func isCompactBoundary(m types.Message) bool {
	if m.Role != types.RoleSystem {
		return false
	}
	for _, b := range m.Content {
		if tb, ok := b.(types.TextBlock); ok {
			if strings.Contains(tb.Text, "compacted") {
				return true
			}
		}
	}
	return false
}
