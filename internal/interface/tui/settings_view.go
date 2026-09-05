package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the settings.
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
		Subtitle: "Configuration options",
		Count:    -1, // no count: 17 settings is noise, not signal
	}))

	// Build settings list content for viewport
	var settingsContent strings.Builder
	currentCat := ""
	// cursorLine tracks the rendered line of every setting row: the
	// viewport scroll must land the cursor row in view exactly, and a
	// per-row average cannot (descriptions and category headers make
	// rows uneven — the old estimate let the cursor scroll out of
	// sight past the last category).
	cursorLine := make([]int, len(m.settings))
	for i, setting := range m.settings {
		if setting.Category != "" && setting.Category != currentCat {
			if currentCat != "" {
				settingsContent.WriteString("\n")
			}
			currentCat = setting.Category
			categoryHeader := SectionHeaderStyle.Render("── " + currentCat + " ──")
			settingsContent.WriteString(categoryHeader)
			settingsContent.WriteString("\n")
		}
		cursorLine[i] = strings.Count(settingsContent.String(), "\n")
		settingsContent.WriteString(m.renderSetting(setting, i == m.cursor))
		settingsContent.WriteString("\n")
	}

	// Update viewport content (settings rows only; the system log lives
	// in the Logs tab), then land the scroll so the cursor row — and
	// its whole block — is on screen.
	m.viewport.SetContent(settingsContent.String())
	if m.cursor >= 0 && m.cursor < len(cursorLine) {
		totalLines := strings.Count(settingsContent.String(), "\n") + 1
		maxOffset := totalLines - m.viewport.Height
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.viewport.YOffset
		// The selected row renders two lines (label + description);
		// unselected rows render one.
		rowEnd := cursorLine[m.cursor] + 1
		if m.cursor < len(m.settings) && m.settings[m.cursor].Description != "" && !m.editing {
			rowEnd++
		}
		if cursorLine[m.cursor] < offset {
			offset = cursorLine[m.cursor]
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

	// Render viewport (scrollable settings list)
	b.WriteString(m.viewport.View())

	// The selected setting's description pins above the footer: walk
	// the list, read what each key does — no row-walking with the eye
	// glued to the right column.
	if desc := m.selectedDescription(); desc != "" {
		b.WriteString(HelpDimStyle.Render("  " + desc))
		b.WriteString("\n")
	}

	// Footer (always visible, not in viewport)
	footerActions := []ActionHint{
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter/Space", Desc: "Edit / toggle"},
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

// selectedDescription is the description of the row the cursor is on,
// empty when the list is empty.
func (m SettingsModel) selectedDescription() string {
	if m.cursor < 0 || m.cursor >= len(m.settings) {
		return ""
	}
	return m.settings[m.cursor].Description
}

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

	// For boolean settings, show checkbox
	if setting.Type == "bool" {
		checkbox := "[ ]"
		if setting.BoolValue {
			checkbox = "[x]"
		}
		if selected {
			checkbox = PromptStyle.Render(checkbox)
		}
		b.WriteString(label)
		b.WriteString(style.Render(checkbox))

		// Description for boolean
		if selected {
			b.WriteString("\n")
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", setting.Description)))
		}
		return b.String()
	}

	b.WriteString(label)

	// Show edit indicator if editing
	if selected && m.editing {
		b.WriteString("\n")
		editLine := fmt.Sprintf("    %s %s",
			HelpDimStyle.Render("→"),
			PromptStyle.Render(m.editBuf+"█"))
		b.WriteString(editLine)
	} else {
		// Show current value
		value := setting.Value
		if value == "" {
			value = "(empty)"
		}
		b.WriteString(valueStyle.Render(value))
		if setting.Type == "choice" && len(setting.Options) > 0 {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  [%s]", strings.Join(setting.Options, "/"))))
		}
	}

	// Description
	if selected && !m.editing {
		b.WriteString("\n")
		b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", setting.Description)))
	}

	return b.String()
}

// Focus focuses the settings view.
