// Home dashboard view — project overview, quick actions, and contextual guidance
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// HomeDelegate handles actions from the home view
// ---------------------------------------------------------------------------
type HomeDelegate interface {
	OnNewChat()
	OnExportSession()
	OnLoadSession(id string)
}

// ---------------------------------------------------------------------------
// ProjectInfo holds contextual project metadata
// ---------------------------------------------------------------------------
type ProjectInfo struct {
	Name             string
	Type             string // "Go", "Node", "Python", etc.
	GitBranch        string
	GitCommit        string
	HasChanges       bool
	UncommittedCount int
	LastCommitMsg    string
}

// ---------------------------------------------------------------------------
// HomeModel is the dashboard view model
// ---------------------------------------------------------------------------
type HomeModel struct {
	width           int
	height          int
	focused         bool
	project         ProjectInfo
	sessions        []SessionInfo
	model           string
	permissionMode  string
	persona         string
	estimatedTokens int

	// Quick action cursor (spans both actions and sessions)
	actionCursor int
	actions      []homeAction

	// Delegate
	delegate HomeDelegate
}

func (m *HomeModel) totalItems() int {
	return len(m.actions) + len(m.sessions)
}

func (m *HomeModel) cursorInActions() bool {
	return m.actionCursor < len(m.actions)
}

func (m *HomeModel) cursorSessionIndex() int {
	return m.actionCursor - len(m.actions)
}

func (m *HomeModel) clampCursor() {
	max := m.totalItems() - 1
	if max < 0 {
		max = 0
	}
	if m.actionCursor > max {
		m.actionCursor = max
	}
	if m.actionCursor < 0 {
		m.actionCursor = 0
	}
}

type homeAction struct {
	Label       string
	Key         string
	Description string
	Handler     func()
}

// NewHomeModel creates a new home dashboard model.
func NewHomeModel() HomeModel {
	return HomeModel{
		actionCursor: 0,
		actions:      make([]homeAction, 0),
	}
}

// SetDelegate sets the home delegate.
func (m *HomeModel) SetDelegate(delegate HomeDelegate) {
	m.delegate = delegate
}

// SetProjectInfo updates the project context.
func (m *HomeModel) SetProjectInfo(info ProjectInfo) {
	m.project = info
}

// SetSessions updates the recent sessions list.
func (m *HomeModel) SetSessions(sessions []SessionInfo) {
	m.sessions = sessions
	m.clampCursor()
}

// SetStatus updates the runtime status display.
func (m *HomeModel) SetStatus(model, permissionMode, persona string, estimatedTokens int) {
	m.model = model
	m.permissionMode = permissionMode
	m.persona = persona
	m.estimatedTokens = estimatedTokens
}

// SetSetupRequired shows a setup prompt when no credentials are configured.
func (m *HomeModel) SetSetupRequired(required bool) {
	// This is a no-op for now; the home view checks m.model == "" as a proxy
}

// Init initializes the home model.
func (m *HomeModel) Init() tea.Cmd {
	m.rebuildActions()
	return nil
}

// Update handles messages.
func (m *HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildActions()

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.actionCursor > 0 {
				m.actionCursor--
			}
		case "down", "j":
			if m.actionCursor < m.totalItems()-1 {
				m.actionCursor++
			}
		case "enter", " ":
			if m.cursorInActions() {
				if m.actionCursor < len(m.actions) && m.actions[m.actionCursor].Handler != nil {
					m.actions[m.actionCursor].Handler()
				}
			} else if m.delegate != nil {
				idx := m.cursorSessionIndex()
				if idx >= 0 && idx < len(m.sessions) {
					m.delegate.OnLoadSession(m.sessions[idx].ID)
				}
			}
		default:
			// Handle individual action shortcuts
			for _, action := range m.actions {
				if action.Key != "" && msg.String() == action.Key {
					if action.Handler != nil {
						action.Handler()
					}
					return m, nil
				}
			}
		}
	}

	return m, nil
}

// View renders the home dashboard.
func (m *HomeModel) View() string {
	if m.width == 0 {
		return "  Loading dashboard..."
	}

	var sections []string

	// Header
	sections = append(sections, RenderHeader(HeaderConfig{
		Title:    "Home",
		Subtitle: "Project dashboard",
	}))

	// Setup required banner
	if m.model == "" {
		sections = append(sections, m.renderSetupBanner())
	}

	// Project card
	sections = append(sections, m.renderProjectCard())

	// Quick actions
	sections = append(sections, m.renderQuickActions())

	// Recent sessions
	if len(m.sessions) > 0 {
		sections = append(sections, m.renderRecentSessions())
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Constrain to available height
	return lipgloss.NewStyle().Height(m.height).Render(content)
}

func (m *HomeModel) rebuildActions() {
	m.actions = []homeAction{
		{Label: "New chat", Key: "n", Description: "Start a fresh conversation", Handler: func() {
			if m.delegate != nil {
				m.delegate.OnNewChat()
			}
		}},
		{Label: "Export session", Key: "e", Description: "Save conversation to file", Handler: func() {
			if m.delegate != nil {
				m.delegate.OnExportSession()
			}
		}},
	}
	m.clampCursor()
}

func (m *HomeModel) renderProjectCard() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Project"))
	b.WriteString("\n\n")

	if m.project.Name != "" {
		b.WriteString(RenderField("Name", m.project.Name))
		b.WriteString("\n")
	}
	if m.project.Type != "" {
		b.WriteString(RenderField("Type", m.project.Type))
		b.WriteString("\n")
	}

	if m.project.GitBranch != "" {
		gitStatus := m.project.GitBranch
		if m.project.GitCommit != "" {
			commit := m.project.GitCommit
			if len(commit) > 7 {
				commit = commit[:7]
			}
			gitStatus += " @ " + commit
		}
		if m.project.HasChanges {
			gitStatus += fmt.Sprintf(" (%d uncommitted)", m.project.UncommittedCount)
		}
		b.WriteString(RenderField("Git", gitStatus))
		b.WriteString("\n")
		if m.project.LastCommitMsg != "" {
			b.WriteString(RenderField("Last commit", truncateString(m.project.LastCommitMsg, m.width-20)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(RenderField("Git", "not a repository"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderQuickActions() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Quick Actions"))
	b.WriteString("\n\n")

	for i, action := range m.actions {
		prefix := "  "
		style := ListItemStyle
		if i == m.actionCursor {
			prefix = IndicatorSelected + " "
			style = ListSelectedStyle
		}
		label := action.Label
		if action.Key != "" {
			label = fmt.Sprintf("%s (%s)", label, action.Key)
		}
		line := fmt.Sprintf("%s%s", prefix, label)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		b.WriteString(HelpDimStyle.Render(fmt.Sprintf("     %s", action.Description)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderRecentSessions() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Recent Sessions"))
	b.WriteString("\n\n")

	count := 3
	if len(m.sessions) < count {
		count = len(m.sessions)
	}

	for i := 0; i < count; i++ {
		s := m.sessions[i]
		label := s.Title
		if label == "" {
			label = fmt.Sprintf("Session %s", s.ID[:8])
		}
		marker := "  "
		style := ListItemStyle
		if s.IsActive {
			marker = IndicatorSelected + " "
			style = ListSelectedStyle
		}
		if m.cursorSessionIndex() == i {
			marker = IndicatorSelected + " "
			style = ListSelectedStyle
		}
		line := fmt.Sprintf("%s%s · %d msgs · %d turns", marker, label, s.MessageCount, s.Turns)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderSetupBanner() string {
	var b strings.Builder
	b.WriteString(ErrorStyle.Render("  [!] Setup Required"))
	b.WriteString("\n")
	b.WriteString(HelpDimStyle.Render("  No API key or model configured."))
	b.WriteString("\n")
	b.WriteString(HelpDimStyle.Render("  Run /login to authenticate, or set AH_API_KEY environment variable."))
	b.WriteString("\n\n")
	return b.String()
}

// Focus focuses the home view.
func (m *HomeModel) Focus() {
	m.focused = true
}

// Blur blurs the home view.
func (m *HomeModel) Blur() {
	m.focused = false
}

// ConsumesTab returns whether this view consumes Tab key.
func (m HomeModel) ConsumesTab() bool {
	return false
}

// ConsumesEsc returns whether this view consumes Esc key.
func (m HomeModel) ConsumesEsc() bool {
	return false
}

// Scroll scrolls the actions list.
func (m *HomeModel) Scroll(lines int) {
	m.actionCursor += lines
	m.clampCursor()
}

// GotoTop scrolls to top action.
func (m *HomeModel) GotoTop() {
	m.actionCursor = 0
}

// GotoBottom scrolls to bottom action.
func (m *HomeModel) GotoBottom() {
	if m.totalItems() > 0 {
		m.actionCursor = m.totalItems() - 1
	}
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
