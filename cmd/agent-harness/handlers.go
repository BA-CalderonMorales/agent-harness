package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"time"
)

// handleUserSubmit processes user message submission.
func (app *App) handleUserSubmit(text string, tuiApp *tui.App) {
	validator := ui.NewTermuxValidator()
	normalizedInput, valid := validator.ValidateInput(text)
	if !valid {
		tuiApp.Send(tui.AgentErrorMsg{Error: fmt.Errorf("invalid input"), Timestamp: time.Now()})
		return
	}

	userMsg := types.Message{
		UUID:      generateUUID(),
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.TextBlock{Text: normalizedInput}},
		Timestamp: time.Now(),
	}
	app.session.AddMessage(userMsg)
	app.sessionManager.SetCurrent(app.session)
	if _, err := app.sessionManager.SaveCurrent(); err != nil {
		// Persistence must never block chatting, but the failure is
		// recorded: a turn lost to a full disk should be traceable.
		diag.Error("session.save.submit", err)
	}

	app.handleAgentLoopAsync(normalizedInput, tuiApp)
}
func (app *App) handleUserCommand(command string, tuiApp *tui.App) {
	if result, handled, err := app.cmdRegistry.Handle(command); handled {
		if err != nil {
			tuiApp.AddMessage("system", sprintf("Error: %v", err))
			return
		}
		if commands.IsQuit(result) {
			tuiApp.Send(tui.QuitMsg{})
			return
		}
		// The /settings sentinel means "the TUI already switched tabs"
		// (the literal command is intercepted in App.Update). The token
		// itself must never reach the transcript as a second message.
		if commands.IsSettings(result) {
			return
		}
		if result != "" {
			tuiApp.AddMessage("system", result)
		}
	} else {
		tuiApp.AddMessage("system", sprintf("Unknown command: %s", command))
	}
}

type tuiChatDelegate struct {
	app    *App
	tuiApp *tui.App
}
