package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Content gutters: a phone pane insets its content so text never
// presses against the device edges. Desktop panes get none — their
// layout is frozen, byte for byte.

// TestPhonePanesGetGutters: the inset is applied twice — the
// sub-models render to the inset width and the layout pads the block.
func TestPhonePanesGetGutters(t *testing.T) {
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if app.chatModel.width != 56 { // 60 - 2*2
		t.Fatalf("phone pane chat width = %d, want 56 (gutter 2)", app.chatModel.width)
	}

	app2 := NewApp()
	app2.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	if app2.chatModel.width != 38 { // 40 - 2*1
		t.Fatalf("small phone chat width = %d, want 38 (gutter 1)", app2.chatModel.width)
	}

	// The rendered content carries the gutter: the chat header row is
	// inset by two spaces on a 60-column pane.
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app.chatModel.AddMessage("system", "gutter check")
	view := app.View()
	found := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "gutter check") {
			if !strings.HasPrefix(ansi.Strip(line), "  ") {
				t.Fatalf("content not inset: %q", ansi.Strip(line))
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("notice not rendered:\n%s", view)
	}
}

// TestDesktopHasNoGutter pins the frozen side: desktop panes render
// full-width content with no padding — the layout that has always
// worked, untouched.
func TestDesktopHasNoGutter(t *testing.T) {
	if got := gutterFor(120); got != 0 {
		t.Fatalf("desktop gutter = %d, want 0", got)
	}
	if got := gutterFor(68); got != 0 {
		t.Fatalf("gutter threshold leaked to width 68: %d", got)
	}

	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	if app.chatModel.width != 120 {
		t.Fatalf("desktop chat width = %d, want the full pane", app.chatModel.width)
	}

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app.chatModel.AddMessage("system", "no gutter check")
	view := app.View()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "no gutter check") {
			if strings.HasPrefix(ansi.Strip(line), " ") {
				t.Fatalf("desktop content grew a gutter: %q", ansi.Strip(line))
			}
			return
		}
	}
	t.Fatalf("notice not rendered:\n%s", view)
}
