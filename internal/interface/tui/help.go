package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Help is the '?' overlay: a scrollable, width-adaptive key map. The
// layout targets every device size — wide terminals read one line per
// binding; narrow ones stack the description under the key; short ones
// scroll instead of clipping the bottom sections.

type helpEntry struct {
	section string // non-empty starts a new section
	key     string
	desc    string
}

type Help struct {
	width    int
	height   int
	viewport viewport.Model
}

// NewHelp creates a new help overlay.
func NewHelp() Help {
	return Help{viewport: newViewport(80, 10)}
}

// Open initializes the help overlay.
func (h *Help) Open(width, height int, context string) {
	h.width = width
	h.height = height
	h.viewport.Width = width
	h.syncHeight()
	h.refresh()
}

func (h *Help) syncHeight() {
	// Panel frame (2) + title (1) + blank (1) sit above the scroller.
	hh := h.height - 5
	if hh < 3 {
		hh = 3
	}
	h.viewport.Height = hh
}

// Update scrolls the overlay while it is open.
func (h Help) Update(msg tea.KeyMsg) Help {
	switch msg.String() {
	case "up", "k":
		h.viewport.ScrollUp(1)
	case "down", "j":
		h.viewport.ScrollDown(1)
	case "g":
		h.viewport.GotoTop()
	case "G":
		h.viewport.GotoBottom()
	}
	return h
}

// helpSections is the single source of the key map: sections and their
// bindings. Duplicates (Shift+Tab listed as navigation and as a mode
// cycle) were merged — one binding, one home.
var helpSections = []helpEntry{
	{section: "Navigation", key: "Tab", desc: "Switch to next tab"},
	{key: "Shift+Tab", desc: "Switch to previous tab"},
	{key: "1-5", desc: "Jump to tab (Home, Chat, Sessions, Logs, Settings)"},
	{key: "h", desc: "Go to Home"},
	{key: "c", desc: "Go to Chat"},
	{key: "j/k, ↑/↓", desc: "Scroll view"},
	{key: "g/G", desc: "Top / bottom"},
	{key: "Ctrl+u/d", desc: "Half page up / down"},
	{section: "Modes", key: "Shift+Tab", desc: "Cycle agent mode: manual → auto → plan → chat (in composer)"},
	{key: "Enter", desc: "Expand the latest tool call or reasoning record"},
	{key: "click", desc: "Expand a specific tool call's full record"},
	{key: "y", desc: "Copy the expanded record — or the latest reply — to the clipboard"},
	{key: "i", desc: "Enter insert mode (type)"},
	{key: "Esc", desc: "Normal mode (cancels a running turn)"},
	{key: "Ctrl+n", desc: "Normal mode, keep the agent running — scroll while it works"},
	{key: "m", desc: "Toggle mouse capture (native select-and-copy)"},
	{key: ":", desc: "Open command palette (normal mode)"},
	{section: "Chat", key: "Enter", desc: "Send message"},
	{key: "t", desc: "Toggle tool-run collapse"},
	{key: "Ctrl+r", desc: "Cycle reasoning effort"},
	{key: "/help", desc: "Show commands"},
	{section: "Session", key: "/clear", desc: "Clear chat history"},
	{key: "/export", desc: "Export session"},
	{key: "/persona", desc: "Switch behavior mode"},
	{key: "/quit", desc: "Exit application"},
	{section: "General", key: "?", desc: "Toggle this help"},
	{key: "Ctrl+c", desc: "Quit application"},
}

// panelWidth keeps the modal readable on huge screens and usable on
// small ones: capped at 76 columns, never wider than the terminal
// minus its frame.
func (h Help) panelWidth() int {
	w := h.width - 4
	if w > 76 {
		w = 76
	}
	if w < 20 {
		w = 20
	}
	return w
}

// View renders the help overlay, centered.
func (h Help) View() string {
	if h.width == 0 {
		return ""
	}

	hint := HelpDimStyle.Render(`"Esc" to close · "j"/"k" to scroll`)
	hintPad := h.panelWidth() - 8 - lipgloss.Width(hint)
	if hintPad < 0 {
		hintPad = 0
	}
	hintRow := strings.Repeat(" ", hintPad) + hint

	panel := PanelStyle.
		Width(h.panelWidth()).
		Height(h.height - 4).
		Render(h.title() + h.viewport.View() + "\n" + hintRow)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, panel)
}

func (h Help) title() string {
	return HelpTitleStyle.Render("  Agent Harness — Keyboard Shortcuts") + "\n"
}

// refresh rebuilds the scroller content for the current width.
func (h *Help) refresh() {
	h.viewport.SetContent(h.renderContent())
	h.viewport.GotoTop()
}

// renderContent lays out the key map for the current width: one line per
// binding when the terminal is wide enough for key and description to
// sit together, stacked key-over-description below that.
func (h *Help) renderContent() string {
	var b strings.Builder
	currentSection := ""

	// Wide layout aligns descriptions in one column; the column width is
	// the widest key in the map, measured by display width (the old
	// %-12s padded by bytes including ANSI codes, so it never aligned).
	keyCol := 0
	if h.panelWidth() >= 60 {
		for _, e := range helpSections {
			if w := lipgloss.Width(e.key); w > keyCol {
				keyCol = w
			}
		}
	}

	for _, e := range helpSections {
		if e.section != "" {
			if currentSection != "" {
				b.WriteString("\n")
			}
			b.WriteString(CategoryStyle.Render("  " + e.section))
			b.WriteString("\n")
			currentSection = e.section
			continue
		}
		if h.panelWidth() < 60 {
			// Narrow: description stacks under its key so nothing wraps
			// mid-sentence.
			b.WriteString("  " + HelpKeyStyle.Render(e.key))
			b.WriteString("\n")
			b.WriteString("      " + HelpDimStyle.Render(wrapText(e.desc, h.panelWidth()-8)))
			b.WriteString("\n")
			continue
		}
		b.WriteString("  " + HelpKeyStyle.Render(e.key) + strings.Repeat(" ", keyCol-lipgloss.Width(e.key)+2))
		b.WriteString(HelpDimStyle.Render(wrapText(e.desc, h.panelWidth()-keyCol-6)))
		b.WriteString("\n")
	}
	return b.String()
}
