package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Test entry point
// ---------------------------------------------------------------------------

func TestChatModel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chat Model Suite")
}

// ---------------------------------------------------------------------------
// Test delegate
// ---------------------------------------------------------------------------

type testChatDelegate struct {
	submittedText string
	commandText   string
}

func (d *testChatDelegate) OnSubmit(text string) tea.Cmd {
	d.submittedText = text
	return nil
}

func (d *testChatDelegate) OnCommand(command string) {
	d.commandText = command
}

// ---------------------------------------------------------------------------
// Specs
// ---------------------------------------------------------------------------

var _ = Describe("ChatModel", func() {
	var chat ChatModel
	var delegate *testChatDelegate

	BeforeEach(func() {
		chat = NewChatModel()
		delegate = &testChatDelegate{}
		chat.SetDelegate(delegate)
	})

	// ========================================================================
	// Paste Detection — Display Behaviour
	// ========================================================================
	Describe("Paste Detection", func() {

		Context("Given a regular typed message", func() {
			It("should display the full message", func() {
				By("typing characters one at a time")
				for _, r := range "hello world" {
					model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
					chat = model.(ChatModel)
				}

				By("pressing Enter to submit")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full text is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Role).To(Equal("user"))
				Expect(chat.messages[0].Content).To(Equal("hello world"))
			})
		})

		Context("Given a bracketed paste above the display threshold", func() {
			It("should display collapsed text", func() {
				By("pasting a long block of text via bracketed paste")
				pasted := strings.Repeat("a", PasteDisplayThreshold+1)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the collapsed placeholder is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 201 characters]"))
			})

			It("should still send the full text to the delegate", func() {
				By("pasting a long block of text via bracketed paste")
				pasted := strings.Repeat("b", PasteDisplayThreshold+50)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the delegate received the full text")
				Expect(delegate.submittedText).To(Equal(pasted))
			})
		})

		Context("Given a bracketed paste below the display threshold", func() {
			It("should display the full text", func() {
				By("pasting a short block of text via bracketed paste")
				pasted := strings.Repeat("c", PasteDisplayThreshold-1)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full text is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal(pasted))
			})
		})

		Context("Given a heuristic paste detection (length jump)", func() {
			It("should collapse text when input grows by more than 20 chars in one keystroke", func() {
				By("typing a short prefix")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi ")})
				chat = model.(ChatModel)

				By("simulating a paste of 210 characters in a single keystroke")
				model, _ = chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(strings.Repeat("x", 210)),
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the collapsed placeholder is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 213 characters]"))
			})
		})

		Context("Given a small length jump (≤20 chars)", func() {
			It("should not trigger heuristic paste detection", func() {
				By("typing a short prefix")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("go ")})
				chat = model.(ChatModel)

				By("adding exactly 20 characters in one keystroke")
				model, _ = chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(strings.Repeat("y", 20)),
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full text is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("go " + strings.Repeat("y", 20)))
			})
		})

		Context("Given a pasted slash command", func() {
			It("should always display the full command, never collapse", func() {
				By("pasting a very long slash command via bracketed paste")
				longCmd := "/model " + strings.Repeat("m", PasteDisplayThreshold+50)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(longCmd),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full command is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal(longCmd))
			})
		})

		Context("Given a long manually-typed message", func() {
			It("should display the full text because no paste was detected", func() {
				By("typing a long message one character at a time")
				longText := strings.Repeat("z", PasteDisplayThreshold+50)
				for _, r := range longText {
					model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
					chat = model.(ChatModel)
				}

				By("pressing Enter to submit")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full text is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal(longText))
			})
		})

		Context("Given a paste followed by minor editing", func() {
			It("should still collapse because the paste flag persists", func() {
				By("pasting a long block of text")
				pasted := strings.Repeat("d", PasteDisplayThreshold+10)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("deleting a few characters (backspace)")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the collapsed placeholder is still displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
			})
		})

		Context("Given a paste that is then cleared", func() {
			It("should reset the paste flag and display the next message normally", func() {
				By("pasting a long block of text")
				pasted := strings.Repeat("e", PasteDisplayThreshold+10)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("clearing the input")
				chat.ClearInput()

				By("typing a normal short message")
				for _, r := range "normal message" {
					model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
					chat = model.(ChatModel)
				}

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the full text is displayed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("normal message"))
			})
		})

		Context("Given Ctrl+C during a pasted input", func() {
			It("should reset the paste flag", func() {
				By("pasting text")
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(strings.Repeat("f", PasteDisplayThreshold+10)),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Ctrl+C")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
				chat = model.(ChatModel)

				By("verifying the paste flag was reset")
				Expect(chat.pasteDetected).To(BeFalse())
			})
		})
	})

	// ========================================================================
	// Multi-line paste edge case
	// ========================================================================
	Describe("Multi-line Input", func() {
		Context("Given Alt+Enter during pasted input", func() {
			It("should allow multi-line input without submitting", func() {
				By("pasting a long block of text")
				pasted := strings.Repeat("g", PasteDisplayThreshold+10)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Alt+Enter to insert a newline")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
				chat = model.(ChatModel)

				By("verifying the input still contains the text and was not submitted")
				Expect(chat.messages).To(HaveLen(0))
				Expect(chat.textarea.Value()).To(ContainSubstring("\n"))
				Expect(chat.pasteDetected).To(BeTrue())
			})
		})
	})
})
