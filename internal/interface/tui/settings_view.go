package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// settingsDetailLines is the fixed height of the focused-row detail
// panel (divider + label + rationale). It is subtracted from the pane
// height alongside the header and footer when budgeting the viewport;
// keep it in sync with View and settings_update.go.
const settingsDetailLines = 3

// View renders the settings.
//
// The list is a column of one-line rows with aligned values, so the eye
// scans labels, not prose. The rationale for the focused row lives in a
// dedicated panel below the list — never crammed under the row, where it
// pushed the list around and read as part of the option.
func (m SettingsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if len(m.settings) == 0 {
		return RenderEmptyState(ViewPort{Width: m.width, Height: m.height}, EmptyState{
			Title:       "No Settings",
			Description: "Settings will appear here when available.",
			Actions: []ActionHint{
				{Key: "r", Desc: "Reload settings"},
			},
		})
	}

	var b strings.Builder

	// Header (always visible, not in viewport)
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Settings",
		Subtitle: "Choose a row, then change its value",
		Count:    -1, // no count: 17 settings is noise, not signal
	}))

	// Build the rows-only list for the viewport. Every row renders one
	// line (an editing row adds its input line); cursorLine tracks each
	// row's rendered line so the scroll can land the selection exactly
	// in view.
	var settingsContent strings.Builder
	currentCat := ""
	cursorLine := make([]int, len(m.settings))
	cursorRowLines := 1
	line := 0
	for i, setting := range m.settings {
		if setting.Category != "" && setting.Category != currentCat {
			if currentCat != "" {
				settingsContent.WriteString("\n")
				line++
			}
			currentCat = setting.Category
			settingsContent.WriteString(SectionHeaderStyle.Render("  " + currentCat))
			settingsContent.WriteString("\n")
			line++
		}
		cursorLine[i] = line
		row := m.renderSetting(setting, i == m.cursor)
		rowLines := strings.Count(row, "\n") + 1
		if i == m.cursor {
			cursorRowLines = rowLines
		}
		line += rowLines
		settingsContent.WriteString(row)
		settingsContent.WriteString("\n")
	}

	// Update viewport content, then land the scroll so the cursor row —
	// and its whole block — is on screen.
	m.viewport.SetContent(settingsContent.String())
	if m.cursor >= 0 && m.cursor < len(cursorLine) {
		totalLines := strings.Count(settingsContent.String(), "\n") + 1
		maxOffset := totalLines - m.viewport.Height
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.viewport.YOffset
		rowStart := cursorLine[m.cursor]
		rowEnd := rowStart + cursorRowLines
		if rowStart < offset {
			offset = rowStart
		} else if rowEnd > offset+m.viewport.Height {
			offset = rowEnd - m.viewport.Height
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}
		m.viewport.SetYOffset(offset)
	}

	// Render viewport (scrollable settings list). bubbles' viewport
	// does not end its output with a newline; without the terminator
	// the detail panel below concatenates onto the last padded row.
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Detail panel: the focused row's name and rationale, in one stable
	// place. Separated by a rule so it reads as context, not as part of
	// the highlighted option.
	b.WriteString(m.renderDetail())
	b.WriteString("\n")

	// Footer explains the action for the selected type.
	action := "Edit text"
	if m.cursor >= 0 && m.cursor < len(m.settings) {
		switch m.settings[m.cursor].Type {
		case "bool":
			action = "Toggle"
		case "choice":
			action = "Next choice"
		}
	}
	footerActions := []ActionHint{
		{Key: "↑/↓", Desc: "Move (wraps)"},
		{Key: "Enter/Space", Desc: action},
		{Key: "←/→", Desc: "Cycle choice"},
		{Key: "r", Desc: "Reload"},
	}
	if m.editing {
		footerActions = []ActionHint{
			{Key: "Enter", Desc: "Save"},
			{Key: "Esc", Desc: "Cancel"},
		}
		if m.editErr != "" {
			footerActions = append(footerActions, ActionHint{Key: "!", Desc: m.editErr})
		}
	}
	b.WriteString(RenderFooter(footerActions))

	return b.String()
}

// renderDetail renders the fixed-height detail panel for the focused
// row: a divider, the setting's name plus a short affordance hint, and
// its rationale clipped to one line. Always exactly settingsDetailLines
// rows so the list budget never drifts.
func (m SettingsModel) renderDetail() string {
	width := m.width
	if width < 12 {
		width = 12
	}
	rule := HelpDimStyle.Render(strings.Repeat("─", width))

	if m.cursor < 0 || m.cursor >= len(m.settings) {
		return rule + "\n\n"
	}
	s := m.settings[m.cursor]

	title := HelpKeyStyle.Render("  " + s.Label)
	if hint := settingActionHint(s); hint != "" {
		title += HelpDimStyle.Render("  ·  " + hint)
	}

	desc := s.Description
	if m.editing {
		desc = "Editing — Enter saves, Esc cancels."
	}
	if desc == "" {
		desc = "No description."
	}
	desc = clipLine(desc, width-4)

	return rule + "\n" + title + "\n" + HelpDimStyle.Render("  "+desc)
}

// settingActionHint names the keys that act on the focused row's type.
func settingActionHint(s Setting) string {
	switch s.Type {
	case "bool":
		return "space toggles"
	case "choice":
		if n := len(distinctOptions(s.Options)); n > 1 {
			return fmt.Sprintf("←/→ cycles %d", n)
		}
		return ""
	default:
		return "enter to edit"
	}
}

// clipLine word-wraps s to width and keeps the first line, marking a
// truncation with an ellipsis. One line, always — the detail panel's
// height is fixed.
func clipLine(s string, width int) string {
	if width <= 1 {
		return ansi.Truncate(s, max(width, 0), "")
	}
	wrapped := ansi.Wordwrap(s, width, " ")
	if i := strings.IndexByte(wrapped, '\n'); i >= 0 {
		return ansi.Truncate(wrapped[:i], width, "…")
	}
	return ansi.Truncate(wrapped, width, "…")
}

// renderSetting renders a single settings row. Rows are one line: the
// label in a fixed column, then the value (checkbox, current value, or
// the editor) with a compact position cue for choices.
func (m SettingsModel) renderSetting(setting Setting, selected bool) string {
	var b strings.Builder

	prefix := IndicatorUnselected
	style := ListItemStyle
	valueStyle := DataValue

	if selected {
		prefix = IndicatorSelected
		style = ListSelectedStyle
		valueStyle = ListSelectedStyle
	}

	// Fixed label column: every row pads its label to the same width so
	// values line up vertically instead of ragged after labels of
	// different lengths (owner M1).
	const labelCol = 18
	label := style.Render(lipgloss.NewStyle().Width(labelCol).Render(prefix + setting.Label))
	b.WriteString(label)

	// Editing: the value is replaced by the input line.
	if selected && m.editing {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("    %s %s",
			HelpDimStyle.Render("→"),
			PromptStyle.Render(m.editBuf+"█")))
		return b.String()
	}

	// For boolean settings, show checkbox
	if setting.Type == "bool" {
		checkbox := "[ ]"
		if setting.BoolValue {
			checkbox = "[x]"
		}
		b.WriteString(valueStyle.Render(checkbox))
		return b.String()
	}

	value := setting.Value
	if value == "" {
		value = "(empty)"
	}
	b.WriteString(valueStyle.Render(value))

	// Choice rows carry a compact position cue (‹2/9›) instead of
	// dumping every option inline — the options are discoverable by
	// cycling, and the cue keeps the value column aligned.
	if cue := choicePosition(setting); cue != "" {
		b.WriteString(HelpDimStyle.Render("  " + cue))
	}

	return b.String()
}

// choicePosition reports where the current value sits among a choice
// row's distinct options, e.g. "‹2/9›". Empty for non-choice rows or
// rows with no options to cycle.
func choicePosition(s Setting) string {
	if s.Type != "choice" {
		return ""
	}
	options := distinctOptions(s.Options)
	if len(options) == 0 {
		return ""
	}
	for i, o := range options {
		if o == s.Value {
			return fmt.Sprintf("‹%d/%d›", i+1, len(options))
		}
	}
	return fmt.Sprintf("‹?/%d›", len(options))
}
