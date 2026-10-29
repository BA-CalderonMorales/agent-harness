package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// effortChip renders the reasoning effort as a self-describing chip: the
// axis the user can act on and the level it is set to, named together.
//
// The level also carries its own emphasis — quiet at low, the rail's own
// weight at medium, accented at high — so the ramp from reserved to
// generous reads at a glance instead of only through the word. That is
// what the bare "effort medium" form failed to do: it named the axis but
// looked like two more values, so the line read as noise.
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

// renderModeLine renders the mode · model · provider · reasoning-effort line
// shown under the input, mirroring modern composer status rows.
func (m ChatModel) renderModeLine() string {
	// The mode is always the first beat of the line: vim-style users read
	// navigate/typing instantly, and the persona never hides it.
	mode := m.modeLabel
	if mode == "" {
		mode = "navigate"
	}
	modeBit := ModePromptStyle.Render(mode)

	// Segment priority when the pane narrows: the mode leads, the
	// model identifies what is answering; persona, effort, and provider
	// yield in that order. A clipped mid-word line helps nobody.
	type segment struct {
		width int
		text  string
		style bool
	}
	segments := []segment{}
	if m.persona != "" {
		segments = append(segments, segment{lipgloss.Width(m.persona), m.persona, false})
	}
	if m.model != "" {
		name := ShortenModelName(m.model)
		segments = append(segments, segment{lipgloss.Width(name), name, false})
	}
	if m.provider != "" {
		segments = append(segments, segment{lipgloss.Width(m.provider), m.provider, false})
	}
	if m.agentMode != "" {
		segments = append(segments, segment{lipgloss.Width(m.agentMode), m.agentMode, true})
	}
	// Reasoning effort is the one segment that cannot read as a bare
	// value: "medium" alone names no axis, and gluing the label onto it
	// left a two-word fragment — "effort medium" — sitting in a rail of
	// single values. The setting was on screen and unreadable. It renders
	// as a self-describing chip instead, in the same bracket language the
	// rest of the app uses for state ([ready], [running], [on]), so the
	// label reads as part of the chip rather than as a stray word.
	effort := m.effort
	if effort == "" {
		effort = "medium"
	}
	chip := effortChip(effort)
	// lipgloss.Width measures the visible text, so the styled chip still
	// reports the width the budget math must reserve for it.
	segments = append(segments, segment{lipgloss.Width(chip), chip, false})

	width := m.composerWidth()
	budget := width - lipgloss.Width(mode)
	keep := make([]bool, len(segments))
	for i := range segments {
		keep[i] = true
		budget -= 3 + segments[i].width // separator + segment
	}
	// Keep the mode and effort signal useful on a narrow pane. Drop
	// optional context first: provider, persona, then model. Effort is
	// the last metadata field to disappear because it describes the
	// active behavior rather than the selected implementation.
	for _, i := range []int{2, 0, 1, 3} {
		if budget >= 0 {
			break
		}
		if i >= len(segments) {
			continue
		}
		if keep[i] {
			keep[i] = false
			budget += segments[i].width + 3
		}
	}
	parts := []string{modeBit}
	for i := range segments {
		if !keep[i] {
			continue
		}
		text := segments[i].text
		switch {
		case segments[i].style && text == "auto":
			// Yolo wears a warning: tools run without asking, and the
			// chip should read as a state, not a decoration.
			text = WarningStyle.Render(text)
		case segments[i].style:
			text = ModePromptStyle.Render(text)
		}
		parts = append(parts, text)
	}
	line := InputMetaStyle.Render(strings.Join(parts, " · "))

	// Symmetric hints on the same row, right-aligned. While typing,
	// "Esc" is the exit hatch; while navigating, "i" (or a tap on
	// mobile panes, which have no `i` affordance) re-enters typing.
	if m.focused {
		// While a turn runs, Esc stops it instead of merely leaving
		// the composer; the hint must say what the key actually does.
		label := `"Esc" to navigate`
		if m.turnInFlight() {
			label = `"Esc" to stop`
		}
		hint := HelpDimStyle.Render(label)
		pad := width - lipgloss.Width(line) - lipgloss.Width(hint)
		if pad > 0 {
			line += strings.Repeat(" ", pad) + hint
		}
	} else {
		word := `"i" to type`
		if isMobilePane(m.width) {
			// Mobile panes enter the composer by tap-to-type
			// (chat_update.go mouse handling), not via `i`.
			word = `tap to type`
		}
		hint := HelpDimStyle.Render(word)
		pad := width - lipgloss.Width(line) - lipgloss.Width(hint)
		if pad > 0 {
			line += strings.Repeat(" ", pad) + hint
		}
	}
	// A wrapped row is unbudgeted height: truncation keeps the frame
	// inside the terminal on narrow screens.
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}
