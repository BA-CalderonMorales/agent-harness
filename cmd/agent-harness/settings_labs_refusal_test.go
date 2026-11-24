package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	tea "github.com/charmbracelet/bubbletea"
)

// The Settings view flips a bool row before it asks the app. When the app
// refuses the toggle, the row must go back to the effective value instead
// of showing the flip it never applied.

func toggleLabRowThroughTUI(t *testing.T, app *App) bool {
	t.Helper()
	app.tuiApp.SetSettingsDelegate(delegateFor(t, app))
	app.tuiApp.SetSettings(app.getSettings())
	app.tuiApp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	selectSettingsRow(app.tuiApp, config.LabLoopTrack)
	app.tuiApp.Update(tea.KeyMsg{Type: tea.KeySpace})
	flushSettingsMessages(app.tuiApp)
	for _, row := range settingsRows(app.tuiApp) {
		if row.Key == config.LabLoopTrack {
			return row.BoolValue
		}
	}
	t.Fatal("Labs row missing")
	return false
}

func TestRefusedLocalLabToggleShowsRealValue(t *testing.T) {
	journeyHome(t)
	if err := os.MkdirAll(".agent-harness", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".agent-harness", "settings.local.json"), []byte(`{"loop_track": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := bootJourneyApp(t)

	if shown := toggleLabRowThroughTUI(t, app); !shown || !app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatalf("after a refused toggle the row shows %v, want the effective value true", shown)
	}
}

func TestRefusedPinnedLabToggleShowsRealValue(t *testing.T) {
	journeyHome(t)
	t.Setenv("AH_LOOP_TRACK", "1")
	app := bootJourneyApp(t)

	if shown := toggleLabRowThroughTUI(t, app); !shown || !app.config.LabEnabled(config.LabLoopTrack) {
		t.Fatalf("after a refused toggle the row shows %v, want the pinned value true", shown)
	}
}
