package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogsModel renders the diagnostics stream as its own tab: a Splunk-like
// table of leveled entries — time, level, site, message — with a table
// header, a selectable cursor, and a detail modal for the full entry
// (long messages and stacks are truncated in the table, never in the
// modal). Desktop and tablet are the primary surfaces: click opens the
// detail; the cursor + Enter path carries small screens where a thumb
// cannot tap a row.

// logsMaxHeight caps the visible scroll rows; the viewport keeps the
// full history, the pane just shows a window of it.
const logsMaxHeight = 20

// logsHeaderRows are the pane rows above the table body: the page
// header (title + blank) and the column header row. Click math uses it
// to map a screen row to a table row.
const logsHeaderRows = 3

// LogEntryMsg delivers a diagnostics entry from the stream (the diag
// sink forwards through the App's drop-safe channel).
type LogEntryMsg struct {
	Entry diag.Entry
}

// levelFilter cycles: everything, warning and up, error and up.
var levelFilters = []struct {
	name   string
	minLvl int
}{{"all", 0}, {"warning+", 1}, {"error+", 2}}

func levelRank(level string) int {
	switch level {
	case diag.LevelInfo:
		return 0
	case diag.LevelWarning:
		return 1
	case diag.LevelError, diag.LevelPanic:
		return 2
	}
	return 0
}

type LogsModel struct {
	width    int
	height   int
	viewport viewport.Model
	focused  bool
	filter   int
	entries  []diag.Entry
	visible  []diag.Entry // filtered rows, index = table row
	cursor   int          // selected row in visible
	detail   *diag.Entry  // non-nil: the detail modal is open
}

// NewLogsModel creates a new logs view model.
func NewLogsModel() LogsModel {
	return LogsModel{viewport: newViewport(80, 10)}
}

// Focus marks the logs view as the keyboard owner.
func (m *LogsModel) Focus() { m.focused = true }

// Blur releases the keyboard.
func (m *LogsModel) Blur() { m.focused = false }

// AppendEntry adds a diagnostics entry and follows the tail.
func (m *LogsModel) AppendEntry(e diag.Entry) {
	m.entries = append(m.entries, e)
	m.refresh()
}

// OpenDetail exposes the detail modal for tests and click handling.
func (m LogsModel) DetailOpen() bool { return m.detail != nil }

// syncHeight sizes the scroll window from the pane height: header (3)
// and footer (2) are reserved, the rest scrolls the stream.
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

// refresh rebuilds the filtered table and keeps the cursor row in view.
func (m *LogsModel) refresh() {
	minRank := levelFilters[m.filter].minLvl
	m.visible = m.visible[:0]
	for _, e := range m.entries {
		if levelRank(e.Level) >= minRank {
			m.visible = append(m.visible, e)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	var b strings.Builder
	for i, e := range m.visible {
		row := m.tableRow(e)
		if i == m.cursor {
			b.WriteString(ListSelectedStyle.Render(row))
		} else {
			b.WriteString(m.rowStyle(e)(row))
		}
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
	m.scrollToCursor()
}

// scrollToCursor keeps the selected row inside the viewport window.
func (m *LogsModel) scrollToCursor() {
	if len(m.visible) == 0 {
		return
	}
	if m.cursor < m.viewport.YOffset {
		m.viewport.SetYOffset(m.cursor)
	} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
	}
}

func (m *LogsModel) tableHeader() string {
	return HelpDimStyle.Render(fmt.Sprintf("%-8s %-7s %-22s %s", "TIME", "LEVEL", "SITE", "DETAIL"))
}

// tableRow renders one entry as a single line — the table never wraps,
// so a click maps cleanly back to a row.
func (m LogsModel) tableRow(e diag.Entry) string {
	line := fmt.Sprintf("%s %-7s %-22s %s",
		e.Timestamp.Local().Format("15:04:05"),
		strings.ToUpper(e.Level),
		e.Site,
		e.Message,
	)
	if e.Caller != "" {
		line += "  (" + filepath.Base(e.Caller) + ")"
	}
	if m.width > 0 && len(line) > m.width-1 {
		line = line[:m.width-2] + "…"
	}
	return line
}

func (LogsModel) rowStyle(e diag.Entry) func(string) string {
	return func(s string) string {
		switch e.Level {
		case diag.LevelWarning:
			return WarningStyle.Render(s)
		case diag.LevelError, diag.LevelPanic:
			return ErrorStyle.Render(s)
		default:
			return s
		}
	}
}

// Update handles resize, cursor keys, the filter cycle, detail open/
// close, and mouse clicks on the table.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.syncHeight()
		m.refresh()
		return m, nil

	case tea.MouseMsg:
		if !m.focused || m.detail != nil {
			return m, nil
		}
		if tea.MouseEvent(msg).Action == tea.MouseActionPress && tea.MouseEvent(msg).Button == tea.MouseButtonLeft {
			// Screen Y → table row: tab bar (1) + page header (2) +
			// column header (1) sit above the viewport, which scrolls.
			contentRow := msg.Y - logsHeaderRows - 1 + m.viewport.YOffset
			if contentRow >= 0 && contentRow < len(m.visible) {
				m.cursor = contentRow
				m.detail = &m.visible[m.cursor]
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.detail != nil {
			// Any key folds the detail modal.
			m.detail = nil
			return m, nil
		}
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.scrollToCursor()
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				m.scrollToCursor()
			}
		case "g":
			m.cursor = 0
			m.scrollToCursor()
		case "G":
			m.cursor = len(m.visible) - 1
			m.scrollToCursor()
		case "enter":
			if len(m.visible) > 0 {
				m.detail = &m.visible[m.cursor]
			}
		case "f":
			m.filter = (m.filter + 1) % len(levelFilters)
			m.refresh()
		}
		return m, nil
	}
	return m, nil
}

// View renders the logs page: table header, stream, footer — or the
// detail modal over it.
func (m LogsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.detail != nil {
		return m.renderDetail()
	}

	filterName := levelFilters[m.filter].name
	var b strings.Builder
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Logs",
		Subtitle: "Diagnostics stream",
		Count:    len(m.entries),
	}))
	b.WriteString("\n")
	b.WriteString(m.tableHeader())
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	if strings.TrimSpace(m.viewport.View()) == "" {
		b.WriteString(HelpDimStyle.Render("  no entries at this level"))
	}
	b.WriteString(RenderFooter([]ActionHint{
		{Key: "↑/↓", Desc: "Select"},
		{Key: "Enter", Desc: "Detail"},
		{Key: "f", Desc: "Filter: " + filterName},
	}))
	return b.String()
}

// renderDetail shows the full entry: every field, the message wrapped to
// the modal width, and the stack when present.
func (m LogsModel) renderDetail() string {
	e := *m.detail
	var b strings.Builder
	b.WriteString(HelpTitleStyle.Render("Log detail"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Level:   %s\n", strings.ToUpper(e.Level)))
	b.WriteString(fmt.Sprintf("Time:    %s\n", e.Timestamp.Local().Format("15:04:05")))
	b.WriteString(fmt.Sprintf("Site:    %s\n", e.Site))
	if e.Caller != "" {
		b.WriteString(fmt.Sprintf("Source:  %s\n", e.Caller))
	}
	b.WriteString("\n")
	b.WriteString(wrapText(e.Message, m.width-10))
	if e.Detail != "" {
		b.WriteString("\n\n" + wrapText(e.Detail, m.width-10))
	}
	if e.Stack != "" {
		b.WriteString("\n\n" + HelpDimStyle.Render(wrapText(e.Stack, m.width-10)))
	}

	panel := PanelStyle.
		Width(m.detailWidth()).
		Height(m.height - 6).
		Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

// detailWidth caps the modal on huge screens and fits small ones.
func (m LogsModel) detailWidth() int {
	w := m.width - 8
	if w > 80 {
		w = 80
	}
	if w < 24 {
		w = 24
	}
	return w
}
