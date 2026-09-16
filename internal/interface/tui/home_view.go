package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// homeChromeRows is the pane height the home view spends on pinned chrome:
// the header (title + tagline) and the footer (blank spacer + hints), two
// rows each. Everything else — the banners, the Project card, Quick
// Actions, and Recent Sessions — scrolls as one surface.
//
// The dashboard used to render as a single block constrained only by
// height, with no scroll window at all. A 100x20 pane therefore showed the
// "Quick Actions" heading and nothing else: the two actions and the whole
// Recent Sessions section were clipped, so there was no way to see — or
// delete — a single session. Constraining the height hid the content; the
// fix is to make it reachable, exactly as the Settings tab does.
const homeChromeRows = 4

// View renders the home dashboard.
func (m *HomeModel) View() string {
	if m.width == 0 {
		return "  Loading dashboard..."
	}

	var b strings.Builder

	// The header is pinned: the title answers "where am I", and scrolling
	// it away would leave the pane unlabelled.
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Agent-Harness",
		Subtitle: m.tagline,
		Count:    -1, // no count: the zero value would render a meaningless "(0)"
	}))

	// The body scrolls as one surface. Landing the focused row exactly in
	// view needs that row's rendered line, which only the pass that
	// writes the body knows — so that pass reports it.
	body, cursorLine, cursorRows := m.buildBody()
	m.viewport.SetContent(body)
	m.syncScrollToCursor(cursorLine, cursorRows)

	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(RenderFooter(m.footerHints()))

	return b.String()
}

// rebuildActions builds the quick-action list. Called on every resize and
// on Init so the handlers always close over the current delegate.
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

// buildBody renders the scrollable dashboard and reports, for each
// cursorable row in cursor order, the line it landed on and how many lines
// it occupies. Accounting for the real rows is what keeps the last session
// reachable: a per-row-average estimate drifts and strands the tail.
func (m *HomeModel) buildBody() (string, []int, []int) {
	var b strings.Builder

	total := m.totalItems()
	cursorLine := make([]int, total)
	cursorRows := make([]int, total)
	line := 0

	write := func(s string) {
		b.WriteString(s)
		line += strings.Count(s, "\n")
	}

	// Setup required banner: driven by the provider probe's misconfigured
	// verdict (the model is defaulted at boot, so an empty-model proxy
	// would never fire and always-[ready] would lie).
	if m.setupRequired {
		write(m.renderSetupBanner())
	}
	if m.unreadableCount > 0 {
		write(WarningStyle.Render("  "+unreadableSessionWarning(m.unreadableCount)) + "\n")
	}

	write(m.renderProjectCard())

	// Quick actions
	write(HeaderSecondary.Render("  Quick Actions") + "\n\n")
	for i := range m.actions {
		row := m.renderActionRow(i)
		cursorLine[i] = line
		cursorRows[i] = strings.Count(row, "\n") + 1
		write(row)
	}
	write("\n")

	// Recent sessions. Every session renders: the viewport scrolls, so
	// there is no longer a reason to show a three-row window and hide the
	// rest — which is how a session the user wanted to delete became
	// unreachable rather than merely off-screen.
	if len(m.sessions) > 0 {
		write(m.renderSessionsHeader())
		for i := range m.sessions {
			row := m.renderSessionRow(i)
			flat := len(m.actions) + i
			cursorLine[flat] = line
			cursorRows[flat] = strings.Count(row, "\n") + 1
			write(row)
		}
		write("\n")
	}

	return b.String(), cursorLine, cursorRows
}

// syncScrollToCursor lands the focused row — and its whole block — inside
// the viewport window, clamping to the content's ends.
func (m *HomeModel) syncScrollToCursor(cursorLine, cursorRows []int) {
	if m.actionCursor < 0 || m.actionCursor >= len(cursorLine) {
		return
	}
	rowStart := cursorLine[m.actionCursor]
	rowEnd := rowStart + cursorRows[m.actionCursor]

	offset := m.viewport.YOffset
	if rowStart < offset {
		offset = rowStart
	} else if rowEnd > offset+m.viewport.Height {
		offset = rowEnd - m.viewport.Height
	}

	maxOffset := m.viewport.TotalLineCount() - m.viewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	m.viewport.SetYOffset(offset)
}

// renderActionRow renders one quick action: its label line and its
// description line, both aligned on the label column.
func (m *HomeModel) renderActionRow(i int) string {
	action := m.actions[i]
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
	// Descriptions align with the label column: both list styles pad 2 on
	// the left and the indicator slot is 2 wide.
	return style.Render(prefix+label) + "\n" +
		HelpDimStyle.Render(fmt.Sprintf("    %s", action.Description)) + "\n"
}

// renderSessionsHeader renders the Recent Sessions heading plus the delete
// affordance (a pending delete names the session it will remove).
func (m *HomeModel) renderSessionsHeader() string {
	head := HeaderSecondary.Render("  Recent Sessions")
	if m.deleting >= 0 {
		title := "(untitled)"
		if m.deleting < len(m.sessions) && m.sessions[m.deleting].Title != "" {
			title = m.sessions[m.deleting].Title
		}
		return head + HelpDimStyle.Render(fmt.Sprintf("   [y] delete %q · [n] cancel", title)) + "\n\n"
	}
	return head + HelpDimStyle.Render("   [d] delete") + "\n\n"
}

// renderSessionRow renders one session row and its trailing newline.
func (m *HomeModel) renderSessionRow(i int) string {
	s := m.sessions[i]
	label := s.Title
	if label == "" {
		label = fmt.Sprintf("Session %s", s.ID[:min(8, len(s.ID))])
	}
	marker := IndicatorUnselected
	style := ListItemStyle
	if s.IsActive || m.cursorSessionIndex() == i {
		marker = IndicatorSelected
		style = ListSelectedStyle
	}
	turns := fmt.Sprintf("%d turns", s.Turns)
	if s.Turns == 1 {
		turns = "1 turn"
	}
	line := fmt.Sprintf("%s%s · %d msgs · %s", marker, label, s.MessageCount, turns)
	// A session line wider than the pane wraps at the terminal: the
	// continuation lands outside the frame border and the chrome shifts —
	// the mobile border-flicker bug. ListItemStyle and ListSelectedStyle
	// each pad 2 cells per side (4 total), so the unstyled text is
	// budgeted for those four cells; the styled row then fits the pane and
	// the ellipsis survives rendering.
	if max := m.width - 4; max > 0 && lipgloss.Width(line) > max {
		line = ansi.Truncate(line, max, "…")
	}
	return style.Render(line) + "\n"
}

// footerHints names the keys that act on the home view, most important
// first, trimmed to what the pane can show.
func (m *HomeModel) footerHints() []ActionHint {
	if m.deleting >= 0 {
		return fitFooterHints([]ActionHint{
			{Key: "y", Desc: "Confirm delete"},
			{Key: "n/Esc", Desc: "Cancel"},
		}, m.width)
	}
	enter := "Open session"
	if m.cursorInActions() {
		enter = "Run action"
	}
	// Ordered by what a reader needs first; the tail is what a narrow pane
	// drops.
	return fitFooterHints([]ActionHint{
		{Key: "↑/↓", Desc: "Move (wraps)"},
		{Key: "Enter", Desc: enter},
		{Key: "d", Desc: "Delete session"},
		{Key: "g/G", Desc: "Ends"},
	}, m.width)
}

// fitFooterHints drops hints from the end until the footer fits the pane.
// A footer wider than the pane wraps at the terminal, and the continuation
// lands outside the frame border and shifts the chrome — the mobile
// border-flicker bug. The footer's budget is a fixed two rows, so the
// chrome yields hints rather than growing a row, the same way the modal
// family yields its hint before its body.
func fitFooterHints(hints []ActionHint, width int) []ActionHint {
	for len(hints) > 1 {
		if width <= 0 || lipgloss.Width(FormatKeyHints(hints)) <= width {
			return hints
		}
		hints = hints[:len(hints)-1]
	}
	return hints
}

// renderSetupBanner renders the misconfigured-provider notice.
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
