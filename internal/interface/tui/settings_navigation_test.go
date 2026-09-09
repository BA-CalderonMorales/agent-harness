package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestSettingsArrowNavigationWrapsInApp(t *testing.T) {
	app := NewApp()
	app.SetSettings([]Setting{{Key: "first", Label: "First"}, {Key: "last", Label: "Last"}})
	app.activeView = viewSettings
	app.settingsModel.Focus()
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyUp})
	app = updated.(*App)
	if app.settingsModel.cursor != 1 {
		t.Fatalf("up from first: cursor = %d, want last", app.settingsModel.cursor)
	}
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = updated.(*App)
	if app.settingsModel.cursor != 0 {
		t.Fatalf("down from last: cursor = %d, want first", app.settingsModel.cursor)
	}
}

func TestSettingsDirectArrowNavigationWraps(t *testing.T) {
	m := NewSettingsModel()
	m.SetSettings([]Setting{{Key: "first"}, {Key: "last"}})
	m.Focus()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(SettingsModel)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want last", m.cursor)
	}
	m.Scroll(1)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want first", m.cursor)
	}
	empty := NewSettingsModel()
	empty.Scroll(-1)
	if empty.cursor != 0 {
		t.Fatalf("empty cursor = %d", empty.cursor)
	}
}
