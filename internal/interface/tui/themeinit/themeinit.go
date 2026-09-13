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
// color support on hosts like Windows PowerShell conhost, dropping the
// profile to ANSI/Ascii and collapsing all 20 themes into one 16-color
// bucket — theme switching then looks dead. Pinning the profile keeps
// the themes distinguishable on any interactive color terminal.
//
// The pin stops where color output stops being useful: piped output, a
// dumb terminal, and the standard opt-outs (NO_COLOR present, CLICOLOR=0)
// stay colorless instead of carrying 24-bit escapes the other end cannot
// render. An interactive TUI session always has a TTY on stdout (tea
// requires it, and bubbletea enables VT processing on Windows), so the
// TTY gate preserves the fix everywhere a human actually watches themes
// while keeping logs and pipes clean.
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
	"golang.org/x/term"
)

func init() {
	lipgloss.SetHasDarkBackground(true)
	lipgloss.SetColorProfile(colorProfile())
}

// colorProfile returns the profile the hardcoded truecolor palette needs
// to stay faithful: truecolor on an interactive terminal that has not
// opted out, colorless output otherwise. NO_COLOR counts when present
// under any value (https://no-color.org); CLICOLOR=0 and TERM=dumb are
// honored the same way.
func colorProfile() termenv.Profile {
	_, noColor := os.LookupEnv("NO_COLOR")
	return resolveColorProfile(noColor, os.Getenv("CLICOLOR"), os.Getenv("TERM"), term.IsTerminal(int(os.Stdout.Fd())))
}

// resolveColorProfile is the pure policy behind colorProfile, kept
// separate so the matrix is testable without a real terminal.
func resolveColorProfile(noColor bool, clicolor, termName string, isTTY bool) termenv.Profile {
	switch {
	case noColor,
		clicolor == "0",
		termName == "dumb",
		!isTTY:
		return termenv.Ascii
	default:
		return termenv.TrueColor
	}
}
