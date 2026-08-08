package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// initCommandsForTUI re-registers commands that need TUI integration.
func (app *App) initCommandsForTUI(tuiApp *tui.App) {
	app.cmdRegistry.Register("clear", "Clear the session history",
		commands.ClearHandler(
			func() error {
				app.session = app.session.Clear()
				app.sessionManager.SetCurrent(app.session)
				return nil
			},
			func(msg string) {
				tuiApp.Send(tui.ClearChatMsg{FollowUpMsg: msg})
			},
		))

	app.cmdRegistry.Register("steer", "Queue a message for current turn",
		commands.SteerHandler(func(msg string) {
			if msg != "" {
				tuiApp.Send(tui.StatusMsg{Text: sprintf("Queued steer message: %s", msg), Type: "info"})
			}
		}))
}

// requireGitRepo returns an error if the app is not inside a git repository.
