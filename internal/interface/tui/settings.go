// Settings view for configuration management

package tui

import (
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
	Category    string // "Provider & Connection", "Model & Agent Behavior", "Workspace & Permissions", "System & Storage"
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

	// systemMessages holds the durable system log rendered at the bottom
	// of the settings page (provider errors, session notices, ...).
	systemMessages []string

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

// SetSystemMessages replaces the system-message log rendered at the bottom
// of the settings page.
func (m *SettingsModel) SetSystemMessages(messages []string) {
	m.systemMessages = append([]string(nil), messages...)
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
