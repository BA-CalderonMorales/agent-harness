package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// cycleChoice moves a choice setting to the next (dir=1) or previous
// (dir=-1) option. The cycle walks distinct values: duplicate options
// collapse to their first occurrence, so a repeated entry can never
// trap the rotation between its indices. A stored value missing from
// the options list snaps deterministically: forward lands on the first
// option, backward on the last. One helper, one fallback — the old
// three-way split (Enter defaulted to -1, arrows to 0) made Enter and
// the arrows disagree about where an unknown value lands.
func cycleChoice(s *Setting, dir int) {
	n := len(s.Options)
	if n == 0 {
		return
	}
	seen := make(map[string]bool, n)
	options := make([]string, 0, n)
	for _, o := range s.Options {
		if !seen[o] {
			seen[o] = true
			options = append(options, o)
		}
	}
	n = len(options)

	idx := -1
	for i, o := range options {
		if o == s.Value {
			idx = i
			break
		}
	}
	if idx < 0 {
		if dir >= 0 {
			s.Value = options[0]
		} else {
			s.Value = options[n-1]
		}
		return
	}
	idx = (idx + dir + n) % n
	s.Value = options[idx]
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
		m.sysViewport.Width = msg.Width

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		if m.editing {
			return m.handleEditMode(msg)
		}

		switch msg.String() {
		case "up", "k":
			if m.inSystemMessages {
				// Scroll the System Messages region; exiting back into the
				// settings list happens at its top edge.
				m.sysViewport.ScrollUp(1)
				if m.sysViewport.AtTop() {
					m.inSystemMessages = false
				}
			} else if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.inSystemMessages {
				m.sysViewport.ScrollDown(1)
			} else if m.cursor < len(m.settings)-1 {
				m.cursor++
			} else if len(m.systemMessages) > 0 {
				// Past the last setting: enter the System Messages region.
				m.inSystemMessages = true
			}

		case "enter", " ":
			if m.inSystemMessages {
				return m, nil
			}
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
					cycleChoice(s, 1)
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				} else {
					m.startEditing()
				}
			}

		case "left", "h":
			if !m.inSystemMessages && m.cursor < len(m.settings) {
				s := &m.settings[m.cursor]
				if s.Type == "choice" && len(s.Options) > 0 {
					cycleChoice(s, -1)
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				}
			}

		case "right", "l":
			if !m.inSystemMessages && m.cursor < len(m.settings) {
				s := &m.settings[m.cursor]
				if s.Type == "choice" && len(s.Options) > 0 {
					cycleChoice(s, 1)
					if m.delegate != nil {
						m.delegate.OnSettingChange(s.Key, s.Value)
					}
				}
			}

		case "r":
			if !m.inSystemMessages && m.delegate != nil {
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
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "must be a number (0.0-2.0)"
		}
		if f < 0 || f > 2 {
			return "must be between 0.0 and 2.0"
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
