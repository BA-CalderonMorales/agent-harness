package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// OnSubmit handles chat submit.
func (d *tuiChatDelegate) OnSubmit(text string) tea.Cmd {
	return func() tea.Msg {
		d.app.handleUserSubmit(text, d.tuiApp)
		return nil
	}
}

// OnCommand handles chat commands.
func (d *tuiChatDelegate) OnCommand(command string) {
	// Commands are handled asynchronously via UserCommandMsg in App.Update to avoid
	// mutating tuiApp state out-of-band during ChatModel.Update.
}
