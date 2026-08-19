package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

// Update processes messages. The receiver is a pointer so the model's
// identity survives every update: cmd-side handlers (wizard, delegates)
// hold *App references and mutate them expecting the changes to render.
// With a value receiver those mutations landed on the pre-update copy
// and vanished — the /login wizard's messages never appeared. Value
// sub-models that return fresh copies (handleKeys, resize) are copied
// back into the live model with *a = next.
func (a *App) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	// Defensive: recover from any panic during update to prevent total app
	// crash. The named returns are the point: a recovered panic otherwise
	// leaves the zero values behind, and bubbletea then dereferences a nil
	// model right after Update returns (a second, confusing crash).
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] App.Update: %v\n", r)
			model = a
			cmd = nil
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
		next, cmd, handled := a.handleKeys(msg)
		*a = next
		if handled {
			return a, cmd
		}
		cmds = append(cmds, cmd)

	// -------------------------------------------------------------------------
	// Window resize (propagated in app_keys.go)
	// -------------------------------------------------------------------------
	case tea.WindowSizeMsg:
		next, cmd := a.resize(msg.Width, msg.Height)
		*a = next
		return a, cmd

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
			a.onUserSubmit(msg.Text, a)
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
			a.onUserCommand(msg.Command, a)
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
	// Login completed - land in chat, ready to type
	// -------------------------------------------------------------------------
	case LoginCompletedMsg:
		// switchView resets the mode to normal; the insert-mode
		// assignments must come after it. The listener chain must be
		// re-armed or every later Send (agent start, chunks, probe
		// results) is dropped and the first chat dies silently.
		cmds = append(cmds, a.switchView(viewChat))
		a.mode = ModeInsert
		a.chatModel.SetModeLabel("typing")
		a.chatModel.Focus()
		cmds = append(cmds, a.listenForMessages())
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
	// Provider picker open request
	// -------------------------------------------------------------------------
	case openProviderPickerMsg:
		a.providerPicker.Open(a.width, a.height)
		return a, nil

	// -------------------------------------------------------------------------
	// Git context collected after boot - the dashboard and welcome fill
	// in when it lands instead of blocking the TUI start.
	// -------------------------------------------------------------------------
	case GitContextMsg:
		if a.onGitContext != nil {
			a.onGitContext(msg.Context, a)
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

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
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	// -------------------------------------------------------------------------
	// Tool executing notification - show in chat
	// -------------------------------------------------------------------------
	case ToolExecutingMsg:
		// Add or update tool message with running status
		a.chatModel.AddOrUpdateToolMessage(msg.ToolID, msg.ToolName, getToolDisplayName(msg.ToolName),
			msg.Command, ToolStatusRunning)
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

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
			// Durable system message (chat + settings log) plus a
			// transient notice on the sessions page the user is on.
			a.sessionsModel.SetNotice(msg.Notice, msg.NoticeType)
			a.logSystemMessage(msg.Notice)
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	case timerTickMsg:
		// P2-5: route timer tick to chat regardless of active view so
		// the elapsed timer doesn't freeze when Chat tab is not active.
		if chatModel, cmd := a.chatModel.Update(msg); chatModel != nil {
			if m, ok := chatModel.(ChatModel); ok {
				a.chatModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	case ProviderReadinessMsg:
		// A probe from a previous provider (or an older key) may finish
		// after a switch started a new probe: only the newest generation
		// may update readiness, or stale verdicts clobber the live one.
		if msg.Gen != 0 && msg.Gen < a.providerReadinessGen {
			return a, nil
		}
		a.providerReadiness = msg.Readiness
		a.providerReadinessMsg = msg.Message
		// A misconfigured probe is the setup dead end: the home banner and
		// the statusbar badge become the fix (l: login), not prose.
		a.homeModel.SetSetupRequired(msg.Readiness == 4)
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
	var cmd2 tea.Cmd
	switch a.activeView {
	case viewHome:
		model, c := a.homeModel.Update(msg)
		if m, ok := model.(*HomeModel); ok {
			a.homeModel = m
		}
		cmd2 = c
	case viewChat:
		if model, c := a.chatModel.Update(msg); model != nil {
			if m, ok := model.(ChatModel); ok {
				a.chatModel = m
			}
			cmd2 = c
		}
	case viewSessions:
		if model, c := a.sessionsModel.Update(msg); model != nil {
			if m, ok := model.(SessionsModel); ok {
				a.sessionsModel = m
			}
			cmd2 = c
		}
	case viewSettings:
		if model, c := a.settingsModel.Update(msg); model != nil {
			if m, ok := model.(SettingsModel); ok {
				a.settingsModel = m
			}
			cmd2 = c
		}
	}
	if cmd2 != nil {
		cmds = append(cmds, cmd2)
	}

	return a, tea.Batch(cmds...)
}

// View renders the TUI.
