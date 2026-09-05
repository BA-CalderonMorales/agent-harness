package tui

import (
	"os"
	"strings"
)

// Terminal environment knowledge. Mobile and tmux hosts get input and
// layout affordances that desktop never sees; desktop must stay
// pixel-identical. Detection lives here so every consumer reads the
// same truth.

var isTermux = detectTermux()

// detectTermux checks if we're running in Termux environment
func detectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// inTmux reports whether the app renders inside tmux (or a compatible
// multiplexer). Function, not var: it is read per render and asserted
// per test.
func inTmux() bool {
	return os.Getenv("TMUX") != "" ||
		strings.HasPrefix(os.Getenv("TERM"), "tmux") ||
		strings.HasPrefix(os.Getenv("TERM"), "screen")
}

// mobilePaneWidth is the pane width below which the app adapts to
// touch input: a phone pane cannot show desktop chrome, and with
// mouse capture on a tap becomes a click event instead of raising
// the soft keyboard.
const mobilePaneWidth = 68

// isMobilePane reports whether a pane of this width renders with the
// touch-first defaults. Desktop panes never cross the threshold, so
// their layout and input behavior are untouched.
func isMobilePane(width int) bool {
	return width > 0 && width < mobilePaneWidth
}
