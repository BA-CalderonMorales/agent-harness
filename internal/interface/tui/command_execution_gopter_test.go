package tui

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestTUICommandExecutionProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Any slash command submitted in ChatModel ALWAYS emits a UserCommandMsg
	properties.Property("ChatModel slash command submission always emits UserCommandMsg", prop.ForAll(
		func(cmdName string) bool {
			if cmdName == "" {
				return true
			}
			chat := NewChatModel()
			SubmitDebounceDuration = 0 // immediate submit
			chat.SetInput("/" + cmdName)
			model, cmd := chat.doSubmit()

			// Check input cleared
			if model.GetInput() != "" {
				return false
			}

			// Check tea.Cmd returns UserCommandMsg
			if cmd == nil {
				return false
			}
			msg := cmd()
			userCmd, ok := msg.(UserCommandMsg)
			if !ok {
				return false
			}
			return userCmd.Command == "/"+cmdName
		},
		gen.AlphaString(),
	))

	// Property 2: App.Update processing UserCommandMsg preserves system outputs in chat model
	properties.Property("App.Update UserCommandMsg preserves command output in chat transcript", prop.ForAll(
		func(cmdName string, response string) bool {
			if cmdName == "" || response == "" {
				return true
			}
			app := NewApp()
			app.SetUserCommandHandler(func(c string, a *App) {
				a.AddMessage("system", response)
			})

			// Dispatch UserCommandMsg
			model, _ := app.Update(UserCommandMsg{Command: "/" + cmdName})
			updatedApp := model.(App)

			msgs := updatedApp.chatModel.messages
			if len(msgs) == 0 {
				return false
			}

			// Verify system message exists with the expected response
			lastMsg := msgs[len(msgs)-1]
			return lastMsg.Role == "system" && lastMsg.Content == response
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property 3: Command palette selection for no-arg commands emits UserCommandMsg
	properties.Property("Command palette no-arg selection emits UserCommandMsg", prop.ForAll(
		func(cmdName string) bool {
			if cmdName == "" {
				return true
			}
			app := NewApp()
			cmdInfo := CommandInfo{
				Command:     "/" + cmdName,
				Args:        "",
				Description: "test desc",
				Category:    "Test",
			}

			_, cmd := app.handlePaletteSelection(&cmdInfo)
			if cmd == nil {
				return false
			}

			msg := cmd()
			userCmd, ok := msg.(UserCommandMsg)
			if !ok {
				return false
			}
			return userCmd.Command == "/"+cmdName
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
