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
