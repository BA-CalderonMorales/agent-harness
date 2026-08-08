// Command approval dialog for interactive mode
// Displays pending commands with accept/reject/approve-all/reject-all options

package tui

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

// ApprovalOption represents a user-selectable option
type ApprovalOption struct {
	Label       string
	Key         string
	Description string
	Decision    approval.Decision
	IsDangerous bool // For styling destructive actions
}

// ApprovalDialogModel manages the command approval UI
type ApprovalDialogModel struct {
	width    int
	height   int
	visible  bool
	request  *approval.ApprovalRequest
	options  []ApprovalOption
	selected int

	// For yolo mode notification
	notification      string
	notificationUntil time.Time
}

// NewApprovalDialog creates a new approval dialog
func NewApprovalDialog() ApprovalDialogModel {
	return ApprovalDialogModel{
		options: defaultApprovalOptions(),
	}
}

func defaultApprovalOptions() []ApprovalOption {
	return []ApprovalOption{
		{
			Label:       "Approve",
			Key:         "a",
			Description: "Run this command",
			Decision:    approval.DecisionApprove,
			IsDangerous: false,
		},
		{
			Label:       "Approve All",
			Key:         "A",
			Description: "Always run similar commands",
			Decision:    approval.DecisionApproveAll,
			IsDangerous: false,
		},
		{
			Label:       "Reject",
			Key:         "r",
			Description: "Skip this command",
			Decision:    approval.DecisionReject,
			IsDangerous: true,
		},
		{
			Label:       "Reject + Suggest",
			Key:         "R",
			Description: "Skip and tell agent what to do instead",
			Decision:    approval.DecisionRejectAll,
			IsDangerous: true,
		},
	}
}

// Show displays the approval dialog for a command
func (m *ApprovalDialogModel) Show(req *approval.ApprovalRequest) {
	m.visible = true
	m.request = req
	m.selected = 0
}

// Hide hides the approval dialog
func (m *ApprovalDialogModel) Hide() {
	m.visible = false
	m.request = nil
}

// IsVisible returns true if the dialog is showing
func (m ApprovalDialogModel) IsVisible() bool {
	return m.visible
}

// GetRequest returns the current approval request
func (m ApprovalDialogModel) GetRequest() *approval.ApprovalRequest {
	return m.request
}

// ShowNotification shows a brief notification (for yolo mode)
func (m *ApprovalDialogModel) ShowNotification(msg string) {
	m.notification = msg
	m.notificationUntil = time.Now().Add(3 * time.Second)
}

// IsShowingNotification returns true if a notification is active
func (m ApprovalDialogModel) IsShowingNotification() bool {
	return time.Now().Before(m.notificationUntil)
}

// ClearNotification clears the notification
func (m *ApprovalDialogModel) ClearNotification() {
	m.notification = ""
	m.notificationUntil = time.Time{}
}

// Init initializes the model
func (m ApprovalDialogModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ApprovalDialogModel) Update(msg tea.Msg) (ApprovalDialogModel, tea.Cmd) {
	// Always process window size updates, even when not visible
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	if !m.visible {
		// Handle notification timeout
		if m.IsShowingNotification() {
			switch msg.(type) {
			case tea.KeyMsg:
				// Any key clears notification
				m.ClearNotification()
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			// Reject on escape
			var reqID string
			if m.request != nil {
				m.request.Respond(approval.DecisionReject)
				reqID = m.request.Command.ID
			}
			m.Hide()
			return m, func() tea.Msg {
				return ApprovalDecisionMsg{Decision: approval.DecisionReject, RequestID: reqID}
			}

		case tea.KeyEnter:
			// Confirm selection
			if m.selected < len(m.options) {
				opt := m.options[m.selected]
				var reqID string
				if m.request != nil {
					m.request.Respond(opt.Decision)
					reqID = m.request.Command.ID
				}
				m.Hide()
				return m, func() tea.Msg {
					return ApprovalDecisionMsg{Decision: opt.Decision, RequestID: reqID}
				}
			}

		case tea.KeyUp, tea.KeyLeft:
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.options) - 1
			}

		case tea.KeyDown, tea.KeyRight:
			m.selected++
			if m.selected >= len(m.options) {
				m.selected = 0
			}
		}

		// Check for direct key presses
		key := msg.String()
		for i, opt := range m.options {
			if opt.Key == key {
				var reqID string
				if m.request != nil {
					m.request.Respond(opt.Decision)
					reqID = m.request.Command.ID
				}
				m.Hide()
				return m, func() tea.Msg {
					return ApprovalDecisionMsg{Decision: opt.Decision, RequestID: reqID}
				}
			}
			// Also check if user pressed the numbered key
			if msg.String() == string(rune('1'+i)) {
				m.selected = i
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}
