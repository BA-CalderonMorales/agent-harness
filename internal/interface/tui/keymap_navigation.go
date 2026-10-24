package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// navigateKeyBindings are the bindings that work in navigate mode.
//
// They were a switch inside handleKeys with one case per key. As data
// they are listable (/keys), testable, and one more shortcut no longer
// means editing a 400-line function.
//
// The guards carry the old conditions exactly: a binding whose view does
// not apply is not claimed, so the key falls through to the view behind
// it, and the "x" binding claims the key in the Logs view only.
var navigateKeyBindings = []keyBinding{
	{
		Keys:        []string{"enter"},
		Action:      "expand record",
		Description: "Expand the most recent tool record (Esc folds it back)",
		When:        func(a *App) bool { return a.activeView == viewChat },
		Run: func(a *App) (tea.Cmd, bool) {
			a.chatModel.ExpandLatestRecord()
			return nil, true
		},
	},
	{
		Keys:        []string{"y"},
		Action:      "copy record",
		Description: "Copy the expanded record, or the latest reply",
		When:        func(a *App) bool { return a.activeView == viewChat },
		Run: func(a *App) (tea.Cmd, bool) {
			if text, label := a.chatModel.CopyRecord(); text != "" {
				copied := copyToClipboard(text)
				a.flashCopyStatus(label, copied)
			} else {
				a.flashCopyStatus("", false)
			}
			return nil, true
		},
	},
	{
		Keys:        []string{"Y"},
		Action:      "copy conversation",
		Description: "Copy the whole conversation, role-labeled",
		When:        func(a *App) bool { return a.activeView == viewChat },
		Run: func(a *App) (tea.Cmd, bool) {
			if text, n := a.chatModel.CopyConversation(); n > 0 {
				copied := copyToClipboard(text)
				a.flashCopyStatus(fmt.Sprintf("conversation (%d messages, %d chars)", n, len(text)), copied)
			} else {
				a.flashCopyStatus("", false)
			}
			return nil, true
		},
	},
	{
		Keys:        []string{"j", "down"},
		Action:      "scroll down",
		Description: "Scroll the active view down",
		Run: func(a *App) (tea.Cmd, bool) {
			a.scrollActiveView(1)
			return nil, true
		},
	},
	{
		Keys:        []string{"k", "up"},
		Action:      "scroll up",
		Description: "Scroll the active view up",
		Run: func(a *App) (tea.Cmd, bool) {
			a.scrollActiveView(-1)
			return nil, true
		},
	},
	{
		Keys:        []string{"g", "home"},
		Action:      "scroll to top",
		Description: "Jump to the top of the active view",
		Run: func(a *App) (tea.Cmd, bool) {
			a.gotoActiveViewTop()
			return nil, true
		},
	},
	{
		Keys:        []string{"G", "end"},
		Action:      "scroll to bottom",
		Description: "Jump to the bottom of the active view",
		Run: func(a *App) (tea.Cmd, bool) {
			a.gotoActiveViewBottom()
			return nil, true
		},
	},
	{
		Keys:        []string{"ctrl+u"},
		Action:      "page up",
		Description: "Scroll the active view up half a page",
		Run: func(a *App) (tea.Cmd, bool) {
			a.scrollActiveView(-a.halfPageLines())
			return nil, true
		},
	},
	{
		Keys:        []string{"ctrl+d"},
		Action:      "page down",
		Description: "Scroll the active view down half a page",
		Run: func(a *App) (tea.Cmd, bool) {
			a.scrollActiveView(a.halfPageLines())
			return nil, true
		},
	},
	{
		Keys:        []string{"t"},
		Action:      "toggle tool details",
		Description: "Collapse or expand the tool run details",
		Run: func(a *App) (tea.Cmd, bool) {
			a.chatModel.ToggleToolsCollapsed()
			return nil, true
		},
	},
	{
		Keys:        []string{"s"},
		Action:      "next steps",
		Description: "Suggest what to do next (chat only)",
		When:        func(a *App) bool { return a.activeView == viewChat },
		Run: func(a *App) (tea.Cmd, bool) {
			a.nextSteps.Open(a.width, a.height, a.chatModel.deriveNextSteps())
			return nil, true
		},
	},
	{
		Keys:        []string{"h"},
		Action:      "home",
		Description: "Go to Home from anywhere but Sessions",
		When:        func(a *App) bool { return a.activeView != viewSessions },
		Run: func(a *App) (tea.Cmd, bool) {
			return a.switchView(viewHome), true
		},
	},
	{
		Keys:        []string{"c"},
		Action:      "chat",
		Description: "Go to Chat from anywhere but Sessions",
		When:        func(a *App) bool { return a.activeView != viewSessions },
		Run: func(a *App) (tea.Cmd, bool) {
			return a.switchView(viewChat), true
		},
	},
	{
		Keys:        []string{"l"},
		Action:      "login",
		Description: "Open the login wizard when the provider is not ready, otherwise the next tab",
		Run: func(a *App) (tea.Cmd, bool) {
			// Routed as a UserCommandMsg (not a direct handler call):
			// handleKeys works on a copy, and a synchronous handler
			// mutation would be clobbered when the copy is written back.
			if a.providerReadiness != 1 {
				return func() tea.Msg { return UserCommandMsg{Command: "/login"} }, true
			}
			return a.switchView((a.activeView + 1) % viewCount), true
		},
	},
	{
		Keys:        []string{"x"},
		Action:      "delete log entry",
		Description: "Delete the selected log entry (Logs only)",
		Run: func(a *App) (tea.Cmd, bool) {
			if a.activeView != viewLogs {
				// Claimed but not handled: the key belongs to the view
				// in front, and the old switch said the same.
				return nil, false
			}
			if _, ok := a.logsModel.DeleteSelected(); ok {
				a.ShowStatus(fmt.Sprintf("Log entry deleted — %d left", a.logsModel.Count()), "info")
				return a.statusFlashCmd(), true
			}
			return nil, true
		},
	},
	{
		Keys:        []string{"X"},
		Action:      "clear logs",
		Description: "Clear the diagnostics stream (Logs only)",
		Run: func(a *App) (tea.Cmd, bool) {
			if a.activeView != viewLogs {
				return nil, false
			}
			cleared := a.logsModel.ClearAll()
			a.ShowStatus(fmt.Sprintf("Diagnostics stream cleared — %d entries gone", cleared), "info")
			return a.statusFlashCmd(), true
		},
	},
}
