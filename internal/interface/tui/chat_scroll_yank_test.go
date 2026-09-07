package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// These tests pin the 0.3.28 scroll-yank regression: the 0.3.27 deferred
// refresh flushed with forceBottom=true, so any timer tick or chunk
// landing while the user was scrolled up snapped the viewport back to
// the bottom — scroll-back was impossible during a live turn.
//
// The fix flushes with follow semantics (wasAtBottom → follow,
// scrolled-up → preserve offset), restoring the pre-0.3.27 behavior of
// the refreshViewport() calls that were converted to refreshDeferred().

// TestDeferredFlushPreservesScrolledUpOffset: a timer tick while
// scrolled up must not move the viewport.
func TestDeferredFlushPreservesScrolledUpOffset(t *testing.T) {
	m := seededScrollModel(t)
	m.viewport.GotoTop()
	offset := m.viewport.YOffset
	if m.viewport.AtBottom() {
		t.Fatal("precondition: viewport must not be at the bottom")
	}

	model, _ := m.Update(timerTickMsg{})
	m2 := model.(ChatModel)

	if m2.viewport.AtBottom() {
		t.Fatal("deferred flush yanked a scrolled-up user to the bottom")
	}
	if m2.viewport.YOffset != offset {
		t.Fatalf("deferred flush moved the viewport: offset %d -> %d", offset, m2.viewport.YOffset)
	}
}

// TestDeferredFlushStillFollowsAtBottom: when the user IS at the
// bottom, a deferred flush keeps auto-following so streaming stays live.
func TestDeferredFlushStillFollowsAtBottom(t *testing.T) {
	m := seededScrollModel(t)
	m.viewport.GotoBottom()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: viewport must be at the bottom")
	}

	model, _ := m.Update(timerTickMsg{})
	m2 := model.(ChatModel)

	if !m2.viewport.AtBottom() {
		t.Fatal("at-bottom user lost auto-follow on a deferred flush")
	}
}

// TestChunkDuringScrollUpPreservesOffset: an agent chunk while scrolled
// up must not yank the user to the bottom.
func TestChunkDuringScrollUpPreservesOffset(t *testing.T) {
	m := seededScrollModel(t)
	m.streaming = true
	m.viewport.GotoTop()
	offset := m.viewport.YOffset

	model, _ := m.Update(AgentChunkMsg{Text: "more", Timestamp: time.Now()})
	m2 := model.(ChatModel)

	if m2.viewport.AtBottom() {
		t.Fatal("chunk delivery yanked a scrolled-up user to the bottom")
	}
	if m2.viewport.YOffset != offset {
		t.Fatalf("chunk delivery moved the viewport: offset %d -> %d", offset, m2.viewport.YOffset)
	}
}

// TestWheelScrollFlushesPendingRebuild: a wheel event with a pending
// deferred rebuild must scroll on fresh content, not stale content.
func TestWheelScrollFlushesPendingRebuild(t *testing.T) {
	m := seededScrollModel(t)
	m.width = 100 // desktop pane, wheel path goes through viewport.Update
	m.viewport.GotoBottom()

	// Mutate content and defer the rebuild without flushing.
	m.PrependSystemNote("pending rebuild marker")
	if !m.refreshPending {
		t.Fatal("precondition: rebuild must be pending")
	}
	before := m.viewport.Height

	wheelUp := tea.MouseMsg(tea.MouseEvent{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	model, _ := m.Update(wheelUp)
	m2 := model.(ChatModel)

	if m2.refreshPending {
		t.Fatal("wheel scroll did not flush the pending rebuild")
	}
	if m2.viewport.Height != before {
		t.Fatalf("wheel scroll changed viewport height: %d -> %d", before, m2.viewport.Height)
	}
	if m2.viewport.AtBottom() {
		t.Fatal("wheel-up left the viewport at the bottom")
	}
}
