// Command approval dialog for interactive mode
// Displays pending commands with accept/reject/approve-all/reject-all options

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

// Init initializes the model
func (m ApprovalDialogModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ApprovalDialogModel) Update(msg tea.Msg) (ApprovalDialogModel, tea.Cmd) {
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
		key := strings.ToLower(msg.String())
		for i, opt := range m.options {
			if strings.ToLower(opt.Key) == key {
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

// View renders the approval dialog
func (m ApprovalDialogModel) View() string {
	if !m.visible && !m.IsShowingNotification() {
		return ""
	}

	if !m.visible && m.IsShowingNotification() {
		// Render notification bar
		return m.renderNotification()
	}

	if m.request == nil {
		return ""
	}

	// Build the dialog content
	var sections []string

	// Title
	title := TitleStyle.Render("Command Approval Required")
	sections = append(sections, title)

	// Command display
	cmd := m.request.Command
	cmdDisplay := m.renderCommandDisplay(cmd)
	sections = append(sections, cmdDisplay)

	// Options
	optionsDisplay := m.renderOptions()
	sections = append(sections, optionsDisplay)

	// Help text
	help := HelpDimStyle.Render("Use arrow keys to navigate, Enter to confirm, ESC to reject")
	sections = append(sections, help)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Create modal dialog
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Background(ColorSurface).
		Padding(2).
		Width(m.width - 10)

	dialog := dialogStyle.Render(content)

	// Center the dialog
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (m ApprovalDialogModel) renderCommandDisplay(cmd approval.CommandInfo) string {
	var sections []string

	// Tool name and warning
	header := ToolCallStyle.Render("[" + cmd.DisplayName + "]")
	if cmd.IsDestructive {
		header = ToolErrorStyle.Render("[" + cmd.DisplayName + " - DESTRUCTIVE]")
	}
	sections = append(sections, header)

	// Command itself in code block style
	cmdStyle := lipgloss.NewStyle().
		Background(ColorHighlight).
		Foreground(ColorText).
		Padding(1, 2).
		Width(m.width - 16)

	cmdText := cmd.Command
	if cmdText == "" {
		cmdText = "(no command details)"
	}

	// Wrap long commands
	wrapped := wrapText(cmdText, m.width-20)
	sections = append(sections, cmdStyle.Render(wrapped))

	// Description if available
	if cmd.Description != "" {
		desc := HelpDimStyle.Render(cmd.Description)
		sections = append(sections, desc)
	}

	// Preview of what will change
	if cmd.Preview != "" {
		sections = append(sections, "")
		previewHeader := WarningStyle.Render("Preview of changes:")
		sections = append(sections, previewHeader)

		previewStyle := lipgloss.NewStyle().
			Background(ColorHighlight).
			Foreground(ColorText).
			Padding(1, 2).
			Width(m.width - 16)

		wrappedPreview := wrapText(cmd.Preview, m.width-20)
		sections = append(sections, previewStyle.Render(wrappedPreview))
	}

	// Risk assessment for bash commands
	if cmd.ToolName == "bash" || cmd.ToolName == "shell" {
		risk := m.assessRisk(cmd.Command)
		if risk != "" {
			sections = append(sections, "")
			sections = append(sections, WarningStyle.Render("Risk assessment: "+risk))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m ApprovalDialogModel) renderOptions() string {
	var options []string

	for i, opt := range m.options {
		selected := i == m.selected
		num := i + 1

		var style lipgloss.Style
		if selected {
			if opt.IsDangerous {
				style = lipgloss.NewStyle().
					Background(ColorError).
					Foreground(ColorText).
					Padding(0, 1).
					Bold(true)
			} else {
				style = lipgloss.NewStyle().
					Background(ColorPrimary).
					Foreground(ColorText).
					Padding(0, 1).
					Bold(true)
			}
		} else {
			if opt.IsDangerous {
				style = lipgloss.NewStyle().
					Foreground(ColorError).
					Padding(0, 1)
			} else {
				style = lipgloss.NewStyle().
					Foreground(ColorText).
					Padding(0, 1)
			}
		}

		label := fmt.Sprintf("%d. %s (%s)", num, opt.Label, opt.Key)
		if selected {
			label = "> " + label
		} else {
			label = "  " + label
		}

		option := style.Render(label)
		options = append(options, option)
	}

	return lipgloss.JoinVertical(lipgloss.Left, options...)
}

func (m ApprovalDialogModel) renderNotification() string {
	if !m.IsShowingNotification() || m.notification == "" {
		return ""
	}

	style := lipgloss.NewStyle().
		Background(ColorInfo).
		Foreground(ColorText).
		Padding(0, 1).
		Width(m.width)

	return style.Render(m.notification)
}

// ApprovalDecisionMsg is sent when a decision is made
type ApprovalDecisionMsg struct {
	Decision  approval.Decision
	RequestID string
}

// assessRisk evaluates the danger level of a bash command.
func (m ApprovalDialogModel) assessRisk(command string) string {
	cmd := strings.ToLower(command)
	switch {
	case strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "rm -fr"):
		return "HIGH — recursive deletion detected"
	case strings.Contains(cmd, "rm "):
		return "MEDIUM — file deletion detected"
	case strings.Contains(cmd, "dd "):
		return "HIGH — disk write detected"
	case strings.Contains(cmd, "> /dev/") || strings.Contains(cmd, ">/dev/"):
		return "HIGH — direct device access detected"
	case strings.Contains(cmd, "curl") && strings.Contains(cmd, "|"):
		return "HIGH — piped network download detected"
	case strings.Contains(cmd, "sudo") || strings.Contains(cmd, "su -"):
		return "MEDIUM — privilege escalation detected"
	case strings.Contains(cmd, "chmod") || strings.Contains(cmd, "chown"):
		return "LOW — permission modification detected"
	default:
		return ""
	}
}

// wrapText wraps text to a maximum width
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= maxWidth {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Wrap long lines
		for len(line) > maxWidth {
			// Find break point
			breakAt := maxWidth
			for breakAt > 0 && line[breakAt] != ' ' {
				breakAt--
			}
			if breakAt == 0 {
				breakAt = maxWidth // Force break
			}

			result.WriteString(line[:breakAt])
			result.WriteString("\n")
			line = strings.TrimSpace(line[breakAt:])
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}
