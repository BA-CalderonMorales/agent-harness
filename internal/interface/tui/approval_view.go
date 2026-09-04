package tui

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

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

	// The dialog must fit the terminal at every size: title and the
	// decision options stay pinned, while the command/preview detail
	// scrolls between them (j/k). On a short terminal the old layout
	// clipped the top and bottom — exactly the command being approved
	// and the keys that answer it.
	var sections []string

	title := TitleStyle.Render("Command Approval Required")
	sections = append(sections, title)

	detailLines := m.detailLines()
	maxScroll := len(detailLines) - m.visibleDetailRows()
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.scroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + m.visibleDetailRows()
	if end > len(detailLines) {
		end = len(detailLines)
	}
	if scroll > end {
		scroll = end
	}
	sections = append(sections, detailLines[scroll:end]...)

	if m.suggestMode {
		// Reject + Suggest input: the text goes to the agent as the deny
		// reason, so it can adapt instead of retrying blind.
		prompt := InfoStyle.Render("Message to agent (what to do instead):")
		input := ListSelectedStyle.Render(m.suggestBuf + "▏")
		sections = append(sections, prompt, input)
		help := HelpDimStyle.Render("Enter: send rejection + message · Esc: back to options")
		sections = append(sections, help)
	} else {
		// Options
		optionsDisplay := m.renderOptions()
		sections = append(sections, optionsDisplay)

		// Help text
		help := HelpDimStyle.Render("Number/letter keys act instantly · Arrows + Enter also work · ESC rejects · j/k scroll")
		sections = append(sections, help)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Create modal dialog
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Background(ColorSurface).
		Padding(1, 2).
		Width(m.width - 10)

	dialog := dialogStyle.Render(content)

	// Center the dialog
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

// visibleDetailRows is how many detail lines fit between the pinned
// title and the pinned options at the current terminal height.
func (m ApprovalDialogModel) visibleDetailRows() int {
	rows := m.height - 13
	if rows < 3 {
		rows = 3
	}
	return rows
}

// detailLines renders the scrollable middle: the command block, its
// description, the change preview, and the risk note, wrapped to the
// dialog width.
func (m ApprovalDialogModel) detailLines() []string {
	if m.request == nil {
		return nil
	}
	var lines []string
	cmdDisplay := m.renderCommandDisplay(m.request.Command)
	lines = append(lines, strings.Split(cmdDisplay, "\n")...)
	return lines
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

		previewWidth := m.width - 16
		if previewWidth < 1 {
			previewWidth = 1
		}
		wrapWidth := m.width - 20
		if wrapWidth < 1 {
			wrapWidth = 1
		}

		previewStyle := lipgloss.NewStyle().
			Background(ColorHighlight).
			Foreground(ColorText).
			Padding(1, 2).
			Width(previewWidth)

		wrappedPreview := wrapText(cmd.Preview, wrapWidth)
		sections = append(sections, previewStyle.Render(wrappedPreview))
	}

	// Risk assessment for shell commands
	if cmd.ToolName == "bash" || cmd.ToolName == "shell" || cmd.ToolName == "execute_command" || cmd.ToolName == "exec" {
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
