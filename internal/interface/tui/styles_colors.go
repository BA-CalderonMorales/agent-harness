package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Color palette — soft, luminous tones (matching lumina-bot aesthetic).
// The defaults below are the default theme; applyTheme swaps them and
// rebuilds every derived style.
// ---------------------------------------------------------------------------
var (
	ColorPrimary   = lipgloss.Color("#B388FF")
	ColorSecondary = lipgloss.Color("#80CBC4")
	ColorAccent    = lipgloss.Color("#FFD54F")
	ColorSuccess   = lipgloss.Color("#69F0AE")
	ColorError     = lipgloss.Color("#FF5252")
	ColorWarning   = lipgloss.Color("#FFB74D")
	ColorInfo      = lipgloss.Color("#64B5F6")
	ColorText      = lipgloss.Color("#E0E0E0")
	ColorTextDim   = lipgloss.Color("#9E9E9E")
	ColorSurface   = lipgloss.Color("#1E1E2E")
	ColorBorder    = lipgloss.Color("#3A3A4A")
	// ColorMuted carries the instruction layer (mode line, hints,
	// descriptions): it must hold 4.5:1 against the surface. The old
	// #5A5A6A managed 2.43:1 — instruction text below the accessibility
	// floor. Keep in sync with themes["default"].
	ColorMuted     = lipgloss.Color("#878799")
	ColorHighlight = lipgloss.Color("#2A2A3E")
)

// buildStyles re-derives every style var from the color vars. Called
// once at init and again on every theme switch — styles capture colors
// at construction, so a palette change must rebuild them.
func buildStyles() {
	applyChatStyles()
	applyCodeStyles()
	applyContentStyles()
	applyDataStyles()
	applyHelpStyles()
	applyInputStyles()
	applyListStyles()
	applyPanelStyles()
	applyStatusStyles()
	applyTabStyles()
}

func init() {
	buildStyles()
}

// ---------------------------------------------------------------------------
