package tui

import (

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testHomeDelegate struct {
	newChatCalled      bool
	runTestsCalled     bool
	exportSessionCalled bool
	switchPersonaCalled bool
}

func (d *testHomeDelegate) OnNewChat()      { d.newChatCalled = true }
func (d *testHomeDelegate) OnRunTests()     { d.runTestsCalled = true }
func (d *testHomeDelegate) OnExportSession() { d.exportSessionCalled = true }
func (d *testHomeDelegate) OnSwitchPersona() { d.switchPersonaCalled = true }
func (d *testHomeDelegate) OnLoadSession(id string) {}


var _ = Describe("HomeModel", func() {
	var home HomeModel
	var delegate *testHomeDelegate

	BeforeEach(func() {
		home = NewHomeModel()
		delegate = &testHomeDelegate{}
		home.SetDelegate(delegate)
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

	Describe("Project Info Rendering", func() {
		BeforeEach(func() {
			m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			home = *m.(*HomeModel)
		})

		Context("Given project info is set", func() {
			It("should render project name and type", func() {
				home.SetProjectInfo(ProjectInfo{Name: "agent-harness", Type: "Go"})
				view := home.View()
				Expect(view).To(ContainSubstring("agent-harness"))
				Expect(view).To(ContainSubstring("Go"))
			})

			It("should render git branch and commit", func() {
				home.SetProjectInfo(ProjectInfo{
					Name:      "agent-harness",
					GitBranch: "main",
					GitCommit: "abc1234def",
					HasChanges: false,
				})
				view := home.View()
				Expect(view).To(ContainSubstring("main"))
				Expect(view).To(ContainSubstring("abc1234"))
			})

			It("should render uncommitted changes count", func() {
				home.SetProjectInfo(ProjectInfo{
					GitBranch:        "develop",
					HasChanges:       true,
					UncommittedCount: 3,
				})
				view := home.View()
				Expect(view).To(ContainSubstring("(3 uncommitted)"))
			})

			It("should render last commit message truncated", func() {
				home.SetProjectInfo(ProjectInfo{
					GitBranch:     "main",
					LastCommitMsg: "This is a very long commit message that should be truncated for display",
				})
				view := home.View()
				Expect(view).To(ContainSubstring("Last commit"))
			})
		})

		Context("Given contextual hints based on project type", func() {
			It("should show Go hint for Go projects", func() {
				home.SetProjectInfo(ProjectInfo{Type: "Go"})
				view := home.View()
				Expect(view).To(ContainSubstring("go test"))
			})

			It("should show Node hint for Node projects", func() {
				home.SetProjectInfo(ProjectInfo{Type: "Node"})
				view := home.View()
				Expect(view).To(ContainSubstring("npm test"))
			})

			It("should show Python hint for Python projects", func() {
				home.SetProjectInfo(ProjectInfo{Type: "Python"})
				view := home.View()
				Expect(view).To(ContainSubstring("pytest"))
			})

			It("should show Rust hint for Rust projects", func() {
				home.SetProjectInfo(ProjectInfo{Type: "Rust"})
				view := home.View()
				Expect(view).To(ContainSubstring("cargo test"))
			})

			It("should show no hint for unknown project types", func() {
				home.SetProjectInfo(ProjectInfo{Type: "Fortran"})
				view := home.View()
				Expect(view).ToNot(ContainSubstring("Tip:"))
			})
		})
	})

	Describe("Quick Actions", func() {
		BeforeEach(func() {
			m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			home = *m.(*HomeModel)
			home.Init()
			home.Focus()
		})

		Context("Given the home view is focused", func() {
			It("should dispatch new chat on 'n' key", func() {
				home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
				Expect(delegate.newChatCalled).To(BeTrue())
			})

			It("should dispatch run tests on 't' key", func() {
				home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
				Expect(delegate.runTestsCalled).To(BeTrue())
			})

			It("should dispatch export session on 'e' key", func() {
				home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
				Expect(delegate.exportSessionCalled).To(BeTrue())
			})

			It("should dispatch switch persona on 'p' key", func() {
				home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
				Expect(delegate.switchPersonaCalled).To(BeTrue())
			})

			It("should dispatch action on Enter when cursor is on it", func() {
				By("moving to 'Run tests' action")
				home.Update(tea.KeyMsg{Type: tea.KeyDown})
				Expect(home.actionCursor).To(Equal(1))

				By("pressing Enter")
				home.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.runTestsCalled).To(BeTrue())
			})
		})
	})

	Describe("Recent Sessions", func() {
		BeforeEach(func() {
			m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			home = *m.(*HomeModel)
		})

		Context("Given recent sessions exist", func() {
			It("should render up to 3 recent sessions", func() {
				home.SetSessions([]SessionInfo{
					{ID: "1", Title: "First", MessageCount: 2, Turns: 1},
					{ID: "2", Title: "Second", MessageCount: 4, Turns: 2},
					{ID: "3", Title: "Third", MessageCount: 6, Turns: 3},
					{ID: "4", Title: "Fourth", MessageCount: 8, Turns: 4},
				})
				view := home.View()
				Expect(view).To(ContainSubstring("First"))
				Expect(view).To(ContainSubstring("Second"))
				Expect(view).To(ContainSubstring("Third"))
				Expect(view).ToNot(ContainSubstring("Fourth"))
			})

			It("should render active session with indicator", func() {
				home.SetSessions([]SessionInfo{
					{ID: "1", Title: "Active", IsActive: true},
				})
				view := home.View()
				Expect(view).To(ContainSubstring("Active"))
			})
		})
	})

	Describe("Status Footer", func() {
		BeforeEach(func() {
			m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			home = *m.(*HomeModel)
		})

		Context("Given various status combinations", func() {
			It("should render model name", func() {
				home.SetStatus("gpt-4", "", "", 0)
				view := home.View()
				Expect(view).To(ContainSubstring("model:"))
				Expect(view).To(ContainSubstring("gpt-4"))
			})

			It("should render persona", func() {
				home.SetStatus("", "", "developer", 0)
				view := home.View()
				Expect(view).To(ContainSubstring("persona: developer"))
			})

			It("should render permission mode", func() {
				home.SetStatus("", "workspace-write", "", 0)
				view := home.View()
				Expect(view).To(ContainSubstring("perms: workspace-write"))
			})

			It("should render estimated tokens", func() {
				home.SetStatus("", "", "", 1500)
				view := home.View()
				Expect(view).To(ContainSubstring("tokens: ~1500"))
			})

			It("should render nothing when all status is empty", func() {
				home.SetStatus("", "", "", 0)
				view := home.View()
				Expect(view).ToNot(ContainSubstring("model:"))
			})
		})
	})

	Describe("Interaction Edge Cases", func() {
		Context("Given actions are not yet rebuilt", func() {
			It("should not panic when pressing Enter", func() {
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
				home.Init()
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

	Describe("Scroll Helpers", func() {
		BeforeEach(func() {
			m, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			home = *m.(*HomeModel)
			home.Init()
		})

		Context("Given action list exists", func() {
			It("should scroll down", func() {
				home.Scroll(1)
				Expect(home.actionCursor).To(Equal(1))
			})

			It("should scroll up", func() {
				home.Scroll(1)
				home.Scroll(-1)
				Expect(home.actionCursor).To(Equal(0))
			})

			It("should goto top", func() {
				home.Scroll(2)
				home.GotoTop()
				Expect(home.actionCursor).To(Equal(0))
			})

			It("should goto bottom", func() {
				home.GotoBottom()
				Expect(home.actionCursor).To(Equal(len(home.actions) - 1))
			})

			It("should clamp scroll to bounds", func() {
				home.Scroll(100)
				Expect(home.actionCursor).To(Equal(len(home.actions) - 1))
				home.Scroll(-100)
				Expect(home.actionCursor).To(Equal(0))
			})
		})
	})
})
