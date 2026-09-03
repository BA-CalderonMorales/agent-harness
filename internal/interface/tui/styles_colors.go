package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Color palette — soft, luminous tones (matching lumina-bot aesthetic)
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
	ColorBg        = lipgloss.Color("#14141E") // whole-TUI surface; children inherit via SGR state
	ColorBorder    = lipgloss.Color("#3A3A4A")
	ColorMuted     = lipgloss.Color("#5A5A6A")
	ColorHighlight = lipgloss.Color("#2A2A3E")
)

// ---------------------------------------------------------------------------

// AppBgStyle paints the whole TUI surface. Terminal SGR is stateful:
// foreground-only children (our transparent markdown, transcript text)
// inherit this background, which is why stripping child backgrounds
// was the prerequisite for a unified surface.
var AppBgStyle = lipgloss.NewStyle().Background(ColorBg)
