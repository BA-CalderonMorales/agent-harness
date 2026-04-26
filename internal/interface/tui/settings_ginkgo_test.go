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

			It("should handle global R and r keys", func() {
				m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
				settings = m.(SettingsModel)
				Expect(delegate.reloaded).To(BeTrue())

				m, _ = settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
				settings = m.(SettingsModel)
				Expect(delegate.reset).To(BeTrue())
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

			It("should render settings count in header", func() {
				settings.SetSettings([]Setting{
					{Key: "a", Label: "A", Type: "string"},
					{Key: "b", Label: "B", Type: "string"},
				})
				view := settings.View()
				Expect(view).To(ContainSubstring("(2)"))
			})
		})
	})
})
