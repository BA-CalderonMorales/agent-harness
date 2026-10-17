package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
)

// ExportPickerModel is the Home page's export modal: a session list the
// user picks one entry from; the host exports that session and the
// existing bottom notification confirms with the path. Esc cancels.

type ExportPickerModel struct {
	visible  bool
	width    int
	height   int
	sessions []SessionInfo
	cursor   int
}

// NewExportPicker creates the export session modal.
func NewExportPicker() ExportPickerModel {
	return ExportPickerModel{}
}

// OpenExportPicker populates and opens the export modal on the Home page.
func (a *App) OpenExportPicker(sessions []SessionInfo) {
	a.exportPicker.Open(a.width, a.height, sessions)
}

// ExportPickerShowing reports whether the modal is open.
func (a App) ExportPickerShowing() bool {
	return a.exportPicker.visible
}

// ExportPickerSelection returns the highlighted session (nil when the
// list is empty).
func (a App) ExportPickerSelection() *SessionInfo {
	if a.exportPicker.cursor < len(a.exportPicker.sessions) {
		s := a.exportPicker.sessions[a.exportPicker.cursor]
		return &s
	}
	return nil
}

// CloseExportPicker folds the modal without acting.
func (a *App) CloseExportPicker() {
	a.exportPicker.visible = false
}

// Open populates and shows the modal.
func (m *ExportPickerModel) Open(width, height int, sessions []SessionInfo) {
	m.width = width
	m.height = height
	m.sessions = sessions
	m.cursor = 0
	m.visible = true
}

// Update handles the modal's keys. It reports whether the modal closed
// and whether the selection should be exported.
func (m ExportPickerModel) Update(msg tea.Msg) (ExportPickerModel, bool, bool) {
	if !m.visible {
		return m, false, false
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		case "enter":
			m.visible = false
			return m, true, len(m.sessions) > 0
		case "esc":
			m.visible = false
			return m, true, false
		}
	}
	return m, false, false
}

// View renders the modal overlay through the shared modal frame, so the
// export list wears the same chrome and scroll window as every other
// overlay.
func (m ExportPickerModel) View(width, height int) string {
	if !m.visible {
		return ""
	}

	items := make([]modalItem, 0, len(m.sessions))
	for i, s := range m.sessions {
		marker, style := IndicatorUnselected, ListItemStyle
		if i == m.cursor {
			marker, style = IndicatorSelected, ListSelectedStyle
		}
		label := fmt.Sprintf("%s  %s  (%d msgs)", shortSessionStamp(s), s.Title, s.MessageCount)
		items = append(items, modalItem{lines: []string{style.Render(marker + label)}})
	}
	if len(items) == 0 {
		items = append(items, modalItem{lines: []string{HelpDimStyle.Render("No sessions to export.")}})
	}

	return renderModal(width, height, modalSpec{
		title:          "Export session",
		hint:           "Pick a session to write to disk.",
		items:          items,
		footer:         "j/k: navigate  Enter: export  Esc: cancel",
		preferredWidth: modalMaxWidth,
		cursor:         m.cursor,
	})
}

// shortSessionStamp renders the session's relative age for the list.
func shortSessionStamp(s SessionInfo) string {
	if s.UpdatedAt.IsZero() {
		return "         "
	}
	d := s.UpdatedAt.Format("Jan 02 15:04")
	return d
}
