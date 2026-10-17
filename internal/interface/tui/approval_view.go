package tui

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/charmbracelet/lipgloss"
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
	return renderModal(m.width, m.height, m.modalSpec())
}

// dialogWidth is the frame's outer width, resolved by the shared modal
// width policy so the approval prompt matches every other overlay
// instead of carrying its own 66-column rule.
func (m ApprovalDialogModel) dialogWidth() int {
	return panelWidth(modalMaxWidth, m.width)
}

// visibleDetailRows uses the shared frame budget after reserving the
// decision choices or suggestion input.
func (m ApprovalDialogModel) visibleDetailRows() int {
	reserved := len(m.options) + 2 // decision rows and window markers
	if m.suggestMode {
		reserved = 4 // suggestion prompt/input and window markers
	}
	rows := modalBodyRows(m.width, m.height, m.modalBaseSpec()) - reserved
	if rows < 0 {
		return 0
	}
	return rows
}

func (m ApprovalDialogModel) modalBaseSpec() modalSpec {
	title := "Approval required"
	if m.request != nil && m.request.Command.IsDestructive {
		title += " · destructive"
	}
	if m.request != nil && m.request.Command.DisplayName != "" {
		title += " · " + m.request.Command.DisplayName
	}
	footer := "1-4 instant · esc rejects · j/k detail"
	if m.suggestMode {
		footer = "Enter: send rejection + message · Esc: back to options"
	}
	return modalSpec{title: title, footer: footer, preferredWidth: modalMaxWidth}
}

func (m ApprovalDialogModel) modalSpec() modalSpec {
	spec := m.modalBaseSpec()
	detailLines := m.detailLines()
	visibleRows := m.visibleDetailRows()
	maxScroll := len(detailLines) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	start := m.scroll
	if start < 0 {
		start = 0
	}
	if start > maxScroll {
		start = maxScroll
	}
	end := start + visibleRows
	if end > len(detailLines) {
		end = len(detailLines)
	}
	for _, line := range detailLines[start:end] {
		spec.items = append(spec.items, modalItem{lines: []string{line}})
	}

	if m.suggestMode {
		spec.items = append(spec.items,
			modalTextItem(InfoStyle.Render("Message to agent (what to do instead):")),
			modalTextItem(ListSelectedStyle.Render(m.suggestBuf+"▏")),
		)
		spec.cursor = len(spec.items) - 1
		return spec
	}

	for i := range m.options {
		spec.items = append(spec.items, modalTextItem(m.renderOption(i)))
	}
	spec.cursor = len(spec.items) - len(m.options) + m.selected
	return spec
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

func (m ApprovalDialogModel) renderOption(i int) string {
	opt := m.options[i]
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

	return row
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
