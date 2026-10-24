package tui

import (
	"strings"

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
		default:
			a.helpModel = a.helpModel.Update(msg)
			return a, nil, true
		}
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

	// When the next-steps picker is open, delegate to it: the modal owns
	// every key until Enter runs the chosen step or Esc cancels. The
	// chosen prompt is submitted exactly like typed input, so the turn it
	// starts is an ordinary turn.
	if a.nextSteps.IsShowing() {
		chosen, closed := a.nextSteps.Update(msg)
		if closed && chosen != nil {
			prompt := chosen.Prompt
			if strings.HasPrefix(prompt, "/") {
				return a, func() tea.Msg { return UserCommandMsg{Command: prompt} }, true
			}
			return a, func() tea.Msg { return UserSubmitMsg{Text: prompt} }, true
		}
		return a, nil, true
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

	// A pending question owns the keyboard before any global chord: the
	// agent is blocked on the answer, and the composer must not see the
	// keystrokes.
	if a.questionDialog.IsVisible() {
		dialog, cmd := a.questionDialog.Update(msg)
		a.questionDialog = dialog
		return a, cmd, true
	}

	// When approval dialog is open, delegate to it
	if a.approvalDialog.IsVisible() {
		dialog, cmd := a.approvalDialog.Update(msg)
		a.approvalDialog = dialog
		return a, cmd, true
	}

	// Global chords: palette and reasoning-effort cycle. The table
	// lives in keymap.go, so the bindings are data and /keys can list
	// them.
	if cmd, handled, claimed := a.handleGlobalBinding(msg); claimed {
		return a, cmd, handled
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
			a.enterNavigationMode()
			return a, func() tea.Msg {
				return AgentCancelMsg{}
			}, true
		}
		if !a.activeViewConsumesEsc() {
			a.enterNavigationMode()
			return a, nil, true
		}
	}

	if !a.activeViewCapturesAllKeys() {
		// Mode switching (keymap_mode.go): type, slash command, mouse
		// capture, navigate mode.
		if cmd, handled, claimed := a.runBinding(modeKeyBindings, msg); claimed {
			return a, cmd, handled
		}

		// View switching shortcuts
		if cmd, handled, claimed := a.handleViewBinding(msg); claimed {
			return a, cmd, handled
		}

		// Navigation in normal mode
		if a.mode == ModeNormal {
			// The Logs detail modal owns j/k (scroll its content) and
			// q (close) — without this the navigate path scrolled the
			// table behind the modal and Esc never reached it.
			if a.activeView == viewLogs && a.logsModel.DetailOpen() {
				switch msg.String() {
				case "esc", "q", "j", "k", "up", "down":
					a.logsModel, _ = a.logsModel.Update(msg)
					return a, nil, true
				}
			}

			// The navigate-mode bindings are data (keymap_navigation.go):
			// the guard decides whether a binding claims the key, and a
			// binding that claims it always consumes it.
			if cmd, handled, claimed := a.runBinding(navigateKeyBindings, msg); claimed {
				return a, cmd, handled
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

	// Reserve space for the fixed chrome: tab bar (3: padding top +
	// content + border bottom) + status bar (3: content + top and
	// bottom padding) = 6 total, plus the app frame's top and bottom
	// border rows. The reserve must match the chrome's real height —
	// one row short and the pane scrolls the tab bar's padding row off
	// the top.
	reserved := 6 + FrameRows

	// The frame insets the content: sub-models render inside the
	// border on every side, so their width shrinks by one column per
	// side and their height by the reserved chrome. Phone panes inset
	// further by the gutter; desktop panes keep the full inner width.
	gutter := gutterFor(width)
	contentMsg := tea.WindowSizeMsg{
		Width:  width - FrameCols - 2*gutter,
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
		a.settingsModel = settingsModel.(SettingsModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// The approval dialog is a full-pane overlay: it renders from the
	// pane's own size, so its frame must track resizes too — a dialog
	// that never learned the pane renders as a sliver.
	a.approvalDialog.width = width
	a.approvalDialog.height = height
	a.questionDialog.width = width
	a.questionDialog.height = height

	// Touch-first default on phone panes in touch hosts (tmux,
	// Termux): with mouse capture on, a tap becomes a click event and
	// the soft keyboard never rises — the dead end whose only exit was
	// the host terminal's drawer button. Capture yields unless the
	// user chose it with m. A narrow desktop pane is not a touch
	// device and keeps the frozen behavior. The flag alone is not the
	// mode: the terminal was started with cell motion enabled, so the
	// matching sequence must ride here or taps still report as clicks.
	if !a.mouseCaptureTouched && (inTmux() || isTermux) {
		mobile := isMobilePane(width)
		switch {
		case mobile && a.mouseCapture:
			a.mouseCapture = false
			cmds = append(cmds, func() tea.Msg { return tea.DisableMouse() })
			if !a.touchModeNoticed {
				a.touchModeNoticed = true
				a.ShowStatus("touch mode: tap raises the keyboard · m for gestures", "info")
				cmds = append(cmds, a.statusFlashCmd())
			}
		case !mobile && !a.mouseCapture:
			a.mouseCapture = true
			cmds = append(cmds, func() tea.Msg { return tea.EnableMouseCellMotion() })
		}
	}

	return a, tea.Batch(cmds...)
}
