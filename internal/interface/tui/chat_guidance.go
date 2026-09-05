package tui

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
)

// The chat empty state: a fresh pane (new session or after /clear)
// should answer three questions before the first keystroke — what can
// I ask, what will the agent do, and which keys move me. One panel,
// one home: the old once-per-session guidance message duplicated it
// and scrolled it away.

// chatEmptyState renders the panel shown while the transcript has no
// messages. Five content lines hard cap: an empty state that grows is
// chrome.
func chatEmptyState(personaName string) string {
	p, err := persona.Parse(personaName)
	if err != nil {
		p = persona.Default()
	}

	lines := []string{
		HelpTitleStyle.Render("  The agent is ready."),
		"",
		HelpDimStyle.Render(`  • Ask it to ` + p.EmptyStateHint()),
		HelpDimStyle.Render(`  • It reads the repo, edits files, and runs commands — approvals stack in the dialog`),
		HelpDimStyle.Render(`  • Tool lines open their full record: click one, or "Enter" for the latest`),
		HelpDimStyle.Render(`  • Sessions save as you go · "h" jumps Home · "4" opens the diagnostics stream`),
		"",
		HelpDimStyle.Render(`  "i" to start · "/" for commands · "?" for the full map`),
	}
	return strings.Join(lines, "\n")
}
