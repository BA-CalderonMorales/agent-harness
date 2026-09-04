package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testSettingsDelegate struct {
	changedKey   string
	changedValue string
	reloaded     bool
	reset        bool
}

func (d *testSettingsDelegate) OnSettingChange(key, value string) {
	d.changedKey = key
	d.changedValue = value
}
func (d *testSettingsDelegate) OnSettingReload() { d.reloaded = true }
func (d *testSettingsDelegate) OnSettingReset()  { d.reset = true }

var _ = Describe("SettingsModel", func() {
	var settings SettingsModel
	var delegate *testSettingsDelegate

	BeforeEach(func() {
		settings = NewSettingsModel()
		delegate = &testSettingsDelegate{}
		settings.SetDelegate(delegate)
		m, _ := settings.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		settings = m.(SettingsModel)
		settings.Focus()
	})

	Describe("Initialization and Empty State", func() {
		Context("Given no settings", func() {
			It("should render empty state", func() {
				Expect(settings.View()).To(ContainSubstring("No Settings"))
			})

			It("should handle navigation keys without panicking", func() {
				Expect(func() {
					settings.Update(tea.KeyMsg{Type: tea.KeyUp})
					settings.Update(tea.KeyMsg{Type: tea.KeyDown})
					settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}).NotTo(Panic())
			})
		})
	})

	Describe("Interaction and Editing", func() {
		Context("Given a list of settings", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string", Description: "UI color scheme"},
					{Key: "auto_save", Label: "Auto Save", BoolValue: false, Type: "bool", Description: "Save automatically"},
					{Key: "font_size", Label: "Font Size", Value: "14", Type: "string"},
					{Key: "provider", Label: "Provider", Value: "openrouter", Type: "choice", Options: []string{"openrouter", "openai", "anthropic"}},
				})
			})

			It("should navigate within bounds", func() {
				for i := 0; i < 10; i++ {
					m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyDown})
					settings = m.(SettingsModel)
				}
				Expect(settings.cursor).To(Equal(3))

				for i := 0; i < 10; i++ {
					m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyUp})
					settings = m.(SettingsModel)
				}
				Expect(settings.cursor).To(Equal(0))
			})

			It("should toggle boolean settings immediately", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyDown}) // Go to auto_save
				settings = m.(SettingsModel)

				By("pressing enter to toggle")
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)

				Expect(settings.settings[1].BoolValue).To(BeTrue())
				Expect(delegate.changedKey).To(Equal("auto_save"))
				Expect(delegate.changedValue).To(Equal("true"))
				Expect(settings.editing).To(BeFalse()) // Should not enter edit mode
			})

			It("should toggle boolean settings back off", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyDown}) // Go to auto_save
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Toggle on
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Toggle off
				settings = m.(SettingsModel)

				Expect(settings.settings[1].BoolValue).To(BeFalse())
				Expect(delegate.changedValue).To(Equal("false"))
			})

			It("should edit string settings", func() {
				By("pressing enter on a string setting")
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				Expect(settings.editing).To(BeTrue())
				Expect(settings.editBuf).To(Equal("dark"))

				By("typing new characters")
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
				settings = m.(SettingsModel)
				Expect(settings.editBuf).To(Equal("dark1"))

				By("backspacing characters")
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				settings = m.(SettingsModel)
				Expect(settings.editBuf).To(Equal("dar"))

				By("backspacing on empty buffer should not panic")
				for i := 0; i < 10; i++ {
					m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
					settings = m.(SettingsModel)
				}
				Expect(settings.editBuf).To(Equal(""))

				By("pressing enter to save")
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("light")})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)

				Expect(settings.editing).To(BeFalse())
				Expect(settings.settings[0].Value).To(Equal("light"))
				Expect(delegate.changedKey).To(Equal("theme"))
				Expect(delegate.changedValue).To(Equal("light"))
			})

			It("should handle escape to cancel editing", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("changed")})
				settings = m.(SettingsModel)

				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEsc})
				settings = m.(SettingsModel)

				Expect(settings.editing).To(BeFalse())
				Expect(settings.settings[0].Value).To(Equal("dark")) // original value
				Expect(delegate.changedValue).To(Equal(""))          // delegate not called
			})

			It("should handle r key for reload", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
				settings = m.(SettingsModel)
				Expect(delegate.reloaded).To(BeTrue())
			})
		})
	})

	Describe("Logs Tab", func() {
		Context("Given system messages exist", func() {
			var logs LogsModel

			BeforeEach(func() {
				logs = NewLogsModel()
				logs, _ = logs.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				logs.SetMessages([]string{
					"Started new chat abc123",
					"Provider unavailable: connection refused",
					"Deleted session def456",
					"Provider ready: 12 models available",
				})
			})

			It("should render the log under its own header", func() {
				view := logs.View()
				Expect(view).To(ContainSubstring("Logs"))
				Expect(view).To(ContainSubstring("Provider unavailable: connection refused"))
			})

			It("should scroll the log with j/k when focused", func() {
				logs.Focus()
				offsetBefore := logs.viewport.YOffset
				m, _ := logs.Update(tea.KeyMsg{Type: tea.KeyUp})
				logs = m
				Expect(logs.viewport.YOffset).To(BeNumerically(">=", offsetBefore))
			})

			It("should render the empty state before any messages", func() {
				empty := NewLogsModel()
				empty, _ = empty.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				Expect(empty.View()).To(ContainSubstring("no system messages"))
			})
		})
	})

	Describe("Focus and Blur", func() {
		Context("Given the view is blurred while editing", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string"},
				})
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("partial")})
				settings = m.(SettingsModel)
			})

			It("should cancel editing on blur", func() {
				Expect(settings.editing).To(BeTrue())
				settings.Blur()
				Expect(settings.editing).To(BeFalse())
				Expect(settings.editBuf).To(Equal(""))
			})

			It("should not consume Tab or Esc when not editing", func() {
				settings.Blur()
				Expect(settings.ConsumesTab()).To(BeFalse())
				Expect(settings.ConsumesEsc()).To(BeFalse())
			})

			It("should consume Tab and Esc when editing", func() {
				Expect(settings.ConsumesTab()).To(BeTrue())
				Expect(settings.ConsumesEsc()).To(BeTrue())
			})
		})
	})

	Describe("Window Size", func() {
		Context("Given the window is resized", func() {
			It("should update viewport dimensions", func() {
				m, _ := settings.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				settings = m.(SettingsModel)
				Expect(settings.width).To(Equal(100))
				Expect(settings.height).To(Equal(50))
			})

			It("should ensure minimum viewport height", func() {
				m, _ := settings.Update(tea.WindowSizeMsg{Width: 80, Height: 4})
				settings = m.(SettingsModel)
				Expect(settings.viewport.Height).To(BeNumerically(">=", 5))
			})
		})
	})

	Describe("Scroll Helpers", func() {
		Context("Given a list of settings", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "a", Label: "A", Value: "1", Type: "string"},
					{Key: "b", Label: "B", Value: "2", Type: "string"},
					{Key: "c", Label: "C", Value: "3", Type: "string"},
					{Key: "d", Label: "D", Value: "4", Type: "string"},
					{Key: "e", Label: "E", Value: "5", Type: "string"},
				})
			})

			It("should scroll down", func() {
				settings.Scroll(2)
				Expect(settings.cursor).To(Equal(2))
			})

			It("should scroll up", func() {
				settings.cursor = 4
				settings.Scroll(-2)
				Expect(settings.cursor).To(Equal(2))
			})

			It("should clamp scroll to bounds", func() {
				settings.Scroll(100)
				Expect(settings.cursor).To(Equal(4))
				settings.Scroll(-100)
				Expect(settings.cursor).To(Equal(0))
			})

			It("should goto top", func() {
				settings.cursor = 3
				settings.GotoTop()
				Expect(settings.cursor).To(Equal(0))
			})

			It("should goto bottom", func() {
				settings.GotoBottom()
				Expect(settings.cursor).To(Equal(4))
			})
		})
	})

	Describe("UpdateSettingValue", func() {
		Context("Given settings exist", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "model", Value: "old-model"},
					{Key: "provider", Value: "openrouter"},
				})
			})

			It("should update value by key", func() {
				settings.UpdateSettingValue("model", "new-model")
				Expect(settings.settings[0].Value).To(Equal("new-model"))
			})

			It("should not affect other settings", func() {
				settings.UpdateSettingValue("model", "new-model")
				Expect(settings.settings[1].Value).To(Equal("openrouter"))
			})

			It("should do nothing for unknown key", func() {
				settings.UpdateSettingValue("unknown", "value")
				Expect(settings.settings[0].Value).To(Equal("old-model"))
				Expect(settings.settings[1].Value).To(Equal("openrouter"))
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given various setting types", func() {
			It("should render string setting with value", func() {
				settings.SetSettings([]Setting{
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("Theme"))
				Expect(view).To(ContainSubstring("dark"))
			})

			It("should render boolean setting with checkbox", func() {
				settings.SetSettings([]Setting{
					{Key: "auto_save", Label: "Auto Save", BoolValue: true, Type: "bool"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("Auto Save"))
				Expect(view).To(ContainSubstring("[x]"))
			})

			It("should render unchecked boolean setting", func() {
				settings.SetSettings([]Setting{
					{Key: "auto_save", Label: "Auto Save", BoolValue: false, Type: "bool"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("[ ]"))
			})

			It("should render '(empty)' for blank string value", func() {
				settings.SetSettings([]Setting{
					{Key: "api_key", Label: "API Key", Value: "", Type: "string"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("(empty)"))
			})

			It("should render editing indicator when in edit mode", func() {
				settings.SetSettings([]Setting{
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string"},
				})
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				view := settings.View()
				Expect(view).To(ContainSubstring("→"))
			})

			It("should render description for selected setting", func() {
				settings.SetSettings([]Setting{
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string", Description: "Choose your color scheme"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("Choose your color scheme"))
			})

			It("should not render the settings count in the header", func() {
				settings.SetSettings([]Setting{
					{Key: "a", Label: "A", Type: "string"},
					{Key: "b", Label: "B", Type: "string"},
				})
				view := settings.View()
				Expect(view).ToNot(ContainSubstring("(2)"))
			})
		})
	})

	Describe("Choice Settings", func() {
		Context("Given a choice-type setting", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "persona", Label: "Persona", Value: "developer", Type: "choice", Options: []string{"developer", "designer", "pm"}},
				})
			})

			It("should cycle forward with Enter/Space", func() {
				settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.changedValue).To(Equal("designer"))

				settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.changedValue).To(Equal("pm"))

				settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.changedValue).To(Equal("developer"))
			})

			It("should cycle backward with left/h", func() {
				settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
				Expect(delegate.changedValue).To(Equal("pm"))
			})

			It("should cycle forward with right/l", func() {
				settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
				Expect(delegate.changedValue).To(Equal("designer"))
			})
		})
	})

	Describe("Edit Validation", func() {
		Context("Given a numeric setting", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "context_length", Label: "Context Length", Value: "4096", Type: "number"},
				})
			})

			It("should reject non-numeric input", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				for i := 0; i < 4; i++ {
					m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
					settings = m.(SettingsModel)
				}
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				Expect(settings.editErr).To(ContainSubstring("positive integer"))
				Expect(delegate.changedKey).To(Equal(""))
			})

			It("should reject empty input", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				for i := 0; i < 10; i++ {
					m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
					settings = m.(SettingsModel)
				}
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				Expect(settings.editErr).NotTo(BeEmpty())
			})

			It("should accept valid numeric input", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				for i := 0; i < 4; i++ {
					m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
					settings = m.(SettingsModel)
				}
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8192")})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				Expect(delegate.changedValue).To(Equal("8192"))
				Expect(settings.editErr).To(BeEmpty())
			})
		})
	})

	Describe("Rune-Safe Backspace", func() {
		Context("Given a setting with multi-byte characters in the edit buffer", func() {
			BeforeEach(func() {
				settings.SetSettings([]Setting{
					{Key: "model", Label: "Model", Value: "", Type: "string"},
				})
			})

			It("should remove one rune at a time", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
				settings = m.(SettingsModel)
				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("café")})
				settings = m.(SettingsModel)
				Expect(settings.editBuf).To(Equal("café"))

				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				settings = m.(SettingsModel)
				Expect(settings.editBuf).To(Equal("caf"))

				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				settings = m.(SettingsModel)
				Expect(settings.editBuf).To(Equal("ca"))
			})
		})
	})

	Describe("CapturesAllKeys", func() {
		It("should return true when editing", func() {
			settings.SetSettings([]Setting{
				{Key: "theme", Label: "Theme", Value: "dark", Type: "string"},
			})
			Expect(settings.CapturesAllKeys()).To(BeFalse())
			m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyEnter})
			settings = m.(SettingsModel)
			Expect(settings.CapturesAllKeys()).To(BeTrue())
		})
	})
})
