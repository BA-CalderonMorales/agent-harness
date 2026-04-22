package tui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tea "github.com/charmbracelet/bubbletea"
)

var _ = Describe("HomeModel", func() {
	var home HomeModel

	BeforeEach(func() {
		home = NewHomeModel()
	})

	Describe("Initialization and Loading", func() {
		Context("Given the model is newly created with no width", func() {
			It("should show a loading message", func() {
				Expect(home.View()).To(ContainSubstring("Loading dashboard..."))
			})
		})

		Context("Given width and height are set but no project info", func() {
			BeforeEach(func() {
				m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
				home = *m.(*HomeModel)
			})

			It("should show the setup required banner if no model is set", func() {
				home.SetStatus("", "workspace-write", "developer", 0)
				Expect(home.View()).To(ContainSubstring("[!] Setup Required"))
			})

			It("should show 'not a repository' if project info is empty", func() {
				Expect(home.View()).To(ContainSubstring("not a repository"))
			})
		})
	})

	Describe("Interaction Edge Cases", func() {
		Context("Given actions are not yet rebuilt", func() {
			It("should not panic when pressing Enter", func() {
				// We don't call Init or rebuildActions yet
				home.Focus()
				Expect(func() {
					home.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}).NotTo(Panic())
			})
		})

		Context("Given rapid navigation", func() {
			BeforeEach(func() {
				m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
				home = *m.(*HomeModel)
				home.Init() // rebuilds actions
				home.Focus()
			})

			It("should clamp the cursor to available actions", func() {
				By("pressing up at the top")
				home.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(home.actionCursor).To(Equal(0))

				By("pressing down many times")
				for i := 0; i < 20; i++ {
					home.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
				Expect(home.actionCursor).To(Equal(len(home.actions) - 1))
			})
		})
	})
})
