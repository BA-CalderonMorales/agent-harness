// Home dashboard view — project overview, quick actions, and contextual guidance

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// HomeDelegate handles actions from the home view
// ---------------------------------------------------------------------------
type HomeDelegate interface {
	OnNewChat()
	OnExportSession()
	OnLoadSession(id string)
	OnDeleteSession(id string)
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
	setupRequired   bool

	// deleting indexes the pending session deletion; -1 when idle. The
	// y/n confirm mirrors the Sessions tab — same verb, same safety.
	deleting int

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
		deleting:     -1,
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

// SetSetupRequired shows the setup banner when the provider probe reports
// a misconfigured state (no key/model), giving the dead end a handle.
func (m *HomeModel) SetSetupRequired(required bool) {
	m.setupRequired = required
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

		// While a deletion is pending, y commits and n/Esc cancels;
		// any other key keeps the pending state and navigates.
		if m.deleting >= 0 {
			switch msg.String() {
			case "y":
				if m.deleting < len(m.sessions) && m.delegate != nil {
					m.delegate.OnDeleteSession(m.sessions[m.deleting].ID)
				}
				m.deleting = -1
				return m, nil
			case "n", "esc":
				m.deleting = -1
				return m, nil
			}
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
		case "d":
			// Delete the session under the cursor (sessions region only;
			// action shortcuts keep precedence elsewhere). Asks first:
			// y commits, n/Esc backs out.
			if !m.cursorInActions() {
				idx := m.cursorSessionIndex()
				if idx >= 0 && idx < len(m.sessions) {
					m.deleting = idx
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

// CapturesAllKeys returns whether this view should receive all keys
// before global shortcuts are applied.
func (m HomeModel) CapturesAllKeys() bool {
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
