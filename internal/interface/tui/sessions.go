// Sessions view for managing chat sessions

package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

// ---------------------------------------------------------------------------
// SessionsDelegate handles session actions
// ---------------------------------------------------------------------------
type SessionsDelegate interface {
	OnSessionSelect(id string)
	OnSessionDelete(id string)
	OnSessionExport(id string)
	OnSessionCopy(id string)
	OnSessionLoad()
	OnSessionNew()
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

	confirmingDelete bool
	deleteTargetIdx  int

	// notice is a transient operation result (deleted/exported/copied/
	// loaded) shown under the header until the user navigates.
	notice     string
	noticeType string

	delegate SessionsDelegate
}

// NewSessionsModel creates a new sessions model.
// CRITICAL FIX: Added viewport for proper scrolling
func NewSessionsModel() SessionsModel {
	return SessionsModel{
		sessions: make([]SessionInfo, 0),
		cursor:   0,
		viewport: newViewport(80, 20),
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

// SetNotice shows a transient operation result on the sessions page; it
// stays until the user navigates.
func (m *SessionsModel) SetNotice(text, noticeType string) {
	m.notice = text
	m.noticeType = noticeType
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
			newVP, cmd := m.viewport.Update(msg)
			m.viewport = newVP
			return m, cmd
		}

		if m.confirmingDelete {
			switch msg.String() {
			case "y", "enter":
				m.confirmingDelete = false
				if m.deleteTargetIdx >= 0 && m.deleteTargetIdx < len(m.sessions) && m.delegate != nil {
					m.delegate.OnSessionDelete(m.sessions[m.deleteTargetIdx].ID)
				}
			case "n", "esc":
				m.confirmingDelete = false
				m.deleteTargetIdx = -1
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			m.notice = ""
			if m.cursor > 0 {
				m.cursor--
				m.syncViewportToCursor()
			}

		case "down", "j":
			m.notice = ""
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.syncViewportToCursor()
			}

		case "enter", " ":
			if m.cursor < len(m.sessions) && m.delegate != nil {
				m.delegate.OnSessionSelect(m.sessions[m.cursor].ID)
			}

		case "n":
			if m.delegate != nil {
				m.delegate.OnSessionNew()
			}

		case "d":
			if m.cursor < len(m.sessions) {
				m.confirmingDelete = true
				m.deleteTargetIdx = m.cursor
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
func (m *SessionsModel) Focus() {
	m.focused = true
}

// Blur blurs the sessions view.
func (m *SessionsModel) Blur() {
	m.focused = false
}

// ConsumesTab returns whether this view consumes Tab key.
func (m SessionsModel) ConsumesTab() bool {
	return m.confirmingDelete
}

// ConsumesEsc returns whether this view consumes Esc key.
func (m SessionsModel) ConsumesEsc() bool {
	return m.confirmingDelete
}

// CapturesAllKeys returns whether this view should receive all keys
// before global shortcuts are applied.
func (m SessionsModel) CapturesAllKeys() bool {
	return m.confirmingDelete
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
