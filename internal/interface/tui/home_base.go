package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Home base.
//
// The Home pane is where the user lands and where they come back to.
// Today it is a stack of cards; the strip at the top is the same facts
// drawn instead of listed: where you are, what is running, and where
// you have been.
//
// It is a sketch, not a picture: quiet marks, one accent, no motion.
// Everything it draws is real state — the workspace, the branch, the
// provider, and the sessions — so it never lies about the machine.

// homeBaseStrip renders the strip. Two lines, and it degrades: a narrow
// pane keeps the workspace and drops the rest.
func (m HomeModel) homeBaseStrip(width int) string {
	if width <= 0 {
		return ""
	}

	// Line one: where you are.
	head := []string{}
	name := m.project.Name
	if name == "" {
		name = "workspace"
	}
	head = append(head, ModePromptStyle.Render("⌂")+" "+InfoStyle.Render(name))

	if branch := m.project.GitBranch; branch != "" {
		mark := ""
		if m.project.HasChanges {
			mark = "*"
		}
		head = append(head, HelpDimStyle.Render(branch+mark))
	}
	if m.project.Type != "" && width >= 46 {
		head = append(head, HelpDimStyle.Render(m.project.Type))
	}
	provider := m.provider
	if provider == "" {
		provider = "local"
	}
	if width >= 34 {
		providerLine := provider
		if m.model != "" && width >= 56 {
			providerLine = provider + "/" + m.model
		}
		head = append(head, HelpDimStyle.Render(providerLine))
	}

	line := strings.Join(head, HelpDimStyle.Render(" · "))
	if lipgloss.Width(line) > width {
		line = ModePromptStyle.Render("⌂") + " " + InfoStyle.Render(name)
	}

	// Line two: where you have been. One mark per session, filled when
	// the session has something unread.
	marks := make([]string, 0, len(m.sessions))
	for _, session := range m.sessions {
		if session.UnreadableCount > 0 {
			marks = append(marks, ModePromptStyle.Render("▪"))
			continue
		}
		marks = append(marks, HelpDimStyle.Render("▫"))
	}
	if len(marks) > 8 {
		marks = marks[:8]
	}
	trail := ""
	if len(m.sessions) > 0 {
		trail = fmt.Sprintf("%s  %d %s", strings.Join(marks, " "), len(m.sessions), pluralSessions(len(m.sessions)))
	}
	if trail != "" && lipgloss.Width(trail) > width {
		trail = fmt.Sprintf("%d %s", len(m.sessions), pluralSessions(len(m.sessions)))
	}

	// The strip is always two lines, so the pinned chrome has a fixed
	// height and the scroll window under it does not jump when the
	// session count changes.
	return line + "\n  " + trail
}

// pluralSessions names the session count the way a person would.
func pluralSessions(n int) string {
	if n == 1 {
		return "session"
	}
	return "sessions"
}
