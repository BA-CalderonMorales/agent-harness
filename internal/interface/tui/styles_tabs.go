package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Tab bar - Golazo-inspired centered design
// ---------------------------------------------------------------------------
var (
	TabNormal          lipgloss.Style
	SectionHeaderStyle lipgloss.Style
	TabActive          lipgloss.Style
	TabBarStyle        lipgloss.Style
	TitleStyle         lipgloss.Style
	SubtitleStyle      lipgloss.Style
)

func applyTabStyles() {
	// TabNormal is the style for inactive tabs
	TabNormal = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ColorTextDim).
		Bold(false)

	// SectionHeaderStyle renders section category headers in views.
	// Deliberately quiet: a dim label beats a loud banner when the eye
	// is scanning settings rows, not navigating pages.
	SectionHeaderStyle = lipgloss.NewStyle().
		Foreground(ColorMuted)

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
}

// ---------------------------------------------------------------------------
// Logo / branding
// ---------------------------------------------------------------------------
var LogoStyle = lipgloss.NewStyle().
	Foreground(ColorPrimary).
	Bold(true).
	Padding(0, 1)

	// ---------------------------------------------------------------------------
