package tui

// First-run navigation guidance: new users land in the chat pane with a
// vim-style composer and no map. The guidance is a system message — it
// scrolls away with the transcript instead of being chrome — and shows
// once per session, plus again after /clear (a wiped pane is a fresh
// first run for whoever is sitting there).

// navigationGuidance is the compact key map shown on first Chat entry
// and on /clear. Three lines hard cap: guidance that grows is chrome.
func navigationGuidance() string {
	return "Quick keys: i type · Esc normal mode · j/k scroll\n" +
		"Enter expands the latest tool/reasoning · Shift+Tab cycles agent modes\n" +
		"/ commands · /clear wipes this pane"
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
