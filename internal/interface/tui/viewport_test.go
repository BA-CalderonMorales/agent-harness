package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Overflow transcript: more content than the viewport holds.
func overflowChat(t *testing.T) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.width = 120
	m.height = 12
	m.resize(120, 12)
	for i := 0; i < 30; i++ {
		m.AddMessage("user", strings.Repeat("line ", 5))
	}
	m.refreshViewportWithFollow(true)
	return m
}

// Regression: letters leaked into bubbles' default viewport keymap —
// 'u' jumped half a page up and 'd' half a page down while the user was
// reading. Raw letters must never scroll; scrolling is app-owned.
func TestViewportIgnoresStrayLetters(t *testing.T) {
	chat := overflowChat(t)
	chat.viewport.GotoBottom()
	offset := chat.viewport.YOffset

	for _, key := range []string{"u", "d", "f", "b", " "} {
		model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		chat = model.(ChatModel)
		if chat.viewport.YOffset != offset {
			t.Fatalf("key %q moved the viewport: offset %d -> %d", key, offset, chat.viewport.YOffset)
		}
	}
}

// Ctrl+u/Ctrl+d are the honest half-page scroll.
func TestHalfPageScrollBindings(t *testing.T) {
	app := &App{
		width: 120, height: 32,
		activeView: viewChat,
		chatModel:  overflowChat(t),
		mode:       ModeNormal,
	}
	app.chatModel.viewport.GotoTop()
	before := app.chatModel.viewport.YOffset
	if app.chatModel.viewport.AtBottom() {
		t.Fatal("precondition: transcript must overflow")
	}

	next, _, _ := app.handleKeys(tea.KeyMsg{Type: tea.KeyCtrlD})
	app2 := next
	if app2.chatModel.viewport.YOffset <= before {
		t.Fatalf("ctrl+d did not scroll down: %d -> %d", before, app2.chatModel.viewport.YOffset)
	}

	next, _, _ = app2.handleKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	app3 := next
	if app3.chatModel.viewport.YOffset >= app2.chatModel.viewport.YOffset {
		t.Fatal("ctrl+u did not scroll up")
	}
}
