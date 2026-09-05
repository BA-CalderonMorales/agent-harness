package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// phoneScrollModel builds a chat whose content overflows the
// viewport and records the viewport height so the scroll step
// can be asserted against the tuned phone value.
func phoneScrollModel(t *testing.T, width int) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.resize(width, 20)
	for i := 0; i < 30; i++ {
		m.AddMessage("user", strings.Repeat("line ", 5))
		m.AddMessage("assistant", strings.Repeat("reply ", 5))
	}
	m.refreshViewportWithFollow(true)
	return m
}

// TestPhoneScrollStep: on a phone pane the wheel scrolls by a
// viewport fraction instead of the fixed 3-line desktop tick —
// taller rows make a fixed tick crawl.
func TestPhoneScrollStep(t *testing.T) {
	m := phoneScrollModel(t, 60) // phone
	m.viewport.GotoTop()
	step := m.viewport.Height / 3
	if step < 1 {
		step = 1
	}

	down := tea.MouseMsg(tea.MouseEvent{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m2, _ := m.Update(down)
	m = m2.(ChatModel)
	if got := m.viewport.YOffset; got != step {
		t.Fatalf("phone wheel down = %d, want %d", got, step)
	}

	up := tea.MouseMsg(tea.MouseEvent{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m2, _ = m.Update(up)
	m = m2.(ChatModel)
	if got := m.viewport.YOffset; got != 0 {
		t.Fatalf("phone wheel up = %d, want 0", got)
	}
}

// TestDesktopScrollStep: desktop keeps the viewport's fixed
// 3-line tick (MouseWheelDelta = 3) — no phone tuning.
func TestDesktopScrollStep(t *testing.T) {
	m := phoneScrollModel(t, 120) // desktop
	m.viewport.GotoTop()
	down := tea.MouseMsg(tea.MouseEvent{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m2, _ := m.Update(down)
	m = m2.(ChatModel)
	if got := m.viewport.YOffset; got != 3 {
		t.Fatalf("desktop wheel down = %d, want 3", got)
	}
}
