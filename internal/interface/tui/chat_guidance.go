package tui

import "strings"

// First-run navigation guidance: new users land in the chat pane with a
// vim-style composer and no map. The guidance is a system message — it
// scrolls away with the transcript instead of being chrome — and shows
// once per session, plus again after /clear (a wiped pane is a fresh
// first run for whoever is sitting there).

// navigationGuidance is the compact key map shown on first Chat entry
// and on /clear. Bullets, one instruction per line, keys in quotes —
// a run-on line made every instruction blur into the next. Five lines
// hard cap: guidance that grows is chrome.
func navigationGuidance() string {
	return strings.Join([]string{
		`• "i" to start chatting`,
		`• "Esc" to stop chatting`,
		`• "j" and "k" to scroll up and down`,
		`• "Enter" expands the latest tool or reasoning`,
		`• "Shift+Tab" cycles agent modes · "/" opens commands`,
	}, "\n")
}

// ShowNavigationGuidance appends the guidance block once per session.
// Later calls are no-ops until ResetNavigationGuidance.
func (m *ChatModel) ShowNavigationGuidance() {
	if m.guidanceShown {
		return
	}
	m.guidanceShown = true
	m.AddMessage("system", navigationGuidance())
}

// ResetNavigationGuidance re-arms the once-per-session guard, so the
// next ShowNavigationGuidance renders the block again (used by /clear).
func (m *ChatModel) ResetNavigationGuidance() {
	m.guidanceShown = false
}
