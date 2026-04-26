package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testSessionsDelegate struct {
	selectedSession string
	deletedSession  string
	exportedSession string
	copiedSession   string
	loaded          bool
}

func (d *testSessionsDelegate) OnSessionSelect(id string) { d.selectedSession = id }
func (d *testSessionsDelegate) OnSessionDelete(id string) { d.deletedSession = id }
func (d *testSessionsDelegate) OnSessionExport(id string) { d.exportedSession = id }
func (d *testSessionsDelegate) OnSessionCopy(id string)   { d.copiedSession = id }
func (d *testSessionsDelegate) OnSessionLoad()            { d.loaded = true }

var _ = Describe("SessionsModel", func() {
	var sessions SessionsModel
	var delegate *testSessionsDelegate

	BeforeEach(func() {
		sessions = NewSessionsModel()
		delegate = &testSessionsDelegate{}
		sessions.SetDelegate(delegate)
		m, _ := sessions.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		sessions = m.(SessionsModel)
		sessions.Focus()
	})

	Describe("Initialization and Loading", func() {
		Context("Given no sessions", func() {
			It("should render an empty state", func() {
				Expect(sessions.View()).To(ContainSubstring("No Sessions"))
			})

			It("should not panic when pressing keys on empty list", func() {
				Expect(func() {
					sessions.Update(tea.KeyMsg{Type: tea.KeyEnter})
					sessions.Update(tea.KeyMsg{Type: tea.KeyUp})
					sessions.Update(tea.KeyMsg{Type: tea.KeyDown})
					sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
					sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
					sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				}).NotTo(Panic())
			})
		})

		Context("Given Init is called", func() {
			It("should notify delegate to load sessions", func() {
				cmd := sessions.Init()
				msg := cmd()
				Expect(msg).To(BeAssignableToTypeOf(SessionsLoadedMsg{}))
				Expect(delegate.loaded).To(BeTrue())
			})
		})
	})

	Describe("Interaction and Edge Cases", func() {
		Context("Given a list of sessions", func() {
			BeforeEach(func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "1", Title: "S1", MessageCount: 2, Turns: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
					{ID: "2", Title: "S2", MessageCount: 4, Turns: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				})
			})

			It("should allow navigation and action dispatch", func() {
				By("pressing down to second session")
				m, _ := sessions.Update(tea.KeyMsg{Type: tea.KeyDown})
				sessions = m.(SessionsModel)
				Expect(sessions.cursor).To(Equal(1))

				By("pressing enter to select")
				sessions.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.selectedSession).To(Equal("2"))

				By("pressing d to delete")
				sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
				Expect(delegate.deletedSession).To(Equal("2"))

				By("pressing e to export")
				sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
				Expect(delegate.exportedSession).To(Equal("2"))

				By("pressing c to copy")
				sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
				Expect(delegate.copiedSession).To(Equal("2"))
			})

			It("should not navigate past bounds", func() {
				for i := 0; i < 5; i++ {
					m, _ := sessions.Update(tea.KeyMsg{Type: tea.KeyDown})
					sessions = m.(SessionsModel)
				}
				Expect(sessions.cursor).To(Equal(1))

				for i := 0; i < 5; i++ {
					m, _ := sessions.Update(tea.KeyMsg{Type: tea.KeyUp})
					sessions = m.(SessionsModel)
				}
				Expect(sessions.cursor).To(Equal(0))
			})

			It("should update cursor if sessions are reduced", func() {
				m, _ := sessions.Update(tea.KeyMsg{Type: tea.KeyDown})
				sessions = m.(SessionsModel)
				Expect(sessions.cursor).To(Equal(1))

				sessions.SetSessions([]SessionInfo{
					{ID: "3", Title: "S3"},
				})
				Expect(sessions.cursor).To(Equal(0))
			})

			It("should trigger refresh with 'r' key", func() {
				sessions.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
				Expect(delegate.loaded).To(BeTrue())
			})
		})
	})

	Describe("Focus and Blur", func() {
		Context("Given the view is blurred", func() {
			BeforeEach(func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "1", Title: "S1"},
				})
				sessions.Blur()
			})

			It("should not dispatch actions on key press", func() {
				sessions.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.selectedSession).To(Equal(""))
			})

			It("should still allow viewport scrolling in background", func() {
				// Viewport scroll messages should still be processed
				Expect(func() {
					sessions.Update(tea.KeyMsg{Type: tea.KeyDown})
				}).NotTo(Panic())
			})
		})

		Context("Given the view is focused after blur", func() {
			It("should dispatch actions again", func() {
				sessions.SetSessions([]SessionInfo{{ID: "1", Title: "S1"}})
				sessions.Blur()
				sessions.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.selectedSession).To(Equal(""))

				sessions.Focus()
				sessions.Update(tea.KeyMsg{Type: tea.KeyEnter})
				Expect(delegate.selectedSession).To(Equal("1"))
			})
		})
	})

	Describe("Window Size", func() {
		Context("Given the window is resized", func() {
			It("should update viewport dimensions", func() {
				m, _ := sessions.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
				sessions = m.(SessionsModel)
				Expect(sessions.width).To(Equal(100))
				Expect(sessions.height).To(Equal(50))
				Expect(sessions.viewport.Width).To(Equal(100))
			})

			It("should ensure minimum viewport height", func() {
				m, _ := sessions.Update(tea.WindowSizeMsg{Width: 80, Height: 4})
				sessions = m.(SessionsModel)
				Expect(sessions.viewport.Height).To(BeNumerically(">=", 5))
			})
		})
	})

	Describe("Scroll Helpers", func() {
		Context("Given a list of sessions", func() {
			BeforeEach(func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "1", Title: "S1"},
					{ID: "2", Title: "S2"},
					{ID: "3", Title: "S3"},
					{ID: "4", Title: "S4"},
					{ID: "5", Title: "S5"},
				})
			})

			It("should scroll down multiple lines", func() {
				sessions.Scroll(2)
				Expect(sessions.cursor).To(Equal(2))
			})

			It("should scroll up multiple lines", func() {
				sessions.cursor = 4
				sessions.Scroll(-2)
				Expect(sessions.cursor).To(Equal(2))
			})

			It("should clamp scroll to bounds", func() {
				sessions.Scroll(100)
				Expect(sessions.cursor).To(Equal(4))

				sessions.Scroll(-100)
				Expect(sessions.cursor).To(Equal(0))
			})

			It("should goto top", func() {
				sessions.cursor = 3
				sessions.GotoTop()
				Expect(sessions.cursor).To(Equal(0))
			})

			It("should goto bottom", func() {
				sessions.GotoBottom()
				Expect(sessions.cursor).To(Equal(4))
			})
		})
	})

	Describe("View Rendering", func() {
		Context("Given sessions with various states", func() {
			It("should render active session with indicator", func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "active-session-123", Title: "Active", IsActive: true, MessageCount: 5, Turns: 2},
					{ID: "other-session-456", Title: "Other", IsActive: false, MessageCount: 1, Turns: 1},
				})
				view := sessions.View()
				Expect(view).To(ContainSubstring("Active"))
				Expect(view).To(ContainSubstring("Other"))
			})

			It("should render session detail panel", func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "detail-test-789", Title: "Detail Test", Model: "gpt-4", MessageCount: 10, Turns: 5, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				})
				view := sessions.View()
				Expect(view).To(ContainSubstring("Session Details"))
				Expect(view).To(ContainSubstring("Detail Test"))
				Expect(view).To(ContainSubstring("gpt-4"))
			})

			It("should not panic with short session ID", func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "short", Title: "Short ID"},
				})
				Expect(func() {
					_ = sessions.View()
				}).NotTo(Panic())
			})

			It("should render untitled for empty title", func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "no-title-abc", Title: ""},
				})
				view := sessions.View()
				Expect(view).To(ContainSubstring("(untitled)"))
			})

			It("should render session count in header", func() {
				sessions.SetSessions([]SessionInfo{
					{ID: "1", Title: "S1"},
					{ID: "2", Title: "S2"},
					{ID: "3", Title: "S3"},
				})
				view := sessions.View()
				Expect(view).To(ContainSubstring("(3)"))
			})
		})
	})

	Describe("Session Item Rendering", func() {
		Context("Given long session titles", func() {
			It("should truncate title to fit width", func() {
				longTitle := "This is a very long session title that should be truncated"
				sessions.SetSessions([]SessionInfo{
					{ID: "1", Title: longTitle},
				})
				view := sessions.View()
				Expect(view).To(ContainSubstring("..."))
			})
		})
	})
})
