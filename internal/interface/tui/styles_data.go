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

// spinnerDots are the braille spinner frames shared by loading messages.
var spinnerDots = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// thinkingSpinner is the spinning-diamond animation used by the live
// thinking indicator in assistant headers: quarter arcs that read as a
// spinning ◆.
var thinkingSpinner = []string{"◐", "◓", "◑", "◒"}

// spinnerFrameAt returns the spinner glyph for a monotonic tick counter.
func spinnerFrameAt(tick int) string {
	if len(spinnerDots) == 0 {
		return " "
	}
	return spinnerDots[tick%len(spinnerDots)]
}

// thinkingFrameAt returns the spinning-diamond glyph for a tick counter.
func thinkingFrameAt(tick int) string {
	if len(thinkingSpinner) == 0 {
		return "◆"
	}
	return thinkingSpinner[tick%len(thinkingSpinner)]
}

// SpinnerRender returns a loading message with a spinner.
func SpinnerRender(msg string) string {
	idx := int(time.Now().UnixMilli()/80) % len(spinnerDots)
	return InfoStyle.Render(spinnerDots[idx]) + " " + HelpDimStyle.Render(msg)
}

// ---------------------------------------------------------------------------
