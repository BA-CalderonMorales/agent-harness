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
	BoolValue   bool // For boolean settings
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
	editErr  string
	viewport viewport.Model

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

// UpdateSettingValue updates a single setting value by key.
func (m *SettingsModel) UpdateSettingValue(key, value string) {
	for i := range m.settings {
		if m.settings[i].Key == key {
			m.settings[i].Value = value
			return
		}
	}
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
				s := &m.settings[m.cursor]
				if s.Type == "bool" {
					s.BoolValue = !s.BoolValue
					if m.delegate != nil {
						value := "false"
						if s.BoolValue {
							value = "true"
						}
						m.delegate.OnSettingChange(s.Key, value)
					}
				} else if s.Type == "choice" && len(s.Options) > 0 {
					idx := -1
					for i, o := range s.Options {
						if o == s.Value {
							idx = i
							break
						}
					}
					idx = (idx + 1) % len(s.Options)
					s.Value = s.Options[idx]
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				} else {
					m.startEditing()
				}
			}

		case "left", "h":
			if m.cursor < len(m.settings) {
				s := &m.settings[m.cursor]
				if s.Type == "choice" && len(s.Options) > 0 {
					idx := 0
					for i, o := range s.Options {
						if o == s.Value {
							idx = i
							break
						}
					}
					idx = (idx - 1 + len(s.Options)) % len(s.Options)
					s.Value = s.Options[idx]
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				}
			}

		case "right", "l":
			if m.cursor < len(m.settings) {
				s := &m.settings[m.cursor]
				if s.Type == "choice" && len(s.Options) > 0 {
					idx := 0
					for i, o := range s.Options {
						if o == s.Value {
							idx = i
							break
						}
					}
					idx = (idx + 1) % len(s.Options)
					s.Value = s.Options[idx]
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				}
			}

		case "r":
			if m.delegate != nil {
				m.delegate.OnSettingReload()
			}
		}
	}

	return m, nil
}

func (m *SettingsModel) handleEditMode(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	m.editErr = ""
	switch msg.Type {
	case tea.KeyEnter:
		if m.cursor < len(m.settings) && m.delegate != nil {
			s := &m.settings[m.cursor]
			if errMsg := m.validateSetting(s, m.editBuf); errMsg != "" {
				m.editErr = errMsg
				return *m, nil
			}
			m.delegate.OnSettingChange(s.Key, m.editBuf)
			s.Value = m.editBuf
		}
		m.editing = false
		m.editBuf = ""

	case tea.KeyEsc:
		m.editing = false
		m.editBuf = ""
		m.editErr = ""

	case tea.KeyBackspace:
		if len(m.editBuf) > 0 {
			runes := []rune(m.editBuf)
			m.editBuf = string(runes[:len(runes)-1])
		}

	case tea.KeyRunes:
		m.editBuf += string(msg.Runes)
	}

	return *m, nil
}

func (m *SettingsModel) validateSetting(s *Setting, value string) string {
	switch s.Key {
	case "context_length", "max_tokens":
		if value == "" {
			return "value required"
		}
		n := 0
		for _, c := range value {
			if c < '0' || c > '9' {
				return "must be a positive integer"
			}
			n = n*10 + int(c-'0')
		}
		if n <= 0 {
			return "must be a positive integer"
		}
	case "temperature":
		if value == "" {
			return "value required"
		}
	case "persona":
		valid := []string{"developer", "designer", "pm", "scientist", "explorer"}
		found := false
		for _, v := range valid {
			if v == value {
				found = true
				break
			}
		}
		if !found {
			return "must be one of: developer, designer, pm, scientist, explorer"
		}
	case "permissions":
		valid := []string{"read-only", "workspace-write", "danger-full-access"}
		found := false
		for _, v := range valid {
			if v == value {
				found = true
				break
			}
		}
		if !found {
			return "must be one of: read-only, workspace-write, danger-full-access"
		}
	}
	return ""
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
		{Key: "Enter/Space", Desc: "Edit / toggle"},
		{Key: "←/→", Desc: "Cycle choice"},
		{Key: "r", Desc: "Reload"},
	}
	if m.editing {
		footerActions = []ActionHint{
			{Key: "Enter", Desc: "Save"},
			{Key: "Esc", Desc: "Cancel"},
		}
		if m.editErr != "" {
			footerActions = append(footerActions, ActionHint{Key: "!", Desc: m.editErr})
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

	// For boolean settings, show checkbox
	if setting.Type == "bool" {
		checkbox := "[ ]"
		if setting.BoolValue {
			checkbox = "[x]"
		}
		if selected {
			checkbox = PromptStyle.Render(checkbox)
		}
		label := style.Render(prefix + checkbox + " " + setting.Label)
		b.WriteString(label)

		// Description for boolean
		if selected {
			b.WriteString("\n")
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("    %s", setting.Description)))
		}
		return b.String()
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
		if setting.Type == "choice" && len(setting.Options) > 0 {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  [%s]", strings.Join(setting.Options, "/"))))
		}
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

// CapturesAllKeys returns whether this view should receive all keys
// before global shortcuts are applied.
func (m SettingsModel) CapturesAllKeys() bool {
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
