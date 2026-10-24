package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// modeKeyBindings are the bindings that work outside the composer.
//
// They live here, next to the other binding tables, so a new shortcut is
// one row instead of one more case in a 400-line function.
var modeKeyBindings = []keyBinding{
	{
		Keys:        []string{"ctrl+n"},
		Action:      "navigate mode",
		Description: "Leave insert mode and navigate",
		Run: func(a *App) (tea.Cmd, bool) {
			a.enterNavigationMode()
			return nil, true
		},
	},
	{
		Keys:        []string{"i"},
		Action:      "type",
		Description: "Focus the composer and start typing",
		When:        func(a *App) bool { return a.mode == ModeNormal },
		Run: func(a *App) (tea.Cmd, bool) {
			a.mode = ModeInsert
			a.chatModel.SetModeLabel("typing")
			a.focusActive()
			return nil, true
		},
	},
	{
		Keys:        []string{"/"},
		Action:      "command",
		Description: "Start a slash command (chat only)",
		When:        func(a *App) bool { return a.mode == ModeNormal && a.activeView == viewChat },
		Run: func(a *App) (tea.Cmd, bool) {
			// Slash commands are typed, and the composer is blurred in
			// navigate mode — a '/' there silently vanished (typed
			// commands died unfired). Focus the composer with the slash
			// pre-inserted: the command is already on screen.
			a.mode = ModeInsert
			a.chatModel.SetModeLabel("typing")
			a.focusActive()
			a.chatModel.SetInput("/")
			return nil, true
		},
	},
	{
		Keys:        []string{"m"},
		Action:      "mouse capture",
		Description: "Hand the mouse back to the terminal for select-and-copy",
		Run: func(a *App) (tea.Cmd, bool) {
			// The app owns the mouse for scroll and click targeting,
			// which makes terminal text selection fight the UI. 'm'
			// hands the mouse back, and 'm' again restores capture.
			// On a phone pane capture-off is the touch default: taps
			// raise the soft keyboard, flicks scroll the terminal.
			a.mouseCapture = !a.mouseCapture
			a.mouseCaptureTouched = true
			if a.mouseCapture {
				a.ShowStatus(`Mouse capture on — "m" to select-copy`, "info")
				return tea.Batch(func() tea.Msg { return tea.EnableMouseCellMotion() }, a.statusFlashCmd()), true
			}
			a.ShowStatus(`Mouse capture off — select to copy · "m" restores`, "info")
			return tea.Batch(func() tea.Msg { return tea.DisableMouse() }, a.statusFlashCmd()), true
		},
	},
}
