package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The status bar teaches the reflexes that matter where you are. In
// tmux on a phone pane the hints name the touch interactions — type,
// gestures — that desktop panes never need. Desktop hints are frozen.

// TestMobileTmuxHintsSwap: a phone pane inside tmux advertises the
// on-the-go interactions, reflecting the current touch mode.
func TestMobileTmuxHintsSwap(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-0/its,1,0")

	// Touch default: capture off, gestures are the toggle's promise.
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	app.statusMessage = "" // the one-time touch-mode flash is transient
	bar := app.renderStatusBar()
	if !strings.Contains(bar, `"i" type`) {
		t.Fatalf("mobile tmux bar missing the type hint: %q", bar)
	}
	if !strings.Contains(bar, `"m" gestures`) {
		t.Fatalf("mobile tmux bar missing the gestures hint: %q", bar)
	}
	if strings.Contains(bar, `"?" help`) {
		t.Fatalf("mobile tmux bar still shows desktop hints: %q", bar)
	}

	// Gestures chosen (capture on): the toggle promises select-copy.
	app.mouseCapture = true
	app.mouseCaptureTouched = true
	bar = app.renderStatusBar()
	if !strings.Contains(bar, `"m" copy`) {
		t.Fatalf("gestures-on bar missing the copy hint: %q", bar)
	}
}

// TestDesktopHintsFrozen: outside tmux the hints are the ones desktop
// has always shown, at any width.
func TestDesktopHintsFrozen(t *testing.T) {
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	bar := app.renderStatusBar()
	if !strings.Contains(bar, `"?" help`) || !strings.Contains(bar, `"m" copy`) {
		t.Fatalf("desktop hints changed: %q", bar)
	}
	if strings.Contains(bar, `"i" type`) || strings.Contains(bar, `"m" gestures`) {
		t.Fatalf("mobile hints leaked outside tmux: %q", bar)
	}
}
