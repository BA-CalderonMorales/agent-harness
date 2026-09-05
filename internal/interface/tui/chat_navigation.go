package tui

import (
	"strings"
)

// SetThinking sets the thinking state.
// When thinking is set to true, this also starts the response timer.
func (m ChatModel) ConsumesTab() bool {
	return m.showSuggestions
}

// ConsumesEsc returns whether this view consumes Esc key.
// When inline suggestions are showing, Esc dismisses them.
func (m ChatModel) ConsumesEsc() bool {
	return m.showSuggestions
}

// CapturesAllKeys returns whether this view should receive all keys
// before global shortcuts are applied.
func (m ChatModel) CapturesAllKeys() bool {
	return m.focused
}

// Scroll scrolls the viewport.
func (m *ChatModel) Scroll(lines int) {
	if lines > 0 {
		m.viewport.ScrollDown(lines)
	} else {
		m.viewport.ScrollUp(-lines)
	}
}

// GotoTop scrolls to top.
func (m *ChatModel) GotoTop() {
	m.viewport.GotoTop()
}

// GotoBottom scrolls to bottom.
func (m *ChatModel) GotoBottom() {
	m.viewport.GotoBottom()
}

// updateOrCreateStreamingMessage updates the assistant message for the current
// streaming turn or creates one. It uses currentStreamingAssistantIdx to track
// the exact message so that mid-stream system/user messages do not break the
// update target, while guaranteeing that a new user turn gets a fresh assistant
// message (fixing the history overwrite bug in Issue #4).
func (m *ChatModel) refreshViewport() {
	m.refreshViewportWithFollow(false)
}

func (m *ChatModel) refreshViewportFollow() {
	m.refreshViewportWithFollow(true)
}

func (m *ChatModel) refreshViewportWithFollow(forceBottom bool) {
	wasAtBottom := m.viewport.AtBottom()
	previousOffset := m.viewport.YOffset
	var content strings.Builder

	// The transcript (m.messages) is the single render source: tool
	// messages live there in order (running in place, finalized in
	// place), so the completedToolMsgs/currentToolMsg duplicates are
	// not rendered - they used to double every tool line. Collapsed
	// runs merge contiguous finalized same-tool messages per turn.
	m.clickIndex = m.clickIndex[:0]
	line := 0
	for i := 0; i < len(m.messages); {
		rendered, next, clicks := m.appendTurnGroupTracked(m.messages, i, m.toolsCollapsed)
		content.WriteString(rendered)
		content.WriteString("\n\n")
		for _, cr := range clicks {
			m.clickIndex = append(m.clickIndex, clickRange{
				start: line + cr.start, end: line + cr.start + cr.lines - 1,
				msgID: cr.msgID,
			})
		}
		lines := strings.Count(rendered, "\n") + 1
		line += lines + 2 // the "\n\n" separator between groups
		i = next
	}

	// Only paint when the built transcript actually differs from the
	// last painted frame: the tick-driven streaming repaints would
	// otherwise SetContent the identical string four times a second
	// even when the stream went quiet.
	painted := content.String()
	if !forceBottom && painted == m.lastPainted && wasAtBottom == m.lastPaintedAtBottom {
		return
	}
	m.lastPainted = painted
	m.lastPaintedAtBottom = wasAtBottom

	m.viewport.SetContent(painted)
	if forceBottom || wasAtBottom {
		m.viewport.GotoBottom()
		return
	}
	// The transcript can shrink (a fold, an expansion closing): a stale
	// offset past the new end makes the viewport's visibleLines slice
	// invert and panic. Clamp before restoring.
	lines := strings.Count(content.String(), "\n") + 1
	maxOffset := lines - m.viewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if previousOffset > maxOffset {
		previousOffset = maxOffset
	}
	m.viewport.SetYOffset(previousOffset)
}

// expandableMessageAtRow returns the message ID occupying the viewport
// content row, or "" when the row belongs to a message with no
// expandable record (plain text, answer bubbles).
func (m *ChatModel) expandableMessageAtRow(row int) string {
	for _, r := range m.clickIndex {
		if row >= r.start && row <= r.end {
			return r.msgID
		}
	}
	return ""
}
