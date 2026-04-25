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
		})
	})
})
