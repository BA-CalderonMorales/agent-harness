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
// The same reasoning pins the color profile. Every theme is authored as
// truecolor hex (see themes.go), so the palette only stays faithful if
// the renderer emits truecolor. termenv's per-host sniff under-detects
// color support on hosts like Windows PowerShell conhost or CI shells
// that report no TTY, dropping the profile to ANSI/Ascii and collapsing
// all 20 themes into one 16-color bucket — theme switching then looks
// dead. Pinning the profile keeps the themes distinguishable on any
// color terminal. NO_COLOR still wins: an explicit no-color opt-out is
// never overridden.
//
// Import order matters: Go initializes a package's imports in lexical
// file order, so cmd/agent-harness imports this package from a file
// that sorts before every file that (transitively) imports tea
// (aa_themeinit.go). That guarantees this init runs before tea's.
package themeinit

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetHasDarkBackground(true)
	lipgloss.SetColorProfile(colorProfile())
}

// colorProfile returns the profile the hardcoded truecolor palette needs
// to stay faithful: truecolor, unless the user explicitly opted out of
// color via NO_COLOR (https://no-color.org), in which case termenv's own
// detection is left to produce the colorless output it already emits.
func colorProfile() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	return termenv.TrueColor
}
