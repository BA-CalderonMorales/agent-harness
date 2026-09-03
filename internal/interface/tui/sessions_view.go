package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// View renders the sessions list.
func (m SessionsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.loading {
		return RenderLoading(ViewPort{Width: m.width, Height: m.height}, "Loading sessions...")
	}

	if len(m.sessions) == 0 {
		return RenderEmptyState(ViewPort{Width: m.width, Height: m.height}, EmptyState{
			Title:       "No Sessions",
			Description: "Start chatting to create your first session.",
			Actions: []ActionHint{
				{Key: "Tab", Desc: "Switch to chat"},
				{Key: "r", Desc: "Refresh"},
			},
		})
	}

	var b strings.Builder

	// Header (consistent with Settings view)
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Sessions",
		Subtitle: "Manage your conversations",
		Count:    len(m.sessions),
	}))

	// Content area height (subtract header height)
	contentHeight := m.height - 3
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Two-pane layout
	listW, detailW := TwoPaneWidths(m.width)

	// Render list
	var listB strings.Builder
	listB.WriteString(ListTitleStyle.Render("  All Sessions") + "\n")

	if m.notice != "" {
		noticeStyle := InfoStyle
		if m.noticeType == "error" {
			noticeStyle = ErrorStyle
		}
		listB.WriteString(noticeStyle.Render("  "+m.notice) + "\n")
	}
	listB.WriteString("\n")

	for i, session := range m.sessions {
		item := m.renderSessionItem(session, i == m.cursor, listW)
		listB.WriteString(item + "\n")
	}

	// List footer
	footerHints := []ActionHint{
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Select"},
		{Key: "n", Desc: "New"},
		{Key: "d", Desc: "Delete"},
		{Key: "e", Desc: "Export"},
		{Key: "c", Desc: "Copy"},
		{Key: "r", Desc: "Refresh"},
	}
	if m.confirmingDelete {
		title := "(untitled)"
		if m.deleteTargetIdx >= 0 && m.deleteTargetIdx < len(m.sessions) {
			t := m.sessions[m.deleteTargetIdx].Title
			if t != "" {
				title = t
			}
		}
		footerHints = []ActionHint{
			{Key: "y", Desc: fmt.Sprintf("Delete %q?", title)},
			{Key: "n/Esc", Desc: "Cancel"},
		}
	}
	listB.WriteString(RenderCompactFooterWrapped(footerHints, listW))

	listContent := lipgloss.NewStyle().Width(listW).Height(contentHeight - 2).Render(listB.String())

	// Render detail
	detailContent := ""
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		detailStr := m.renderSessionDetail(m.sessions[m.cursor])
		detailContent = DetailPanelStyle.Width(detailW).Height(contentHeight - 4).Render(detailStr)
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listContent, detailContent))

	return b.String()
}

func (m SessionsModel) renderSessionItem(session SessionInfo, selected bool, width int) string {
	prefix := IndicatorUnselected
	style := ListItemStyle

	if selected {
		prefix = IndicatorSelected
		style = ListSelectedStyle
	}

	// Build label
	label := session.Title
	if label == "" {
		label = fmt.Sprintf("Session %s", session.ID[:8])
	}
	if len(label) > width-12 {
		label = label[:width-15] + "..."
	}

	line := style.Render(prefix + label)

	// Status indicator. The active session gets the active marker, not
	// "[running]" — an idle open session is not executing anything.
	status := StatusNeutral
	if session.IsActive {
		status = StatusActive
	}
	statusStr := RenderStatusBadge(status)
	if statusStr != "" {
		line += " " + statusStr
	}

	return line
}

func (m SessionsModel) renderSessionDetail(session SessionInfo) string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("Session Details"))
	b.WriteString("\n\n")

	// ID
	idDisplay := session.ID
	if len(idDisplay) > 16 {
		idDisplay = idDisplay[:16] + "..."
	}
	b.WriteString(RenderField("ID", idDisplay))
	b.WriteString("\n")

	// Title
	title := session.Title
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString(RenderField("Title", title))
	b.WriteString("\n")

	// Model
	b.WriteString(RenderField("Model", session.Model))
	b.WriteString("\n\n")

	// Stats
	b.WriteString(HeaderTertiary.Render("Statistics"))
	b.WriteString("\n")
	b.WriteString(RenderField("Messages", fmt.Sprintf("%d", session.MessageCount)))
	b.WriteString("\n")
	b.WriteString(RenderField("Turns", fmt.Sprintf("%d", session.Turns)))
	b.WriteString("\n\n")

	// Timestamps
	b.WriteString(HeaderTertiary.Render("Timestamps"))
	b.WriteString("\n")
	b.WriteString(RenderField("Created", session.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")
	b.WriteString(RenderField("Updated", session.UpdatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")

	return b.String()
}

// Focus focuses the sessions view.
