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

	delegate SettingsDelegate
}

// NewSettingsModel creates a new settings model.
func NewSettingsModel() SettingsModel {
	return SettingsModel{
		settings: make([]Setting, 0),
		cursor:   0,
		viewport: newViewport(80, 20),
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

// Scroll moves the selection. Single steps wrap; page jumps stop at the ends.
func (m *SettingsModel) Scroll(lines int) {
	n := len(m.settings)
	if n == 0 {
		return
	}
	if lines == -1 && m.cursor == 0 {
		m.cursor = n - 1
		return
	}
	if lines == 1 && m.cursor == n-1 {
		m.cursor = 0
		return
	}
	m.cursor = max(0, min(n-1, m.cursor+lines))
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
