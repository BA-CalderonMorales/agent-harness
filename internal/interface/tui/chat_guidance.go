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
// messages — centered in the pane, Sessions-style: a title, the
// persona's seed ask, and the keys that move you.
func chatEmptyState(personaName string, width, height int) string {
	p, err := persona.Parse(personaName)
	if err != nil {
		p = persona.Default()
	}
	// A pane too short for the panel still gets a panel: the caller
	// passes the rows left after the notices, which can be zero or
	// negative on a very short pane.
	if height < 1 {
		height = 1
	}
	if width < 1 {
		width = 1
	}

	block := strings.Join([]string{
		HelpTitleStyle.Render("The agent is ready."),
		"",
		HelpDimStyle.Render("Ask it to " + p.EmptyStateHint() + "."),
		"",
		HelpDimStyle.Render(`"i" to start · "/" commands · "?" help`),
	}, "\n")
	return viewPadded(width, height, block)
}
