package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
	"strings"
)

// ---------------------------------------------------------------------------
// JSON / code display
// ---------------------------------------------------------------------------
var (
	JSONCodeBlockStyle lipgloss.Style
	CodeKeyStyle       lipgloss.Style
	CodeStringStyle    lipgloss.Style
	CodeNumberStyle    lipgloss.Style
)

func applyCodeStyles() {
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
}

// fitBlock hard-caps a multi-line block's row width: word wrap for
// clean breaks first, a character wrap as the ceiling for unbreakable
// tokens. lipgloss.Place centers and pads but never shrinks, so an
// overflowing block would push past the pane and wrap — unbudgeted
// height. Both reflow wrappers are ANSI-aware; styles survive.
func fitBlock(width int, content string) string {
	if width < 1 {
		width = 1
	}
	return wrap.String(wordwrap.String(content, width), width)
}

// fitBlockCode wraps code — command lines in an approval view. Spaces
// are the only break points: a hyphen is semantic in a flag or a
// hostname, and a token split mid-word invites the reader to approve
// something they misread. The character ceiling still applies for a
// single token wider than the line.
func fitBlockCode(width int, content string) string {
	if width < 1 {
		width = 1
	}
	ww := wordwrap.NewWriter(width)
	ww.Breakpoints = []rune{}
	ww.Write([]byte(content))
	ww.Close()
	return wrap.String(ww.String(), width)
}

// viewPadded centers content within the given width and height, after
// fitting the block to the width.
func viewPadded(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, fitBlock(width, content))
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
