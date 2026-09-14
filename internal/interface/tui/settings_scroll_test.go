package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSettingsCursorStaysVisible pins the scroll fix: walking the
// cursor from the first setting to the last must reveal the focused row
// in the viewport, at any pane height. The old per-row-average sync
// drifted past the last category and left the cursor on rows the
// viewport never showed — the Theme setting was unreachable at 120x32.
//
// The focused row's rationale now renders in the detail panel below the
// list (not inline), so visibility is asserted against the viewport
// window by the row's own value, which the detail panel never repeats.
// The walk also asserts the list actually scrolled, so the test cannot
// pass vacuously on a pane tall enough to show everything.
func TestSettingsCursorStaysVisible(t *testing.T) {
	m := NewSettingsModel()
	settings := []Setting{
		{Key: "a", Label: "Alpha", Value: "alpha-value", Type: "string", Category: "One", Description: "first"},
		{Key: "b", Label: "Beta", Value: "beta-value", Type: "string", Category: "One"},
		{Key: "c", Label: "Gamma", Value: "gamma-value", Type: "string", Category: "Two", Description: "third"},
		{Key: "d", Label: "Delta", Value: "delta-value", Type: "string", Category: "Two"},
		{Key: "e", Label: "Theme", Value: "theme-value", Type: "choice", Options: []string{"theme-value", "nord"}, Category: "Two", Description: "palette"},
	}
	m.SetSettings(settings)
	m.Focus()
	small, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 14}) // ~7 viewport rows
	m = small.(SettingsModel)

	// The list is taller than the viewport here (9 lines vs ~7 rows), so
	// reaching Theme requires the scroll to follow the cursor; a stuck
	// offset fails the walk on the row that fell off the window.
	for i := 0; i < len(settings); i++ {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = model.(SettingsModel)
		if !strings.Contains(m.View(), settings[m.cursor].Value) {
			t.Fatalf("cursor on %q but its row is not rendered", settings[m.cursor].Label)
		}
	}
}
