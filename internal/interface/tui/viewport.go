package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
)

// newViewport constructs the app's viewport with an explicit scroll
// policy: bubbles' default keymap (u/d/f/b/space page scrolling) is
// disabled on purpose. Raw letters fed to a panel's viewport must never
// jump the screen half a page — scrolling belongs to the app-level
// bindings (j/k, arrows, g/G, Ctrl+u/Ctrl+d, pgup/pgdn).
func newViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.KeyMap = viewport.KeyMap{}
	return vp
}
