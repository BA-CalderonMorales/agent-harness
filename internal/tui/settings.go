// Settings view for configuration management

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// SettingsDelegate handles settings actions
// ---------------------------------------------------------------------------
type SettingsDelegate interface {
	OnSettingChange(key, value string)
	OnSettingReset()
	OnSettingReload()
}

// ---------------------------------------------------------------------------
// Setting represents a configuration setting
// ---------------------------------------------------------------------------
type Setting struct {
	Key         string
	Label       string
	Value       string
	Description string
	Type        string // "string", "bool", "number", "choice"
	Options     []string
}

// ---------------------------------------------------------------------------
// SettingsModel is the settings view model
// ---------------------------------------------------------------------------
type SettingsModel struct {
	width    int
	height   int
	settings []Setting
	cursor   int
	focused  bool
	editing  bool
	editBuf  string
	viewport viewport.Model

	// Delegate
	delegate SettingsDelegate
}

// NewSettingsModel creates a new settings model.
func NewSettingsModel() SettingsModel {
	return SettingsModel{
		settings: make([]Setting, 0),
		cursor:   0,
		viewport: viewport.New(80, 20),
	}
}

// SetDelegate sets the settings delegate.
func (m *SettingsModel) SetDelegate(delegate SettingsDelegate) {
	m.delegate = delegate
}

// SetSettings updates the settings list.
func (m *SettingsModel) SetSettings(settings []Setting) {
	m.settings = settings
}

// Init initializes the settings model.
func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for header (3 lines) and footer (2 lines)
		vpHeight := msg.Height - 5
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		if m.editing {
			return m.handleEditMode(msg)
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.settings)-1 {
				m.cursor++
			}

		case "enter", " ":
			if m.cursor < len(m.settings) {
				m.startEditing()
			}

		case "r":
			if m.delegate != nil {
				m.delegate.OnSettingReload()
			}

		case "R":
			if m.delegate != nil {
				m.delegate.OnSettingReset()
			}
		}
	}

	return m, nil
}

func (m *SettingsModel) handleEditMode(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Save value
		if m.cursor < len(m.settings) && m.delegate != nil {
			m.delegate.OnSettingChange(m.settings[m.cursor].Key, m.editBuf)
			m.settings[m.cursor].Value = m.editBuf
		}
		m.editing = false
		m.editBuf = ""

	case tea.KeyEsc:
		// Cancel editing
		m.editing = false
		m.editBuf = ""

	case tea.KeyBackspace:
		if len(m.editBuf) > 0 {
			m.editBuf = m.editBuf[:len(m.editBuf)-1]
		}

	case tea.KeyRunes:
		m.editBuf += string(msg.Runes)
	}

	return *m, nil
}

func (m *SettingsModel) startEditing() {
	if m.cursor < len(m.settings) {
		m.editing = true
		m.editBuf = m.settings[m.cursor].Value
	}
}

// View renders the settings.
func (m SettingsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if len(m.settings) == 0 {
		return RenderEmptyState(ViewPort{Width: m.width, Height: m.height}, EmptyState{
			Title:       "No Settings",
			Description: "Settings will appear here when available.",
			Actions: []ActionHint{
				{Key: "r", Desc: "Reload settings"},
			},
		})
	}

	var b strings.Builder

	// Header (always visible, not in viewport)
	b.WriteString(RenderHeader(HeaderConfig{
		Title:    "Settings",
		Subtitle: "Configuration options",
		Count:    len(m.settings),
	}))

	// Build settings list content for viewport
	var settingsContent strings.Builder
	for i, setting := range m.settings {
		settingsContent.WriteString(m.renderSetting(setting, i == m.cursor))
		settingsContent.WriteString("\n")
	}

	// Update viewport content
	m.viewport.SetContent(settingsContent.String())

	// Render viewport (scrollable settings list)
	b.WriteString(m.viewport.View())

	// Footer (always visible, not in viewport)
	footerActions := []ActionHint{
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Edit"},
		{Key: "r", Desc: "Reload"},
		{Key: "R", Desc: "Reset all"},
	}
	if m.editing {
		footerActions = []ActionHint{
			{Key: "Enter", Desc: "Save"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	b.WriteString(RenderFooter(footerActions))

	return b.String()
}

func (m SettingsModel) renderSetting(setting Setting, selected bool) string {
	var b strings.Builder

	prefix := IndicatorUnselected
	style := ListItemStyle
	valueStyle := DataValue

	if selected {
		prefix = IndicatorSelected
		style = ListSelectedStyle
		valueStyle = ListSelectedStyle
	}

	// Label and value
	label := style.Render(prefix + setting.Label)
	b.WriteString(label)

	// Show edit indicator if editing
	if selected && m.editing {
		b.WriteString("\n")
		editLine := fmt.Sprintf("    %s %s",
			HelpDimStyle.Render("→"),
			PromptStyle.Render(m.editBuf+"█"))
		b.WriteString(editLine)
	} else {
		// Show current value
		value := setting.Value
		if value == "" {
			value = "(empty)"
		}
		b.WriteString(" ")
		b.WriteString(valueStyle.Render(value))
	}

	// Description
	if selected && !m.editing {
		b.WriteString("\n")
		b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", setting.Description)))
	}

	return b.String()
}

// Focus focuses the settings view.
func (m *SettingsModel) Focus() {
	m.focused = true
}

// Blur blurs the settings view.
func (m *SettingsModel) Blur() {
	m.focused = false
	m.editing = false
	m.editBuf = ""
}

// ConsumesTab returns whether this view consumes Tab key.
func (m SettingsModel) ConsumesTab() bool {
	return m.editing
}

// ConsumesEsc returns whether this view consumes Esc key.
func (m SettingsModel) ConsumesEsc() bool {
	return m.editing
}

// Scroll scrolls the list and updates viewport.
// CRITICAL FIX: Also scrolls the viewport to ensure all settings are visible
func (m *SettingsModel) Scroll(lines int) {
	oldCursor := m.cursor
	if lines > 0 {
		for i := 0; i < lines && m.cursor < len(m.settings)-1; i++ {
			m.cursor++
		}
	} else {
		for i := 0; i < -lines && m.cursor > 0; i++ {
			m.cursor--
		}
	}
	// Scroll viewport to keep cursor visible
	if m.cursor != oldCursor {
		m.syncViewportToCursor()
	}
}

// syncViewportToCursor ensures the cursor is visible in the viewport
func (m *SettingsModel) syncViewportToCursor() {
	// Approximate line height per setting (2 lines: label/value + description)
	lineHeight := 2
	cursorLine := m.cursor * lineHeight

	// If cursor is above viewport, scroll up
	if cursorLine < m.viewport.YOffset {
		m.viewport.SetYOffset(cursorLine)
	}

	// If cursor is below viewport, scroll down
	viewportBottom := m.viewport.YOffset + m.viewport.Height
	cursorBottom := cursorLine + lineHeight
	if cursorBottom > viewportBottom {
		newOffset := cursorBottom - m.viewport.Height
		if newOffset < 0 {
			newOffset = 0
		}
		m.viewport.SetYOffset(newOffset)
	}
}

// GotoTop scrolls to top.
func (m *SettingsModel) GotoTop() {
	m.cursor = 0
}

// GotoBottom scrolls to bottom.
func (m *SettingsModel) GotoBottom() {
	if len(m.settings) > 0 {
		m.cursor = len(m.settings) - 1
	}
}
