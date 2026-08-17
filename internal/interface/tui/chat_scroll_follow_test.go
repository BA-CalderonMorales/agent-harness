package tui

import (
	"strings"
	"testing"
)

// seededScrollModel builds a chat whose content overflows the viewport,
// so scroll position is meaningful.
func seededScrollModel(t *testing.T) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.width = 120
	m.height = 12
	m.resize(120, 12)
	for i := 0; i < 30; i++ {
		m.AddMessage("user", strings.Repeat("line ", 5))
		m.AddMessage("assistant", strings.Repeat("reply ", 5))
	}
	m.refreshViewportWithFollow(true)
	return m
}

// TestBackgroundToolEventNeverYanksScrolledUpUser pins the scroll-yank
// bug: a background tool event (permission checkpoint / tool lifecycle)
// must preserve a non-bottom offset - the user reading the tool wall
// while typing must not be snapped back to the bottom mid-turn.
func TestBackgroundToolEventNeverYanksScrolledUpUser(t *testing.T) {
	m := seededScrollModel(t)
	m.viewport.GotoTop()
	offset := m.viewport.YOffset
	if m.viewport.AtBottom() {
		t.Fatal("precondition: viewport must not be at the bottom")
	}

	m.AddOrUpdateToolMessage("t1", "bash", "Shell", "ls", ToolStatusRunning)

	if m.viewport.AtBottom() {
		t.Fatal("background tool event yanked a scrolled-up user to the bottom")
	}
	if m.viewport.YOffset != offset {
		t.Fatalf("background tool event moved the viewport: offset %d -> %d", offset, m.viewport.YOffset)
	}
}

// TestBackgroundToolEventStillFollowsAtBottom: when the user IS at the
// bottom, background tool events keep auto-following so a normal turn
// still reads live.
func TestBackgroundToolEventStillFollowsAtBottom(t *testing.T) {
	m := seededScrollModel(t)
	m.viewport.GotoBottom()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: viewport must be at the bottom")
	}

	m.AddOrUpdateToolMessage("t2", "bash", "Shell", "ls", ToolStatusRunning)

	if !m.viewport.AtBottom() {
		t.Fatal("at-bottom user lost auto-follow on a background tool event")
	}
}

// TestUserSubmitStillYanksToBottom: a genuine user action (submit) keeps
// the follow behavior - handing control to the agent snaps to the live
// bottom.
func TestUserSubmitStillYanksToBottom(t *testing.T) {
	m := seededScrollModel(t)
	m.viewport.GotoTop()

	m.AddMessage("user", "keep going")

	if !m.viewport.AtBottom() {
		t.Fatal("user submit must follow to the bottom")
	}
}
