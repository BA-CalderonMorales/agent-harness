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
	// Global keys
	// -------------------------------------------------------------------------
	case tea.KeyMsg:
		// Help toggle — only in normal mode to avoid interfering with typing
		if msg.String() == "?" && !a.showHelp && a.mode == ModeNormal {
			a.showHelp = true
			a.helpModel.Open(a.width, a.height, "")
			return a, nil
		}

		// When help is open, delegate scrolling to the help viewport
		if a.showHelp {
			switch msg.String() {
			case "?", "esc", "q":
				a.showHelp = false
				return a, nil
			}
			return a, nil
		}

		// When command palette is open, delegate to it
		if a.commandPalette.IsShowing() {
			closed, cmd := a.commandPalette.Update(msg)
			if closed {
				if selected := a.commandPalette.SelectedCommand(); selected != nil {
					return a.handlePaletteSelection(selected)
				}
			}
			return a, cmd
		}

		// When model picker is open, delegate to it
		if a.modelPicker.IsShowing() {
			closed, cmd := a.modelPicker.Update(msg)
			if closed {
				if selected := a.modelPicker.SelectedModel(); selected != nil {
					cmdText := "/model " + selected.ID
					if a.onUserCommand != nil {
						a.onUserCommand(cmdText, &a)
					}
				}
			}
			return a, cmd
		}

		// When approval dialog is open, delegate to it
		if a.approvalDialog.IsVisible() {
			dialog, cmd := a.approvalDialog.Update(msg)
			a.approvalDialog = dialog
			return a, cmd
		}

		// Global chords: palette and reasoning-effort cycle.
		switch msg.String() {
		case "ctrl+p":
			return a, func() tea.Msg { return openCommandPaletteMsg{} }
		case "ctrl+r":
			if a.onUserCommand != nil {
				a.onUserCommand("/effort", &a)
			}
			return a, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			if a.activeView == viewChat && a.chatModel.GetInput() != "" {
				a.chatModel.ClearInput()
				a.ShowStatus("Input cleared. Press Ctrl+C again to quit.", "info")
				return a, nil
			}
			return a, tea.Quit

		case tea.KeyTab:
			if !a.activeViewConsumesTab() {
				return a, a.switchView((a.activeView + 1) % viewCount)
			}

		case tea.KeyShiftTab:
			if !a.activeViewConsumesTab() {
				return a, a.switchView((a.activeView - 1 + viewCount) % viewCount)
			}

		case tea.KeyEsc:
			// If agent is running, cancel it first
			if a.agentCancelFunc != nil {
				a.CancelAgent()
				return a, func() tea.Msg {
					return AgentCancelMsg{}
				}
			}
			if !a.activeViewConsumesEsc() {
				a.mode = ModeNormal
				a.chatModel.SetModeLabel("navigate")
				a.blurActive()
				return a, nil
			}
		}

		if !a.activeViewCapturesAllKeys() {
			// Mode switching
			switch msg.String() {
			case "ctrl+n":
				a.mode = ModeNormal
				a.chatModel.SetModeLabel("navigate")
				a.blurActive()
				return a, nil
			case "i":
				if a.mode == ModeNormal {
					a.mode = ModeInsert
					a.chatModel.SetModeLabel("typing")
					a.focusActive()
					return a, nil
				}
			}

			// View switching shortcuts
			switch msg.String() {
			case "ctrl+1", "1":
				return a, a.switchView(viewHome)
			case "ctrl+2", "2":
				return a, a.switchView(viewChat)
			case "ctrl+3", "3":
				return a, a.switchView(viewSessions)
			case "ctrl+4", "4":
				return a, a.switchView(viewSettings)
			}

			// Navigation in normal mode
			if a.mode == ModeNormal {
				switch msg.String() {
				case "j", "down":
					a.scrollActiveView(1)
					return a, nil
				case "k", "up":
					a.scrollActiveView(-1)
					return a, nil
				case "g", "home":
					a.gotoActiveViewTop()
					return a, nil
				case "G", "end":
					a.gotoActiveViewBottom()
					return a, nil
				case "h":
					if a.activeView != viewSessions {
						return a, a.switchView(viewHome)
					}
				case "c":
					if a.activeView != viewSessions {
						return a, a.switchView(viewChat)
					}
				}
			}
		}

	// -------------------------------------------------------------------------
	// Window resize
	// -------------------------------------------------------------------------
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		// Reserve space for tab bar (3: padding top + content + border bottom)
		// + status bar (2: content + padding bottom) = 5 total
		reserved := 5
		contentMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: msg.Height - reserved,
		}

		// Propagate to sub-models
		homeModel, cmd := a.homeModel.Update(contentMsg)
		if m, ok := homeModel.(*HomeModel); ok {
			a.homeModel = m
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		if chatModel, cmd := a.chatModel.Update(contentMsg); chatModel != nil {
			if m, ok := chatModel.(ChatModel); ok {
				a.chatModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		if sessionsModel, cmd := a.sessionsModel.Update(contentMsg); sessionsModel != nil {
			if m, ok := sessionsModel.(SessionsModel); ok {
				a.sessionsModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		if settingsModel, cmd := a.settingsModel.Update(contentMsg); settingsModel != nil {
			if m, ok := settingsModel.(SettingsModel); ok {
				a.settingsModel = m
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		return a, tea.Batch(cmds...)

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
		a.settingsModel.UpdateSettingValue("model", msg.Model)
		a.homeModel.SetStatus(msg.Model, msg.PermissionMode, msg.Persona, msg.EstTokens)
		a.homeModel.SetSessions(msg.Sessions)
		a.sessionsModel.SetSessions(msg.Sessions)
		if msg.Notice != "" {
			a.statusMessage = msg.Notice
			a.statusType = msg.NoticeType
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
			a.statusMessage = msg.Notice
			a.statusType = msg.NoticeType
		}
		cmds = append(cmds, a.listenForMessages())
		return a, tea.Batch(cmds...)

	case ProviderReadinessMsg:
		a.providerReadiness = msg.Readiness
		a.providerReadinessMsg = msg.Message
		// Update status bar with readiness information
		switch msg.Readiness {
		case 1: // ProviderReady
			a.statusMessage = fmt.Sprintf("Provider ready: %s", msg.Message)
			a.statusType = "success"
		case 2: // ProviderWarning
			a.statusMessage = fmt.Sprintf("Provider warning: %s", msg.Message)
			a.statusType = "warning"
		case 3: // ProviderUnavailable
			a.statusMessage = fmt.Sprintf("Provider unavailable: %s", msg.Message)
			a.statusType = "error"
		case 4: // ProviderMisconfigured
			a.statusMessage = fmt.Sprintf("Provider misconfigured: %s", msg.Message)
			a.statusType = "error"
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
