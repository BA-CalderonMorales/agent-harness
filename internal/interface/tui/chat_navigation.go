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

// Scroll scrolls the viewport. Pending rebuilds flush first: scrolling
// must operate on the content the user actually sees.
func (m *ChatModel) Scroll(lines int) {
	m.flushDeferredRefresh()
	if lines > 0 {
		m.viewport.ScrollDown(lines)
	} else {
		m.viewport.ScrollUp(-lines)
	}
}

// GotoTop scrolls to top.
func (m *ChatModel) GotoTop() {
	m.flushDeferredRefresh()
	m.viewport.GotoTop()
}

// GotoBottom scrolls to bottom.
func (m *ChatModel) GotoBottom() {
	m.flushDeferredRefresh()
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

// refreshDeferred marks the transcript dirty and defers the rebuild to
// the next View() — the frame coalesces all pending refreshes into
// one. Mutators that fire many times per frame (per-chunk stream
// updates, session load appends) must use this path; the O(transcript)
// assembly runs at most once per frame instead of once per mutation.
func (m *ChatModel) refreshDeferred() {
	m.refreshPending = true
}

// flushDeferredRefresh performs the deferred rebuild if one is pending.
// Follow semantics (wasAtBottom → follow, scrolled-up → preserve offset)
// are the viewport's own job; forcing the bottom here would yank a
// scrolled-up user back down on the next chunk or timer tick, making
// scroll-back impossible while a turn is live.
func (m *ChatModel) flushDeferredRefresh() {
	if !m.refreshPending {
		return
	}
	m.refreshPending = false
	m.refreshViewportWithFollow(false)
}

func (m *ChatModel) refreshViewportWithFollow(forceBottom bool) {
	// Any immediate rebuild supersedes a pending deferred one.
	m.refreshPending = false
	wasAtBottom := m.viewport.AtBottom()
	previousOffset := m.viewport.YOffset

	// The transcript (m.messages) is the single render source: tool
	// messages live there in order (running in place, finalized in
	// place), so the completedToolMsgs/currentToolMsg duplicates are
	// not rendered - they used to double every tool line. Collapsed
	// runs merge contiguous finalized same-tool messages per turn.
	//
	// Structural prefix reuse (goal 0.3.29 Task 5): the group cache
	// hands back immutable strings, so a frame's block list shares
	// almost all of its entries with the previous frame's. Instead of
	// re-joining ~40KB of unchanged bytes every frame (292µs measured),
	// find the longest common block prefix with lastBlocks and
	// concatenate prevPainted[:prevPrefixLen] + the new tail. Only a
	// mid-transcript change (a fold, an expansion) forces the full
	// re-join, and those are rare, user-paced events.
	m.clickIndex = m.clickIndex[:0]
	line := 0
	blocks := m.blockScratch[:0]
	for i := 0; i < len(m.messages); {
		blockStart := line
		rendered, next, clicks := m.appendTurnGroupCached(m.messages, i)
		blocks = append(blocks, rendered)
		for _, cr := range clicks {
			m.clickIndex = append(m.clickIndex, clickRange{
				start: blockStart + cr.start,
				end:   blockStart + cr.start + cr.lines - 1,
				msgID: cr.msgID,
			})
		}
		line += strings.Count(rendered, "\n") + 2 // block + "\n\n" separator
		i = next
	}
	m.blockScratch = blocks

	// Longest common prefix with the previous frame's blocks.
	common := 0
	for common < len(blocks) && common < len(m.lastBlocks) && blocks[common] == m.lastBlocks[common] {
		common++
	}
	// Byte length of the common prefix as previously painted (each
	// block was followed by "\n\n").
	prevPrefixLen := 0
	for k := 0; k < common; k++ {
		prevPrefixLen += len(m.lastBlocks[k]) + 2
	}

	var painted string
	if common > 0 && prevPrefixLen <= len(m.lastPainted) {
		// Reuse: the unchanged head bytes are literally the same string
		// content; join only the changed tail.
		var tail strings.Builder
		for k := common; k < len(blocks); k++ {
			tail.WriteString(blocks[k])
			tail.WriteString("\n\n")
		}
		painted = m.lastPainted[:prevPrefixLen] + tail.String()
	} else {
		var content strings.Builder
		for _, blk := range blocks {
			content.WriteString(blk)
			content.WriteString("\n\n")
		}
		painted = content.String()
	}
	m.lastBlocks = append(m.lastBlocks[:0], blocks...)
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
	lines := strings.Count(painted, "\n") + 1
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
