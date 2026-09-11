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

// TestColorProfileTrueColor pins the color profile: themes are authored
// as truecolor hex, so without NO_COLOR the renderer must be truecolor
// for the 20 themes to stay distinguishable on any color terminal.
func TestColorProfileTrueColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := colorProfile(); got != termenv.TrueColor {
		t.Fatalf("colorProfile() = %v, want TrueColor", got)
	}
}

// TestColorProfileHonorsNoColor pins the opt-out: NO_COLOR must drop the
// renderer to a colorless profile rather than being overridden by the
// truecolor pin.
func TestColorProfileHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := colorProfile(); got != termenv.Ascii {
		t.Fatalf("colorProfile() = %v, want Ascii when NO_COLOR is set", got)
	}
}
