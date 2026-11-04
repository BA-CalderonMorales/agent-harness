package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// Esc backs out one layer. A local confirmation or editor must win over
// the global "stop the running turn" action; otherwise backing out of a
// delete prompt can kill an agent turn the user was still waiting on.
func TestEscBacksOutOfLocalStateBeforeStoppingTurn(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*App)
		check func(*testing.T, *App)
	}{
		{
			name: "home delete confirmation",
			setup: func(a *App) {
				a.activeView = viewHome
				a.homeModel.SetSessions([]SessionInfo{{ID: "s1", Title: "one"}})
				a.homeModel.Focus()
				a.homeModel.deleting = 0
			},
			check: func(t *testing.T, a *App) {
				if a.homeModel.deleting != -1 {
					t.Fatalf("delete confirmation still armed: %d", a.homeModel.deleting)
				}
			},
		},
		{
			name: "settings editor",
			setup: func(a *App) {
				a.activeView = viewSettings
				a.settingsModel.settings = []Setting{{Key: "tagline", Label: "Home Tagline", Value: "old", Type: "string"}}
				a.settingsModel.Focus()
				a.settingsModel.startEditing()
			},
			check: func(t *testing.T, a *App) {
				if a.settingsModel.editing {
					t.Fatal("settings editor still open")
				}
			},
		},
		{
			name: "logs detail",
			setup: func(a *App) {
				a.activeView = viewLogs
				a.logsModel.Focus()
				a.logsModel.detail = &diag.Entry{Level: diag.LevelInfo, Site: "test", Message: "detail"}
			},
			check: func(t *testing.T, a *App) {
				if a.logsModel.DetailOpen() {
					t.Fatal("logs detail still open")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.width, app.height = 80, 24
			app.mode = ModeNormal
			tt.setup(app)

			cancelled := false
			app.SetAgentCancelFunc(func() { cancelled = true })

			app.Update(tea.KeyMsg{Type: tea.KeyEsc})

			if cancelled {
				t.Fatal("Esc backed out of a local state and also stopped the turn")
			}
			tt.check(t, app)
		})
	}
}
