package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Touch-first input on phone panes: with mouse capture on, a tap
// becomes a click event and Termux never raises the soft keyboard —
// the dead end whose only exit was the host terminal's drawer button.
// On a mobile pane capture yields by default, taps raise the keyboard,
// and the composer answers a tap with a focus request. Desktop panes
// keep capture on and never see the notice.

func bootAt(t *testing.T, width, height int) *App {
	t.Helper()
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return app
}

// TestMobilePaneYieldsCapture: a phone pane defaults to capture off
// with the one-time touch-mode notice; a desktop pane keeps capture
// on and never notices anything.
func TestMobilePaneYieldsCapture(t *testing.T) {
	phone := bootAt(t, 60, 20)
	if phone.mouseCapture {
		t.Fatal("phone pane kept mouse capture; taps cannot raise the keyboard")
	}
	if !strings.Contains(phone.statusMessage, "touch mode") {
		t.Fatalf("phone pane never announced touch mode: %q", phone.statusMessage)
	}

	desktop := bootAt(t, 120, 32)
	if !desktop.mouseCapture {
		t.Fatal("desktop pane lost mouse capture")
	}
	if desktop.statusMessage != "" {
		t.Fatalf("desktop pane was nagged: %q", desktop.statusMessage)
	}
}

// TestToggledCaptureOutranksThePane: a chosen mode outranks the
// default — resizing must not flip capture back.
func TestToggledCaptureOutranksThePane(t *testing.T) {
	phone := bootAt(t, 60, 20)
	phone.mouseCapture = true
	phone.mouseCaptureTouched = true
	phone.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if !phone.mouseCapture {
		t.Fatal("resize clobbered a chosen capture mode")
	}

	desktop := bootAt(t, 120, 32)
	desktop.mouseCapture = false
	desktop.mouseCaptureTouched = true
	desktop.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	if desktop.mouseCapture {
		t.Fatal("resize clobbered a chosen capture mode")
	}
}

// TestTapToTypeOnComposer: a press on the composer region asks the App
// to enter insert mode — through the real dispatch, so the mode line
// tells the truth.
func TestTapToTypeOnComposer(t *testing.T) {
	app := bootAt(t, 120, 32)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app.chatModel.lastComposerTop = viewportTopOffset + 10

	press := tea.MouseMsg(tea.MouseEvent{
		X: 10, Y: app.chatModel.lastComposerTop + 1,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	_, cmd := app.Update(press)
	if cmd == nil {
		t.Fatal("composer press produced no focus request")
	}
	msg := cmd()
	if _, ok := msg.(ComposerFocusMsg); !ok {
		t.Fatalf("composer press produced %T, want ComposerFocusMsg", msg)
	}
	app.Update(msg)
	if app.mode != ModeInsert {
		t.Fatalf("tap-to-type left mode %v, want insert", app.mode)
	}

	// A press on a transcript row must NOT steal focus: taps browse.
	app2 := bootAt(t, 120, 32)
	app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	press2 := tea.MouseMsg(tea.MouseEvent{
		X: 10, Y: viewportTopOffset + 1,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	_, cmd2 := app2.Update(press2)
	if cmd2 != nil {
		if msg2 := cmd2(); isComposerFocus(msg2) {
			t.Fatal("transcript press asked to type")
		}
	}
	if app2.mode == ModeInsert {
		t.Fatal("transcript press flipped the mode to insert")
	}
}

func isComposerFocus(msg tea.Msg) bool {
	_, ok := msg.(ComposerFocusMsg)
	return ok
}

// TestSuggestionsFitAPhonePane: the dropdown never buries the
// transcript on a phone; desktop keeps its six.
func TestSuggestionsFitAPhonePane(t *testing.T) {
	phone := bootAt(t, 60, 20)
	phone.chatModel.width = 60
	if got := phone.chatModel.suggestionMaxVisible(); got != 3 {
		t.Fatalf("phone suggestions = %d, want 3", got)
	}

	desktop := bootAt(t, 120, 32)
	desktop.chatModel.width = 120
	if got := desktop.chatModel.suggestionMaxVisible(); got != 6 {
		t.Fatalf("desktop suggestions = %d, want 6", got)
	}
}
