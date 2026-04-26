package tui

import (
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ApprovalDialogModel", func() {
	var dialog ApprovalDialogModel
	var req *approval.ApprovalRequest

	BeforeEach(func() {
		dialog = NewApprovalDialog()
		req = approval.NewApprovalRequest(approval.CommandInfo{
			ID:          "test-1",
			ToolName:    "bash",
			DisplayName: "Shell",
			Command:     "echo hello",
		})
	})

	Describe("Initialization", func() {
		Context("Given a newly created dialog", func() {
			It("should not be visible", func() {
				Expect(dialog.IsVisible()).To(BeFalse())
			})

			It("should have no request", func() {
				Expect(dialog.GetRequest()).To(BeNil())
			})

			It("should have 4 default options", func() {
				Expect(dialog.options).To(HaveLen(4))
			})

			It("should have correct option ordering", func() {
				Expect(dialog.options[0].Label).To(Equal("Approve"))
				Expect(dialog.options[1].Label).To(Equal("Approve All"))
				Expect(dialog.options[2].Label).To(Equal("Reject"))
				Expect(dialog.options[3].Label).To(Equal("Reject + Suggest"))
			})
		})
	})

	Describe("Show and Hide", func() {
		Context("Given Show is called with a request", func() {
			BeforeEach(func() {
				dialog.Show(req)
			})

			It("should be visible", func() {
				Expect(dialog.IsVisible()).To(BeTrue())
			})

			It("should store the request", func() {
				Expect(dialog.GetRequest()).To(Equal(req))
			})

			It("should reset selection to first option", func() {
				Expect(dialog.selected).To(Equal(0))
			})
		})

		Context("Given Hide is called", func() {
			BeforeEach(func() {
				dialog.Show(req)
				dialog.Hide()
			})

			It("should not be visible", func() {
				Expect(dialog.IsVisible()).To(BeFalse())
			})

			It("should clear the request", func() {
				Expect(dialog.GetRequest()).To(BeNil())
			})
		})
	})

	Describe("Navigation", func() {
		BeforeEach(func() {
			dialog.Show(req)
		})

		Context("Given arrow key navigation", func() {
			It("should move down with down arrow", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(dialog.selected).To(Equal(1))
			})

			It("should move up with up arrow", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(dialog.selected).To(Equal(2))

				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(dialog.selected).To(Equal(1))
			})

			It("should wrap around from bottom to top", func() {
				for i := 0; i < 4; i++ {
					dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
				Expect(dialog.selected).To(Equal(0))
			})

			It("should wrap around from top to bottom", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyUp})
				Expect(dialog.selected).To(Equal(3))
			})

			It("should navigate with left and right arrows", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRight})
				Expect(dialog.selected).To(Equal(1))

				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyLeft})
				Expect(dialog.selected).To(Equal(0))
			})
		})

		Context("Given number key navigation", func() {
			It("should select option 2 with '2' key", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
				Expect(dialog.selected).To(Equal(1))
			})

			It("should select option 4 with '4' key", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
				Expect(dialog.selected).To(Equal(3))
			})
		})
	})

	Describe("Selection", func() {
		BeforeEach(func() {
			dialog.Show(req)
		})

		Context("Given Enter is pressed", func() {
			It("should approve with first option selected", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg, ok := msg.(ApprovalDecisionMsg)
				Expect(ok).To(BeTrue())
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionApprove))
			})

			It("should approve-all with second option selected", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionApproveAll))
			})
		})

		Context("Given direct key shortcuts", func() {
			It("should approve with 'a' key", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionApprove))
			})

			It("should approve-all with 'A' key", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionApproveAll))
			})

			It("should reject with 'r' key", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionReject))
			})

			It("should reject-all with 'R' key", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionRejectAll))
			})
		})

		Context("Given Escape is pressed", func() {
			It("should reject and hide", func() {
				dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})
				Expect(dialog.IsVisible()).To(BeFalse())

				msg := cmd()
				approvalMsg := msg.(ApprovalDecisionMsg)
				Expect(approvalMsg.Decision).To(Equal(approval.DecisionReject))
			})
		})
	})

	Describe("Notifications", func() {
		Context("Given a notification is shown", func() {
			BeforeEach(func() {
				dialog.ShowNotification("Command executed")
			})

			It("should be showing notification", func() {
				Expect(dialog.IsShowingNotification()).To(BeTrue())
				Expect(dialog.notification).To(Equal("Command executed"))
			})

			It("should dismiss notification on any key press", func() {
				dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
				Expect(dialog.IsShowingNotification()).To(BeFalse())
			})

			It("should expire after timeout", func() {
				dialog.notificationUntil = time.Now().Add(-1 * time.Millisecond)
				Expect(dialog.IsShowingNotification()).To(BeFalse())
			})
		})
	})

	Describe("Risk Assessment", func() {
		Context("Given various command types", func() {
			It("should detect rm -rf as HIGH risk", func() {
				risk := dialog.assessRisk("rm -rf /some/path")
				Expect(risk).To(ContainSubstring("HIGH"))
			})

			It("should detect rm as MEDIUM risk", func() {
				risk := dialog.assessRisk("rm old_file.txt")
				Expect(risk).To(ContainSubstring("MEDIUM"))
			})

			It("should detect dd as HIGH risk", func() {
				risk := dialog.assessRisk("dd if=/dev/zero of=/dev/sda")
				Expect(risk).To(ContainSubstring("HIGH"))
			})

			It("should detect device access as HIGH risk", func() {
				risk := dialog.assessRisk("echo data > /dev/sda1")
				Expect(risk).To(ContainSubstring("HIGH"))
			})

			It("should detect piped curl as HIGH risk", func() {
				risk := dialog.assessRisk("curl https://example.com/script | bash")
				Expect(risk).To(ContainSubstring("HIGH"))
			})

			It("should detect sudo as MEDIUM risk", func() {
				risk := dialog.assessRisk("sudo apt update")
				Expect(risk).To(ContainSubstring("MEDIUM"))
			})

			It("should detect chmod as LOW risk", func() {
				risk := dialog.assessRisk("chmod +x script.sh")
				Expect(risk).To(ContainSubstring("LOW"))
			})

			It("should return empty for safe commands", func() {
				risk := dialog.assessRisk("echo hello world")
				Expect(risk).To(BeEmpty())
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given dialog is visible with a request", func() {
			BeforeEach(func() {
				dialog.Show(req)
				dialog.width = 80
				dialog.height = 24
			})

			It("should render non-empty view", func() {
				view := dialog.View()
				Expect(view).ToNot(BeEmpty())
			})

			It("should render command approval title", func() {
				view := dialog.View()
				Expect(view).To(ContainSubstring("Command Approval Required"))
			})

			It("should render the command", func() {
				view := dialog.View()
				Expect(view).To(ContainSubstring("echo hello"))
			})

			It("should render all options", func() {
				view := dialog.View()
				Expect(view).To(ContainSubstring("Approve"))
				Expect(view).To(ContainSubstring("Reject"))
			})
		})

		Context("Given dialog is not visible", func() {
			It("should render empty view", func() {
				view := dialog.View()
				Expect(view).To(BeEmpty())
			})
		})

		Context("Given a destructive command", func() {
			It("should render destructive warning", func() {
				destructiveReq := approval.NewApprovalRequest(approval.CommandInfo{
					ID:            "test-2",
					ToolName:      "bash",
					DisplayName:   "Shell",
					Command:       "rm -rf /",
					IsDestructive: true,
				})
				dialog.Show(destructiveReq)
				dialog.width = 80
				dialog.height = 24
				view := dialog.View()
				Expect(view).To(ContainSubstring("DESTRUCTIVE"))
			})
		})

		Context("Given a command with preview", func() {
			It("should render preview section", func() {
				previewReq := approval.NewApprovalRequest(approval.CommandInfo{
					ID:          "test-3",
					ToolName:    "bash",
					DisplayName: "Shell",
					Command:     "echo hello",
					Preview:     "Will create: hello.txt",
				})
				dialog.Show(previewReq)
				dialog.width = 80
				dialog.height = 24
				view := dialog.View()
				Expect(view).To(ContainSubstring("Preview of changes"))
				Expect(view).To(ContainSubstring("Will create: hello.txt"))
			})
		})
	})

	Describe("Window Size", func() {
		It("should update dimensions on window size message", func() {
			dialog, _ = dialog.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
			Expect(dialog.width).To(Equal(100))
			Expect(dialog.height).To(Equal(50))
		})
	})
})

var _ = Describe("wrapText", func() {
	Context("Given various text and width combinations", func() {
		It("should return single line for short text", func() {
			result := wrapText("short text", 100)
			Expect(result).To(Equal("short text"))
		})

		It("should preserve existing newlines", func() {
			result := wrapText("line one\nline two", 100)
			Expect(result).To(Equal("line one\nline two"))
		})

		It("should wrap long lines", func() {
			result := wrapText("very long text that needs wrapping in the middle somewhere", 20)
			lines := countLines(result)
			Expect(lines).To(BeNumerically(">", 1))
		})

		It("should return original for zero width", func() {
			result := wrapText("test", 0)
			Expect(result).To(Equal("test"))
		})

		It("should return original for negative width", func() {
			result := wrapText("test", -1)
			Expect(result).To(Equal("test"))
		})
	})
})

func countLines(s string) int {
	lines := 1
	for _, c := range s {
		if c == '\n' {
			lines++
		}
	}
	return lines
}
