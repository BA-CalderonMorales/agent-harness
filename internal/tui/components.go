// Shared layout components for consistent TUI rendering

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// ViewPort represents the available screen dimensions for a view.
// ---------------------------------------------------------------------------
type ViewPort struct {
	Width  int
	Height int
}

// ---------------------------------------------------------------------------
// ListItem represents a single item in a list view.
// ---------------------------------------------------------------------------
type ListItem struct {
	ID       string
	Label    string
	Subtitle string
	Status   ItemStatus
}

// ---------------------------------------------------------------------------
// ItemStatus represents the state of a list item.
// ---------------------------------------------------------------------------
type ItemStatus int

const (
	StatusNeutral ItemStatus = iota
	StatusRunning
	StatusComplete
	StatusError
	StatusWarning
	StatusDisabled
	StatusNew
)

// ---------------------------------------------------------------------------
// ActionHint represents a keyboard shortcut and its description.
// ---------------------------------------------------------------------------
type ActionHint struct {
	Key  string
	Desc string
}

// ---------------------------------------------------------------------------
// EmptyState represents the content for an empty view.
// ---------------------------------------------------------------------------
type EmptyState struct {
	Title       string
	Description string
	Actions     []ActionHint
}

// ---------------------------------------------------------------------------
// TwoPaneLayout represents a list+detail view configuration.
// ---------------------------------------------------------------------------
type TwoPaneLayout struct {
	ListTitle   string
	ListWidth   int
	DetailTitle string
	Cursor      int
	Items       []ListItem
	Footer      []ActionHint
}

// ---------------------------------------------------------------------------
// HeaderConfig represents a view header configuration.
// ---------------------------------------------------------------------------
type HeaderConfig struct {
	Title    string
	Subtitle string
	Badge    string
	Count    int
}

// ---------------------------------------------------------------------------
// Status rendering
// ---------------------------------------------------------------------------

// StatusString returns a text indicator for an item status.
func StatusString(s ItemStatus) string {
	switch s {
	case StatusRunning:
		return IndicatorRunning
	case StatusComplete:
		return IndicatorComplete
	case StatusError:
		return IndicatorError
	case StatusWarning:
		return IndicatorWarning
	case StatusDisabled:
		return IndicatorDisabled
	case StatusNew:
		return IndicatorNew
	default:
		return ""
	}
}

// StatusStyle returns the lipgloss style for an item status.
func StatusStyle(s ItemStatus) lipgloss.Style {
	switch s {
	case StatusRunning:
		return BadgeRunning
	case StatusComplete:
		return BadgeEnabled
	case StatusError:
		return ErrorStyle
	case StatusWarning:
		return WarningStyle
	case StatusDisabled:
		return BadgeDisabled
	case StatusNew:
		return InfoStyle
	default:
		return StatusLabel
	}
}

// RenderStatusBadge returns the styled status indicator string.
func RenderStatusBadge(s ItemStatus) string {
	str := StatusString(s)
	if str == "" {
		return ""
	}
	return StatusStyle(s).Render(str)
}

// ---------------------------------------------------------------------------
// Header rendering
// ---------------------------------------------------------------------------

// RenderHeader returns a standardized view header.
func RenderHeader(cfg HeaderConfig) string {
	var parts []string

	title := ListTitleStyle.Render(cfg.Title)
	parts = append(parts, title)

	if cfg.Subtitle != "" {
		parts = append(parts, HelpDimStyle.Render("  "+cfg.Subtitle))
	}

	if cfg.Badge != "" {
		parts = append(parts, "  "+InfoStyle.Render("["+cfg.Badge+"]"))
	}

	if cfg.Count >= 0 {
		parts = append(parts, HelpDimStyle.Render(fmt.Sprintf("  (%d)", cfg.Count)))
	}

	return strings.Join(parts, "") + "\n\n"
}

// ---------------------------------------------------------------------------
// Footer rendering
// ---------------------------------------------------------------------------

// RenderFooter returns a standardized footer with action hints.
func RenderFooter(actions []ActionHint) string {
	if len(actions) == 0 {
		return ""
	}

	var hints []string
	for _, a := range actions {
		hints = append(hints, HelpKeyStyle.Render(a.Key)+": "+HelpDimStyle.Render(a.Desc))
	}

	return "\n" + strings.Join(hints, "  ")
}

// RenderCompactFooter returns a minimal footer for simple views.
func RenderCompactFooter(actions []ActionHint) string {
	if len(actions) == 0 {
		return ""
	}

	var parts []string
	for _, a := range actions {
		parts = append(parts, a.Key+": "+a.Desc)
	}

	return "\n" + HelpDimStyle.Render("  "+strings.Join(parts, "  "))
}

// ---------------------------------------------------------------------------
// Empty state rendering
// ---------------------------------------------------------------------------

// RenderEmptyState returns a centered empty state display.
func RenderEmptyState(vp ViewPort, state EmptyState) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(ListTitleStyle.Render("  "+state.Title) + "\n\n")
	b.WriteString(HelpDimStyle.Render("  "+state.Description) + "\n")

	if len(state.Actions) > 0 {
		b.WriteString("\n")
		for _, a := range state.Actions {
			b.WriteString("  " + HelpKeyStyle.Render(a.Key) + ": " + HelpDimStyle.Render(a.Desc) + "\n")
		}
	}

	return viewPadded(vp.Width, vp.Height, b.String())
}

// RenderEmptyStateInline returns an inline empty state for embedded lists.
func RenderEmptyStateInline(state EmptyState) string {
	var b strings.Builder

	b.WriteString(ListTitleStyle.Render("  "+state.Title) + "\n\n")
	b.WriteString(HelpDimStyle.Render("  "+state.Description) + "\n")

	if len(state.Actions) > 0 {
		b.WriteString("\n")
		for _, a := range state.Actions {
			b.WriteString("  " + HelpKeyStyle.Render(a.Key) + ": " + HelpDimStyle.Render(a.Desc) + "\n")
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Error state rendering
// ---------------------------------------------------------------------------

// RenderErrorState returns a standardized error display.
func RenderErrorState(vp ViewPort, err error, retryAction string) string {
	msg := "Unknown error"
	if err != nil {
		msg = err.Error()
	}

	content := ErrorStyle.Render("  [error] "+msg) + "\n\n"
	if retryAction != "" {
		content += HelpDimStyle.Render("  " + retryAction)
	}

	return viewPadded(vp.Width, vp.Height, content)
}

// RenderErrorInline returns an inline error for embedded displays.
func RenderErrorInline(err error, retryAction string) string {
	msg := "Unknown error"
	if err != nil {
		msg = err.Error()
	}

	content := ErrorStyle.Render("  [error] "+msg) + "\n"
	if retryAction != "" {
		content += HelpDimStyle.Render("  " + retryAction)
	}

	return content
}

// ---------------------------------------------------------------------------
// Loading state rendering
// ---------------------------------------------------------------------------

// RenderLoading returns a centered loading spinner.
func RenderLoading(vp ViewPort, message string) string {
	return viewPadded(vp.Width, vp.Height, SpinnerRender(message))
}

// RenderLoadingInline returns an inline loading indicator.
func RenderLoadingInline(message string) string {
	return SpinnerRender(message)
}

// ---------------------------------------------------------------------------
// List rendering
// ---------------------------------------------------------------------------

// RenderListItem returns a single list item string.
func RenderListItem(item ListItem, index, cursor int, width int) string {
	prefix := IndicatorUnselected
	style := ListItemStyle

	if index == cursor {
		prefix = IndicatorSelected
		style = ListSelectedStyle
	}

	line := style.Render(prefix + item.Label)

	// Add status indicator if present
	if item.Status != StatusNeutral {
		statusStr := RenderStatusBadge(item.Status)
		if statusStr != "" {
			line += " " + statusStr
		}
	}

	result := line

	// Add subtitle on next line if present
	if item.Subtitle != "" {
		result += "\n" + HelpDimStyle.Render("    "+Truncate(item.Subtitle, width-6))
	}

	return result
}

// ---------------------------------------------------------------------------
// Two-pane layout rendering
// ---------------------------------------------------------------------------

// TwoPaneWidths calculates consistent pane widths.
func TwoPaneWidths(totalWidth int) (listWidth, detailWidth int) {
	listWidth = 36
	if totalWidth < 80 {
		listWidth = totalWidth / 2
	}
	detailWidth = totalWidth - listWidth - 2
	if detailWidth < 10 {
		detailWidth = 10
	}
	return listWidth, detailWidth
}

// RenderTwoPane renders a consistent list+detail layout.
func RenderTwoPane(vp ViewPort, layout TwoPaneLayout, renderDetail func() string) string {
	listW, detailW := TwoPaneWidths(vp.Width)

	// Render list
	var listB strings.Builder
	listB.WriteString(ListTitleStyle.Render("  "+layout.ListTitle) + "\n\n")

	if len(layout.Items) == 0 {
		listB.WriteString(HelpDimStyle.Render("  No items.") + "\n")
	} else {
		for i, item := range layout.Items {
			listB.WriteString(RenderListItem(item, i, layout.Cursor, listW) + "\n")
		}
	}

	if len(layout.Footer) > 0 {
		listB.WriteString(RenderCompactFooter(layout.Footer))
	}

	listContent := lipgloss.NewStyle().Width(listW).Height(vp.Height).Render(listB.String())

	// Render detail
	detailContent := ""
	if len(layout.Items) > 0 && layout.Cursor >= 0 && layout.Cursor < len(layout.Items) {
		detailStr := renderDetail()
		detailContent = DetailPanelStyle.Width(detailW).Height(vp.Height - 2).Render(detailStr)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, detailContent)
}

// ---------------------------------------------------------------------------
// FormatKeyHints formats action hints for display.
// ---------------------------------------------------------------------------
func FormatKeyHints(actions []ActionHint) string {
	var parts []string
	for _, a := range actions {
		parts = append(parts, a.Key+": "+a.Desc)
	}
	return strings.Join(parts, "  ")
}

// Help represents a help overlay model.
type Help struct {
	width   int
	height  int
	context string
}

// NewHelp creates a new help overlay.
func NewHelp() Help {
	return Help{}
}

// Open initializes the help overlay.
func (h *Help) Open(width, height int, context string) {
	h.width = width
	h.height = height
	h.context = context
}

// View renders the help overlay.
func (h Help) View() string {
	if h.width == 0 {
		return ""
	}

	content := h.renderContent()
	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center,
		PanelStyle.Width(h.width-4).Height(h.height-4).Render(content))
}

func (h Help) renderContent() string {
	var lines []string

	lines = append(lines, HelpTitleStyle.Render("  Agent Harness - Keyboard Shortcuts"))
	lines = append(lines, "")

	// Navigation
	lines = append(lines, CategoryStyle.Render("  Navigation"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Tab"), HelpDimStyle.Render("Switch to next tab")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Shift+Tab"), HelpDimStyle.Render("Switch to previous tab")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("1-4"), HelpDimStyle.Render("Jump to tab 1-4")))
	lines = append(lines, "")

	// Chat
	lines = append(lines, CategoryStyle.Render("  Chat"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Enter"), HelpDimStyle.Render("Send message")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("↑/↓"), HelpDimStyle.Render("Navigate history")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/help"), HelpDimStyle.Render("Show commands")))
	lines = append(lines, "")

	// Session
	lines = append(lines, CategoryStyle.Render("  Session"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/clear"), HelpDimStyle.Render("Clear chat history")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/export"), HelpDimStyle.Render("Export session")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/quit"), HelpDimStyle.Render("Exit application")))
	lines = append(lines, "")

	// General
	lines = append(lines, CategoryStyle.Render("  General"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("?"), HelpDimStyle.Render("Toggle this help")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+C"), HelpDimStyle.Render("Quit application")))

	return strings.Join(lines, "\n")
}
