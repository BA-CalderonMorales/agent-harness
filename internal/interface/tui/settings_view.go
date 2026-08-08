package tui

import (
	"fmt"
	"strings"
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
		Count:    len(m.settings),
	}))

	// Build settings list content for viewport
	var settingsContent strings.Builder
	currentCat := ""
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
		settingsContent.WriteString(m.renderSetting(setting, i == m.cursor))
		settingsContent.WriteString("\n")
	}

	// System Messages section at the bottom of the page: durable log of
	// provider errors and session notices, scrollable with the settings.
	if len(m.systemMessages) > 0 {
		settingsContent.WriteString("\n")
		settingsContent.WriteString(SectionHeaderStyle.Render("── System Messages ──"))
		settingsContent.WriteString("\n")
		for _, line := range m.systemMessages {
			settingsContent.WriteString("  " + HelpDimStyle.Render(line))
			settingsContent.WriteString("\n")
		}
	}

	// Update viewport content
	m.viewport.SetContent(settingsContent.String())

	// Render viewport (scrollable settings list)
	b.WriteString(m.viewport.View())

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

	// For boolean settings, show checkbox
	if setting.Type == "bool" {
		checkbox := "[ ]"
		if setting.BoolValue {
			checkbox = "[x]"
		}
		if selected {
			checkbox = PromptStyle.Render(checkbox)
		}
		label := style.Render(prefix + checkbox + " " + setting.Label)
		b.WriteString(label)

		// Description for boolean
		if selected {
			b.WriteString("\n")
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", setting.Description)))
		}
		return b.String()
	}

	// Label and value
	label := style.Render(prefix + setting.Label)
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
		b.WriteString(" ")
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
