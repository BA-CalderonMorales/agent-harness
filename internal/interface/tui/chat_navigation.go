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
	for i := 0; i < len(m.messages); {
		i = m.appendCollapsedMessage(&content, m.messages, i, m.toolsCollapsed)
	}

	m.viewport.SetContent(content.String())
	if forceBottom || wasAtBottom {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetYOffset(previousOffset)
}
