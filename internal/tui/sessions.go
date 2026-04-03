// Sessions view for managing chat sessions

package tui

import (
	"fmt"
	"strings"
	"time"

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
	width   int
	height  int
	sessions []SessionInfo
	cursor   int
	focused  bool
	loading  bool

	// Delegate
	delegate SessionsDelegate
}

// NewSessionsModel creates a new sessions model.
func NewSessionsModel() SessionsModel {
	return SessionsModel{
		sessions: make([]SessionInfo, 0),
		cursor:   0,
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
func (m SessionsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m SessionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
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

		case "r":
			if m.delegate != nil {
				m.delegate.OnSessionLoad()
			}
		}
	}

	return m, nil
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

	// Two-pane layout
	listW, detailW := TwoPaneWidths(m.width)

	// Render list
	var listB strings.Builder
	listB.WriteString(ListTitleStyle.Render("  Sessions") + "\n\n")

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
		{Key: "r", Desc: "Refresh"},
	}))

	listContent := lipgloss.NewStyle().Width(listW).Height(m.height - 2).Render(listB.String())

	// Render detail
	detailContent := ""
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		detailStr := m.renderSessionDetail(m.sessions[m.cursor])
		detailContent = DetailPanelStyle.Width(detailW).Height(m.height - 4).Render(detailStr)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, detailContent)
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

// Scroll scrolls the list.
func (m *SessionsModel) Scroll(lines int) {
	if lines > 0 {
		for i := 0; i < lines && m.cursor < len(m.sessions)-1; i++ {
			m.cursor++
		}
	} else {
		for i := 0; i < -lines && m.cursor > 0; i++ {
			m.cursor--
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
