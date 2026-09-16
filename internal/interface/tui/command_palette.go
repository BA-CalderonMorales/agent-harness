package tui

import (
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Command Palette — interactive slash command discovery and selection
// Triggered by typing "/" in chat input when empty
// ---------------------------------------------------------------------------

type commandInfo = commands.CommandInfo

type CommandInfo = commands.CommandInfo

// CommandPaletteModel is the interactive command palette
type CommandPaletteModel struct {
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	commands    []commandInfo
	filtered    []commandInfo
	cursor      int
	searchQuery string
	selected    *commandInfo
	showing     bool
}

// NewCommandPalette creates an empty command palette. The command list is
// populated exclusively by SetCommands (the app feeds it the live
// SlashRegistry metadata at boot); a hardcoded seed would drift from the
// registry and resurrect phantom commands.
func NewCommandPalette() CommandPaletteModel {
	m := CommandPaletteModel{
		commands: make([]commandInfo, 0),
		filtered: make([]commandInfo, 0),
		cursor:   0,
		showing:  false,
	}
	return m
}

// SetCommands updates the list of available commands in the palette dynamically.
func (m *CommandPaletteModel) SetCommands(cmds []CommandInfo) {
	m.commands = append([]CommandInfo(nil), cmds...)
	m.filtered = m.commands
	m.cursor = 0
}

// modalSpec is the palette's frame description: View renders it and Open
// sizes the viewport from it, so the body can never overflow the frame.
func (m CommandPaletteModel) modalSpec() modalSpec {
	return modalSpec{
		title:          "Commands",
		hint:           "Type to filter, Enter to select.",
		footer:         "j/k: navigate  Enter: select  Tab: auto-complete  Esc: cancel",
		preferredWidth: modalMaxWidth,
	}
}

// Open shows the command palette
func (m *CommandPaletteModel) Open(width, height int) {
	m.width = width
	m.height = height
	m.showing = true
	m.searchQuery = ""
	m.selected = nil
	m.filtered = m.commands
	m.cursor = 0

	spec := m.modalSpec()
	panelW := panelWidth(spec.preferredWidth, width)
	vpH := modalBodyRows(width, height, spec)
	if vpH < 3 {
		vpH = 3
	}

	if !m.ready {
		m.viewport = newViewport(modalInnerWidth(panelW), vpH)
		m.ready = true
	} else {
		m.viewport.Width = modalInnerWidth(panelW)
		m.viewport.Height = vpH
	}

	m.updateContent()
}

// Close hides the command palette
func (m *CommandPaletteModel) Close() {
	m.showing = false
	m.searchQuery = ""
}

// IsShowing returns whether the palette is visible
func (m CommandPaletteModel) IsShowing() bool {
	return m.showing
}

// SelectedCommand returns the chosen command (nil if none)
func (m CommandPaletteModel) SelectedCommand() *commandInfo {
	return m.selected
}

// Update handles key events for the palette.
// Returns (closed, cmd).
func (m *CommandPaletteModel) Update(msg tea.Msg) (closed bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.Close()
			return true, nil
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.selected = &m.filtered[m.cursor]
				m.Close()
				return true, nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.updateContent()
			}
			return false, nil
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.updateContent()
			}
			return false, nil
		case "pgup":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateContent()
			return false, nil
		case "pgdown":
			m.cursor += 10
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateContent()
			return false, nil
		case "home", "g":
			m.cursor = 0
			m.updateContent()
			return false, nil
		case "end", "G":
			m.cursor = len(m.filtered) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateContent()
			return false, nil
		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.applyFilter()
			} else {
				m.Close()
				return true, nil
			}
		case "tab":
			if len(m.filtered) > 0 {
				m.selected = &m.filtered[0]
				m.Close()
				return true, nil
			}
		default:
			if len(msg.String()) == 1 {
				ch := msg.String()[0]
				if ch >= ' ' && ch <= '~' && ch != '/' {
					m.searchQuery += strings.ToLower(string(ch))
					m.applyFilter()
				}
			}
		}

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		return false, cmd
	}

	return false, cmd
}

func (m *CommandPaletteModel) applyFilter() {
	if m.searchQuery == "" {
		m.filtered = make([]commandInfo, len(m.commands))
		copy(m.filtered, m.commands)
	} else {
		// Fresh backing array: m.filtered must never alias m.commands,
		// or the first keystroke's appends corrupt the source list
		// (searching "persona" duplicated the entry into itself).
		m.filtered = make([]commandInfo, 0, len(m.commands))
		query := strings.ToLower(m.searchQuery)
		for _, cmd := range m.commands {
			if strings.Contains(strings.ToLower(cmd.Command), query) ||
				strings.Contains(strings.ToLower(cmd.Description), query) ||
				strings.Contains(strings.ToLower(cmd.Category), query) {
				m.filtered = append(m.filtered, cmd)
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.updateContent()
}

func (m *CommandPaletteModel) updateContent() {
	m.fitViewport()
	m.viewport.SetContent(m.buildContent())
	m.syncViewportToCursor()
}

// fitViewport sizes the viewport to the frame's body budget, capped to
// the rows the current list needs so the panel hugs its content like
// the other overlays instead of reserving a screen of blank rows.
func (m *CommandPaletteModel) fitViewport() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	spec := m.modalSpec()
	// Width first: row wrapping depends on it, and the row count decides
	// the height.
	m.viewport.Width = modalInnerWidth(panelWidth(spec.preferredWidth, m.width))

	want := len(strings.Split(strings.TrimRight(m.buildContent(), "\n"), "\n"))
	if budget := modalBodyRows(m.width, m.height, spec); want > budget {
		want = budget
	}
	if want < 3 {
		want = 3
	}
	m.viewport.Height = want
}

func (m *CommandPaletteModel) syncViewportToCursor() {
	visualLine := 0
	for i := 0; i < m.cursor && i < len(m.filtered); i++ {
		if i == 0 || m.filtered[i].Category != m.filtered[i-1].Category {
			visualLine++
		}
		visualLine++
	}

	// The frame owns the title and hint, so only the live search line
	// can sit above the first command row.
	headerLines := 0
	if m.searchQuery != "" {
		headerLines = 2
	}
	visualLine += headerLines

	if visualLine < m.viewport.YOffset {
		m.viewport.SetYOffset(visualLine)
	} else if visualLine >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(visualLine - m.viewport.Height + 1)
	}
}

// buildContent renders the scrollable body only: the frame owns the
// title and key hints, so they never scroll out of reach.
func (m CommandPaletteModel) buildContent() string {
	var b strings.Builder

	if m.searchQuery != "" {
		b.WriteString("Search: " + InfoStyle.Render(m.searchQuery) + "\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(HelpDimStyle.Render("No commands match your search."))
		return b.String()
	}

	currentCategory := ""
	for i, cmd := range m.filtered {
		if cmd.Category != currentCategory {
			currentCategory = cmd.Category
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(CategoryStyle.Render(currentCategory) + "\n")
		}
		b.WriteString(m.renderCommandLine(cmd, i == m.cursor) + "\n")
	}

	return b.String()
}

func (m CommandPaletteModel) renderCommandLine(cmd commandInfo, isSelected bool) string {
	// Equal-width markers: a selected row must not shift the columns of
	// the command names and descriptions below it.
	indicator := IndicatorUnselected
	if isSelected {
		indicator = IndicatorSelected
	}

	cmdText := cmd.Command
	if cmd.Args != "" {
		cmdText += " " + HelpDimStyle.Render(cmd.Args)
	}
	desc := HelpDimStyle.Render(cmd.Description)

	availableWidth := m.viewport.Width - 4
	cmdWidth := lipgloss.Width(cmdText)
	descWidth := lipgloss.Width(cmd.Description)

	if cmdWidth+descWidth+4 > availableWidth && availableWidth > 20 {
		line := indicator + cmdText + "\n      " + desc
		if isSelected {
			return ListSelectedStyle.Render(line)
		}
		return line
	}

	maxCmdWidth := 25
	padding := ""
	if cmdWidth < maxCmdWidth {
		padding = strings.Repeat(" ", maxCmdWidth-cmdWidth)
	}

	line := indicator + cmdText + padding + "  " + desc
	if isSelected {
		return ListSelectedStyle.Render(line)
	}
	return line
}

// paletteFooterHint returns a footer hint that fits the palette panel's
// inner width. The panel is viewport.Width wide minus its border and
// padding; the longest variant that still fits wins, so the footer never
// clips or wraps at narrow terminal widths.
func paletteFooterHint(panelWidth int, canScroll bool) string {
	inner := panelWidth - 4 // border (2) + padding (2)
	if inner < 10 {
		inner = 10
	}
	scroll := ""
	if canScroll {
		scroll = " scroll"
	}
	// Richest first: the loop returns the first variant that fits, so a
	// shortest-first list always returned the tersest hint no matter how
	// much room the panel had.
	candidates := []string{
		"j/k: navigate  Enter: select  Tab: auto-complete",
		"j/k:nav Enter:select Tab:auto",
		"j/k:nav Enter:select",
		"Esc:cancel Enter:select",
	}
	for _, base := range candidates {
		if lipgloss.Width(base+scroll) <= inner {
			return HelpDimStyle.Render(base + scroll)
		}
	}
	return HelpDimStyle.Render(candidates[len(candidates)-1])
}

// View renders the command palette through the shared modal frame.
func (m CommandPaletteModel) View(width, height int) string {
	if !m.ready || !m.showing {
		return ""
	}

	spec := m.modalSpec()
	spec.footer = paletteFooterHint(modalInnerWidth(panelWidth(spec.preferredWidth, width)), m.viewport.ScrollPercent() < 1.0)
	spec.items = []modalItem{modalTextItem(m.viewport.View())}
	spec.cursor = -1
	return renderModal(width, height, spec)
}
