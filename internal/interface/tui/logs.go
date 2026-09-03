package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// LogsModel renders the durable system log as its own tab: one
// scrollable, timestamped region instead of a section bolted under the
// settings list (the old placement overflowed the settings pane and
// scrolled the tab bar off the screen).

// logsMaxHeight caps the visible scroll rows; the viewport keeps the
// full history, the pane just shows a window of it.
const logsMaxHeight = 20

type LogsModel struct {
	width    int
	height   int
	viewport viewport.Model
	focused  bool
	count    int
}

// NewLogsModel creates a new logs view model.
func NewLogsModel() LogsModel {
	return LogsModel{viewport: newViewport(80, 10)}
}

// Focus marks the logs view as the keyboard owner.
func (m *LogsModel) Focus() { m.focused = true }

// Blur releases the keyboard.
func (m *LogsModel) Blur() { m.focused = false }

// SetMessages replaces the system log content.
func (m *LogsModel) SetMessages(messages []string) {
	content := strings.Join(messages, "\n")
	m.viewport.SetContent(content)
	m.count = len(messages)
	if m.width > 0 {
		m.viewport.Width = m.width
	}
	m.syncHeight()
	m.viewport.GotoBottom()
}

// syncHeight sizes the scroll window from the pane height: header (3)
// and footer (2) are reserved, the rest scrolls the log.
func (m *LogsModel) syncHeight() {
	h := m.height - 5
	if h > logsMaxHeight {
		h = logsMaxHeight
	}
	if h < 3 {
		h = 3
	}
	m.viewport.Height = h
}

// Update handles resize and scroll keys.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.syncHeight()
		return m, nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}
		return m, nil
	}
	return m, nil
}

// View renders the logs page.
func (m LogsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Logs",
		Subtitle: "System messages",
		Count:    m.count,
	}))
	b.WriteString(m.viewport.View())
	if strings.TrimSpace(m.viewport.View()) == "" {
		b.WriteString("  " + HelpDimStyle.Render("(no system messages yet)"))
	}
	b.WriteString(RenderFooter([]ActionHint{
		{Key: "↑/↓", Desc: "Scroll"},
		{Key: "g/G", Desc: "Top / bottom"},
	}))
	return b.String()
}
