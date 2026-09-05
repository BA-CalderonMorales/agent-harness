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

	// Title row: what is being asked, and of which tool. Destructive
	// commands wear the warning, never a decoration.
	title := TitleStyle.Render("Approval required")
	if m.request.Command.IsDestructive {
		title = WarningStyle.Render("Approval required · destructive")
	}
	tool := ToolCallStyle.Render(" · " + m.request.Command.DisplayName)
	sections = append(sections, title+tool)

	sections = append(sections, "")

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
		help := HelpDimStyle.Render(fitBlock(m.dialogWidth()-4, "Enter: send rejection + message · Esc: back to options"))
		sections = append(sections, prompt, input, help)
	} else {
		// Options
		optionsDisplay := m.renderOptions()
		sections = append(sections, optionsDisplay)

		// Help text — short enough to hold one row at dialog width,
		// wrapped where it must. A hint that splits mid-word is strain,
		// not help.
		sections = append(sections, "")
		help := HelpDimStyle.Render(fitBlock(m.dialogWidth()-4, "1-4 instant · esc rejects · j/k detail"))
		sections = append(sections, help)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// The frame hugs its content up to a comfortable reading width and
	// yields to the pane below it — a modal that touches the pane edges
	// on a phone reads as broken, and a 100-column slab on desktop
	// reads as lazy.
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(m.dialogWidth())

	dialog := dialogStyle.Render(content)

	// Center the dialog
	return placeOverlay(m.width, m.height, dialog)
}

// dialogWidth is the frame's outer width: a comfortable read on
// desktop, pane-bound on a phone.
func (m ApprovalDialogModel) dialogWidth() int {
	w := 66
	if m.width-2 < w {
		w = m.width - 2
	}
	if w < 10 {
		w = 10
	}
	return w
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
	// Widths derive from the dialog, never the pane: the frame yields
	// to the pane, and everything inside it follows the frame.
	inner := m.dialogWidth() - 4
	textW := inner - 2
	if textW < 10 {
		textW = 10
	}

	var sections []string

	// The command is the thing being approved: prompt glyph + bright
	// text, wrapped to the frame. No background fill — the command
	// reads on the terminal's own background like every other block.
	cmdText := cmd.Command
	if cmdText == "" {
		cmdText = "(no command details)"
	}
	wrapped := fitBlockCode(textW, cmdText)
	prompt := PromptStyle.Render("$")
	sections = append(sections, prompt+" "+CodeStringStyle.Render(wrapped))

	// Description if available — bash tools echo the command as the
	// description, and showing the command twice asks the reader to
	// diff it; suppress the echo.
	if cmd.Description != "" && cmd.Description != cmd.Command {
		desc := HelpDimStyle.Render(fitBlock(inner, cmd.Description))
		sections = append(sections, desc)
	}

	// Preview of what will change
	if cmd.Preview != "" {
		sections = append(sections, "")
		sections = append(sections, WarningStyle.Render("Preview of changes:"))
		sections = append(sections, HelpDimStyle.Render(fitBlockCode(inner, wrapText(cmd.Preview, textW))))
	}

	// Risk assessment for shell commands
	if cmd.ToolName == "bash" || cmd.ToolName == "shell" || cmd.ToolName == "execute_command" || cmd.ToolName == "exec" {
		risk := m.assessRisk(cmd.Command)
		if risk != "" {
			sections = append(sections, "")
			sections = append(sections, m.riskStyle(risk).Render(fitBlock(inner, risk)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// riskStyle wears the severity: HIGH burns, MEDIUM warns, LOW dims.
func (m ApprovalDialogModel) riskStyle(risk string) lipgloss.Style {
	switch {
	case strings.HasPrefix(risk, "HIGH"):
		return ErrorStyle
	case strings.HasPrefix(risk, "MEDIUM"):
		return WarningStyle
	default:
		return HelpDimStyle
	}
}

func (m ApprovalDialogModel) renderOptions() string {
	var options []string

	for i, opt := range m.options {
		selected := i == m.selected

		// Keycap + verb: the key is the affordance (instant), the
		// number is the fallback. The description rides along when the
		// frame is wide enough to carry it without wrapping.
		key := PromptStyle.Render(opt.Key)
		label := opt.Label
		row := fmt.Sprintf("%s · %s", key, label)
		if selected {
			row = ModePromptStyle.Render("▸ " + row)
		} else {
			row = "  " + row
		}
		if opt.Description != "" && m.dialogWidth() >= 52 {
			row += HelpDimStyle.Render(" — " + opt.Description)
		}
		if opt.IsDangerous {
			row = ErrorStyle.Render(row)
		} else if selected {
			row = ModePromptStyle.Render(row)
		}
		row = fitBlock(m.dialogWidth()-4, row)

		option := row
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
