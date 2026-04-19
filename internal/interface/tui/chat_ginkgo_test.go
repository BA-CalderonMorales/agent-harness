package tui

import (
	"strings"
	"testing"
	"time"

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
		SubmitDebounceDuration = 0 // immediate submit for legacy specs
	})

	// ========================================================================
	// Tool Display — Single-line replacement and status line
	// ========================================================================
	Describe("Tool Display", func() {
		Context("Given a tool starts during an agent turn", func() {
			It("should show the tool message in the viewport", func() {
				By("starting the agent turn")
				chat.AddMessage("user", "run a command")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("receiving a tool start message")
				model, _ = chat.Update(AgentToolStartMsg{
					ToolID:       "tool-1",
					ToolName:     "bash",
					DisplayName:  "Shell",
					ActivityDesc: "ls -la",
				})
				chat = model.(ChatModel)

				By("verifying the tool message was created")
				Expect(chat.currentToolMsg).ToNot(BeNil())
				Expect(chat.currentToolMsg.ToolName).To(Equal("bash"))
				Expect(chat.currentToolMsg.ToolStatus).To(Equal(ToolStatusRunning))
			})
		})

		Context("Given a tool completes successfully", func() {
			It("should update the tool status to success", func() {
				chat.AddMessage("user", "run a command")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				model, _ = chat.Update(AgentToolStartMsg{
					ToolID:       "tool-1",
					ToolName:     "bash",
					DisplayName:  "Shell",
					ActivityDesc: "ls -la",
				})
				chat = model.(ChatModel)

				By("receiving tool done")
				model, _ = chat.Update(AgentToolDoneMsg{ToolID: "tool-1", Success: true})
				chat = model.(ChatModel)

				By("verifying the completed tool is stored")
				Expect(chat.completedToolMsg).ToNot(BeNil())
				Expect(chat.completedToolMsg.ToolStatus).To(Equal(ToolStatusSuccess))
				Expect(chat.currentToolMsg).To(BeNil())
			})
		})

		Context("Given the agent turn completes after a tool ran", func() {
			It("should move the completed tool into message history", func() {
				chat.AddMessage("user", "run a command")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				model, _ = chat.Update(AgentToolStartMsg{
					ToolID:       "tool-1",
					ToolName:     "bash",
					DisplayName:  "Shell",
					ActivityDesc: "ls -la",
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(AgentToolDoneMsg{ToolID: "tool-1", Success: true})
				chat = model.(ChatModel)

				By("completing the agent turn")
				model, _ = chat.Update(AgentDoneMsg{FullResponse: "Done"})
				chat = model.(ChatModel)

				By("verifying the tool is in message history")
				Expect(chat.completedToolMsg).To(BeNil())
				var toolMsgs []ChatMessage
				for _, msg := range chat.messages {
					if msg.IsTool {
						toolMsgs = append(toolMsgs, msg)
					}
				}
				Expect(toolMsgs).To(HaveLen(1))
				Expect(toolMsgs[0].ToolStatus).To(Equal(ToolStatusSuccess))
			})
		})

		Context("Given multiple tools run in one turn", func() {
			It("should clear the previous completed tool when a new one starts", func() {
				chat.AddMessage("user", "run commands")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("starting first tool")
				model, _ = chat.Update(AgentToolStartMsg{
					ToolID:       "tool-1",
					ToolName:     "read",
					DisplayName:  "Read File",
					ActivityDesc: "cat main.go",
				})
				chat = model.(ChatModel)

				By("completing first tool")
				model, _ = chat.Update(AgentToolDoneMsg{ToolID: "tool-1", Success: true})
				chat = model.(ChatModel)

				By("starting second tool")
				model, _ = chat.Update(AgentToolStartMsg{
					ToolID:       "tool-2",
					ToolName:     "bash",
					DisplayName:  "Shell",
					ActivityDesc: "go test",
				})
				chat = model.(ChatModel)

				By("verifying the previous completed tool was cleared")
				Expect(chat.completedToolMsg).To(BeNil())
				Expect(chat.currentToolMsg).ToNot(BeNil())
				Expect(chat.currentToolMsg.ToolName).To(Equal("bash"))
			})
		})
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

	// ========================================================================
	// Multiline Paste Truncation
	// ========================================================================
	Describe("Multiline Paste Truncation", func() {
		Context("Given a bracketed paste with multiple lines below the char threshold", func() {
			It("should collapse and show line count", func() {
				By("pasting a short multiline block via bracketed paste")
				pasted := "Line one\nLine two\nLine three"
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the collapsed placeholder shows line count")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Role).To(Equal("user"))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 3 lines, 28 characters]"))
				Expect(delegate.submittedText).To(Equal(pasted))
			})
		})

		Context("Given a bracketed paste with multiple lines above the char threshold", func() {
			It("should collapse and show line count", func() {
				By("pasting a long multiline block via bracketed paste")
				pasted := strings.Repeat("a", 100) + "\n" + strings.Repeat("b", 110)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying the collapsed placeholder shows line count and chars")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 2 lines, 211 characters]"))
			})
		})

		Context("Given a single-line bracketed paste below the threshold", func() {
			It("should display the full text", func() {
				By("pasting a short single-line block via bracketed paste")
				pasted := "Short single line"
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

		Context("Given Ctrl+J (line feed) during manual typing", func() {
			It("should insert a newline in the textarea", func() {
				By("typing 'hello'")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
				chat = model.(ChatModel)

				By("pressing Ctrl+J to insert a newline")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
				chat = model.(ChatModel)

				By("typing 'world'")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})
				chat = model.(ChatModel)

				By("verifying the textarea contains the newline")
				Expect(chat.textarea.Value()).To(Equal("hello\nworld"))
			})
		})

		Context("Given a heuristic paste containing Ctrl+J line feeds", func() {
			It("should preserve newlines and submit as one message", func() {
				By("typing a short prefix")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("go ")})
				chat = model.(ChatModel)

				By("simulating a paste of 25 characters")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", 25))})
				chat = model.(ChatModel)

				By("receiving a Ctrl+J line feed from the terminal")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
				chat = model.(ChatModel)

				By("typing more text after the line feed")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("second line")})
				chat = model.(ChatModel)

				By("pressing Enter to submit")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying a single message was submitted with newlines preserved")
				Expect(chat.messages).To(HaveLen(1))
				Expect(delegate.submittedText).To(Equal("go " + strings.Repeat("x", 25) + "\nsecond line"))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
			})
		})
	})

	// ========================================================================
	// Submit Debounce — Termux paste spam prevention
	// ========================================================================
	Describe("Submit Debounce", func() {
		BeforeEach(func() {
			SubmitDebounceDuration = 10 * time.Millisecond
		})

		AfterEach(func() {
			SubmitDebounceDuration = 0
		})

		Context("Given a Termux paste where newlines arrive as KeyEnter", func() {
			It("should submit exactly one message after the timer fires", func() {
				By("pasting the first line")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line1")})
				chat = model.(ChatModel)

				By("receiving Enter as a pasted newline")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("pasting the second line before the debounce fires")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line2")})
				chat = model.(ChatModel)

				By("verifying no messages were submitted yet")
				Expect(chat.messages).To(HaveLen(0))
				Expect(chat.textarea.Value()).To(Equal("line1\nline2"))

				By("pressing Enter to submit after the paste")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("firing the debounce timer")
				gen := chat.pendingSubmitGen
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying exactly one message was submitted")
				Expect(chat.messages).To(HaveLen(1))
				Expect(delegate.submittedText).To(Equal("line1\nline2"))
			})
		})

		Context("Given consecutive Enter events during a paste", func() {
			It("should treat them as blank lines and still submit once", func() {
				By("typing a character")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
				chat = model.(ChatModel)

				By("receiving two consecutive Enters in the paste")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("typing the next line")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
				chat = model.(ChatModel)

				By("pressing Enter to submit after the paste")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("firing the debounce timer")
				gen := chat.pendingSubmitGen
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying the message contains blank lines")
				Expect(delegate.submittedText).To(Equal("a\n\nb"))
			})
		})

		Context("Given Ctrl+J arrives while a submit is pending", func() {
			It("should cancel the pending submit and insert a newline", func() {
				By("typing a character and pressing Enter")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("pressing Ctrl+J before the timer fires")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
				chat = model.(ChatModel)

				By("verifying the textarea has a newline but no submit yet")
				Expect(chat.textarea.Value()).To(Equal("a\n"))
				Expect(chat.messages).To(HaveLen(0))

				By("firing the stale timer")
				gen := chat.pendingSubmitGen - 1
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying the stale timer did not submit")
				Expect(chat.messages).To(HaveLen(0))
			})
		})

		Context("Given an empty input and Enter is pressed", func() {
			It("should not start a debounce timer", func() {
				By("pressing Enter with empty input")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying no pending submit was started")
				Expect(chat.pendingSubmit).To(BeFalse())
			})
		})

		Context("Given a backspace arrives while a submit is pending", func() {
			It("should cancel the pending submit and NOT insert a newline", func() {
				By("typing 'ab' and pressing Enter")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("pressing Backspace before the timer fires")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				chat = model.(ChatModel)

				By("verifying the textarea reflects a backspace, not a newline")
				Expect(chat.textarea.Value()).To(Equal("a"))
				Expect(chat.pendingSubmit).To(BeFalse())
			})
		})

		Context("Given an escape key arrives while a submit is pending", func() {
			It("should cancel the pending submit and NOT insert a newline", func() {
				By("typing text and pressing Enter")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("pressing Escape before the timer fires")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEsc})
				chat = model.(ChatModel)

				By("verifying the textarea is unchanged and submit was cancelled")
				Expect(chat.textarea.Value()).To(Equal("hello"))
				Expect(chat.pendingSubmit).To(BeFalse())
			})
		})

		Context("Given a spam paste of 5 short lines in Termux", func() {
			It("should result in exactly one submission with collapsed display", func() {
				lines := []string{
					"Line 1: First requirement",
					"Line 2: Second requirement",
					"Line 3: Third requirement",
					"Line 4: Fourth requirement",
					"Line 5: Fifth requirement",
				}

				By("simulating Termux paste with Enter-separated short lines")
				for i, line := range lines {
					model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
					chat = model.(ChatModel)

					if i < len(lines)-1 {
						model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
						chat = model.(ChatModel)
					}
				}

				By("verifying zero messages mid-paste")
				Expect(chat.messages).To(HaveLen(0))

				By("pressing Enter to submit")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("firing the debounce timer")
				gen := chat.pendingSubmitGen
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying exactly one collapsed message")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
				Expect(delegate.submittedText).To(ContainSubstring("Line 1"))
				Expect(delegate.submittedText).To(ContainSubstring("Line 5"))
			})
		})
	})

	// ========================================================================
	// Short multiline paste detection (Termux without bracketed paste)
	// ========================================================================
	Describe("Short Multiline Paste Detection", func() {
		BeforeEach(func() {
			SubmitDebounceDuration = 10 * time.Millisecond
		})

		AfterEach(func() {
			SubmitDebounceDuration = 0
		})

		Context("Given 5 short lines pasted in Termux (no bracketed paste)", func() {
			It("should collapse because the second line triggers paste detection", func() {
				By("pasting line 1 (short, does not trigger heuristic)")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
				chat = model.(ChatModel)

				By("receiving Enter then line 2 (paste stream detected)")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bye")})
				chat = model.(ChatModel)

				By("pressing Enter to submit after the paste")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("firing the debounce timer")
				gen := chat.pendingSubmitGen
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying the display is collapsed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
				Expect(delegate.submittedText).To(Equal("hi\nbye"))
			})
		})
	})

	// ========================================================================
	// Edge Cases
	// ========================================================================
	Describe("Edge Cases", func() {
		Context("Given a paste at exactly the display threshold", func() {
			It("should NOT collapse a single-line paste of exactly 200 chars", func() {
				pasted := strings.Repeat("x", PasteDisplayThreshold)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal(pasted))
			})
		})

		Context("Given a paste one character above the threshold", func() {
			It("should collapse a single-line paste of 201 chars", func() {
				pasted := strings.Repeat("x", PasteDisplayThreshold+1)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 201 characters]"))
			})
		})

		Context("Given a paste with leading and trailing whitespace", func() {
			It("should preserve whitespace in submitted text but collapse display", func() {
				pasted := "  " + strings.Repeat("a", PasteDisplayThreshold) + "  "
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 204 characters]"))
				Expect(delegate.submittedText).To(Equal(pasted))
			})
		})

		Context("Given a very long single-line paste", func() {
			It("should show the exact character count in the collapse placeholder", func() {
				pasted := strings.Repeat("z", 10000)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 10000 characters]"))
			})
		})

		Context("Given a paste containing only newlines", func() {
			It("should collapse and show line count", func() {
				pasted := "\n\n\n"
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(Equal("[Pasted text, 4 lines, 3 characters]"))
			})
		})

		Context("Given two consecutive pastes without clearing", func() {
			It("should collapse both independently", func() {
				By("pasting the first long text")
				pasted1 := strings.Repeat("a", PasteDisplayThreshold+10)
				model, _ := chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted1),
					Paste: true,
				})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("pasting the second long text")
				pasted2 := strings.Repeat("b", PasteDisplayThreshold+20)
				model, _ = chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted2),
					Paste: true,
				})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying both messages are collapsed")
				Expect(chat.messages).To(HaveLen(2))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
				Expect(chat.messages[1].Content).To(ContainSubstring("[Pasted text,"))
			})
		})

		Context("Given suggestions are showing and a paste arrives with a space", func() {
			It("should dismiss suggestions and accept the pasted text", func() {
				By("typing '/' to trigger suggestions")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
				chat = model.(ChatModel)
				Expect(chat.showSuggestions).To(BeTrue())

				By("pasting text containing a space (no longer a slash command)")
				pasted := strings.Repeat("x", PasteDisplayThreshold+10) + " world"
				model, _ = chat.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune(pasted),
					Paste: true,
				})
				chat = model.(ChatModel)

				By("verifying suggestions were dismissed")
				Expect(chat.showSuggestions).To(BeFalse())
				Expect(chat.textarea.Value()).To(Equal("/" + pasted))
			})
		})
	})

	// ========================================================================
	// Follow-up Question Rendering
	// ========================================================================
	Describe("Follow-up Question Rendering", func() {
		Context("Given a user asks a question, agent responds, then user asks a follow-up", func() {
			It("should keep both user messages visible in the message list", func() {
				By("submitting the first question")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q1")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("simulating agent start")
				model, _ = chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("simulating agent response")
				model, _ = chat.Update(AgentChunkMsg{Text: "a1"})
				chat = model.(ChatModel)
				model, _ = chat.Update(AgentDoneMsg{FullResponse: "a1"})
				chat = model.(ChatModel)

				By("submitting the follow-up question")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q2")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying all three messages are present")
				Expect(chat.messages).To(HaveLen(3))
				Expect(chat.messages[0].Role).To(Equal("user"))
				Expect(chat.messages[0].Content).To(Equal("q1"))
				Expect(chat.messages[1].Role).To(Equal("assistant"))
				Expect(chat.messages[1].Content).To(Equal("a1"))
				Expect(chat.messages[2].Role).To(Equal("user"))
				Expect(chat.messages[2].Content).To(Equal("q2"))
			})
		})

		Context("Given a follow-up is submitted while the agent is still thinking", func() {
			It("should refresh the viewport so the new user message is visible", func() {
				By("submitting the first question and starting agent")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q1")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)
				model, _ = chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("submitting a follow-up while thinking")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q2")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying both user messages exist")
				Expect(chat.messages).To(HaveLen(2))
				Expect(chat.messages[0].Content).To(Equal("q1"))
				Expect(chat.messages[1].Content).To(Equal("q2"))
			})
		})
	})

	// ========================================================================
	// Streaming robustness — mid-stream messages
	// ========================================================================
	Describe("Streaming Robustness", func() {
		Context("Given a system message is added while the agent is streaming", func() {
			It("should continue updating the correct assistant message", func() {
				By("starting the agent response")
				chat.AddMessage("user", "hello")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("receiving the first chunk")
				model, _ = chat.Update(AgentChunkMsg{Text: "Hello"})
				chat = model.(ChatModel)
				Expect(chat.messages).To(HaveLen(2))
				Expect(chat.messages[1].Content).To(Equal("Hello"))

				By("adding a system message mid-stream (simulating /steer confirmation)")
				chat.AddMessage("system", "Steered: check tests")

				By("receiving the next chunk")
				model, _ = chat.Update(AgentChunkMsg{Text: " world"})
				chat = model.(ChatModel)

				By("verifying the assistant message was updated, not a new one created")
				Expect(chat.messages).To(HaveLen(3))
				Expect(chat.messages[1].Role).To(Equal("assistant"))
				Expect(chat.messages[1].Content).To(Equal("Hello world"))
				Expect(chat.messages[2].Role).To(Equal("system"))
			})
		})
	})

	// ========================================================================
	// Steer Queue
	// ========================================================================
	Describe("Steer Queue", func() {
		Context("Given a steer message is queued during an agent turn", func() {
			It("should auto-submit the steer message after AgentDoneMsg", func() {
				By("starting an agent turn")
				chat.AddMessage("user", "hello")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				By("queuing a steer message")
				chat.QueueSteer("check the tests")
				Expect(chat.GetSteerQueue()).To(HaveLen(1))

				By("completing the agent turn")
				model, _ = chat.Update(AgentDoneMsg{FullResponse: "done"})
				chat = model.(ChatModel)

				By("verifying the steer message was added to chat")
				Expect(chat.messages).To(HaveLen(3)) // user, assistant, steer-user
				Expect(chat.messages[2].Role).To(Equal("user"))
				Expect(chat.messages[2].Content).To(Equal("check the tests"))

				By("verifying the delegate received the steer text")
				Expect(delegate.submittedText).To(Equal("check the tests"))
				Expect(chat.GetSteerQueue()).To(HaveLen(0))
			})
		})

		Context("Given multiple steer messages are queued", func() {
			It("should submit only the first one after AgentDoneMsg", func() {
				chat.AddMessage("user", "hello")
				model, _ := chat.Update(AgentStartMsg{})
				chat = model.(ChatModel)

				chat.QueueSteer("first")
				chat.QueueSteer("second")

				model, _ = chat.Update(AgentDoneMsg{FullResponse: "done"})
				chat = model.(ChatModel)

				Expect(chat.messages).To(HaveLen(3))
				Expect(chat.messages[2].Content).To(Equal("first"))
				Expect(chat.GetSteerQueue()).To(HaveLen(1))
			})
		})
	})

	// ========================================================================
	// Paste detection via pending-submit Enter
	// ========================================================================
	Describe("Paste Detection via Debounce Enter", func() {
		BeforeEach(func() {
			SubmitDebounceDuration = 10 * time.Millisecond
		})

		AfterEach(func() {
			SubmitDebounceDuration = 0
		})

		Context("Given Enter arrives while a submit is pending", func() {
			It("should mark pasteDetected so the final display collapses", func() {
				By("typing a long first line and pressing Enter")
				longLine := strings.Repeat("a", PasteDisplayThreshold+1)
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(longLine)})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("receiving another Enter while pending (simulating pasted newline)")
				Expect(chat.pendingSubmit).To(BeTrue())
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				By("verifying pasteDetected was set")
				Expect(chat.pasteDetected).To(BeTrue())

				By("submitting after the paste")
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)
				gen := chat.pendingSubmitGen
				model, _ = chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)

				By("verifying the display is collapsed")
				Expect(chat.messages).To(HaveLen(1))
				Expect(chat.messages[0].Content).To(ContainSubstring("[Pasted text,"))
			})
		})
	})

	// ========================================================================
	// Focus / Blur interactions
	// ========================================================================
	Describe("Focus and Blur", func() {
		BeforeEach(func() {
			SubmitDebounceDuration = 10 * time.Millisecond
		})

		AfterEach(func() {
			SubmitDebounceDuration = 0
		})

		Context("Given the view is blurred while a submit is pending", func() {
			It("should cancel the pending submit", func() {
				By("typing text and pressing Enter")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				Expect(chat.pendingSubmit).To(BeTrue())

				By("blurring the chat view")
				chat.Blur()

				By("verifying the pending submit was cancelled")
				Expect(chat.pendingSubmit).To(BeFalse())
			})
		})

		Context("Given a stale timer fires after blur", func() {
			It("should NOT submit because generation no longer matches", func() {
				By("typing text and pressing Enter")
				model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
				chat = model.(ChatModel)
				model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
				chat = model.(ChatModel)

				oldGen := chat.pendingSubmitGen

				By("blurring the view (increments generation)")
				chat.Blur()

				By("firing a timer with the old generation")
				model, _ = chat.Update(submitTimerMsg{generation: oldGen})
				chat = model.(ChatModel)

				By("verifying no message was submitted")
				Expect(chat.messages).To(HaveLen(0))
			})
		})
	})
})
