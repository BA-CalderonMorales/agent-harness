package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

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
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("h"), HelpDimStyle.Render("Go to Home")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("c"), HelpDimStyle.Render("Go to Chat")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("l"), HelpDimStyle.Render("Next tab (login wizard when setup needed)")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("j/k, ↑/↓"), HelpDimStyle.Render("Scroll view")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("g/G"), HelpDimStyle.Render("Top / bottom")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+u/d"), HelpDimStyle.Render("Half page up / down")))
	lines = append(lines, "")
	lines = append(lines, CategoryStyle.Render("  Modes"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Shift+Tab"), HelpDimStyle.Render("Cycle agent mode: manual → auto → plan → chat (in composer)")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("i"), HelpDimStyle.Render("Enter insert mode (type)")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Esc"), HelpDimStyle.Render("Normal mode (cancels a running turn)")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+n"), HelpDimStyle.Render("Normal mode, keep the agent running — scroll while it works")))
	lines = append(lines, "")
	lines = append(lines, CategoryStyle.Render("  Commands"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+P"), HelpDimStyle.Render("Open command palette")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render(":"), HelpDimStyle.Render("Open command palette (normal mode)")))
	lines = append(lines, "")
	lines = append(lines, CategoryStyle.Render("  Chat"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Enter"), HelpDimStyle.Render("Send message")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("t"), HelpDimStyle.Render("Toggle tool-run collapse")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+R"), HelpDimStyle.Render("Cycle reasoning effort")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/help"), HelpDimStyle.Render("Show commands")))
	lines = append(lines, "")

	// Session
	lines = append(lines, CategoryStyle.Render("  Session"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/clear"), HelpDimStyle.Render("Clear chat history")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/export"), HelpDimStyle.Render("Export session")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/persona"), HelpDimStyle.Render("Switch behavior mode")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("/quit"), HelpDimStyle.Render("Exit application")))
	lines = append(lines, "")

	// General
	lines = append(lines, CategoryStyle.Render("  General"))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("?"), HelpDimStyle.Render("Toggle this help")))
	lines = append(lines, fmt.Sprintf("    %-12s %s", HelpKeyStyle.Render("Ctrl+C"), HelpDimStyle.Render("Quit application")))

	return strings.Join(lines, "\n")
}
