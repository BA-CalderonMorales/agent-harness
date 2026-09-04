package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSettingsCursorStaysVisible pins the scroll fix: walking the
// cursor to the last setting must reveal its row in the viewport, at
// any pane height. The old per-row-average sync drifted past the last
// category and left the cursor on rows the viewport never showed —
// the Theme setting was unreachable at 120x32.
func TestSettingsCursorStaysVisible(t *testing.T) {
	m := NewSettingsModel()
	settings := []Setting{
		{Key: "a", Label: "Alpha", Value: "1", Type: "string", Category: "One", Description: "first"},
		{Key: "b", Label: "Beta", Value: "2", Type: "string", Category: "One"},
		{Key: "c", Label: "Gamma", Value: "3", Type: "string", Category: "Two", Description: "third"},
		{Key: "d", Label: "Delta", Value: "4", Type: "string", Category: "Two"},
		{Key: "e", Label: "Theme", Value: "default", Type: "choice", Options: []string{"default", "nord"}, Category: "Two", Description: "palette"},
	}
	m.SetSettings(settings)
	m.Focus()
	small, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 14}) // ~9 viewport rows
	m = small.(SettingsModel)

	for i := 0; i < len(settings); i++ {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = model.(SettingsModel)
		view := m.View()
		if !strings.Contains(view, settings[i].Label) {
			t.Fatalf("cursor on %q but its row is not rendered (viewport stuck at offset %d)", settings[i].Label, m.viewport.YOffset)
		}
	}
}
