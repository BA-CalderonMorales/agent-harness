package tui

import (
	"github.com/charmbracelet/lipgloss"
	"time"
)

// Data display styles
var (
	DataLabel = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Width(12)

	DataValue = lipgloss.NewStyle().
			Foreground(ColorText)

	DataMono = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorHighlight)
)

// ---------------------------------------------------------------------------
// Badges / indicators (text-based, no emojis)
// ---------------------------------------------------------------------------
var (
	BadgeEnabled = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	BadgeDisabled = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	BadgeRunning = lipgloss.NewStyle().
			Foreground(ColorInfo).
			Bold(true)

	BadgeWarning = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)
)

// ---------------------------------------------------------------------------
// Text indicators (replacing emoji for minimalism)
// ---------------------------------------------------------------------------
const (
	IndicatorSelected   = "> "
	IndicatorUnselected = "  "
	IndicatorActive     = "(*)"
	IndicatorRunning    = "[running]"
	IndicatorComplete   = "[done]"
	IndicatorError      = "[error]"
	IndicatorWarning    = "[warn]"
	IndicatorDisabled   = "[off]"
	IndicatorEnabled    = "[on]"
	IndicatorNew        = "[new]"
)

// ---------------------------------------------------------------------------
// Loading spinner
// ---------------------------------------------------------------------------

// SpinnerRender returns a loading message with a spinner.
func SpinnerRender(msg string) string {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(time.Now().UnixMilli()/80) % len(dots)
	return InfoStyle.Render(dots[idx]) + " " + HelpDimStyle.Render(msg)
}

// ---------------------------------------------------------------------------
