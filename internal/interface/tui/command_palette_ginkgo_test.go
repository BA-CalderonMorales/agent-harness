package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = Describe("CommandPaletteModel", func() {
	var palette CommandPaletteModel

	BeforeEach(func() {
		palette = NewCommandPalette()
	})

	Describe("Initialization", func() {
		Context("Given a newly created palette", func() {
			It("should not be showing", func() {
				Expect(palette.IsShowing()).To(BeFalse())
			})

			It("should have all commands preloaded", func() {
				Expect(palette.commands).ToNot(BeEmpty())
				Expect(len(palette.commands)).To(BeNumerically(">", 10))
			})

			It("should have no selected command", func() {
				Expect(palette.SelectedCommand()).To(BeNil())
			})
		})
	})

	Describe("Open and Close", func() {
		Context("Given the palette is opened", func() {
			BeforeEach(func() {
				palette.Open(80, 24)
			})

			It("should be showing", func() {
				Expect(palette.IsShowing()).To(BeTrue())
			})

			It("should reset search and cursor", func() {
				Expect(palette.searchQuery).To(Equal(""))
				Expect(palette.cursor).To(Equal(0))
			})

			It("should set width and height", func() {
				Expect(palette.width).To(Equal(80))
				Expect(palette.height).To(Equal(24))
			})

			It("should initialize the viewport", func() {
				Expect(palette.ready).To(BeTrue())
				Expect(palette.viewport.Width).To(BeNumerically(">", 0))
				Expect(palette.viewport.Height).To(BeNumerically(">", 0))
			})
		})

		Context("Given the palette is closed", func() {
			BeforeEach(func() {
				palette.Open(80, 24)
				palette.Close()
			})

			It("should not be showing", func() {
				Expect(palette.IsShowing()).To(BeFalse())
			})

			It("should clear the search query", func() {
				Expect(palette.searchQuery).To(Equal(""))
			})
		})
	})

	Describe("Navigation", func() {
		BeforeEach(func() {
			palette.Open(80, 24)
		})

		Context("Given a list of commands", func() {
			It("should move cursor down with down arrow", func() {
				By("pressing down")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(1))
			})

			It("should move cursor down with 'j'", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(1))
			})

			It("should move cursor up with up arrow", func() {
				By("moving down first")
				palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(palette.cursor).To(Equal(2))

				By("pressing up")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(1))
			})

			It("should move cursor up with 'k'", func() {
				palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(palette.cursor).To(Equal(1))

				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(0))
			})

			It("should clamp cursor at top", func() {
				By("pressing up at the top")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(0))
			})

			It("should clamp cursor at bottom", func() {
				By("pressing down past the end")
				for i := 0; i < len(palette.commands)+5; i++ {
					palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
				Expect(palette.cursor).To(Equal(len(palette.commands) - 1))
			})

			It("should jump 10 items with page down", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(10))
			})

			It("should jump 10 items with page up", func() {
				By("moving to end first")
				palette.Update(tea.KeyMsg{Type: tea.KeyEnd})
				lastIdx := palette.cursor

				By("pressing page up")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyPgUp})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(maxInt(0, lastIdx-10)))
			})

			It("should go to first item with home", func() {
				palette.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				Expect(palette.cursor).To(Equal(10))

				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyHome})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(0))
			})

			It("should go to last item with end", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyEnd})
				Expect(closed).To(BeFalse())
				Expect(palette.cursor).To(Equal(len(palette.commands) - 1))
			})
		})
	})

	Describe("Filtering", func() {
		BeforeEach(func() {
			palette.Open(80, 24)
		})

		Context("Given typing search characters", func() {
			It("should filter commands by name", func() {
				By("typing 'clear'")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				Expect(closed).To(BeFalse())
				closed, _ = palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
				Expect(closed).To(BeFalse())
				closed, _ = palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
				Expect(closed).To(BeFalse())
				closed, _ = palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
				Expect(closed).To(BeFalse())
				closed, _ = palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
				Expect(closed).To(BeFalse())

				By("verifying filter results contain /clear")
				Expect(palette.filtered).ToNot(BeEmpty())
				found := false
				for _, cmd := range palette.filtered {
					if cmd.Command == "/clear" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should filter commands by description", func() {
				By("typing 'git'")
				for _, r := range "git" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				By("verifying /diff is found via 'git diff' description")
				found := false
				for _, cmd := range palette.filtered {
					if cmd.Command == "/diff" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should show empty results for non-matching query", func() {
				By("typing 'zzzzzz'")
				for _, r := range "zzzzzz" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(palette.filtered).To(BeEmpty())
			})

			It("should reset filter when backspacing to empty", func() {
				By("typing 'cl'")
				palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
				filteredLen := len(palette.filtered)
				Expect(filteredLen).To(BeNumerically("<", len(palette.commands)))

				By("backspacing both characters")
				palette.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				palette.Update(tea.KeyMsg{Type: tea.KeyBackspace})

				Expect(palette.filtered).To(HaveLen(len(palette.commands)))
			})
		})
	})

	Describe("Selection", func() {
		BeforeEach(func() {
			palette.Open(80, 24)
		})

		Context("Given a command is selected with Enter", func() {
			It("should close the palette and set selected command", func() {
				By("pressing enter on the first command")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(closed).To(BeTrue())
				Expect(palette.IsShowing()).To(BeFalse())
				Expect(palette.SelectedCommand()).ToNot(BeNil())
			})
		})

		Context("Given a command is selected with Tab", func() {
			It("should close the palette and select first filtered command", func() {
				By("typing 'clea' to filter")
				for _, r := range "clea" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				By("pressing tab to auto-complete")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyTab})
				Expect(closed).To(BeTrue())
				Expect(palette.SelectedCommand()).ToNot(BeNil())
			})
		})

		Context("Given Enter is pressed with no filtered results", func() {
			It("should not close the palette", func() {
				By("typing a non-matching query")
				for _, r := range "zzzzzz" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				By("pressing enter")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(closed).To(BeFalse())
				Expect(palette.IsShowing()).To(BeTrue())
			})
		})
	})

	Describe("Cancellation", func() {
		BeforeEach(func() {
			palette.Open(80, 24)
		})

		Context("Given Esc is pressed", func() {
			It("should close the palette without selecting", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyEsc})
				Expect(closed).To(BeTrue())
				Expect(palette.IsShowing()).To(BeFalse())
				Expect(palette.SelectedCommand()).To(BeNil())
			})
		})

		Context("Given 'q' is pressed", func() {
			It("should close the palette without selecting", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
				Expect(closed).To(BeTrue())
				Expect(palette.IsShowing()).To(BeFalse())
			})
		})

		Context("Given backspace is pressed with empty query", func() {
			It("should close the palette", func() {
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				Expect(closed).To(BeTrue())
				Expect(palette.IsShowing()).To(BeFalse())
			})
		})
	})

	Describe("Edge Cases", func() {
		Context("Given the palette is not showing", func() {
			It("should ignore all key events", func() {
				By("pressing keys without opening")
				closed, _ := palette.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(closed).To(BeFalse())
				Expect(palette.IsShowing()).To(BeFalse())
			})
		})

		Context("Given a very narrow terminal", func() {
			It("should constrain viewport width to minimum", func() {
				palette.Open(20, 10)
				Expect(palette.viewport.Width).To(BeNumerically(">=", 30))
			})
		})

		Context("Given a very short terminal", func() {
			It("should constrain viewport height to minimum", func() {
				palette.Open(80, 5)
				Expect(palette.viewport.Height).To(BeNumerically(">=", 1))
			})
		})

		Context("Given cursor is at end and filter reduces results", func() {
			It("should clamp cursor to new filtered length", func() {
				palette.Open(80, 24)
				By("moving cursor to end")
				palette.Update(tea.KeyMsg{Type: tea.KeyEnd})
				oldCursor := palette.cursor
				Expect(oldCursor).To(Equal(len(palette.commands) - 1))

				By("typing a restrictive filter")
				for _, r := range "clear" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(palette.cursor).To(BeNumerically("<=", len(palette.filtered)-1))
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given the palette is open", func() {
			BeforeEach(func() {
				palette.Open(80, 24)
			})

			It("should render non-empty view", func() {
				view := palette.View(80, 24)
				Expect(view).ToNot(BeEmpty())
				Expect(view).To(ContainSubstring("Commands"))
			})

			It("should render search query when present", func() {
				palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				view := palette.View(80, 24)
				Expect(view).To(ContainSubstring("Search:"))
				Expect(view).To(ContainSubstring("c"))
			})

			It("should render empty state for no matches", func() {
				for _, r := range "zzzzzz" {
					palette.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}
				view := palette.View(80, 24)
				Expect(view).To(ContainSubstring("No commands match"))
			})
		})

		Context("Given the palette is closed", func() {
			It("should render empty view", func() {
				view := palette.View(80, 24)
				Expect(view).To(BeEmpty())
			})
		})
	})
})
