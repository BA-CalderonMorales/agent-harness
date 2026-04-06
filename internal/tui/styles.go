// Design system for agent-harness TUI
// Visual language: soft, luminous tones with text-based indicators (no emojis)

package tui

import (
	"strings"
	"time"

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
	ColorBorder    = lipgloss.Color("#3A3A4A")
	ColorMuted     = lipgloss.Color("#5A5A6A")
	ColorHighlight = lipgloss.Color("#2A2A3E")
)

// ---------------------------------------------------------------------------
// Tab bar - Golazo-inspired centered design
// ---------------------------------------------------------------------------
var (
	// TabNormal is the style for inactive tabs
	TabNormal = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorTextDim).
			Bold(false)

	// TabActive is the style for the active tab with visual indicator
	TabActive = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorPrimary).
			Bold(true).
			Underline(true)

	// TabBarStyle with elegant bottom border
	TabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorBorder).
			PaddingBottom(0)

	// TitleStyle for view headers (golazo-inspired)
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	// SubtitleStyle for view subtitles
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Padding(0, 1)
)

// ---------------------------------------------------------------------------
// Logo / branding
// ---------------------------------------------------------------------------
var LogoStyle = lipgloss.NewStyle().
	Foreground(ColorPrimary).
	Bold(true).
	Padding(0, 1)

// ---------------------------------------------------------------------------
// Content area
// ---------------------------------------------------------------------------
var (
	ContentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	PanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	DetailPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)
)

// ---------------------------------------------------------------------------
// Chat conversation
// ---------------------------------------------------------------------------
var (
	UserPromptStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	MessageStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MessageBubbleUser = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(ColorSecondary).
				PaddingLeft(1)

	MessageBubbleAssistant = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(ColorPrimary).
				PaddingLeft(1)

	ToolCallStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent).
			Foreground(ColorAccent).
			Padding(0, 1)

	// ToolCommandPreviewStyle - grey preview of actual command being executed
	// Used for human-readable command preview (like Kimi does)
	ToolCommandPreviewStyle = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				Italic(true)

	ToolRunningStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(ColorInfo).
				Foreground(ColorInfo).
				Padding(0, 1)

	ToolThinkingStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorWarning).
				Foreground(ColorWarning).
				Padding(0, 1)

	ToolDoneStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Foreground(ColorSuccess).
			Padding(0, 1)

	ToolErrorStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Foreground(ColorError).
			Padding(0, 1)

	StreamingStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Italic(true)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	ScrollHintStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)
)

// ---------------------------------------------------------------------------
// Input area - Golazo-inspired styling
// ---------------------------------------------------------------------------
var (
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	// InputContainerStyle - styled container for the input area
	// CRITICAL FIX: Consistent background, no strange color changes
	InputContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(true).
				BorderForeground(ColorBorder).
				Background(ColorSurface).
				Padding(0, 1)

	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	InputHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------
var (
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Padding(0, 1)

	StatusOnline = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StatusOffline = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	StatusLabel = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	StatusHintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StatusConnecting = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)
)

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------
var (
	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	HelpDimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	HelpSectionSep = lipgloss.NewStyle().
			Foreground(ColorBorder)

	CategoryStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginTop(1).
			MarginBottom(0)
)

// ---------------------------------------------------------------------------
// List / table styles
// ---------------------------------------------------------------------------
var (
	ListTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 0, 1, 0)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 2)

	ListSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Padding(0, 2).
				Background(ColorHighlight)

	ListDimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Padding(0, 2)

	ListHeaderStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Underline(true)

	ListSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)
)

// ---------------------------------------------------------------------------
// Design System - Panel styles
// ---------------------------------------------------------------------------
var (
	PanelPrimary = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	PanelSecondary = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	PanelHighlight = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorHighlight).
			Padding(1, 2)
)

// Header styles
var (
	HeaderPrimary = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	HeaderSecondary = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	HeaderTertiary = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)
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

// ToolSpinnerRender returns a tool-specific spinner animation
func ToolSpinnerRender(frame int) string {
	frames := []string{"◐", "◓", "◑", "◒"}
	return frames[frame%len(frames)]
}

// ---------------------------------------------------------------------------
// JSON / code display
// ---------------------------------------------------------------------------
var (
	CodeBlockStyle = lipgloss.NewStyle().
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
