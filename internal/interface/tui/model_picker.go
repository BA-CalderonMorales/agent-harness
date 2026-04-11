package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Model picker — interactive selection of AI models
// ---------------------------------------------------------------------------

// ModelItem represents a selectable model
type ModelItem struct {
	ID          string
	Name        string
	Provider    string
	Description string
	ContextLen  int
	IsDefault   bool
}

// ModelPickerModel is the interactive model picker
type ModelPickerModel struct {
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	models      []ModelItem
	filtered    []ModelItem
	cursor      int
	searchQuery string
	selected    *ModelItem
	showing     bool
}

// NewModelPicker creates a new model picker instance
func NewModelPicker() ModelPickerModel {
	return ModelPickerModel{
		models:   make([]ModelItem, 0),
		filtered: make([]ModelItem, 0),
		cursor:   0,
	}
}

// Open initializes the model picker overlay
func (m *ModelPickerModel) Open(width, height int) {
	m.width = width
	m.height = height
	m.showing = true
	m.searchQuery = ""
	m.selected = nil

	panelW := 80
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

	m.cursor = 0
	m.applyFilter()
}

// Close hides the model picker
func (m *ModelPickerModel) Close() {
	m.showing = false
	m.searchQuery = ""
}

// IsShowing returns whether the picker is currently visible
func (m ModelPickerModel) IsShowing() bool {
	return m.showing
}

// SelectedModel returns the chosen model (nil if none selected)
func (m ModelPickerModel) SelectedModel() *ModelItem {
	return m.selected
}

// SetModels populates the picker with available models
func (m *ModelPickerModel) SetModels(models []ModelItem) {
	m.models = models
	m.applyFilter()
}

// Update handles key/mouse events for the picker.
// Returns (closed, cmd).
func (m *ModelPickerModel) Update(msg tea.Msg) (closed bool, cmd tea.Cmd) {
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
			}
		default:
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.searchQuery += strings.ToLower(msg.String())
				m.applyFilter()
			}
		}

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		return false, cmd
	}

	return false, cmd
}

func (m *ModelPickerModel) ensureCursorVisible() {
	headerLines := 3
	if m.searchQuery != "" {
		headerLines = 4
	}
	visualLine := headerLines + m.cursor

	if visualLine < m.viewport.YOffset {
		m.viewport.SetYOffset(visualLine)
	} else if visualLine >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(visualLine - m.viewport.Height + 1)
	}
}

func (m *ModelPickerModel) applyFilter() {
	if m.searchQuery == "" {
		m.filtered = make([]ModelItem, len(m.models))
		copy(m.filtered, m.models)
	} else {
		m.filtered = m.filtered[:0]
		query := strings.ToLower(m.searchQuery)
		for _, model := range m.models {
			if strings.Contains(strings.ToLower(model.Name), query) ||
				strings.Contains(strings.ToLower(model.ID), query) ||
				strings.Contains(strings.ToLower(model.Provider), query) {
				m.filtered = append(m.filtered, model)
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

func (m *ModelPickerModel) updateContent() {
	m.viewport.SetContent(m.buildContent())
	m.ensureCursorVisible()
}

func (m ModelPickerModel) buildContent() string {
	var b strings.Builder

	b.WriteString(HelpTitleStyle.Render("Select Model") + "\n\n")

	if m.searchQuery != "" {
		b.WriteString("Filter: " + InfoStyle.Render(m.searchQuery) + " " + HelpDimStyle.Render("(type to filter, Backspace to clear)") + "\n\n")
	} else {
		b.WriteString(HelpDimStyle.Render("Type to filter models  /  j/k: navigate  Enter: select  Esc: cancel") + "\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(HelpDimStyle.Render("No models match your filter."))
		return b.String()
	}

	for i, model := range m.filtered {
		b.WriteString(m.renderModelLine(model, i == m.cursor) + "\n")
	}

	b.WriteString("\n" + HelpDimStyle.Render(fmt.Sprintf("Showing %d of %d models", len(m.filtered), len(m.models))))

	return b.String()
}

func (m ModelPickerModel) renderModelLine(model ModelItem, isSelected bool) string {
	indicator := "  "
	style := lipgloss.NewStyle()

	if isSelected {
		indicator = IndicatorSelected + " "
		style = ListSelectedStyle
	}

	providerLabel := ""
	switch model.Provider {
	case "openai":
		providerLabel = "[OpenAI] "
	case "openrouter":
		providerLabel = "[OR] "
	case "anthropic":
		providerLabel = "[Anthropic] "
	default:
		providerLabel = "[" + model.Provider + "] "
	}

	name := model.Name
	if name == "" {
		name = model.ID
	}

	maxLen := m.viewport.Width - 20
	if maxLen < 20 {
		maxLen = 20
	}
	displayName := name
	if len(displayName) > maxLen {
		displayName = displayName[:maxLen-3] + "..."
	}

	defaultTag := ""
	if model.IsDefault {
		defaultTag = " " + SuccessStyle.Render("[default]")
	}

	line := indicator + providerLabel + displayName + defaultTag

	if isSelected {
		return style.Render(line)
	}
	return line
}

// View renders the model picker panel centered in the terminal
func (m ModelPickerModel) View(width, height int) string {
	if !m.ready || !m.showing {
		return ""
	}

	body := m.viewport.View()

	panel := lipgloss.NewStyle().
		Width(m.viewport.Width).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1)

	rendered := panel.Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, rendered)
}
