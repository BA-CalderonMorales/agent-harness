package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
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

	// Embedded-command hint: "5/theme ember" skips the command parser
	// (it doesn't start with /) and runs a paid turn on what is almost
	// certainly a mistyped command. When the text is short and embeds a
	// registered command token, say so — the turn still runs, but the
	// user learns the right way in the same breath.
	if cmd, ok := embeddedCommand(normalizedInput, app.cmdRegistry); ok {
		tuiApp.Send(tui.AgentSystemNoteMsg{
			Text: sprintf("That message contains the %s command — submit it on its own line (nothing before the /) to run it.", cmd),
		})
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

// embeddedCommand reports a registered slash command embedded in a
// short chat message that does not start with one — the signature of a
// stray keystroke in front of a command ("5/theme ember").
func embeddedCommand(text string, reg *commands.SlashRegistry) (string, bool) {
	if strings.HasPrefix(strings.TrimSpace(text), "/") || len(text) > 40 || reg == nil {
		return "", false
	}
	for _, cmd := range reg.GetCompletions() {
		if strings.Contains(text, cmd+" ") || strings.HasSuffix(text, cmd) {
			return cmd, true
		}
	}
	return "", false
}
