package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Command Palette — interactive slash command discovery and selection
// Triggered by typing "/" in chat input when empty
// ---------------------------------------------------------------------------

type commandInfo struct {
	Command     string
	Args        string
	Description string
	Category    string
}

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

// NewCommandPalette creates a new command palette with all available commands
func NewCommandPalette() CommandPaletteModel {
	m := CommandPaletteModel{
		commands: getAgentHarnessCommands(),
		filtered: make([]commandInfo, 0),
		cursor:   0,
		showing:  false,
	}
	m.filtered = m.commands
	return m
}

func getAgentHarnessCommands() []commandInfo {
	return []commandInfo{
		{Command: "/help", Description: "Show available commands", Category: "Session"},
		{Command: "/status", Description: "Show session status", Category: "Session"},
		{Command: "/clear", Description: "Clear chat history", Category: "Session"},
		{Command: "/compact", Description: "Compact session to reduce tokens", Category: "Session"},
		{Command: "/export", Args: "[path]", Description: "Export conversation to file", Category: "Session"},
		{Command: "/session", Args: "[list|load <id>]", Description: "Manage sessions", Category: "Session"},
		{Command: "/reset", Description: "Reset agent harness (destructive)", Category: "Session"},
		{Command: "/quit", Description: "Exit application", Category: "Session"},

		{Command: "/model", Args: "[model-id]", Description: "Show or change model", Category: "Model"},
		{Command: "/current-model", Description: "Show current model", Category: "Model"},
		{Command: "/cost", Description: "Show token usage and cost", Category: "Output"},
		{Command: "/diff", Description: "Show git diff", Category: "Output"},
		{Command: "/version", Description: "Show version", Category: "System"},
		{Command: "/config", Description: "Show configuration", Category: "System"},
		{Command: "/permissions", Args: "[mode]", Description: "Show or change permission mode", Category: "System"},
		{Command: "/workspace", Description: "Show workspace information", Category: "System"},
		{Command: "/agents", Description: "Show available agents", Category: "Tools"},
		{Command: "/skills", Description: "Show available skills", Category: "Tools"},
		{Command: "/persona", Args: "[name]", Description: "Show or change persona", Category: "Tools"},
		{Command: "/audit", Description: "Show recent tool activity", Category: "System"},
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

	panelW := 70
	if width-8 < panelW {
		panelW = width - 8
	}
	if panelW < 30 {
		panelW = 30
	}

	minHeight := 8
	vpH := height - 6
	if vpH < minHeight {
		vpH = minHeight
	}
	maxVpH := height - 4
	if vpH > maxVpH {
		vpH = maxVpH
	}

	if !m.ready {
		m.viewport = viewport.New(panelW, vpH)
		m.ready = true
	} else {
		m.viewport.Width = panelW
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
		m.filtered = m.filtered[:0]
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
	m.viewport.SetContent(m.buildContent())
	m.syncViewportToCursor()
}

func (m *CommandPaletteModel) syncViewportToCursor() {
	visualLine := 0
	for i := 0; i < m.cursor && i < len(m.filtered); i++ {
		if i == 0 || m.filtered[i].Category != m.filtered[i-1].Category {
			visualLine++
		}
		visualLine++
	}

	headerLines := 3
	if m.searchQuery != "" {
		headerLines = 4
	}
	visualLine += headerLines

	if visualLine < m.viewport.YOffset {
		m.viewport.SetYOffset(visualLine)
	} else if visualLine >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(visualLine - m.viewport.Height + 1)
	}
}

func (m CommandPaletteModel) buildContent() string {
	var b strings.Builder

	b.WriteString(HelpTitleStyle.Render("Commands") + "\n")
	b.WriteString(HelpDimStyle.Render("Type / then search, Enter to select, Esc to cancel") + "\n\n")

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
	indicator := "  "
	if isSelected {
		indicator = IndicatorSelected + " "
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

// View renders the command palette centered
func (m CommandPaletteModel) View(width, height int) string {
	if !m.ready || !m.showing {
		return ""
	}

	body := m.viewport.View()

	pct := m.viewport.ScrollPercent()
	var scrollHint string
	if width < 50 {
		scrollHint = HelpDimStyle.Render("Esc:cancel Enter:select")
		if pct < 1.0 {
			scrollHint = HelpDimStyle.Render("j/k:nav Enter:select Esc:cancel")
		}
	} else if width < 70 {
		scrollHint = HelpDimStyle.Render("j/k:nav Enter:select Tab:auto")
		if pct < 1.0 {
			scrollHint = HelpDimStyle.Render("j/k:nav Enter:select Tab:auto scroll")
		}
	} else {
		scrollHint = HelpDimStyle.Render("j/k: navigate  Enter: select  Tab: auto-complete")
		if pct < 1.0 {
			scrollHint = HelpDimStyle.Render("j/k: navigate  Enter: select  Tab: auto-complete  scroll")
		}
	}

	content := body + "\n" + scrollHint

	panel := lipgloss.NewStyle().
		Width(m.viewport.Width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1)

	rendered := panel.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, rendered)
}
