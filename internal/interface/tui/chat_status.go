package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	m.turnTools = nil
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

// thinkingHintThreshold is how long the first token may stay pending before
// the UI assumes a slow local model and shows the explanatory progress line.
const thinkingHintThreshold = 5 * time.Second

// thinkingQuips is the rotating verb behind the thinking badge. One
// slice, one clock — the whole personality of the wait state, and
// deliberately nothing more.
var thinkingQuips = []string{"thinking", "pondering", "brewing", "scheming", "conjuring"}

// thinkingBadge renders the live wait state: a twinkling star that
// hue-shifts green↔blue, a rotating quip in bright info color, and a
// cycling dot trail. tick steps at 250ms from the header clock.
func (m ChatModel) thinkingBadge(tick int) string {
	glyphs := []string{"✦", "✧"}
	styles := []lipgloss.Style{SuccessStyle, InfoStyle}
	glyph := styles[tick%len(styles)].Render(glyphs[tick%len(glyphs)])
	quip := thinkingQuips[tick/8%len(thinkingQuips)] // full cycle ≈ 2s per word
	dots := strings.Repeat("·", 1+tick%3)
	return glyph + InfoStyle.Render(fmt.Sprintf(" %s", quip)) + HelpDimStyle.Render(dots)
}

// thinkingHint returns the prompt-eval progress line once the first token
// has been pending long enough that a slow local model (CPU prompt eval)
// is the likely explanation. Before the threshold, and once streaming has
// started, the header's live elapsed clock is enough.
func thinkingHint(elapsed time.Duration) string {
	if elapsed < thinkingHintThreshold {
		return ""
	}
	return "still thinking — first token can take minutes on CPU models"
}

// reasoningPreviewTruncate is the visible tail length of the live
// reasoning stream. The head of a reasoning trace is stale by the time
// it renders; the tail is where the model is now.
const reasoningPreviewTruncate = 100

// reasoningPreview renders the tail of the live reasoning stream for the
// wait state. Empty when the model sent no reasoning (or the default
// "Thinking..." placeholder is still in place).
func reasoningPreview(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || text == "Thinking..." {
		return ""
	}
	// Single line: reasoning paragraphs collapse for the preview.
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > reasoningPreviewTruncate {
		text = "…" + text[len(text)-reasoningPreviewTruncate:]
	}
	return text
}

// ConsumesTab returns whether this view consumes Tab key.
// When inline suggestions are showing, Tab is used for auto-completion.
//
// No repaint here: chunk-sized updates land dozens of times a second,
// and each full-transcript render (markdown included) is the streaming
// flicker. The turn timer's tick repaints everything accumulated — 4
// frames a second is smooth; a render per token is a strobe.
// currentReasoningText is the reasoning safe to persist on a message:
// empty while the thinking text is loop status ("Thinking...",
// "Connecting to …") — status drives the badge, never the record.
func (m ChatModel) currentReasoningText() string {
	if m.thinkingIsStatus {
		return ""
	}
	return m.thinkingText
}

func (m *ChatModel) updateOrCreateStreamingMessage(content string) {
	parts := m.deriveParts(content)
	if msg := m.streamingAssistant(); msg != nil {
		msg.Content = content
		msg.Parts = parts
		msg.ReasoningText = m.currentReasoningText()
		msg.ResponseTime = m.elapsed
		msg.StreamedChunks = m.chunkCount
		msg.Thinking = true
	} else {
		id := fmt.Sprintf("assistant-%d", time.Now().UnixNano())
		m.messages = append(m.messages, ChatMessage{
			ID:             id,
			Role:           "assistant",
			Content:        content,
			Parts:          parts,
			Timestamp:      time.Now(),
			ReasoningText:  m.currentReasoningText(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
			Thinking:       true,
		})
		m.currentStreamingAssistantID = id
		m.currentStreamingAssistantIdx = len(m.messages) - 1
	}
}

// deriveParts splits the turn's stream buffer at the tool marks:
// prose runs alternate with the calls that interrupted them.
func (m *ChatModel) deriveParts(buffer string) []TurnPart {
	if len(m.turnTools) == 0 {
		return nil
	}
	parts := make([]TurnPart, 0, len(m.turnTools)+1)
	prev := 0
	for _, mark := range m.turnTools {
		at := mark.At
		if at > len(buffer) {
			at = len(buffer)
		}
		if at > prev {
			parts = append(parts, TurnPart{Text: buffer[prev:at]})
		}
		parts = append(parts, TurnPart{ToolID: mark.ToolID})
		prev = at
	}
	if prev < len(buffer) {
		parts = append(parts, TurnPart{Text: buffer[prev:]})
	}
	return parts
}

// finalizeStreamingMessage finalizes the streaming message for the current turn.
// The lookup goes by stable ID: a mid-turn prepend (provider probe,
// auto-save notice) shifts every index, and an index-based finalize used to
// write the reply into the wrong message - the user's bubble stayed empty
// while a system note absorbed the content.
func (m *ChatModel) finalizeStreamingMessage(content string) {
	parts := m.deriveParts(content)
	if msg := m.streamingAssistant(); msg != nil {
		msg.Content = content
		msg.Parts = parts
		msg.ReasoningText = m.currentReasoningText()
		msg.Timestamp = time.Now()
		msg.ResponseTime = m.elapsed
		msg.StreamedChunks = m.chunkCount
		msg.Thinking = false
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:           "assistant",
			Content:        content,
			Parts:          parts,
			Timestamp:      time.Now(),
			ReasoningText:  m.currentReasoningText(),
			ResponseTime:   m.elapsed,
			StreamedChunks: m.chunkCount,
			Thinking:       false,
		})
	}
	m.turnTools = nil
	m.currentStreamingAssistantIdx = -1
	m.currentStreamingAssistantID = ""
	m.turnTools = nil
	m.refreshViewport()
}

// refreshViewport refreshes the viewport content.
