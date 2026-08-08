package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) switchView(v viewID) tea.Cmd {
	a.blurActive()
	a.activeView = v
	a.setViewMode(v)
	a.focusActive()
	return a.initActiveView()
}

func (a *App) setViewMode(v viewID) {
	if v == viewChat {
		a.mode = ModeInsert
		// Transient footer messages (e.g. settings confirmations) must not
		// linger under the chat composer; the chat pane owns its own
		// system messages.
		a.statusMessage = ""
		a.statusType = ""
	} else {
		a.mode = ModeNormal
	}
}

func (a *App) focusActive() {
	a.tabActivity[a.activeView] = false
	switch a.activeView {
	case viewHome:
		a.homeModel.Focus()
	case viewChat:
		a.chatModel.Focus()
	case viewSessions:
		a.sessionsModel.Focus()
	case viewSettings:
		a.settingsModel.Focus()
	}
}

func (a *App) blurActive() {
	switch a.activeView {
	case viewHome:
		a.homeModel.Blur()
	case viewChat:
		a.chatModel.Blur()
	case viewSessions:
		a.sessionsModel.Blur()
	case viewSettings:
		a.settingsModel.Blur()
	}
}

func (a *App) initActiveView() tea.Cmd {
	switch a.activeView {
	case viewHome:
		return a.homeModel.Init()
	case viewChat:
		return a.chatModel.Init()
	case viewSessions:
		return a.sessionsModel.Init()
	case viewSettings:
		return a.settingsModel.Init()
	}
	return nil
}

func (a *App) activeViewConsumesTab() bool {
	switch a.activeView {
	case viewHome:
		return a.homeModel.ConsumesTab()
	case viewChat:
		return a.chatModel.ConsumesTab()
	case viewSessions:
		return a.sessionsModel.ConsumesTab()
	case viewSettings:
		return a.settingsModel.ConsumesTab()
	}
	return false
}

func (a *App) activeViewConsumesEsc() bool {
	switch a.activeView {
	case viewHome:
		return a.homeModel.ConsumesEsc()
	case viewChat:
		return a.chatModel.ConsumesEsc()
	case viewSessions:
		return a.sessionsModel.ConsumesEsc()
	case viewSettings:
		return a.settingsModel.ConsumesEsc()
	}
	return false
}

func (a *App) activeViewCapturesAllKeys() bool {
	// In normal mode, only Settings editing captures all keys
	// In insert mode, both Chat and Settings editing capture all keys
	if a.mode == ModeNormal {
		if a.activeView == viewSettings {
			return a.settingsModel.CapturesAllKeys()
		}
		return false
	}
	// Insert mode
	switch a.activeView {
	case viewChat:
		return a.chatModel.CapturesAllKeys()
	case viewSettings:
		return a.settingsModel.CapturesAllKeys()
	}
	return false
}

func (a *App) scrollActiveView(lines int) {
	switch a.activeView {
	case viewHome:
		a.homeModel.Scroll(lines)
	case viewChat:
		a.chatModel.Scroll(lines)
	case viewSessions:
		a.sessionsModel.Scroll(lines)
	case viewSettings:
		a.settingsModel.Scroll(lines)
	}
}

func (a *App) gotoActiveViewTop() {
	switch a.activeView {
	case viewHome:
		a.homeModel.GotoTop()
	case viewChat:
		a.chatModel.GotoTop()
	case viewSessions:
		a.sessionsModel.GotoTop()
	case viewSettings:
		a.settingsModel.GotoTop()
	}
}

func (a *App) gotoActiveViewBottom() {
	switch a.activeView {
	case viewHome:
		a.homeModel.GotoBottom()
	case viewChat:
		a.chatModel.GotoBottom()
	case viewSessions:
		a.sessionsModel.GotoBottom()
	case viewSettings:
		a.settingsModel.GotoBottom()
	}
}

// ---------------------------------------------------------------------------
// Public API for external interaction
// ---------------------------------------------------------------------------

// AddMessage adds a message to the chat.
