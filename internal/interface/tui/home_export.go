package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// View renders the modal overlay.
func (m ExportPickerModel) View(width, height int) string {
	if !m.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(HelpTitleStyle.Render("Export session"))
	b.WriteString("\n")
	b.WriteString(HelpDimStyle.Render("Pick a session to export · Enter exports · Esc cancels"))
	b.WriteString("\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(HelpDimStyle.Render("  No sessions to export."))
	}

	for i, s := range m.sessions {
		label := fmt.Sprintf("%s  %s  (%d msgs)", shortSessionStamp(s), s.Title, s.MessageCount)
		if i == m.cursor {
			b.WriteString(ListSelectedStyle.Render(IndicatorSelected + " " + label))
		} else {
			b.WriteString(ListItemStyle.Render(IndicatorUnselected + " " + label))
		}
		b.WriteString("\n")
	}

	content := PanelPrimary.Width(panelWidth(width)).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// panelWidth sizes the modal: half the terminal with a floor wide
// enough that the hint line and session rows never wrap.
func panelWidth(width int) int {
	w := width / 2
	if w < 70 {
		w = 70
	}
	if w > width-4 {
		w = width - 4
	}
	if w < 20 {
		w = 20
	}
	return w
}

// shortSessionStamp renders the session's relative age for the list.
func shortSessionStamp(s SessionInfo) string {
	if s.UpdatedAt.IsZero() {
		return "         "
	}
	d := s.UpdatedAt.Format("Jan 02 15:04")
	return d
}
