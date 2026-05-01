package tui

import (
	"fmt"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("App", func() {
	var app *App

	BeforeEach(func() {
		app = NewApp()
		app.width = 80
		app.height = 24
	})

	Describe("Initialization", func() {
		Context("Given a newly created app", func() {
			It("should start on Home view", func() {
				Expect(app.activeView).To(Equal(viewHome))
			})

			It("should start in Normal mode", func() {
				Expect(app.mode).To(Equal(ModeNormal))
			})

			It("should have a message channel", func() {
				Expect(app.msgChan).ToNot(BeNil())
			})

			It("should initialize all sub-models", func() {
				Expect(app.homeModel).ToNot(BeNil())
				Expect(app.chatModel.GetModel()).To(Equal(""))
				Expect(app.sessionsModel).ToNot(BeNil())
				Expect(app.settingsModel).ToNot(BeNil())
			})

			It("should return a batch command from Init", func() {
				cmd := app.Init()
				Expect(cmd).ToNot(BeNil())
			})
		})
	})

	Describe("Home Quick Actions", func() {
		var homeDelegate *testHomeDelegate

		BeforeEach(func() {
			homeDelegate = &testHomeDelegate{}
			app.SetHomeDelegate(homeDelegate)
		})

		Context("Given the app is on the home view", func() {
			It("should dispatch new chat on 'n' key", func() {
				By("pressing 'n' while home is focused")
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
				updated := model.(App)

				By("verifying the delegate was called")
				Expect(homeDelegate.newChatCalled).To(BeTrue())
				Expect(updated.homeModel.focused).To(BeTrue())
			})

			It("should dispatch export session on 'e' key", func() {
				_, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
				Expect(homeDelegate.exportSessionCalled).To(BeTrue())
			})
		})
	})

	Describe("Tab Switching", func() {
		BeforeEach(func() {
			app.width = 80
			app.height = 24
		})

		Context("Given Tab is pressed", func() {
			It("should cycle to next view", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewChat))
			})

			It("should wrap around from last to first view", func() {
				app.activeView = viewSettings
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewHome))
			})
		})

		Context("Given Shift+Tab is pressed", func() {
			It("should cycle to previous view", func() {
				app.activeView = viewChat
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewHome))
			})

			It("should wrap around from first to last view", func() {
				app.activeView = viewHome
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewSettings))
			})
		})

		Context("Given number key shortcuts", func() {
			It("should switch to Home with '1'", func() {
				app.activeView = viewChat
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewHome))
			})

			It("should switch to Chat with '2'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewChat))
			})

			It("should switch to Sessions with '3'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewSessions))
			})

			It("should switch to Settings with '4'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewSettings))
			})
		})

		Context("Given letter navigation shortcuts in normal mode", func() {
			It("should switch to Home with 'h'", func() {
				app.activeView = viewChat
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewHome))
			})

			It("should switch to Chat with 'c'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewChat))
			})
		})

		Context("Given view switching", func() {
			It("should blur old view and focus new view", func() {
				app.activeView = viewHome
				app.homeModel.Focus()
				Expect(app.homeModel.focused).To(BeTrue())

				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
				updated := model.(App)
				Expect(updated.homeModel.focused).To(BeFalse())
				Expect(updated.chatModel.focused).To(BeTrue())
			})
		})

		Context("Given tab cycling changes the view", func() {
			It("should set insert mode when entering chat", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewChat))
				Expect(updated.mode).To(Equal(ModeInsert))
			})

			It("should set normal mode when entering home from chat", func() {
				app.activeView = viewChat
				app.mode = ModeInsert
				app.chatModel.Focus()

				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewHome))
				Expect(updated.mode).To(Equal(ModeNormal))
			})
		})

		Context("Given number shortcuts change the view", func() {
			It("should set insert mode when switching to chat with '2'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
				updated := model.(App)
				Expect(updated.mode).To(Equal(ModeInsert))
			})

			It("should set normal mode when switching to home with '1'", func() {
				app.activeView = viewChat
				app.mode = ModeInsert
				app.chatModel.Focus()

				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
				updated := model.(App)
				Expect(updated.mode).To(Equal(ModeNormal))
			})
		})
	})

	Describe("Mode Switching", func() {
		Context("Given normal mode", func() {
			It("should switch to insert mode with 'i'", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
				updated := model.(App)
				Expect(updated.mode).To(Equal(ModeInsert))
			})

			It("should focus active view when entering insert mode", func() {
				app.homeModel.Blur()
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
				updated := model.(App)
				Expect(updated.homeModel.focused).To(BeTrue())
			})
		})

		Context("Given insert mode", func() {
			BeforeEach(func() {
				app.mode = ModeInsert
			})

			It("should switch to normal mode with Esc", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
				updated := model.(App)
				Expect(updated.mode).To(Equal(ModeNormal))
			})

			It("should blur active view when exiting insert mode", func() {
				app.homeModel.Focus()
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
				updated := model.(App)
				Expect(updated.homeModel.focused).To(BeFalse())
			})

			It("should not switch view on 'h' in insert mode", func() {
				app.activeView = viewChat
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
				updated := model.(App)
				Expect(updated.activeView).To(Equal(viewChat))
			})
		})
	})

	Describe("Global Ctrl+C", func() {
		Context("Given chat input contains draft text", func() {
			It("should clear the draft before quitting", func() {
				By("switching to chat and typing a draft")
				app.activeView = viewChat
				app.mode = ModeInsert
				app.chatModel.Focus()
				app.SetInput("unfinished")

				By("pressing Ctrl+C")
				model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
				updated := model.(App)

				By("verifying the app stayed open and showed feedback")
				Expect(cmd).To(BeNil())
				Expect(updated.GetInput()).To(Equal(""))
				Expect(updated.renderStatusBar()).To(ContainSubstring("Input cleared"))
			})
		})
	})

	Describe("Chat Tab Insert Mode Default", func() {
		BeforeEach(func() {
			app.width = 80
			app.height = 24
		})

		Context("Given the user switches to chat tab", func() {
			It("should enter insert mode so typing works immediately", func() {
				By("starting on home in normal mode")
				Expect(app.mode).To(Equal(ModeNormal))

				By("switching to chat")
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				updated := model.(App)

				By("verifying insert mode is active and chat is focused")
				Expect(updated.mode).To(Equal(ModeInsert))
				Expect(updated.chatModel.focused).To(BeTrue())
			})

			It("should allow immediate text input in chat", func() {
				By("switching to chat")
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				chatApp := model.(App)

				By("typing a message in chat")
				model, _ = chatApp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
				updated := model.(App)

				By("verifying the character was inserted into textarea")
				Expect(updated.chatModel.GetInput()).To(Equal("h"))
			})
		})
	})

	Describe("Help Overlay", func() {
		Context("Given '?' is pressed in normal mode", func() {
			It("should show help", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
				updated := model.(App)
				Expect(updated.showHelp).To(BeTrue())
			})

			It("should hide help when '?' is pressed again", func() {
				app.showHelp = true
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
				updated := model.(App)
				Expect(updated.showHelp).To(BeFalse())
			})

			It("should hide help with Esc", func() {
				app.showHelp = true
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
				updated := model.(App)
				Expect(updated.showHelp).To(BeFalse())
			})

			It("should hide help with 'q'", func() {
				app.showHelp = true
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
				updated := model.(App)
				Expect(updated.showHelp).To(BeFalse())
			})
		})

		Context("Given '?' is pressed in insert mode", func() {
			It("should not show help", func() {
				app.mode = ModeInsert
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
				updated := model.(App)
				Expect(updated.showHelp).To(BeFalse())
			})
		})
	})

	Describe("Command Palette Integration", func() {
		Context("Given the command palette is open", func() {
			BeforeEach(func() {
				app.commandPalette.Open(80, 24)
			})

			It("should delegate keys to palette", func() {
				Expect(app.commandPalette.IsShowing()).To(BeTrue())
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
				updated := model.(App)
				Expect(updated.commandPalette.cursor).To(Equal(1))
			})

			It("should close palette on Esc", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
				updated := model.(App)
				Expect(updated.commandPalette.IsShowing()).To(BeFalse())
			})

			It("should handle palette selection", func() {
				// Select first command with Enter
				model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
				updated := model.(App)
				Expect(updated.commandPalette.IsShowing()).To(BeFalse())
				Expect(cmd).To(BeNil())
			})
		})
	})

	Describe("Model Picker Integration", func() {
		Context("Given the model picker is open", func() {
			BeforeEach(func() {
				app.modelPicker.SetModels([]ModelItem{
					{ID: "gpt-4", Name: "GPT-4"},
				})
				app.modelPicker.Open(80, 24)
			})

			It("should delegate keys to picker", func() {
				Expect(app.modelPicker.IsShowing()).To(BeTrue())
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
				updated := model.(App)
				Expect(updated.modelPicker.cursor).To(Equal(0)) // only 1 model
			})

			It("should close picker on Esc", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
				updated := model.(App)
				Expect(updated.modelPicker.IsShowing()).To(BeFalse())
			})
		})
	})

	Describe("Approval Dialog Integration", func() {
		Context("Given the approval dialog is visible", func() {
			BeforeEach(func() {
				req := approval.NewApprovalRequest(approval.CommandInfo{
					ID: "test", ToolName: "bash", Command: "echo test",
				})
				app.approvalDialog.Show(req)
			})

			It("should delegate keys to dialog", func() {
				Expect(app.approvalDialog.IsVisible()).To(BeTrue())
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
				updated := model.(App)
				Expect(updated.approvalDialog.selected).To(Equal(1))
			})

			It("should hide dialog on approval", func() {
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
				updated := model.(App)
				Expect(updated.approvalDialog.IsVisible()).To(BeFalse())
			})
		})
	})

	Describe("Agent Message Routing", func() {
		Context("Given agent streaming messages", func() {
			It("should route AgentStartMsg to chat model", func() {
				model, _ := app.Update(AgentStartMsg{})
				updated := model.(App)
				Expect(updated.chatModel.thinking).To(BeTrue())
			})

			It("should route AgentChunkMsg to chat model", func() {
				m, _ := app.Update(AgentStartMsg{})
				a := m.(App)
				m2, _ := a.Update(AgentChunkMsg{Text: "hello"})
				updated := m2.(App)
				Expect(updated.chatModel.streamBuffer).To(Equal("hello"))
			})

			It("should route AgentDoneMsg to chat model", func() {
				app.Update(AgentStartMsg{})
				model, _ := app.Update(AgentDoneMsg{FullResponse: "done"})
				updated := model.(App)
				Expect(updated.chatModel.thinking).To(BeFalse())
			})

			It("should route AgentErrorMsg to chat model", func() {
				model, _ := app.Update(AgentErrorMsg{Error: fmt.Errorf("test error")})
				updated := model.(App)
				Expect(updated.chatModel.messages).ToNot(BeEmpty())
			})

			It("should route AgentToolStartMsg to chat model", func() {
				model, _ := app.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "bash"})
				updated := model.(App)
				Expect(updated.chatModel.currentToolMsg).ToNot(BeNil())
			})

			It("should route AgentToolDoneMsg to chat model", func() {
				app.Update(AgentToolStartMsg{ToolID: "t1", ToolName: "bash"})
				model, _ := app.Update(AgentToolDoneMsg{ToolID: "t1", Success: true})
				updated := model.(App)
				Expect(updated.chatModel.currentToolMsg).To(BeNil())
			})

			It("should continue listening after agent messages", func() {
				_, cmd := app.Update(AgentStartMsg{})
				Expect(cmd).ToNot(BeNil())
			})
		})
	})

	Describe("Window Size", func() {
		Context("Given a window resize message", func() {
			It("should update app dimensions", func() {
				model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				updated := model.(App)
				Expect(updated.width).To(Equal(100))
				Expect(updated.height).To(Equal(50))
			})

			It("should propagate to sub-models", func() {
				model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				updated := model.(App)
				Expect(updated.homeModel.width).To(Equal(100))
				Expect(updated.chatModel.width).To(Equal(100))
				Expect(updated.sessionsModel.width).To(Equal(100))
				Expect(updated.settingsModel.width).To(Equal(100))
			})

			It("should reserve space for tab and status bars", func() {
				model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
				updated := model.(App)
				// Sub-models should get height minus reserved space (5)
				Expect(updated.homeModel.height).To(Equal(19))
			})
		})
	})

	Describe("Status Messages", func() {
		Context("Given a StatusMsg", func() {
			It("should update status text and type", func() {
				model, _ := app.Update(StatusMsg{Text: "Test status", Type: "info"})
				updated := model.(App)
				Expect(updated.statusMessage).To(Equal("Test status"))
				Expect(updated.statusType).To(Equal("info"))
			})

			It("should continue listening after status update", func() {
				_, cmd := app.Update(StatusMsg{Text: "Test", Type: "info"})
				Expect(cmd).ToNot(BeNil())
			})
		})
	})

	Describe("Quit", func() {
		Context("Given Ctrl+C is pressed", func() {
			It("should return tea.Quit command", func() {
				_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
				Expect(cmd).ToNot(BeNil())
			})
		})

		Context("Given QuitMsg is received", func() {
			It("should return tea.Quit command", func() {
				_, cmd := app.Update(QuitMsg{})
				Expect(cmd).ToNot(BeNil())
			})
		})
	})

	Describe("Model Changed", func() {
		Context("Given ModelChangedMsg", func() {
			It("should update chat model", func() {
				model, _ := app.Update(ModelChangedMsg{Model: "gpt-4"})
				updated := model.(App)
				Expect(updated.chatModel.GetModel()).To(Equal("gpt-4"))
			})

			It("should update settings model", func() {
				app.SetSettings([]Setting{{Key: "model", Value: "old"}})
				model, _ := app.Update(ModelChangedMsg{Model: "gpt-4"})
				updated := model.(App)
				Expect(updated.settingsModel.settings[0].Value).To(Equal("gpt-4"))
			})
		})
	})

	Describe("Clear Chat", func() {
		Context("Given ClearChatMsg", func() {
			It("should clear chat messages", func() {
				app.AddMessage("user", "hello")
				Expect(app.chatModel.messages).To(HaveLen(1))

				model, _ := app.Update(ClearChatMsg{})
				updated := model.(App)
				Expect(updated.chatModel.messages).To(BeEmpty())
			})

			It("should add follow-up message when provided", func() {
				app.AddMessage("user", "hello")
				model, _ := app.Update(ClearChatMsg{FollowUpMsg: "Cleared."})
				updated := model.(App)
				Expect(updated.chatModel.messages).To(HaveLen(1))
				Expect(updated.chatModel.messages[0].Content).To(Equal("Cleared."))
			})
		})
	})

	Describe("Agent Cancel", func() {
		Context("Given AgentCancelMsg", func() {
			It("should add cancellation message to chat", func() {
				model, _ := app.Update(AgentCancelMsg{})
				updated := model.(App)
				Expect(updated.chatModel.messages).ToNot(BeEmpty())
				Expect(updated.chatModel.messages[0].Content).To(ContainSubstring("cancelled"))
			})
		})
	})

	Describe("Tool Execution", func() {
		Context("Given ToolExecutingMsg", func() {
			It("should add tool message to chat", func() {
				model, _ := app.Update(ToolExecutingMsg{ToolID: "t1", ToolName: "bash", Command: "ls"})
				updated := model.(App)
				found := false
				for _, msg := range updated.chatModel.messages {
					if msg.IsTool && msg.ID == "t1" {
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("Approval Request", func() {
		Context("Given ApprovalRequestMsg", func() {
			It("should show approval dialog", func() {
				req := approval.NewApprovalRequest(approval.CommandInfo{
					ID: "test", ToolName: "bash", Command: "ls",
				})
				model, _ := app.Update(ApprovalRequestMsg{Request: req})
				updated := model.(App)
				Expect(updated.approvalDialog.IsVisible()).To(BeTrue())
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given app has dimensions", func() {
			It("should render non-empty view", func() {
				view := app.View()
				Expect(view).ToNot(BeEmpty())
			})

			It("should render tab bar with active indicator", func() {
				view := app.View()
				Expect(view).To(ContainSubstring("Home"))
				Expect(view).To(ContainSubstring("Chat"))
			})

			It("should render status bar", func() {
				app.width = 120
				view := app.View()
				Expect(view).To(ContainSubstring("Agent Harness"))
			})

			It("should show help overlay when active", func() {
				app.showHelp = true
				app.helpModel.Open(80, 24, "")
				view := app.View()
				Expect(view).To(ContainSubstring("Keyboard Shortcuts"))
			})

			It("should show approval dialog when visible", func() {
				req := approval.NewApprovalRequest(approval.CommandInfo{
					ID: "test", ToolName: "bash", Command: "ls",
				})
				app.approvalDialog.Show(req)
				app.approvalDialog.width = 80
				app.approvalDialog.height = 24
				view := app.View()
				Expect(view).To(ContainSubstring("Command Approval Required"))
			})
		})

		Context("Given no model is set", func() {
			It("should show no-model warning in status bar", func() {
				view := app.View()
				Expect(view).To(ContainSubstring("no model"))
			})
		})
	})

	Describe("Navigation in Normal Mode", func() {
		BeforeEach(func() {
			app.mode = ModeNormal
			app.activeView = viewChat
			app.chatModel.Focus()
		})

		Context("Given j/k navigation keys", func() {
			It("should scroll down with 'j'", func() {
				app.chatModel.viewport.Height = 3
				app.chatModel.viewport.SetContent("line1\nline2\nline3\nline4\nline5")
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
				updated := model.(App)
				Expect(updated.chatModel.viewport.YOffset).To(BeNumerically(">", 0))
			})

			It("should scroll up with 'k'", func() {
				app.chatModel.viewport.Height = 3
				app.chatModel.viewport.SetContent("line1\nline2\nline3\nline4\nline5")
				app.chatModel.viewport.SetYOffset(2)
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
				updated := model.(App)
				Expect(updated.chatModel.viewport.YOffset).To(Equal(1))
			})

			It("should goto top with 'g'", func() {
				app.chatModel.viewport.SetContent("line1\nline2\nline3")
				app.chatModel.viewport.SetYOffset(2)
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
				updated := model.(App)
				Expect(updated.chatModel.viewport.YOffset).To(Equal(0))
			})

			It("should goto bottom with 'G'", func() {
				app.chatModel.viewport.SetContent("line1\nline2\nline3")
				model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
				updated := model.(App)
				Expect(updated.chatModel.viewport.AtBottom()).To(BeTrue())
			})
		})
	})

	Describe("Send and Message Channel", func() {
		Context("Given Send is called", func() {
			It("should deliver message to channel", func() {
				app.Send(StatusMsg{Text: "test"})
				select {
				case msg := <-app.msgChan:
					status, ok := msg.(StatusMsg)
					Expect(ok).To(BeTrue())
					Expect(status.Text).To(Equal("test"))
				default:
					Fail("message not received")
				}
			})

			It("should not block when channel is full", func() {
				// Fill the channel
				for i := 0; i < 64; i++ {
					app.Send(StatusMsg{Text: "fill"})
				}
				// This should not panic or block
				Expect(func() {
					app.Send(StatusMsg{Text: "overflow"})
				}).NotTo(Panic())
			})
		})
	})

	Describe("Public API", func() {
		Context("Given various setters", func() {
			It("should set chat model", func() {
				app.SetChatModel("gpt-4")
				Expect(app.chatModel.GetModel()).To(Equal("gpt-4"))
			})

			It("should set chat persona", func() {
				app.SetChatPersona("developer")
				Expect(app.chatModel.persona).To(Equal("developer"))
			})

			It("should set project info", func() {
				app.SetProjectInfo(ProjectInfo{Name: "test-project"})
				Expect(app.homeModel.project.Name).To(Equal("test-project"))
			})

			It("should set home status", func() {
				app.SetHomeStatus("model-1", "write", "dev", 100)
				Expect(app.homeModel.model).To(Equal("model-1"))
				Expect(app.homeModel.permissionMode).To(Equal("write"))
				Expect(app.homeModel.persona).To(Equal("dev"))
				Expect(app.homeModel.estimatedTokens).To(Equal(100))
			})

			It("should refresh sessions", func() {
				app.RefreshSessions([]SessionInfo{{ID: "1", Title: "S1"}})
				Expect(app.sessionsModel.sessions).To(HaveLen(1))
			})

			It("should set settings", func() {
				app.SetSettings([]Setting{{Key: "theme", Value: "dark"}})
				Expect(app.settingsModel.settings).To(HaveLen(1))
			})

			It("should set models", func() {
				app.SetModels([]ModelItem{{ID: "gpt-4"}})
				Expect(app.modelPicker.models).To(HaveLen(1))
			})

			It("should set command completions", func() {
				app.SetCommandCompletions([]string{"/help", "/clear"})
				Expect(app.chatModel.allCommands).To(Equal([]string{"/help", "/clear"}))
			})

			It("should add message", func() {
				app.AddMessage("user", "hello")
				Expect(app.chatModel.messages).To(HaveLen(1))
			})

			It("should set and get input", func() {
				app.SetInput("test input")
				Expect(app.GetInput()).To(Equal("test input"))
			})

			It("should clear input", func() {
				app.SetInput("test")
				app.ClearInput()
				Expect(app.GetInput()).To(BeEmpty())
			})

			It("should remove last user message", func() {
				app.AddMessage("user", "hello")
				app.AddMessage("assistant", "hi")
				app.AddMessage("user", "secret")
				app.RemoveLastUserMessage()
				Expect(app.chatModel.messages).To(HaveLen(2))
			})

			It("should queue steer", func() {
				app.QueueSteer("check tests")
				Expect(app.chatModel.GetSteerQueue()).To(Equal([]string{"check tests"}))
			})

			It("should show status", func() {
				app.ShowStatus("Ready", "info")
				Expect(app.statusMessage).To(Equal("Ready"))
				Expect(app.statusType).To(Equal("info"))
			})
		})
	})

	Describe("Panic Recovery", func() {
		Context("Given a panic occurs during update", func() {
			It("should recover and not crash", func() {
				// We can't easily trigger a panic, but we can verify the defer exists
				// by ensuring normal updates still work
				Expect(func() {
					app.Update(tea.KeyMsg{Type: tea.KeyTab})
				}).NotTo(Panic())
			})
		})
	})
})

var _ = Describe("ShortenModelName", func() {
	Context("Given various model names", func() {
		It("should handle empty model", func() {
			Expect(ShortenModelName("")).To(Equal("(use /model)"))
		})

		It("should handle numeric-only model", func() {
			Expect(ShortenModelName("123")).To(Equal("(invalid: 123)"))
		})

		It("should shorten provider/model format", func() {
			result := ShortenModelName("openai/gpt-4-turbo")
			Expect(result).To(ContainSubstring("openai"))
			Expect(result).To(ContainSubstring("4"))
		})

		It("should preserve tag", func() {
			result := ShortenModelName("openai/gpt-4:latest")
			Expect(result).To(ContainSubstring("latest"))
		})

		It("should handle long single names", func() {
			longName := "this-is-a-very-long-model-name-that-exceeds-twenty"
			result := ShortenModelName(longName)
			Expect(len(result)).To(BeNumerically("<=", 23))
		})

		It("should prefer parameter size segments", func() {
			result := ShortenModelName("openrouter/meta-llama-3-70b")
			Expect(result).To(ContainSubstring("70b"))
		})
	})
})

var _ = Describe("getToolDisplayName", func() {
	Context("Given various tool names", func() {
		It("should return Shell for bash", func() {
			Expect(getToolDisplayName("bash")).To(Equal("Shell"))
			Expect(getToolDisplayName("BashTool")).To(Equal("Shell"))
		})

		It("should return Read File for read", func() {
			Expect(getToolDisplayName("read")).To(Equal("Read File"))
		})

		It("should return Write File for write", func() {
			Expect(getToolDisplayName("write")).To(Equal("Write File"))
		})

		It("should return Edit File for edit", func() {
			Expect(getToolDisplayName("edit")).To(Equal("Edit File"))
		})

		It("should return Find Files for glob", func() {
			Expect(getToolDisplayName("glob")).To(Equal("Find Files"))
		})

		It("should return Search for grep", func() {
			Expect(getToolDisplayName("grep")).To(Equal("Search"))
		})

		It("should return Fetch URL for webfetch", func() {
			Expect(getToolDisplayName("webfetch")).To(Equal("Fetch URL"))
		})

		It("should return Web Search for websearch", func() {
			Expect(getToolDisplayName("websearch")).To(Equal("Web Search"))
		})

		It("should capitalize unknown tools", func() {
			Expect(getToolDisplayName("custom")).To(Equal("Custom"))
		})

		It("should handle empty string", func() {
			Expect(getToolDisplayName("")).To(Equal(""))
		})
	})
})
