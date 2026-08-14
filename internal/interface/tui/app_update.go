package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Defensive: recover from any panic during update to prevent total app crash
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] App.Update: %v\n", r)
		}
	}()

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// -------------------------------------------------------------------------
	// Global keys (dispatched in app_keys.go)
	// -------------------------------------------------------------------------
	case tea.KeyMsg:
		var cmd tea.Cmd
		var handled bool
		a, cmd, handled = a.handleKeys(msg)
		if handled {
			return a, cmd
		}
		cmds = append(cmds, cmd)

	// -------------------------------------------------------------------------
	// Window resize (propagated in app_keys.go)
	// -------------------------------------------------------------------------
	case tea.WindowSizeMsg:
		return a.resize(msg.Width, msg.Height)

	// -------------------------------------------------------------------------
	// Status messages
	// -------------------------------------------------------------------------
	case StatusMsg:
		a.statusMessage = msg.Text
		a.statusType = msg.Type
		// Continue listening for more messages
		cmds = append(cmds, a.listenForMessages())
		// Return early - status is handled at app level
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// User submission - handled synchronously so mutations are captured
	// -------------------------------------------------------------------------
	case UserSubmitMsg:
		if a.onUserSubmit != nil {
			a.onUserSubmit(msg.Text, &a)
		}

	// -------------------------------------------------------------------------
	// View switching request
	// -------------------------------------------------------------------------
	case SwitchViewMsg:
		return a, a.switchView(msg.View)

	// -------------------------------------------------------------------------
	// User command - handled synchronously so mutations are captured
	// -------------------------------------------------------------------------
	case UserCommandMsg:
		if msg.Command == "/settings" {
			a.switchView(viewSettings)
			a.chatModel.AddMessage("system", "Switched to Settings tab.")
		}
		if a.onUserCommand != nil {
			a.onUserCommand(msg.Command, &a)
		}

	// -------------------------------------------------------------------------
	// Streaming messages from agent loop - forward to chat
	// These are handled HERE ONLY to avoid double-processing
	// -------------------------------------------------------------------------
	case StreamStartMsg, StreamChunkMsg, StreamMessageMsg, StreamErrorMsg, StreamDoneMsg,
		AgentStartMsg, AgentChunkMsg, AgentToolStartMsg, AgentToolDoneMsg, AgentDoneMsg, AgentErrorMsg, AgentConnectingMsg:
		if chatModel, cmd := a.chatModel.Update(msg); chatModel != nil {
			if m, ok := chatModel.(ChatModel); ok {
				a.chatModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		// Continue listening for more messages
		cmds = append(cmds, a.listenForMessages())
		// Return early - do NOT delegate to active view (would cause duplicates)
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Command palette open request
	// -------------------------------------------------------------------------
	case openCommandPaletteMsg:
		a.commandPalette.Open(a.width, a.height)
		return a, nil

	// -------------------------------------------------------------------------
	// Model picker open request
	// -------------------------------------------------------------------------
	case openModelPickerMsg:
		a.modelPicker.Open(a.width, a.height)
		return a, nil

	// -------------------------------------------------------------------------
	// Git context collected after boot - the dashboard and welcome fill
	// in when it lands instead of blocking the TUI start.
	// -------------------------------------------------------------------------
	case GitContextMsg:
		if a.onGitContext != nil {
			a.onGitContext(msg.Context, &a)
		}
		return a, nil

	// -------------------------------------------------------------------------
	// Quit request
	// -------------------------------------------------------------------------
	case QuitMsg:
		return a, tea.Quit

	// -------------------------------------------------------------------------
	// Model changed - update chat model AND settings so all tabs reflect new model
	// -------------------------------------------------------------------------
	case ModelChangedMsg:
		a.chatModel.SetModel(msg.Model)
		a.settingsModel.UpdateSettingValue("model", msg.Model)
		a.syncComposerContext()
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Clear chat request - handle globally so it works from any view
	// -------------------------------------------------------------------------
	case ClearChatMsg:
		if chatModel, cmd := a.chatModel.Update(msg); chatModel != nil {
			if m, ok := chatModel.(ChatModel); ok {
				a.chatModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		a.syncComposerContext()
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Approval request - show the approval dialog
	// -------------------------------------------------------------------------
	case ApprovalRequestMsg:
		a.approvalDialog.Show(msg.Request)
		return a, nil

	// -------------------------------------------------------------------------
	// Tool executing notification - show in chat
	// -------------------------------------------------------------------------
	case ToolExecutingMsg:
		// Add or update tool message with running status
		a.chatModel.AddOrUpdateToolMessage(msg.ToolID, msg.ToolName, getToolDisplayName(msg.ToolName),
			msg.Command, ToolStatusRunning)
		return a, nil

	// -------------------------------------------------------------------------
	// Agent cancellation - handle cancel signal
	// -------------------------------------------------------------------------
	case AgentCancelMsg:
		if chatModel, cmd := a.chatModel.Update(msg); chatModel != nil {
			if m, ok := chatModel.(ChatModel); ok {
				a.chatModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case SessionActivatedMsg:
		a.chatModel.SetMessages(msg.Transcript)
		a.chatModel.SetModel(msg.Model)
		a.chatModel.SetPersona(msg.Persona)
		a.syncComposerContext()
		a.settingsModel.UpdateSettingValue("model", msg.Model)
		a.homeModel.SetStatus(msg.Model, msg.PermissionMode, msg.Persona, msg.EstTokens)
		a.homeModel.SetSessions(msg.Sessions)
		a.sessionsModel.SetSessions(msg.Sessions)
		if msg.Notice != "" {
			// Session notices belong in the conversation pane (first
			// message, under the chat header) and in the Settings system
			// log, not in the footer.
			a.logSystemMessage(msg.Notice)
		}
		if msg.SwitchToChat {
			cmds = append(cmds, a.switchView(viewChat))
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	case SessionsRefreshedMsg:
		a.homeModel.SetSessions(msg.Sessions)
		a.sessionsModel.SetSessions(msg.Sessions)
		if msg.Notice != "" {
			// Same as above: durable system message, not footer clutter.
			a.logSystemMessage(msg.Notice)
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	case ProviderReadinessMsg:
		a.providerReadiness = msg.Readiness
		a.providerReadinessMsg = msg.Message
		// Every readiness state is a durable system message: it lands
		// exactly once at the top of the chat pane and in the Settings
		// page's System Messages section. Nothing provider-related ever
		// clutters the footer.
		switch msg.Readiness {
		case 1: // ProviderReady
			a.logSystemMessage(fmt.Sprintf("Provider ready: %s", msg.Message))
		case 2: // ProviderWarning
			a.logSystemMessage(fmt.Sprintf("Provider warning: %s", msg.Message))
		case 3: // ProviderUnavailable
			a.logSystemMessage(fmt.Sprintf("Provider unavailable: %s", msg.Message))
		case 4: // ProviderMisconfigured
			a.logSystemMessage(fmt.Sprintf("Provider misconfigured: %s", msg.Message))
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)
	}

	// -------------------------------------------------------------------------
	// Delegate to active view (non-streaming messages only)
	// -------------------------------------------------------------------------
	var cmd tea.Cmd
	switch a.activeView {
	case viewHome:
		model, c := a.homeModel.Update(msg)
		if m, ok := model.(*HomeModel); ok {
			a.homeModel = m
		}
		cmd = c
	case viewChat:
		if model, c := a.chatModel.Update(msg); model != nil {
			if m, ok := model.(ChatModel); ok {
				a.chatModel = m
			}
			cmd = c
		}
	case viewSessions:
		if model, c := a.sessionsModel.Update(msg); model != nil {
			if m, ok := model.(SessionsModel); ok {
				a.sessionsModel = m
			}
			cmd = c
		}
	case viewSettings:
		if model, c := a.settingsModel.Update(msg); model != nil {
			if m, ok := model.(SettingsModel); ok {
				a.settingsModel = m
			}
			cmd = c
		}
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View renders the TUI.
