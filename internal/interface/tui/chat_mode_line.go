package tui

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// effortChip names the setting as well as its value.
func effortChip(effort string) string {
	label := "[effort: " + effort + "]"
	switch effort {
	case "low":
		return HelpDimStyle.Render(label)
	case "high":
		return InfoStyle.Render(label)
	default:
		return label
	}
}

// renderModeLine reserves the next-action hint before fitting metadata.
// Whole fields yield as the pane narrows; a setting never becomes a
// clipped fragment and the stop key stays visible during a turn.
func (m ChatModel) renderModeLine() string {
	mode := m.modeLabel
	if mode == "" {
		mode = "navigate"
	}
	effort := m.effort
	if effort == "" {
		effort = "medium"
	}

	fields := []string{ModePromptStyle.Render(mode)}
	for _, field := range []string{
		ShortenModelName(m.model), connectionSummary(m.provider),
		approvalModeLabel(m.agentMode), m.persona, effortChip(effort),
	} {
		fields = append(fields, field)
	}
	width := m.composerWidth()
	if width <= 0 {
		width = 80
	}
	hint := m.composerActionHint()
	// Extended typing help is optional; preserve the model and effort first.
	core := lipgloss.Width(mode) + lipgloss.Width(fields[1]) + lipgloss.Width(fields[5]) + 6
	if lipgloss.Width(hint)+2+core > width && m.focused && !m.turnInFlight() {
		hint = `"Esc" to navigate`
	}
	if lipgloss.Width(hint)+2+lipgloss.Width(mode)+3+lipgloss.Width(fields[5]) > width && !m.turnInFlight() {
		hint = ""
	}
	// If even the mode and hint cannot share a row, the mode wins.
	available := width
	if hint != "" {
		available -= lipgloss.Width(hint) + 2
	}
	if available < lipgloss.Width(mode) {
		hint, available = "", width
	}
	join := func() string {
		var visible []string
		for _, field := range fields {
			if field != "" {
				visible = append(visible, field)
			}
		}
		return strings.Join(visible, " · ")
	}
	line := join()
	// Persona and connection yield first. Model identity, effort and
	// approval state remain useful longer. During a turn the stop hint
	// takes priority over optional settings.
	for _, i := range []int{4, 2, 3, 1, 5} {
		if lipgloss.Width(line) <= available {
			break
		}
		fields[i] = ""
		line = join()
	}
	line = InputMetaStyle.Render(ansi.Truncate(line, available, ""))
	if hint != "" {
		gap := max(2, width-lipgloss.Width(line)-lipgloss.Width(hint))
		line += strings.Repeat(" ", gap) + HelpDimStyle.Render(hint)
	}
	return line
}

// connectionSummary uses the same connection names as /login.
func connectionSummary(provider string) string {
	if provider == "" {
		return ""
	}
	kind := "hosted"
	if config.IsLocalProvider(provider) {
		kind = "local"
	}
	return kind + ": " + connectionLabel(provider)
}

// Approval labels explain the mode without changing its permission gates.
func approvalModeLabel(mode string) string {
	switch mode {
	case "manual":
		return ModePromptStyle.Render("manual: ask")
	case "auto":
		return WarningStyle.Render("auto: no prompts")
	case "plan":
		return ModePromptStyle.Render("plan: read only")
	case "chat":
		return ModePromptStyle.Render("chat: no tools")
	default:
		return mode
	}
}

func (m ChatModel) composerActionHint() string {
	if m.turnInFlight() {
		return `"Esc" to stop`
	}
	if m.focused {
		if m.width >= 140 {
			return `Enter send · Alt+Enter newline · "Esc" to navigate`
		}
		if m.width >= 78 {
			return `Enter send · "Esc" to navigate`
		}
		return `"Esc" to navigate`
	}
	if isMobilePane(m.width) {
		return "tap to type"
	}
	return `"i" to type`
}
