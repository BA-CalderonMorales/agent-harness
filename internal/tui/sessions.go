// Sessions view for managing chat sessions

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// SessionsDelegate handles session actions
// ---------------------------------------------------------------------------
type SessionsDelegate interface {
	OnSessionSelect(id string)
	OnSessionDelete(id string)
	OnSessionExport(id string)
	OnSessionCopy(id string) // Copy conversation to clipboard
	OnSessionLoad()
}

// ---------------------------------------------------------------------------
// SessionInfo represents session metadata
// ---------------------------------------------------------------------------
type SessionInfo struct {
	ID           string
	Title        string
	MessageCount int
	Turns        int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Model        string
	IsActive     bool
}

// ---------------------------------------------------------------------------
// SessionsModel is the sessions view model
// ---------------------------------------------------------------------------
type SessionsModel struct {
	width    int
	height   int
	sessions []SessionInfo
	cursor   int
	focused  bool
	loading  bool
	viewport viewport.Model

	// Delegate
	delegate SessionsDelegate
}

// NewSessionsModel creates a new sessions model.
// CRITICAL FIX: Added viewport for proper scrolling
func NewSessionsModel() SessionsModel {
	return SessionsModel{
		sessions: make([]SessionInfo, 0),
		cursor:   0,
		viewport: viewport.New(80, 20),
	}
}

// SetDelegate sets the sessions delegate.
func (m *SessionsModel) SetDelegate(delegate SessionsDelegate) {
	m.delegate = delegate
}

// SetSessions updates the sessions list.
func (m *SessionsModel) SetSessions(sessions []SessionInfo) {
	m.sessions = sessions
	if m.cursor >= len(sessions) && len(sessions) > 0 {
		m.cursor = len(sessions) - 1
	}
}

// Init initializes the sessions model.
// CRITICAL FIX: Load sessions immediately on init to show current session
func (m SessionsModel) Init() tea.Cmd {
	return func() tea.Msg {
		if m.delegate != nil {
			m.delegate.OnSessionLoad()
		}
		return SessionsLoadedMsg{}
	}
}

// SessionsLoadedMsg is sent when sessions have been loaded
type SessionsLoadedMsg struct{}

// Update handles messages.
func (m SessionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// CRITICAL FIX: Update viewport size for proper scrolling
		headerHeight := 3
		footerHeight := 2
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		if m.viewport.Height < 5 {
			m.viewport.Height = 5
		}
		
	case tea.KeyMsg:
		if !m.focused {
			// Still update viewport for background scrolling
			newVP, cmd := m.viewport.Update(msg)
			m.viewport = newVP
			return m, cmd
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.syncViewportToCursor()
			}

		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.syncViewportToCursor()
			}

		case "enter", " ":
			if m.cursor < len(m.sessions) && m.delegate != nil {
				m.delegate.OnSessionSelect(m.sessions[m.cursor].ID)
			}

		case "d":
			if m.cursor < len(m.sessions) && m.delegate != nil {
				m.delegate.OnSessionDelete(m.sessions[m.cursor].ID)
			}

		case "e":
			if m.cursor < len(m.sessions) && m.delegate != nil {
				m.delegate.OnSessionExport(m.sessions[m.cursor].ID)
			}

		case "c":
			if m.cursor < len(m.sessions) && m.delegate != nil {
				m.delegate.OnSessionCopy(m.sessions[m.cursor].ID)
			}

		case "r":
			if m.delegate != nil {
				m.delegate.OnSessionLoad()
			}
		}
	}

	// Update viewport for scroll messages
	newVP, cmd := m.viewport.Update(msg)
	m.viewport = newVP
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// syncViewportToCursor ensures the selected session is visible
func (m *SessionsModel) syncViewportToCursor() {
	cursorLine := m.cursor * 2 // Approximate 2 lines per item
	if cursorLine < m.viewport.YOffset {
		m.viewport.SetYOffset(cursorLine)
	}
	viewportBottom := m.viewport.YOffset + m.viewport.Height
	if cursorLine+2 > viewportBottom {
		newOffset := cursorLine + 2 - m.viewport.Height
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	}
}

// View renders the sessions list.
func (m SessionsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.loading {
		return RenderLoading(ViewPort{Width: m.width, Height: m.height}, "Loading sessions...")
	}

	if len(m.sessions) == 0 {
		return RenderEmptyState(ViewPort{Width: m.width, Height: m.height}, EmptyState{
			Title:       "No Sessions",
			Description: "Start chatting to create your first session.",
			Actions: []ActionHint{
				{Key: "Tab", Desc: "Switch to chat"},
				{Key: "r", Desc: "Refresh"},
			},
		})
	}

	var b strings.Builder

	// Header (consistent with Settings view)
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Sessions",
		Subtitle: "Manage your conversations",
		Count:    len(m.sessions),
	}))

	// Content area height (subtract header height)
	contentHeight := m.height - 3
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Two-pane layout
	listW, detailW := TwoPaneWidths(m.width)

	// Render list
	var listB strings.Builder
	listB.WriteString(ListTitleStyle.Render("  All Sessions") + "\n\n")

	for i, session := range m.sessions {
		item := m.renderSessionItem(session, i == m.cursor, listW)
		listB.WriteString(item + "\n")
	}

	// List footer
	listB.WriteString(RenderCompactFooter([]ActionHint{
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Select"},
		{Key: "d", Desc: "Delete"},
		{Key: "e", Desc: "Export"},
		{Key: "c", Desc: "Copy"},
		{Key: "r", Desc: "Refresh"},
	}))

	listContent := lipgloss.NewStyle().Width(listW).Height(contentHeight - 2).Render(listB.String())

	// Render detail
	detailContent := ""
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		detailStr := m.renderSessionDetail(m.sessions[m.cursor])
		detailContent = DetailPanelStyle.Width(detailW).Height(contentHeight - 4).Render(detailStr)
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listContent, detailContent))

	return b.String()
}

func (m SessionsModel) renderSessionItem(session SessionInfo, selected bool, width int) string {
	prefix := IndicatorUnselected
	style := ListItemStyle

	if selected {
		prefix = IndicatorSelected
		style = ListSelectedStyle
	}

	// Build label
	label := session.Title
	if label == "" {
		label = fmt.Sprintf("Session %s", session.ID[:8])
	}
	if len(label) > width-12 {
		label = label[:width-15] + "..."
	}

	line := style.Render(prefix + label)

	// Status indicator
	status := StatusNeutral
	if session.IsActive {
		status = StatusRunning
	}
	statusStr := RenderStatusBadge(status)
	if statusStr != "" {
		line += " " + statusStr
	}

	return line
}

func (m SessionsModel) renderSessionDetail(session SessionInfo) string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("Session Details"))
	b.WriteString("\n\n")

	// ID
	b.WriteString(RenderField("ID", session.ID[:16]+"..."))
	b.WriteString("\n")

	// Title
	title := session.Title
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString(RenderField("Title", title))
	b.WriteString("\n")

	// Model
	b.WriteString(RenderField("Model", session.Model))
	b.WriteString("\n\n")

	// Stats
	b.WriteString(HeaderTertiary.Render("Statistics"))
	b.WriteString("\n")
	b.WriteString(RenderField("Messages", fmt.Sprintf("%d", session.MessageCount)))
	b.WriteString("\n")
	b.WriteString(RenderField("Turns", fmt.Sprintf("%d", session.Turns)))
	b.WriteString("\n\n")

	// Timestamps
	b.WriteString(HeaderTertiary.Render("Timestamps"))
	b.WriteString("\n")
	b.WriteString(RenderField("Created", session.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")
	b.WriteString(RenderField("Updated", session.UpdatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")

	return b.String()
}

// Focus focuses the sessions view.
func (m *SessionsModel) Focus() {
	m.focused = true
}

// Blur blurs the sessions view.
func (m *SessionsModel) Blur() {
	m.focused = false
}

// ConsumesTab returns whether this view consumes Tab key.
func (m SessionsModel) ConsumesTab() bool {
	return false
}

// ConsumesEsc returns whether this view consumes Esc key.
func (m SessionsModel) ConsumesEsc() bool {
	return false
}

// Scroll scrolls the list and viewport.
// CRITICAL FIX: Ensures selected item is visible
func (m *SessionsModel) Scroll(lines int) {
	oldCursor := m.cursor
	if lines > 0 {
		for i := 0; i < lines && m.cursor < len(m.sessions)-1; i++ {
			m.cursor++
		}
	} else {
		for i := 0; i < -lines && m.cursor > 0; i++ {
			m.cursor--
		}
	}
	// Sync viewport to keep cursor visible (approx 2 lines per item)
	if m.cursor != oldCursor {
		cursorLine := m.cursor * 2
		if cursorLine < m.viewport.YOffset {
			m.viewport.SetYOffset(cursorLine)
		}
		viewportBottom := m.viewport.YOffset + m.viewport.Height
		if cursorLine+2 > viewportBottom {
			newOffset := cursorLine + 2 - m.viewport.Height
			if newOffset < 0 {
				newOffset = 0
			}
			m.viewport.SetYOffset(newOffset)
		}
	}
}

// GotoTop scrolls to top.
func (m *SessionsModel) GotoTop() {
	m.cursor = 0
}

// GotoBottom scrolls to bottom.
func (m *SessionsModel) GotoBottom() {
	if len(m.sessions) > 0 {
		m.cursor = len(m.sessions) - 1
	}
}

// Helper function (copied from components.go)
func RenderField(label, value string) string {
	return fmt.Sprintf("  %s %s",
		DataLabel.Render(fmt.Sprintf("%-14s", label)),
		DataValue.Render(value))
}
