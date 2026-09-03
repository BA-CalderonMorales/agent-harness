package tui

import (
	"path/filepath"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) AddMessage(role, content string) {
	a.chatModel.AddMessage(role, content)
}

// ReplaceWelcomeMessage swaps the boot welcome in place when late context
// (git) arrives.
func (a *App) ReplaceWelcomeMessage(content string) {
	a.chatModel.ReplaceWelcomeMessage(content)
}

// SetInput sets the chat input text.
func (a *App) SetInput(text string) {
	a.chatModel.SetInput(text)
}

// GetInput returns the current chat input.
func (a *App) GetInput() string {
	return a.chatModel.GetInput()
}

// ClearInput clears the chat input.
func (a *App) ClearInput() {
	a.chatModel.ClearInput()
}

// SetChatMessages replaces the visible chat transcript with session messages.
func (a *App) SetChatMessages(messages []types.Message) {
	a.chatModel.SetMessages(messages)
}

// RemoveLastUserMessage removes the most recent user message from chat display.
func (a *App) RemoveLastUserMessage() {
	a.chatModel.RemoveLastUserMessage()
}

// QueueSteer adds a message to the steer queue for auto-submission after the
// current agent turn completes.
func (a *App) QueueSteer(text string) {
	a.chatModel.QueueSteer(text)
}

// SetThinking sets the thinking state.
func (a *App) SetThinking(thinking bool, text string) {
	a.chatModel.SetThinking(thinking, text)
}

// ShowStatus shows a status message.
func (a *App) ShowStatus(text string, statusType string) {
	a.statusMessage = text
	a.statusType = statusType
	a.statusGen++
}

// statusFlashCmd schedules the expiry of the current status: statuses
// are hints, not state — they clear after 3s unless replaced.
func (a App) statusFlashCmd() tea.Cmd {
	gen := a.statusGen
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return clearStatusMsg{generation: gen}
	})
}

// RefreshSessions refreshes the sessions list.
func (a *App) RefreshSessions(sessions []SessionInfo) {
	a.sessionsModel.SetSessions(sessions)
	a.homeModel.SetSessions(sessions)
}

// SetSettings sets the settings list.
func (a *App) SetSettings(settings []Setting) {
	a.settingsModel.SetSettings(settings)
}

// SetModels sets the available models for the model picker.
func (a *App) SetModels(models []ModelItem) {
	a.modelPicker.SetModels(models)
}

// SetChatModel sets the current model name for display in the status bar.
func (a *App) SetChatModel(model string) {
	a.chatModel.SetModel(model)
}

// SetRuntimeContext sets compact runtime metadata for the bottom status line.
func (a *App) SetRuntimeContext(provider, effortProfile, workspacePath string) {
	a.provider = provider
	a.effortProfile = effortProfile
	a.workspacePath = workspacePath
	if workspacePath != "" {
		a.workspaceName = filepath.Base(workspacePath)
	}
	a.chatModel.SetProvider(provider)
	a.chatModel.SetEffort(effortProfile)
}

// SetTelemetry pushes context-usage and cost numbers for the bottom bar.
func (a *App) SetTelemetry(estTokens, contextLen int, cost float64) {
	a.estTokens = estTokens
	a.contextLen = contextLen
	a.costTotal = cost
}

// syncComposerContext refreshes the chat mode line from the app-level
// runtime context so the visible mode · model · provider · effort always
// reflects the active configuration. Some paths (login wizard, message
// events) skip the client rebuild that normally pushes this state.
func (a *App) syncComposerContext() {
	a.chatModel.SetProvider(a.provider)
	a.chatModel.SetEffort(a.effortProfile)
}

// maxSystemLog caps the durable system-message log shown in settings.
const maxSystemLog = 50

// logSystemMessage records a durable system message: it lands exactly once
// as the first note of the chat conversation and is appended to the system
// log rendered at the bottom of the settings page. Consecutive duplicates
// (e.g. repeated provider probes) are suppressed.
func (a *App) logSystemMessage(text string) {
	if text == "" {
		return
	}
	// Dedupe against the chat-head note and the previous log entry.
	if len(a.chatModel.messages) > 0 && a.chatModel.messages[0].Role == "system" && a.chatModel.messages[0].Content == text {
		return
	}
	if len(a.systemLog) > 0 && a.systemLog[len(a.systemLog)-1] == text {
		return
	}

	a.chatModel.PrependSystemNote(text)
	a.systemLog = append(a.systemLog, text)
	if len(a.systemLog) > maxSystemLog {
		a.systemLog = a.systemLog[len(a.systemLog)-maxSystemLog:]
	}
	a.settingsModel.SetSystemMessages(a.systemLog)
}

// SetChatPersona sets the current persona for contextual UI behavior.
func (a *App) SetChatPersona(persona string) {
	a.chatModel.SetPersona(persona)
}

// SetProjectInfo updates the home dashboard project context.
func (a *App) SetProjectInfo(info ProjectInfo) {
	a.homeModel.SetProjectInfo(info)
}

// SetHomeStatus updates the home dashboard status line.
func (a *App) SetHomeStatus(model, permissionMode, persona string, estimatedTokens int) {
	a.homeModel.SetStatus(model, permissionMode, persona, estimatedTokens)
}

// SetCommandCompletions sets available slash commands for inline autocomplete.
func (a *App) SetCommandCompletions(commands []string) {
	a.chatModel.SetCommandCompletions(commands)
}

// SetCommands sets available slash commands for the command palette.
func (a *App) SetCommands(commands []CommandInfo) {
	a.commandPalette.SetCommands(commands)
}

// handlePaletteSelection handles a command selected from the palette.
// Commands with no arguments are executed immediately via UserCommandMsg.
// /model with no args opens the model picker.
// Everything else is inserted into the input with a trailing space.
func (a *App) handlePaletteSelection(selected *commandInfo) (App, tea.Cmd) {
	cmdName := selected.Command

	if cmdName == "/model" && selected.Args == "" {
		a.modelPicker.Open(a.width, a.height)
		return *a, nil
	}

	noArgCommands := map[string]bool{
		"/help":          true,
		"/status":        true,
		"/clear":         true,
		"/compact":       true,
		"/cost":          true,
		"/diff":          true,
		"/version":       true,
		"/config":        true,
		"/workspace":     true,
		"/quit":          true,
		"/current-model": true,
		"/reset":         true,
		"/agents":        true,
		"/skills":        true,
		"/settings":      true,
	}

	if selected.Args == "" || noArgCommands[cmdName] {
		a.chatModel.AddMessage("user", cmdName)
		return *a, func() tea.Msg {
			return UserCommandMsg{Command: cmdName}
		}
	}

	a.chatModel.SetInput(cmdName + " ")
	return *a, nil
}

// ShortenModelName returns a compact display name for a model.
// Never returns "default" - always shows something actionable or informative.
