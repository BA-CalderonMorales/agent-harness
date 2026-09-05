// Command approval dialog for interactive mode
// Displays pending commands with accept/reject/approve-all/reject-all options

package tui

import (
	"strings"

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

	// scroll offsets the command/preview detail when the dialog is
	// taller than the terminal: the command is never clipped off screen,
	// the decision options stay pinned.
	scroll int
	// Suggest mode: the "Reject + Suggest" option opens a one-line
	// input; the text is delivered to the agent as the deny reason.
	suggestMode bool
	suggestBuf  string

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
			Description: "tell the agent what to do instead",
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
	m.suggestMode = false
	m.suggestBuf = ""
	m.scroll = 0
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

// respond delivers the chosen option: it answers the waiting goroutine
// (the live path) and emits the decision message. One home for the
// respond+hide pair — the old code repeated it three times and the
// variants drifted.
func (m ApprovalDialogModel) respond(opt ApprovalOption) (ApprovalDialogModel, tea.Cmd) {
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

// respondSuggest delivers a Reject + Suggest decision carrying the
// user's free-text note to the waiting goroutine.
func (m ApprovalDialogModel) respondSuggest() (ApprovalDialogModel, tea.Cmd) {
	var reqID string
	if m.request != nil {
		m.request.RespondSuggest(approval.DecisionRejectAll, strings.TrimSpace(m.suggestBuf))
		reqID = m.request.Command.ID
	}
	m.Hide()
	return m, func() tea.Msg {
		return ApprovalDecisionMsg{Decision: approval.DecisionRejectAll, RequestID: reqID}
	}
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
		// Suggest mode captures typing until Enter (send) or Esc (back).
		if m.suggestMode {
			switch msg.Type {
			case tea.KeyEnter:
				return m.respondSuggest()
			case tea.KeyEsc:
				m.suggestMode = false
				m.suggestBuf = ""
				return m, nil
			case tea.KeyBackspace:
				if len(m.suggestBuf) > 0 {
					runes := []rune(m.suggestBuf)
					m.suggestBuf = string(runes[:len(runes)-1])
				}
				return m, nil
			case tea.KeyRunes, tea.KeySpace:
				m.suggestBuf += msg.String()
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			// Reject on escape
			return m.respond(m.options[2])

		case tea.KeyEnter:
			// Confirm selection
			if m.selected < len(m.options) {
				return m.respond(m.options[m.selected])
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

		case tea.KeyRunes:
			// j/k scroll the command detail when it outgrows the
			// terminal — the decision options stay pinned either way.
			switch string(msg.Runes) {
			case "j":
				m.scroll++
			case "k":
				if m.scroll > 0 {
					m.scroll--
				}
			}
		}

		// Check for direct key presses. Number keys confirm on press —
		// moving the cursor and forcing a second Enter made the advertised
		// "1." shortcut a lie for keyboard-first users. "Reject + Suggest"
		// opens the message input instead of an instant reject: the label
		// promised a suggestion, so the flow must collect one.
		key := msg.String()
		for _, opt := range m.options {
			if opt.Key == key {
				if opt.Decision == approval.DecisionRejectAll {
					m.suggestMode = true
					return m, nil
				}
				return m.respond(opt)
			}
		}
		if len(key) == 1 && key[0] >= '1' && key[0] <= '0'+byte(len(m.options)) {
			opt := m.options[key[0]-'1']
			if opt.Decision == approval.DecisionRejectAll {
				m.suggestMode = true
				return m, nil
			}
			return m.respond(opt)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}
