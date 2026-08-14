package themeinit

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
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
