package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// View renders the home dashboard.
func (m *HomeModel) View() string {
	if m.width == 0 {
		return "  Loading dashboard..."
	}

	var sections []string

	// Header. Count stays -1: the zero value would render a meaningless
	// "(0)" on a view with no countable collection.
	sections = append(sections, RenderHeader(HeaderConfig{
		Title:    "Home",
		Subtitle: "Project dashboard",
		Count:    -1,
	}))

	// Setup required banner: driven by the provider probe's misconfigured
	// verdict (the model is defaulted at boot, so an empty-model proxy
	// would never fire and always-[ready] would lie).
	if m.setupRequired {
		sections = append(sections, m.renderSetupBanner())
	}

	// Project card
	sections = append(sections, m.renderProjectCard())

	// Quick actions
	sections = append(sections, m.renderQuickActions())

	// Recent sessions
	if len(m.sessions) > 0 {
		sections = append(sections, m.renderRecentSessions())
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Constrain to available height
	return lipgloss.NewStyle().Height(m.height).Render(content)
}

func (m *HomeModel) rebuildActions() {
	m.actions = []homeAction{
		{Label: "New chat", Key: "n", Description: "Start a fresh conversation", Handler: func() {
			if m.delegate != nil {
				m.delegate.OnNewChat()
			}
		}},
		{Label: "Export session", Key: "e", Description: "Save conversation to file", Handler: func() {
			if m.delegate != nil {
				m.delegate.OnExportSession()
			}
		}},
	}
	m.clampCursor()
}

func (m *HomeModel) renderProjectCard() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Project"))
	b.WriteString("\n\n")

	if m.project.Name != "" {
		b.WriteString(RenderField("Name", m.project.Name))
		b.WriteString("\n")
	}
	if m.project.Type != "" {
		b.WriteString(RenderField("Type", m.project.Type))
		b.WriteString("\n")
	}

	if m.project.GitBranch != "" {
		gitStatus := m.project.GitBranch
		if m.project.GitCommit != "" {
			commit := m.project.GitCommit
			if len(commit) > 7 {
				commit = commit[:7]
			}
			gitStatus += " @ " + commit
		}
		if m.project.HasChanges {
			gitStatus += fmt.Sprintf(" (%d uncommitted)", m.project.UncommittedCount)
		}
		b.WriteString(RenderField("Git", gitStatus))
		b.WriteString("\n")
		if m.project.LastCommitMsg != "" {
			b.WriteString(RenderField("Last commit", truncateString(m.project.LastCommitMsg, m.width-20)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(RenderField("Git", "not a repository"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderQuickActions() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Quick Actions"))
	b.WriteString("\n\n")

	for i, action := range m.actions {
		prefix := IndicatorUnselected
		style := ListItemStyle
		if i == m.actionCursor {
			prefix = IndicatorSelected
			style = ListSelectedStyle
		}
		label := action.Label
		if action.Key != "" {
			label = fmt.Sprintf("%s (%s)", label, action.Key)
		}
		line := fmt.Sprintf("%s%s", prefix, label)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		// Descriptions align with the label column: both list styles pad
		// 2 on the left and the indicator slot is 2 wide.
		b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", action.Description)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderRecentSessions() string {
	var b strings.Builder

	b.WriteString(HeaderSecondary.Render("  Recent Sessions"))
	b.WriteString(HelpDimStyle.Render("   [d] delete"))
	b.WriteString("\n\n")

	count := 3
	if len(m.sessions) < count {
		count = len(m.sessions)
	}

	for i := 0; i < count; i++ {
		s := m.sessions[i]
		label := s.Title
		if label == "" {
			label = fmt.Sprintf("Session %s", s.ID[:8])
		}
		marker := IndicatorUnselected
		style := ListItemStyle
		if s.IsActive {
			marker = IndicatorSelected
			style = ListSelectedStyle
		}
		if m.cursorSessionIndex() == i {
			marker = IndicatorSelected
			style = ListSelectedStyle
		}
		line := fmt.Sprintf("%s%s · %d msgs · %d turns", marker, label, s.MessageCount, s.Turns)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (m *HomeModel) renderSetupBanner() string {
	var b strings.Builder
	b.WriteString(ErrorStyle.Render("  [!] Setup Required"))
	b.WriteString("\n")
	b.WriteString(HelpDimStyle.Render("  No API key or model configured."))
	b.WriteString("\n")
	b.WriteString(HelpDimStyle.Render("  Press l to log in, or set the AH_API_KEY environment variable."))
	b.WriteString("\n\n")
	return b.String()
}

// Focus focuses the home view.
