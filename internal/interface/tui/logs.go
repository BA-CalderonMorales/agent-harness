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
	width        int
	height       int
	viewport     viewport.Model
	focused      bool
	filter       int
	entries      []diag.Entry
	visible      []diag.Entry // filtered rows, index = table row
	cursor       int          // selected row in visible
	detail       *diag.Entry  // non-nil: the detail modal is open
	detailScroll int          // detail modal scroll offset
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
			// The ▸ marker carries the selection on themes where the
			// highlight background blends into the page background —
			// the marker is the load-bearing cue, the style the bonus.
			b.WriteString(ListSelectedStyle.Render("▸ " + row))
		} else {
			b.WriteString(m.rowStyle(e)("  " + row))
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
	return HelpDimStyle.Render(fmt.Sprintf("  %-8s %-7s %-22s %s", "TIME", "LEVEL", "SITE", "DETAIL"))
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

// ConsumesEsc keeps the global Esc handler from stealing the key while
// the detail modal is open — Esc there means "close the modal".
func (m *LogsModel) ConsumesEsc() bool { return m.detail != nil }

// MoveCursor shifts the selection by lines (the navigate-mode j/k
// entry point) and keeps the row in view.
func (m *LogsModel) MoveCursor(lines int) {
	if len(m.visible) == 0 {
		return
	}
	m.cursor += lines
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.visible)-1 {
		m.cursor = len(m.visible) - 1
	}
	m.refresh()
}

// CursorTop selects the first entry.
func (m *LogsModel) CursorTop() {
	m.cursor = 0
	m.refresh()
}

// CursorBottom selects the last entry.
func (m *LogsModel) CursorBottom() {
	if len(m.visible) > 0 {
		m.cursor = len(m.visible) - 1
	}
	m.refresh()
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
			// The detail modal scrolls its content (j/k) and folds on
			// Esc — folding on any key made a long stack unreadable.
			switch msg.String() {
			case "esc", "q":
				m.detail = nil
				m.detailScroll = 0
			case "j", "down":
				m.detailScroll++
			case "k", "up":
				if m.detailScroll > 0 {
					m.detailScroll--
				}
			}
			return m, nil
		}
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.refresh()
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				m.refresh()
			}
		case "g":
			m.cursor = 0
			m.refresh()
		case "G":
			m.cursor = len(m.visible) - 1
			m.refresh()
		case "enter":
			if len(m.visible) > 0 {
				m.detail = &m.visible[m.cursor]
				m.detailScroll = 0
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
	hints := []ActionHint{
		{Key: "↑/↓", Desc: "Select"},
		{Key: "Enter", Desc: "Detail"},
		{Key: "f", Desc: "Filter: " + filterName},
	}
	if len(m.visible) > 0 {
		hints = append(hints, ActionHint{
			Key:  fmt.Sprintf("%d/%d", m.cursor+1, len(m.visible)),
			Desc: "Selected",
		})
	}

	// Body: everything above the hint line.
	var body strings.Builder
	body.WriteString(RenderHeader(HeaderConfig{
		Title:    "Logs",
		Subtitle: "Diagnostics stream",
		Count:    len(m.entries),
	}))
	body.WriteString("\n")
	body.WriteString(m.tableHeader())
	body.WriteString("\n")
	body.WriteString(m.viewport.View())
	if strings.TrimSpace(m.viewport.View()) == "" {
		body.WriteString(HelpDimStyle.Render("  no entries at this level"))
	}

	// The hint line pins to the pane's bottom row — bottom-left, one
	// row above the status bar — instead of floating after the last
	// entry. The gap between the stream and the hints is padding.
	lines := strings.Split(body.String(), "\n")
	pane := m.height
	if pane < 4 {
		pane = 4
	}
	// RenderFooter emits a blank separator line before the hints.
	pad := pane - len(lines) - 2
	if pad < 0 {
		pad = 0
	}
	for i := 0; i < pad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, strings.TrimPrefix(RenderFooter(hints), "\n"))
	return strings.Join(lines, "\n")
}

// renderDetail shows the full entry: every field, the message wrapped to
// the modal width, and the stack when present — scrolled with j/k when
// the content outgrows the modal, with the exit hint pinned bottom-right.
func (m LogsModel) renderDetail() string {
	e := *m.detail
	var lines []string
	lines = append(lines, HelpTitleStyle.Render("Log detail"), "")
	lines = append(lines, fmt.Sprintf("Level:   %s", strings.ToUpper(e.Level)))
	lines = append(lines, fmt.Sprintf("Time:    %s", e.Timestamp.Local().Format("15:04:05")))
	lines = append(lines, fmt.Sprintf("Site:    %s", e.Site))
	if e.Caller != "" {
		lines = append(lines, fmt.Sprintf("Source:  %s", e.Caller))
	}
	lines = append(lines, "")
	lines = append(lines, strings.Split(wrapText(e.Message, m.detailWidth()-8), "\n")...)
	if e.Detail != "" {
		lines = append(lines, "")
		lines = append(lines, strings.Split(wrapText(e.Detail, m.detailWidth()-8), "\n")...)
	}
	if e.Stack != "" {
		lines = append(lines, "")
		lines = append(lines, strings.Split(HelpDimStyle.Render(wrapText(e.Stack, m.detailWidth()-8)), "\n")...)
	}

	// The exit hint owns the modal's bottom row, right-aligned — pinned
	// to the frame, never floating after the content. The content
	// window gets every row above it.
	hint := HelpDimStyle.Render(`"Esc" to close · "j"/"k" to scroll`)
	hintPad := m.detailWidth() - 8 - lipgloss.Width(hint)
	if hintPad < 0 {
		hintPad = 0
	}
	hintRow := strings.Repeat(" ", hintPad) + hint

	bodyRows := m.height - 10
	if bodyRows < 4 {
		bodyRows = 4
	}
	contentRows := bodyRows - 1
	maxScroll := len(lines) - contentRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	end := m.detailScroll + contentRows
	if end > len(lines) {
		end = len(lines)
	}
	window := lines[m.detailScroll:end]

	var body []string
	body = append(body, window...)
	for len(body) < contentRows {
		body = append(body, "")
	}
	body = append(body, hintRow)

	panel := PanelStyle.
		Width(m.detailWidth()).
		Height(bodyRows).
		Render(strings.Join(body, "\n"))
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
