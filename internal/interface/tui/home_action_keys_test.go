package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type homeFeedbackDelegate struct {
	testHomeDelegate
	app *App
}

func (d *homeFeedbackDelegate) OnNewChat() {
	d.app.Send(SessionActivatedMsg{SwitchToChat: true, Notice: "Started new chat"})
}

func (d *homeFeedbackDelegate) OnExportSession() {
	d.app.OpenExportPicker([]SessionInfo{{ID: "saved", Title: "Saved conversation"}})
}

// Exercise the app's global key routing before the Home action. Calling
// HomeModel.Update directly misses the focus loss caused by Esc/Ctrl+N.
func TestHomeActionsKeepFeedbackAfterNavigationKeys(t *testing.T) {
	for _, reset := range []struct {
		name string
		key  tea.KeyType
	}{
		{"startup", 0},
		{"escape", tea.KeyEsc},
		{"ctrl-n", tea.KeyCtrlN},
	} {
		for _, action := range []string{"new-chat", "export"} {
			for _, activation := range []string{"enter", "space", "shortcut"} {
				t.Run(reset.name+"/"+action+"/"+activation, func(t *testing.T) {
					app := NewApp()
					app.SetHomeDelegate(&homeFeedbackDelegate{app: app})
					app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
					if reset.name != "startup" {
						app.Update(tea.KeyMsg{Type: reset.key})
					}
					if action == "export" {
						app.Update(tea.KeyMsg{Type: tea.KeyDown})
					}
					key := tea.KeyMsg{Type: tea.KeyEnter}
					switch activation {
					case "space":
						key.Type = tea.KeySpace
					case "shortcut":
						shortcut := 'n'
						if action == "export" {
							shortcut = 'e'
						}
						key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{shortcut}}
					}
					app.Update(key)
					if action == "export" {
						if !app.ExportPickerShowing() || !strings.Contains(app.View(), "Saved conversation") {
							t.Fatal("Export session did not display the session picker")
						}
						return
					}
					select {
					case msg := <-app.msgChan:
						app.Update(msg)
					default:
						t.Fatal("New chat produced no activation message")
					}
					if app.activeView != viewChat || !strings.Contains(app.View(), "Started new chat") {
						t.Fatal("New chat did not open Chat with its confirmation")
					}
				})
			}
		}
	}
}
