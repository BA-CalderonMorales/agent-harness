package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

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
			if !m.inSystemMessages && m.cursor < len(m.settings) {
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
			if !m.inSystemMessages && m.cursor < len(m.settings) {
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
