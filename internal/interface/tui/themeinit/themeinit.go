// Package themeinit pins the terminal theme before any Bubble Tea
// program runs.
//
// bubbletea's package init calls lipgloss.HasDarkBackground() (see
// tea_init.go), which makes termenv query the terminal for its
// background color. When the terminal's reply races or lags, the query
// stalls boot for termenv's OSCTimeout (5s) — and glamour's auto style
// repeats the same query on the first markdown render. agent-harness
// renders a hardcoded dark palette everywhere (no AdaptiveColor), so
// the queries are pure overhead: pinning the dark background here makes
// boot deterministic on any terminal.
//
// Import order matters: Go initializes a package's imports in lexical
// file order, so cmd/agent-harness imports this package from a file
// that sorts before every file that (transitively) imports tea
// (aa_themeinit.go). That guarantees this init runs before tea's.
package themeinit

import "github.com/charmbracelet/lipgloss"

func init() {
	lipgloss.SetHasDarkBackground(true)
}
