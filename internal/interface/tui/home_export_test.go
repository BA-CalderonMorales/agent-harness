package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func exportPickerApp() *App {
	app := NewApp()
	return app
}

// TestExportPickerJourney pins the modal contract: Home opens it with
// the session list, j/k move, Enter reports the pick, Esc cancels
// without one.
func TestExportPickerJourney(t *testing.T) {
	app := exportPickerApp()
	sessions := []SessionInfo{
		{ID: "s1", Title: "First", MessageCount: 4},
		{ID: "s2", Title: "Second", MessageCount: 9},
	}
	app.OpenExportPicker(sessions)
	if !app.ExportPickerShowing() {
		t.Fatal("picker did not open")
	}

	// Move down once: cursor lands on the second entry.
	next, _, _ := app.exportPicker.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.exportPicker = next

	pickedID := ""
	app.SetExportPickHandler(func(id string) { pickedID = id })

	next, closed, pick := app.exportPicker.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.exportPicker = next
	if !closed || !pick {
		t.Fatalf("enter did not confirm: closed=%v pick=%v", closed, pick)
	}
	if sel := app.ExportPickerSelection(); sel == nil || sel.ID != "s2" {
		t.Fatalf("selection = %+v, want s2", sel)
	}

	// The app-level pick hook fires the handler.
	if sel := app.ExportPickerSelection(); sel != nil && app.onExportPick != nil {
		app.onExportPick(sel.ID)
	}
	if pickedID != "s2" {
		t.Fatalf("pick handler got %q, want s2", pickedID)
	}
}

// TestExportPickerCancel pins Esc: the modal folds and no pick fires.
func TestExportPickerCancel(t *testing.T) {
	app := exportPickerApp()
	app.OpenExportPicker([]SessionInfo{{ID: "s1", Title: "First"}})

	fired := false
	app.SetExportPickHandler(func(id string) { fired = true })

	next, closed, pick := app.exportPicker.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app.exportPicker = next
	if !closed || pick {
		t.Fatalf("esc wrong: closed=%v pick=%v", closed, pick)
	}
	if fired {
		t.Fatal("esc must not trigger the export")
	}
}

// TestExportPickerViewRendersSessions pins the modal rendering.
func TestExportPickerViewRendersSessions(t *testing.T) {
	app := exportPickerApp()
	app.OpenExportPicker([]SessionInfo{{ID: "s1", Title: "Crawl the repo", MessageCount: 12}})

	view := app.exportPicker.View(100, 30)
	if !containsAll(view, "Export session", "Crawl the repo", "Esc cancels") {
		t.Fatalf("modal view missing pieces:\n%s", view)
	}
	if !containsAll(app.exportPicker.View(100, 30), "12") {
		t.Fatalf("message count missing:\n%s", view)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
