package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// overflowLines reports every rendered row wider than the pane. A row
// wider than the pane wraps: unbudgeted height, ghost chrome, horizontal
// scroll — the whole family of small-screen failures in one measurement.
func overflowLines(view string, width int) []int {
	var bad []int
	for i, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > width {
			bad = append(bad, i)
		}
	}
	return bad
}

// buildApp boots an App at the given size and drives it through
// App.Update only — sub-models are never poked directly.
func buildApp(t *testing.T, width, height int) *App {
	t.Helper()
	app := NewApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: width, Height: height})
	next, ok := model.(*App)
	if !ok {
		t.Fatalf("Update returned %T", model)
	}
	return next
}

// TestChromeRowsNeverOverflow walks every tab at the widths small
// screens actually use and asserts no rendered row exceeds the pane.
// Fixed chrome that wraps is unbudgeted height: the goal is zero,
// content rows included — the viewport clips its own content, chrome
// has no such net.
func TestChromeRowsNeverOverflow(t *testing.T) {
	longStatus := strings.Repeat("status ", 14) + "end"

	for _, width := range []int{50, 60, 70, 90} {
		for _, tab := range []viewID{viewHome, viewChat, viewSessions, viewLogs, viewSettings} {
			app := buildApp(t, width, 20)
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(fmt.Sprintf("%d", tab+1))})

			scenarios := []struct {
				name  string
				setup func(a *App)
			}{
				{"normal", func(a *App) {}},
				{"insert", func(a *App) {
					a.mode = ModeInsert
					a.focusActive()
				}},
				{"status", func(a *App) { a.ShowStatus(longStatus, "info") }},
				{"status-error", func(a *App) { a.ShowStatus(longStatus, "error") }},
			}
			for _, sc := range scenarios {
				app2 := buildApp(t, width, 20)
				app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(fmt.Sprintf("%d", tab+1))})
				sc.setup(app2)
				view := app2.View()
				if bad := overflowLines(view, width); len(bad) > 0 {
					lines := strings.Split(view, "\n")
					t.Errorf("w%d tab %d %s: %d row(s) overflow: %v; first=%q",
						width, tab, sc.name, len(bad), bad, lines[bad[0]])
				}
			}
		}
	}
}

// TestChatChromeNeverOverflowWithContent pins the chrome under a busy
// transcript: suggestions open, tool rows expanded, streaming hints —
// at 70 columns, the acceptance width.
func TestChatChromeNeverOverflowWithContent(t *testing.T) {
	for _, width := range []int{50, 60, 70} {
		app := buildApp(t, width, 20)
		app.activeView = viewChat
		app.mode = ModeInsert
		app.chatModel.Focus()

		app.chatModel.SetModel("claude-sonnet-4-5-20250929-with-a-very-long-identifier")
		app.chatModel.AddMessage("user", strings.Repeat("please summarize ", 30))
		app.chatModel.AddMessage("assistant", strings.Repeat("response body ", 40))
		app.chatModel.AddMessage("tool", strings.Repeat("tool detail ", 30))

		// Suggestions dropdown open on a slash command.
		app.chatModel.SetInput("/")
		app.chatModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

		view := app.View()
		if bad := overflowLines(view, width); len(bad) > 0 {
			lines := strings.Split(view, "\n")
			t.Errorf("w%d busy chat: %d row(s) overflow: %v; first=%q",
				width, len(bad), bad, lines[bad[0]])
		}
	}
}

// TestDialogFramesNeverOverflow covers the full-screen overlays: their
// frames replace the layout, so a frame wider than the pane wraps too.
func TestDialogFramesNeverOverflow(t *testing.T) {
	for _, width := range []int{50, 60, 70} {
		app := buildApp(t, width, 20)

		app.commandPalette.Open(width, 20)
		if bad := overflowLines(app.View(), width); len(bad) > 0 {
			t.Errorf("w%d command palette: %d row(s) overflow", width, len(bad))
		}

		app2 := buildApp(t, width, 20)
		app2.modelPicker.Open(width, 20)
		if bad := overflowLines(app2.View(), width); len(bad) > 0 {
			t.Errorf("w%d model picker: %d row(s) overflow", width, len(bad))
		}

		app3 := buildApp(t, width, 20)
		app3.loginDialog.Open(width, 20, StoredCredentials{})
		if bad := overflowLines(app3.View(), width); len(bad) > 0 {
			t.Errorf("w%d login dialog: %d row(s) overflow", width, len(bad))
		}
	}
}

// TestOverlaysTopPinOnPhonePanes: half a phone pane hides behind the
// soft keyboard — a vertically centered modal is clipped at both ends
// exactly when the user needs to read it. Overlays pin to the top on
// a phone pane; desktop keeps the centered seat.
func TestOverlaysTopPinOnPhonePanes(t *testing.T) {
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: 55, Height: 20})
	app.commandPalette.Open(55, 20)

	view := app.View()
	for row, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "─") || strings.Contains(line, "═") || ansi.StringWidth(strings.TrimSpace(line)) < 10 {
			continue
		}
		stripped := strings.TrimSpace(ansi.Strip(line))
		if stripped == "" {
			continue
		}
		if row > 10 {
			t.Fatalf("phone pane: overlay content first appears at row %d — a keyboard covers that", row)
		}
		return
	}
	t.Fatalf("phone pane: overlay rendered no content rows:\n%s", view)
}
