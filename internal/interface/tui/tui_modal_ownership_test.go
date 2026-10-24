package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// Every overlay that owns the keyboard must keep the keystrokes: a
// character typed into a modal must never reach the composer, and a
// global chord must not fire behind it.
//
// This is the input-ownership contract, and the terminal harness is what
// makes it checkable: the composer is in insert mode, so a leak would
// show up as text in it.

func TestOverlaysOwnTheKeyboard(t *testing.T) {
	cases := []struct {
		name    string
		open    func(*App)
		visible func(*App) bool
	}{
		{
			name: "approval",
			open: func(a *App) {
				a.approvalDialog.Show(approval.NewApprovalRequest(approval.CommandInfo{
					ID: "cmd-1", ToolName: "bash", DisplayName: "bash", Command: "ls",
				}))
			},
			visible: func(a *App) bool { return a.approvalDialog.IsVisible() },
		},
		{
			name: "question",
			open: func(a *App) {
				a.questionDialog.Show(NewQuestionRequest(nil, "Proceed?", []string{"yes", "no"}))
			},
			visible: func(a *App) bool { return a.questionDialog.IsVisible() },
		},
		{
			name: "login",
			open: func(a *App) {
				a.loginDialog.Open(80, 24, StoredCredentials{})
			},
			visible: func(a *App) bool { return a.loginDialog.IsShowing() },
		},
		{
			name: "provider picker",
			open: func(a *App) {
				a.providerPicker.Open(80, 24)
			},
			visible: func(a *App) bool { return a.providerPicker.IsShowing() },
		},
		{
			name: "model picker",
			open: func(a *App) {
				a.modelPicker.Open(80, 24)
			},
			visible: func(a *App) bool { return a.modelPicker.IsShowing() },
		},
		{
			name: "command palette",
			open: func(a *App) {
				a.commandPalette.Open(80, 24)
			},
			visible: func(a *App) bool { return a.commandPalette.IsShowing() },
		},
		{
			name: "export picker",
			open: func(a *App) {
				a.exportPicker.Open(80, 24, nil)
			},
			visible: func(a *App) bool { return a.exportPicker.visible },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp()
			app.mode = ModeInsert
			app.chatModel.Focus()
			h := ttui.New(t, app)
			h.Resize(80, 24)

			tc.open(app)
			h.Resize(80, 24) // force a frame after the overlay opened

			if !tc.visible(app) {
				t.Fatalf("%s overlay is not visible after opening", tc.name)
			}

			h.Type("x")
			if got := app.chatModel.GetInput(); got != "" {
				t.Fatalf("%s overlay leaked a keystroke into the composer: %q", tc.name, got)
			}

			// A global chord must not fire behind the overlay either.
			h.Key(tea.KeyMsg{Type: tea.KeyCtrlP})
			if app.commandPalette.IsShowing() && tc.name != "command palette" {
				t.Fatalf("%s overlay let ctrl+p through to the command palette", tc.name)
			}
		})
	}
}
