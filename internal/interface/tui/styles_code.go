package tui

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// ---------------------------------------------------------------------------
// JSON / code display
// ---------------------------------------------------------------------------
var (
	JSONCodeBlockStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Foreground(ColorText).
				Padding(0, 1)

	CodeKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	CodeStringStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	CodeNumberStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)
)

// viewPadded centers content within the given width and height.
func viewPadded(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// ---------------------------------------------------------------------------
// ANSI escape sequence sanitization
// ---------------------------------------------------------------------------

// SanitizeANSI removes all ANSI/VT escape sequences from s.
func SanitizeANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b { // ESC
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '[': // CSI sequence
				i++
				for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
					if s[i] >= 0x40 && s[i] <= 0x7E {
						break
					}
					i++
				}
				if i < len(s) {
					i++
				}
			case ']': // OSC sequence
				i++
				for i < len(s) {
					if s[i] == 0x07 { // BEL
						i++
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			default:
				i++
			}
			continue
		}
		if s[i] < 0x20 && s[i] != '\n' && s[i] != '\r' && s[i] != '\t' {
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

// Truncate truncates a string to max length with ellipsis.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// RenderStatus returns the styled status indicator string.
func RenderStatus(status string, isActive bool) string {
	switch status {
	case "running":
		return BadgeRunning.Render(IndicatorRunning)
	case "complete", "done":
		return BadgeEnabled.Render(IndicatorComplete)
	case "error":
		return ErrorStyle.Render(IndicatorError)
	case "warning":
		return WarningStyle.Render(IndicatorWarning)
	case "disabled":
		return BadgeDisabled.Render(IndicatorDisabled)
	case "new":
		return InfoStyle.Render(IndicatorNew)
	default:
		if isActive {
			return BadgeEnabled.Render(IndicatorEnabled)
		}
		return ""
	}
}
