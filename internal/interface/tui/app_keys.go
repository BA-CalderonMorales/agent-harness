package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// agentModeCycle is the Shift+Tab ring shown in the composer chip. The
// host maps these display names onto execution machinery.
var agentModeCycle = []string{"manual", "auto", "plan", "chat"}

// nextDisplayMode returns the mode that follows cur in the ring.
func nextDisplayMode(cur string) string {
	for i, m := range agentModeCycle {
		if m == cur {
			return agentModeCycle[(i+1)%len(agentModeCycle)]
		}
	}
	return agentModeCycle[0]
}

// NextDisplayMode exposes the Shift+Tab ring to hosts so /mode cycles
// identically.
func NextDisplayMode(cur string) string { return nextDisplayMode(cur) }

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

	// When the export picker is open, delegate to it: the modal owns
	// every key until Enter exports the selection or Esc cancels.
	if a.exportPicker.visible {
		next, closed, pick := a.exportPicker.Update(msg)
		a.exportPicker = next
		if closed && pick && a.onExportPick != nil {
			if sel := a.ExportPickerSelection(); sel != nil {
				a.onExportPick(sel.ID)
			}
		}
		return a, nil, true
	}

	// When the provider picker is open, delegate to it. A pick closes the
	// picker and hands the provider to the app, which opens the model
	// picker — a fast provider switch never asks for the API key again.
	if a.providerPicker.IsShowing() {
		done, cancelled, provider := a.providerPicker.Update(msg)
		if done || cancelled {
			a.providerPicker.Close()
			if done && a.onProviderPick != nil {
				a.onProviderPick(provider, &a)
			}
		}
		return a, nil, true
	}

	// When the login dialog is open, delegate to it. It renders its own
	// masked input, so every key (including pastes) stays inside the
	// modal and never reaches the composer.
	if a.loginDialog.IsShowing() {
		done, cancelled, provider, apiKey, model := a.loginDialog.Update(msg)
		if done || cancelled {
			a.loginDialog.Close()
			if done && a.onLogin != nil {
				a.onLogin(provider, apiKey, model, &a)
			}
		}
		return a, nil, true
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
	case ":":
		// k9s-style: ':' opens the command palette from any normal-mode
		// view. In insert mode ':' types into the composer untouched.
		if a.mode == ModeNormal {
			return a, func() tea.Msg { return openCommandPaletteMsg{} }, true
		}
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
		// Inside the composer, Shift+Tab cycles the agent mode chip
		// (manual → auto → plan → chat) without leaving the keyboard.
		// The host learns about it via AgentModeChangedMsg: handleKeys
		// runs on a value copy, so the machinery sync must ride the
		// message loop, not a captured pointer.
		if a.activeView == viewChat && a.mode == ModeInsert {
			next := nextDisplayMode(a.chatModel.agentMode)
			a.chatModel.agentMode = next
			return a, func() tea.Msg { return AgentModeChangedMsg{Mode: next} }, true
		}
		if !a.activeViewConsumesTab() {
			return a, a.switchView((a.activeView - 1 + viewCount) % viewCount), true
		}

	case tea.KeyEsc:
		// An expanded tool record folds first — Esc means "back out of
		// what I opened" before it means anything else.
		if a.activeView == viewChat && a.chatModel.CollapseRecordExpansion() {
			return a, nil, true
		}
		// If agent is running, cancel it first — and land in normal
		// mode: Esc signaled "stop, let me look", so the next j/k must
		// scroll instead of typing into a composer the user believes
		// they left.
		if a.agentCancelFunc != nil {
			a.CancelAgent()
			a.mode = ModeNormal
			a.chatModel.SetModeLabel("navigate")
			a.blurActive()
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
		case "/":
			// Slash commands are typed, and the composer is blurred in
			// navigate mode — a '/' there silently vanished (typed
			// commands died unfired). Focus the composer with the slash
			// pre-inserted: the command is already on screen.
			if a.mode == ModeNormal && a.activeView == viewChat {
				a.mode = ModeInsert
				a.chatModel.SetModeLabel("typing")
				a.focusActive()
				a.chatModel.SetInput("/")
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
			return a, a.switchView(viewLogs), true
		case "ctrl+5", "5":
			return a, a.switchView(viewSettings), true
		}

		// Navigation in normal mode
		if a.mode == ModeNormal {
			switch msg.String() {
			case "enter":
				// Chat: expand the most recent tool call's full record
				// (Esc folds it back).
				if a.activeView == viewChat {
					a.chatModel.ExpandLatestRecord()
					return a, nil, true
				}
			case "y":
				// Chat: copy the expanded record — or the latest reply
				// when nothing is expanded — to the clipboard. The TUI
				// captures the mouse, so selection-based copying fights
				// the UI; 'y' hands the specific text over instead.
				if a.activeView == viewChat {
					if text, label := a.chatModel.CopyRecord(); text != "" {
						copied := copyToClipboard(text)
						a.flashCopyStatus(label, copied)
					} else {
						a.flashCopyStatus("", false)
					}
					return a, nil, true
				}
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
			case "ctrl+u":
				a.scrollActiveView(-a.halfPageLines())
				return a, nil, true
			case "ctrl+d":
				a.scrollActiveView(a.halfPageLines())
				return a, nil, true
			case "h":
				if a.activeView != viewSessions {
					return a, a.switchView(viewHome), true
				}
			case "c":
				if a.activeView != viewSessions {
					return a, a.switchView(viewChat), true
				}
			case "l":
				// Vim-right: the next tab, matching h for Home. The
				// setup dead-end keeps its advertised handle — the
				// statusbar badge and home banner say "(l: login)" only
				// when the provider is not ready, so 'l' opens the
				// wizard exactly when the affordance is on screen.
				// Routed as a UserCommandMsg (not a direct handler
				// call): handleKeys works on a copy, and a synchronous
				// handler mutation (startLogin opens the dialog) would
				// be clobbered when *a = next copies the pre-call state
				// back. The msg path runs on the live app, exactly like
				// a typed /login.
				if a.providerReadiness != 1 {
					return a, func() tea.Msg { return UserCommandMsg{Command: "/login"} }, true
				}
				return a, a.switchView((a.activeView + 1) % viewCount), true
			case "t":
				// Toggle tool-run collapsing: the long-horizon trace
				// reads as count lines by default; 't' expands or
				// collapses the detail. Errors and running tools are
				// never hidden by either state.
				a.chatModel.ToggleToolsCollapsed()
				return a, nil, true
			}

			// Navigate-mode affordance: an unmatched printable key in
			// normal mode is almost always someone starting to type.
			// Flash the mode instead of silently doing nothing (or
			// worse, having an earlier binding fire).
			if a.activeView == viewChat && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
				a.ShowStatus("Navigate mode — press i to type", "info")
				return a, a.statusFlashCmd(), true
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

	if logsModel, cmd := a.logsModel.Update(contentMsg); true {
		a.logsModel = logsModel
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
