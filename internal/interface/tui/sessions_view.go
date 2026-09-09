package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
		if m.unreadableCount > 0 {
			return RenderEmptyState(ViewPort{Width: m.width, Height: m.height}, EmptyState{
				Title:       "Sessions unavailable",
				Description: unreadableSessionWarning(m.unreadableCount),
				Actions:     []ActionHint{{Key: "r", Desc: "Refresh"}},
			})
		}
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
	if m.unreadableCount > 0 {
		listB.WriteString(WarningStyle.Render("  "+unreadableSessionWarning(m.unreadableCount)) + "\n")
	}
	listB.WriteString("\n")

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

	// Single-pane full width layout on mobile
	if isMobilePane(m.width) {
		for i, session := range m.sessions {
			item := m.renderSessionItem(session, i == m.cursor, m.width)
			listB.WriteString(item + "\n")
		}
		listB.WriteString(RenderCompactFooterWrapped(footerHints, m.width))
		listContent := lipgloss.NewStyle().Width(m.width).Height(contentHeight - 2).Render(listB.String())
		b.WriteString(listContent)
		return b.String()
	}

	// Two-pane layout on desktop
	listW, detailW := TwoPaneWidths(m.width)

	for i, session := range m.sessions {
		item := m.renderSessionItem(session, i == m.cursor, listW)
		listB.WriteString(item + "\n")
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

	// Build label: title (or id) plus how long ago it moved — the
	// field that actually tells two sessions apart.
	label := session.Title
	if label == "" {
		if len(session.ID) >= 8 {
			label = fmt.Sprintf("Session %s", session.ID[:8])
		} else {
			label = fmt.Sprintf("Session %s", session.ID)
		}
	}
	age := RelativeTime(session.UpdatedAt)

	// Status indicator. The active session gets the active marker, not
	// "[running]" — an idle open session is not executing anything.
	status := StatusNeutral
	if session.IsActive {
		status = StatusActive
	}
	statusStr := RenderStatusBadge(status)
	statusW := 0
	if statusStr != "" {
		statusW = lipgloss.Width(statusStr) + 1
	}

	// Budget available space for the label inside the styled row:
	// ListItemStyle and ListSelectedStyle pad 2 on each side (4 total),
	// prefix takes 2, age takes len(age)+1, and status takes statusW.
	overhead := 4 + 2 + len(age) + 1 + statusW
	avail := width - overhead
	if avail < 4 {
		avail = 4
	}
	if lipgloss.Width(label) > avail {
		label = ansi.Truncate(label, avail, "…")
	}

	line := style.Render(prefix + label + " " + HelpDimStyle.Render(age))
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
