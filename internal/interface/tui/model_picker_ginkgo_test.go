package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModelPickerModel", func() {
	var picker ModelPickerModel

	BeforeEach(func() {
		picker = NewModelPicker()
	})

	Describe("Initialization", func() {
		Context("Given a newly created picker", func() {
			It("should not be showing", func() {
				Expect(picker.IsShowing()).To(BeFalse())
			})

			It("should have empty models list", func() {
				Expect(picker.models).To(BeEmpty())
			})

			It("should have no selected model", func() {
				Expect(picker.SelectedModel()).To(BeNil())
			})
		})
	})

	Describe("Open and Close", func() {
		Context("Given the picker is opened", func() {
			BeforeEach(func() {
				picker.Open(80, 24)
			})

			It("should be showing", func() {
				Expect(picker.IsShowing()).To(BeTrue())
			})

			It("should reset search and cursor", func() {
				Expect(picker.searchQuery).To(Equal(""))
				Expect(picker.cursor).To(Equal(0))
			})

			It("should initialize the viewport", func() {
				Expect(picker.ready).To(BeTrue())
				Expect(picker.viewport.Width).To(BeNumerically(">", 0))
			})
		})

		Context("Given the picker is closed", func() {
			BeforeEach(func() {
				picker.Open(80, 24)
				picker.Close()
			})

			It("should not be showing", func() {
				Expect(picker.IsShowing()).To(BeFalse())
			})

			It("should clear the search query", func() {
				Expect(picker.searchQuery).To(Equal(""))
			})
		})
	})

	Describe("SetModels", func() {
		Context("Given models are set", func() {
			It("should populate the models list", func() {
				models := []ModelItem{
					{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
					{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
				}
				picker.SetModels(models)
				Expect(picker.models).To(HaveLen(2))
			})

			It("should apply filter immediately", func() {
				models := []ModelItem{
					{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
					{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
				}
				picker.SetModels(models)
				Expect(picker.filtered).To(HaveLen(2))
			})
		})
	})

	Describe("Navigation", func() {
		BeforeEach(func() {
			picker.SetModels([]ModelItem{
				{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
				{ID: "gpt-3.5", Name: "GPT-3.5", Provider: "openai"},
				{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
				{ID: "claude-3-haiku", Name: "Claude 3 Haiku", Provider: "anthropic"},
				{ID: "llama-3", Name: "Llama 3", Provider: "openrouter"},
			})
			picker.Open(80, 24)
		})

		Context("Given a list of models", func() {
			It("should move cursor down with down arrow", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(1))
			})

			It("should move cursor down with 'j'", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(1))
			})

			It("should move cursor up with up arrow", func() {
				picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(picker.cursor).To(Equal(2))

				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(1))
			})

			It("should move cursor up with 'k'", func() {
				picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(picker.cursor).To(Equal(1))

				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(0))
			})

			It("should clamp cursor at top", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(0))
			})

			It("should clamp cursor at bottom", func() {
				for i := 0; i < 20; i++ {
					picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
				Expect(picker.cursor).To(Equal(4))
			})

			It("should jump with page down", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(4)) // only 5 models, 0+10 clamped to 4
			})

			It("should jump with page up from bottom", func() {
				picker.Update(tea.KeyMsg{Type: tea.KeyEnd})
				Expect(picker.cursor).To(Equal(4))

				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyPgUp})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(0)) // 4-10 clamped to 0
			})

			It("should go to first item with home", func() {
				picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(picker.cursor).To(Equal(2))

				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyHome})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(0))
			})

			It("should go to last item with end", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnd})
				Expect(closed).To(BeFalse())
				Expect(picker.cursor).To(Equal(4))
			})
		})
	})

	Describe("Filtering", func() {
		BeforeEach(func() {
			picker.SetModels([]ModelItem{
				{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
				{ID: "gpt-3.5", Name: "GPT-3.5", Provider: "openai"},
				{ID: "claude-3-opus", Name: "Claude 3 Opus", Provider: "anthropic"},
				{ID: "claude-3-haiku", Name: "Claude 3 Haiku", Provider: "anthropic"},
				{ID: "llama-3-70b", Name: "Llama 3 70B", Provider: "openrouter"},
			})
			picker.Open(80, 24)
		})

		Context("Given typing search characters", func() {
			It("should filter models by name", func() {
				for _, r := range "claude" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(picker.filtered).To(HaveLen(2))
				Expect(picker.filtered[0].Name).To(ContainSubstring("Claude"))
			})

			It("should filter models by provider", func() {
				for _, r := range "openai" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(picker.filtered).To(HaveLen(2))
				for _, m := range picker.filtered {
					Expect(m.Provider).To(Equal("openai"))
				}
			})

			It("should filter models by ID", func() {
				for _, r := range "70b" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(picker.filtered).To(HaveLen(1))
				Expect(picker.filtered[0].ID).To(Equal("llama-3-70b"))
			})

			It("should show empty results for non-matching query", func() {
				for _, r := range "zzzzzz" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(picker.filtered).To(BeEmpty())
			})

			It("should reset filter when backspacing to empty", func() {
				for _, r := range "clau" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}
				Expect(len(picker.filtered)).To(BeNumerically("<", len(picker.models)))

				for i := 0; i < 4; i++ {
					picker.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				}

				Expect(picker.filtered).To(HaveLen(len(picker.models)))
			})
		})
	})

	Describe("Selection", func() {
		BeforeEach(func() {
			picker.SetModels([]ModelItem{
				{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
				{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
			})
			picker.Open(80, 24)
		})

		Context("Given a model is selected with Enter", func() {
			It("should close the picker and set selected model", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(closed).To(BeTrue())
				Expect(picker.IsShowing()).To(BeFalse())
				Expect(picker.SelectedModel()).ToNot(BeNil())
				Expect(picker.SelectedModel().ID).To(Equal("gpt-4"))
			})
		})

		Context("Given Enter is pressed with no filtered results", func() {
			It("should not close the picker", func() {
				for _, r := range "zzzzzz" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(closed).To(BeFalse())
				Expect(picker.IsShowing()).To(BeTrue())
			})
		})
	})

	Describe("Cancellation", func() {
		BeforeEach(func() {
			picker.SetModels([]ModelItem{
				{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
			})
			picker.Open(80, 24)
		})

		Context("Given Esc is pressed", func() {
			It("should close the picker without selecting", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEsc})
				Expect(closed).To(BeTrue())
				Expect(picker.IsShowing()).To(BeFalse())
				Expect(picker.SelectedModel()).To(BeNil())
			})
		})

		Context("Given 'q' is pressed", func() {
			It("should close the picker without selecting", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
				Expect(closed).To(BeTrue())
				Expect(picker.IsShowing()).To(BeFalse())
				Expect(picker.SelectedModel()).To(BeNil())
			})
		})
	})

	Describe("Edge Cases", func() {
		Context("Given the picker is not showing", func() {
			It("should ignore all key events", func() {
				closed, _ := picker.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(closed).To(BeFalse())
				Expect(picker.IsShowing()).To(BeFalse())
			})
		})

		Context("Given cursor is at end and filter reduces results", func() {
			It("should clamp cursor to new filtered length", func() {
				picker.SetModels([]ModelItem{
					{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
					{ID: "gpt-3.5", Name: "GPT-3.5", Provider: "openai"},
					{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
				})
				picker.Open(80, 24)

				By("moving cursor to end")
				picker.Update(tea.KeyMsg{Type: tea.KeyEnd})
				Expect(picker.cursor).To(Equal(2))

				By("typing a restrictive filter")
				for _, r := range "gpt" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}

				Expect(picker.cursor).To(BeNumerically("<=", len(picker.filtered)-1))
			})
		})

		Context("Given an empty model list", func() {
			It("should handle navigation without panic", func() {
				picker.SetModels([]ModelItem{})
				picker.Open(80, 24)

				Expect(func() {
					picker.Update(tea.KeyMsg{Type: tea.KeyDown})
					picker.Update(tea.KeyMsg{Type: tea.KeyUp})
					picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
					picker.Update(tea.KeyMsg{Type: tea.KeyEnd})
					picker.Update(tea.KeyMsg{Type: tea.KeyHome})
				}).NotTo(Panic())
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given the picker is open with models", func() {
			BeforeEach(func() {
				picker.SetModels([]ModelItem{
					{ID: "gpt-4", Name: "GPT-4", Provider: "openai", IsDefault: true},
					{ID: "claude-3", Name: "Claude 3", Provider: "anthropic"},
				})
				picker.Open(80, 24)
			})

			It("should render non-empty view", func() {
				view := picker.View(80, 24)
				Expect(view).ToNot(BeEmpty())
				Expect(view).To(ContainSubstring("Select Model"))
			})

			It("should render default tag on default model", func() {
				view := picker.View(80, 24)
				Expect(view).To(ContainSubstring("[default]"))
			})

			It("should render provider labels", func() {
				view := picker.View(80, 24)
				Expect(view).To(ContainSubstring("[OpenAI]"))
				Expect(view).To(ContainSubstring("[Anthropic]"))
			})

			It("should render search query when present", func() {
				picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				view := picker.View(80, 24)
				Expect(view).To(ContainSubstring("Filter:"))
			})

			It("should render empty state for no matches", func() {
				for _, r := range "zzzzzz" {
					picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				}
				view := picker.View(80, 24)
				Expect(view).To(ContainSubstring("No models match"))
			})
		})

		Context("Given the picker is closed", func() {
			It("should render empty view", func() {
				view := picker.View(80, 24)
				Expect(view).To(BeEmpty())
			})
		})
	})
})
