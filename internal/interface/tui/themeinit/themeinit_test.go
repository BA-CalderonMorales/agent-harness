package themeinit

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestPinnedDarkBackground verifies the pin resolves HasDarkBackground
// without a terminal query (the query path is what stalls boot when a
// terminal's reply races). After this package's init, the value is
// explicit and the query's sync.Once never fires.
func TestPinnedDarkBackground(t *testing.T) {
	if !lipgloss.HasDarkBackground() {
		t.Fatal("themeinit must pin a dark background; the TUI palette is hardcoded dark")
	}
}

// TestColorProfileTrueColor pins the interactive path: themes are
// authored as truecolor hex, so a terminal a human watches must render
// truecolor for the 20 themes to stay distinguishable.
func TestColorProfileTrueColor(t *testing.T) {
	if got := resolveColorProfile(false, "", "xterm-256color", true); got != termenv.TrueColor {
		t.Fatalf("resolveColorProfile() = %v, want TrueColor on an interactive terminal", got)
	}
}

// TestResolveColorProfileMatrix pins every colorless case: explicit
// opt-outs, dumb terminals, and piped output must never carry 24-bit
// escapes the other end cannot render.
func TestResolveColorProfileMatrix(t *testing.T) {
	cases := map[string]struct {
		noColor  bool
		clicolor string
		termName string
		isTTY    bool
	}{
		"NO_COLOR set":                {noColor: true, termName: "xterm-256color", isTTY: true},
		"CLICOLOR=0":                  {clicolor: "0", termName: "xterm-256color", isTTY: true},
		"TERM=dumb":                   {termName: "dumb", isTTY: true},
		"piped stdout":                {termName: "xterm-256color", isTTY: false},
		"piped stdout with COLORTERM": {termName: "tmux-256color", isTTY: false},
		"all signals at once":         {noColor: true, clicolor: "0", termName: "dumb", isTTY: false},
	}
	for name, tc := range cases {
		if got := resolveColorProfile(tc.noColor, tc.clicolor, tc.termName, tc.isTTY); got != termenv.Ascii {
			t.Errorf("%s: resolveColorProfile() = %v, want Ascii", name, got)
		}
	}
}

// TestColorProfileHonorsNoColor pins the opt-out wiring: NO_COLOR must
// drop the renderer to a colorless profile rather than being overridden
// by the truecolor pin. Presence counts under any value, and the wiring
// holds regardless of whether the test runner itself has a TTY.
func TestColorProfileHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := colorProfile(); got != termenv.Ascii {
		t.Fatalf("colorProfile() = %v, want Ascii when NO_COLOR is set", got)
	}
}

// TestColorProfileHonorsEmptyNoColor pins the spec reading: NO_COLOR
// counts when present, even when set to the empty string.
func TestColorProfileHonorsEmptyNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := colorProfile(); got != termenv.Ascii {
		t.Fatalf("colorProfile() = %v, want Ascii when NO_COLOR is present but empty", got)
	}
}
