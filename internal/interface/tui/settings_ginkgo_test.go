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
					{Key: "theme", Label: "Theme", Value: "dark", Type: "string"},
					{Key: "auto_save", Label: "Auto Save", BoolValue: false, Type: "bool"},
					{Key: "font_size", Label: "Font Size", Value: "14", Type: "string"},
				})
			})

			It("should navigate within bounds", func() {
				for i := 0; i < 5; i++ {
					m, _ := settings.Update(tea.KeyMsg{Type: tea.KeyDown})
					settings = m.(SettingsModel)
				}
				Expect(settings.cursor).To(Equal(2))

				for i := 0; i < 5; i++ {
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
})
