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
	title       string
}

// SetTitle sets the header line (e.g. "Models - openrouter" after a
// provider switch).
func (m *ModelPickerModel) SetTitle(title string) {
	m.title = title
}

// NewModelPicker creates a new model picker instance
func NewModelPicker() ModelPickerModel {
	return ModelPickerModel{
		models:   make([]ModelItem, 0),
		filtered: make([]ModelItem, 0),
		cursor:   0,
	}
}

// modalSpec is the picker's frame description. View renders it and Open
// sizes the viewport from it, so the two can never disagree about how
// much room the body has — the old code sized the viewport from its own
// guesses and the panel from another, which is how a picker ends up
// overflowing its own frame.
func (m ModelPickerModel) modalSpec() modalSpec {
	title := m.title
	if title == "" {
		title = "Select Model"
	}
	hint := fmt.Sprintf("Showing %d of %d models", len(m.filtered), len(m.models))
	if m.searchQuery != "" {
		hint = fmt.Sprintf("Filter: %s  ·  showing %d of %d", m.searchQuery, len(m.filtered), len(m.models))
	}
	return modalSpec{
		title:          title,
		hint:           hint,
		footer:         "Type to filter  j/k: navigate  Enter: select  Esc: cancel",
		preferredWidth: modalMaxWidth,
	}
}

// Open initializes the model picker overlay
func (m *ModelPickerModel) Open(width, height int) {
	m.width = width
	m.height = height
	m.showing = true
	m.searchQuery = ""
	m.selected = nil

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
	// The frame owns the title and hint now, so the body starts at the
	// first model row; only the live filter line sits above them.
	headerLines := 0
	if m.searchQuery != "" {
		headerLines = 2
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
		m.filtered = make([]ModelItem, 0, len(m.models))
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
	m.fitViewport()
	m.viewport.SetContent(m.buildContent())
	m.ensureCursorVisible()
}

// fitViewport sizes the viewport to the frame's body budget, capped to
// the rows the current list needs so the panel hugs its content like the
// other overlays instead of reserving a screen of blank rows.
func (m *ModelPickerModel) fitViewport() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	spec := m.modalSpec()
	// Width first: row truncation depends on it.
	m.viewport.Width = modalInnerWidth(panelWidth(spec.preferredWidth, m.width))

	want := len(m.filtered)
	if want == 0 {
		want = 1
	}
	if m.searchQuery != "" {
		want += 2 // the filter line and its blank
	}
	if budget := modalBodyRows(m.width, m.height, spec); want > budget {
		want = budget
	}
	if want < 3 {
		want = 3
	}
	m.viewport.Height = want
}

// buildContent renders the scrollable body only: the frame owns the
// title, the count hint, and the key hints, so they cannot scroll away
// and cannot be duplicated.
func (m ModelPickerModel) buildContent() string {
	var b strings.Builder

	if m.searchQuery != "" {
		b.WriteString("Filter: " + InfoStyle.Render(m.searchQuery) + " " + HelpDimStyle.Render("(type to filter, Backspace to clear)") + "\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(HelpDimStyle.Render("No models match your filter."))
		return b.String()
	}

	for i, model := range m.filtered {
		b.WriteString(m.renderModelLine(model, i == m.cursor) + "\n")
	}

	return b.String()
}

func (m ModelPickerModel) renderModelLine(model ModelItem, isSelected bool) string {
	// Both markers are the same width, so selecting a row never shifts its
	// columns; the old selected marker was one cell wider and every
	// selected row sat a column right of its neighbours.
	indicator := IndicatorUnselected
	style := lipgloss.NewStyle()

	if isSelected {
		indicator = IndicatorSelected
		style = ListSelectedStyle
	}

	// A model with no provider recorded shows no tag at all: the old
	// default rendered an empty "[] " that read like a broken checkbox.
	providerLabel := ""
	switch model.Provider {
	case "":
	case "openai":
		providerLabel = "[OpenAI] "
	case "openrouter":
		providerLabel = "[OR] "
	case "anthropic":
		providerLabel = "[Anthropic] "
	case "fireworks":
		providerLabel = "[FW] "
	case "nvidia":
		providerLabel = "[NVIDIA] "
	case "omniroute":
		providerLabel = "[Omni] "
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

// View renders the model picker through the shared modal frame.
func (m ModelPickerModel) View(width, height int) string {
	if !m.ready || !m.showing {
		return ""
	}

	spec := m.modalSpec()
	spec.items = []modalItem{modalTextItem(m.viewport.View())}
	spec.cursor = -1
	return renderModal(width, height, spec)
}
