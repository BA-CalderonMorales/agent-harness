package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeys processes global keys before view dispatch. It returns
// the possibly-mutated App, the accumulated command, and whether the
// key was fully consumed (false means normal message processing
// continues).
func (a App) handleKeys(msg tea.KeyMsg) (App, tea.Cmd, bool) {
	// Help toggle — only in normal mode to avoid interfering with typing
	if msg.String() == "?" && !a.showHelp && a.mode == ModeNormal {
		a.showHelp = true
		a.helpModel.Open(a.width, a.height, "")
		return a, nil, true
	}

	// When help is open, delegate scrolling to the help viewport
	if a.showHelp {
		switch msg.String() {
		case "?", "esc", "q":
			a.showHelp = false
			return a, nil, true
		}
		return a, nil, true
	}

	// When command palette is open, delegate to it
	if a.commandPalette.IsShowing() {
		closed, cmd := a.commandPalette.Update(msg)
		if closed {
			if selected := a.commandPalette.SelectedCommand(); selected != nil {
				mm, cc := a.handlePaletteSelection(selected)
				return mm, cc, true
			}
		}
		return a, cmd, true
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
		return a, cmd, true
	}

	// When approval dialog is open, delegate to it
	if a.approvalDialog.IsVisible() {
		dialog, cmd := a.approvalDialog.Update(msg)
		a.approvalDialog = dialog
		return a, cmd, true
	}

	// Global chords: palette and reasoning-effort cycle.
	switch msg.String() {
	case "ctrl+p":
		return a, func() tea.Msg { return openCommandPaletteMsg{} }, true
	case "ctrl+r":
		if a.onUserCommand != nil {
			a.onUserCommand("/effort", &a)
		}
		return a, nil, true
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		if a.activeView == viewChat && a.chatModel.GetInput() != "" {
			a.chatModel.ClearInput()
			a.ShowStatus("Input cleared. Press Ctrl+C again to quit.", "info")
			return a, nil, true
		}
		return a, tea.Quit, true

	case tea.KeyTab:
		if !a.activeViewConsumesTab() {
			return a, a.switchView((a.activeView + 1) % viewCount), true
		}

	case tea.KeyShiftTab:
		if !a.activeViewConsumesTab() {
			return a, a.switchView((a.activeView - 1 + viewCount) % viewCount), true
		}

	case tea.KeyEsc:
		// If agent is running, cancel it first
		if a.agentCancelFunc != nil {
			a.CancelAgent()
			return a, func() tea.Msg {
				return AgentCancelMsg{}
			}, true
		}
		if !a.activeViewConsumesEsc() {
			a.mode = ModeNormal
			a.chatModel.SetModeLabel("navigate")
			a.blurActive()
			return a, nil, true
		}
	}

	if !a.activeViewCapturesAllKeys() {
		// Mode switching
		switch msg.String() {
		case "ctrl+n":
			a.mode = ModeNormal
			a.chatModel.SetModeLabel("navigate")
			a.blurActive()
			return a, nil, true
		case "i":
			if a.mode == ModeNormal {
				a.mode = ModeInsert
				a.chatModel.SetModeLabel("typing")
				a.focusActive()
				return a, nil, true
			}
		}

		// View switching shortcuts
		switch msg.String() {
		case "ctrl+1", "1":
			return a, a.switchView(viewHome), true
		case "ctrl+2", "2":
			return a, a.switchView(viewChat), true
		case "ctrl+3", "3":
			return a, a.switchView(viewSessions), true
		case "ctrl+4", "4":
			return a, a.switchView(viewSettings), true
		}

		// Navigation in normal mode
		if a.mode == ModeNormal {
			switch msg.String() {
			case "j", "down":
				a.scrollActiveView(1)
				return a, nil, true
			case "k", "up":
				a.scrollActiveView(-1)
				return a, nil, true
			case "g", "home":
				a.gotoActiveViewTop()
				return a, nil, true
			case "G", "end":
				a.gotoActiveViewBottom()
				return a, nil, true
			case "h":
				if a.activeView != viewSessions {
					return a, a.switchView(viewHome), true
				}
			case "c":
				if a.activeView != viewSessions {
					return a, a.switchView(viewChat), true
				}
			}
		}
	}
	return a, nil, false
}

// resize propagates a terminal resize to the active sub-models.
func (a App) resize(width, height int) (App, tea.Cmd) {
	var cmds []tea.Cmd
	a.width = width
	a.height = height

	// Reserve space for tab bar (3: padding top + content + border bottom)
	// + status bar (2: content + padding bottom) = 5 total
	reserved := 5
	contentMsg := tea.WindowSizeMsg{
		Width:  width,
		Height: height - reserved,
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
}
