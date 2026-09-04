package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// LogsModel renders the diagnostics stream as its own tab: a Splunk-like
// view of leveled entries with timestamps, source sites, and the exact
// file:line each entry was raised from. Level colors carry the severity
// scan; 'f' cycles the filter (all → warning+ → error+).

// logsMaxHeight caps the visible scroll rows; the viewport keeps the
// full history, the pane just shows a window of it.
const logsMaxHeight = 20

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

// Update handles resize, scroll keys, and the filter cycle.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.syncHeight()
		m.refresh()
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
		case "f":
			m.filter = (m.filter + 1) % len(levelFilters)
			m.refresh()
		}
		return m, nil
	}
	return m, nil
}

// refresh rebuilds the viewport content from the current entries and
// filter, keeping the tail in view.
func (m *LogsModel) refresh() {
	content := m.renderEntries()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// renderEntries formats the filtered stream: time, level, site, message,
// and the dim caller file:line.
func (m *LogsModel) renderEntries() string {
	minRank := levelFilters[m.filter].minLvl
	var b strings.Builder
	shown := 0
	for _, e := range m.entries {
		if levelRank(e.Level) < minRank {
			continue
		}
		shown++
		line := fmt.Sprintf("%s %-7s %s",
			e.Timestamp.Local().Format("15:04:05"),
			strings.ToUpper(e.Level),
			e.Site,
		)
		if e.Message != "" {
			line += "  " + e.Message
		}
		var styled string
		switch e.Level {
		case diag.LevelWarning:
			styled = WarningStyle.Render(line)
		case diag.LevelError, diag.LevelPanic:
			styled = ErrorStyle.Render(line)
		default:
			styled = HelpDimStyle.Render(line)
		}
		if e.Caller != "" {
			styled += HelpDimStyle.Render("  (" + filepath.Base(e.Caller) + ")")
		}
		b.WriteString(styled)
		b.WriteString("\n")
	}
	if shown == 0 {
		b.WriteString(HelpDimStyle.Render("  no entries at this level"))
	}
	return b.String()
}

// View renders the logs page.
func (m LogsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	filterName := levelFilters[m.filter].name
	var b strings.Builder
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Logs",
		Subtitle: "Diagnostics stream",
		Count:    len(m.entries),
	}))
	b.WriteString(m.viewport.View())
	if strings.TrimSpace(m.viewport.View()) == "" {
		b.WriteString("  " + HelpDimStyle.Render("(no entries yet)"))
	}
	b.WriteString(RenderFooter([]ActionHint{
		{Key: "↑/↓", Desc: "Scroll"},
		{Key: "g/G", Desc: "Top / bottom"},
		{Key: "f", Desc: "Filter: " + filterName},
	}))
	return b.String()
}
