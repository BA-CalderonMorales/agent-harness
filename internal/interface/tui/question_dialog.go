package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// QuestionResponse is what the host sends back to a blocked tool call.
type QuestionResponse struct {
	Answer    string
	Cancelled bool
}

// QuestionRequest is a pending ask_user_question call. It mirrors
// ApprovalRequest: the agent goroutine waits on Response while the TUI
// owns rendering and input, so the terminal is never read behind Bubble
// Tea's back.
type QuestionRequest struct {
	Question string
	Options  []string
	Response chan QuestionResponse
	Context  context.Context

	finished   chan struct{}
	finishOnce sync.Once
}

// NewQuestionRequest creates a request bound to the turn's context. A
// cancelled turn resolves the request as cancelled rather than leaving
// the agent goroutine blocked forever.
func NewQuestionRequest(parent context.Context, question string, options []string) *QuestionRequest {
	ctx := context.Background()
	bound := false
	if parent != nil {
		ctx = parent
		bound = true
	}
	req := &QuestionRequest{
		Question: question,
		Options:  options,
		Response: make(chan QuestionResponse, 1),
		Context:  ctx,
		finished: make(chan struct{}),
	}
	if bound {
		go func() {
			select {
			case <-ctx.Done():
				req.Cancel()
			case <-req.finished:
			}
		}()
	}
	return req
}

// Respond delivers an answer and releases the waiting tool call.
func (r *QuestionRequest) Respond(answer string) {
	if r.Context != nil && r.Context.Err() != nil {
		r.Cancel()
		return
	}
	select {
	case r.Response <- QuestionResponse{Answer: answer}:
	default:
	}
	r.finishOnce.Do(func() { close(r.finished) })
}

// Cancel resolves the request without an answer (Esc, or turn abort).
func (r *QuestionRequest) Cancel() {
	select {
	case r.Response <- QuestionResponse{Cancelled: true}:
	default:
	}
	r.finishOnce.Do(func() { close(r.finished) })
}

// QuestionDialogModel is the modal that collects the answer.
type QuestionDialogModel struct {
	width    int
	height   int
	visible  bool
	request  *QuestionRequest
	selected int
	input    []rune
	notice   string
}

// NewQuestionDialog creates an empty question dialog.
func NewQuestionDialog() QuestionDialogModel {
	return QuestionDialogModel{}
}

// Show opens the dialog for one request, resetting any previous state.
func (m *QuestionDialogModel) Show(req *QuestionRequest) {
	m.visible = true
	m.request = req
	m.selected = 0
	m.input = nil
	m.notice = ""
}

// IsVisible reports whether the dialog owns the keyboard.
func (m QuestionDialogModel) IsVisible() bool {
	return m.visible && m.request != nil
}

// Update handles one key. It is only called while the dialog is visible.
func (m QuestionDialogModel) Update(msg tea.KeyMsg) (QuestionDialogModel, tea.Cmd) {
	if m.request == nil {
		m.visible = false
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.cancel()
		return m, nil

	case tea.KeyEnter:
		m.submit()
		return m, nil

	case tea.KeyUp, tea.KeyShiftTab:
		m.move(-1)
		return m, nil

	case tea.KeyDown, tea.KeyTab:
		m.move(1)
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		m.notice = ""
		return m, nil

	case tea.KeyCtrlU:
		m.input = nil
		m.notice = ""
		return m, nil

	case tea.KeyCtrlW:
		m.input = trimLastWord(m.input)
		m.notice = ""
		return m, nil

	case tea.KeySpace:
		m.input = append(m.input, ' ')
		m.notice = ""
		return m, nil

	case tea.KeyRunes:
		// A bare digit picks a numbered option before any typing has
		// started, matching the approval dialog's instant keys. Once the
		// user types, digits are literal answer text.
		if len(m.input) == 0 && len(msg.Runes) == 1 && len(m.request.Options) > 0 {
			if d := msg.Runes[0]; d >= '1' && d <= '9' {
				if idx := int(d - '1'); idx < len(m.request.Options) {
					m.selected = idx
					m.submitOption()
					return m, nil
				}
			}
		}
		m.input = append(m.input, msg.Runes...)
		m.notice = ""
		return m, nil
	}

	return m, nil
}

func (m *QuestionDialogModel) move(delta int) {
	if len(m.request.Options) == 0 {
		return
	}
	m.selected = (m.selected + delta + len(m.request.Options)) % len(m.request.Options)
}

func (m *QuestionDialogModel) submit() {
	if answer := strings.TrimSpace(string(m.input)); answer != "" {
		m.respond(answer)
		return
	}
	if len(m.request.Options) > 0 {
		m.submitOption()
		return
	}
	m.notice = "Type an answer, or press Esc to skip."
}

func (m *QuestionDialogModel) submitOption() {
	answer := strings.TrimSpace(m.request.Options[m.selected])
	if answer == "" {
		m.notice = "That option is empty; type an answer instead."
		return
	}
	m.respond(answer)
}

func (m *QuestionDialogModel) respond(answer string) {
	req := m.request
	m.visible = false
	m.request = nil
	m.input = nil
	m.notice = ""
	if req != nil {
		req.Respond(answer)
	}
}

// CancelPending resolves a visible question as skipped and closes it.
// The turn-abort path calls this so a cancelled agent does not leave a
// dead modal owning the keyboard.
func (m *QuestionDialogModel) CancelPending() {
	if m.request == nil {
		m.visible = false
		return
	}
	m.cancel()
}

func (m *QuestionDialogModel) cancel() {
	req := m.request
	m.visible = false
	m.request = nil
	m.input = nil
	m.notice = ""
	if req != nil {
		req.Cancel()
	}
}

func trimLastWord(input []rune) []rune {
	i := len(input)
	for i > 0 && input[i-1] == ' ' {
		i--
	}
	for i > 0 && input[i-1] != ' ' {
		i--
	}
	return input[:i]
}

// dialogWidth resolves the panel width from the shared modal policy.
func (m QuestionDialogModel) dialogWidth() int {
	return panelWidth(modalMaxWidth, m.width)
}

func (m QuestionDialogModel) modalSpec() modalSpec {
	inner := modalInnerWidth(m.dialogWidth())
	if m.request == nil {
		return modalSpec{title: "Question", preferredWidth: modalMaxWidth}
	}

	items := []modalItem{
		{lines: wrapLines(inner-2, PromptStyle.Render("? ")+m.request.Question)},
	}

	for i, option := range m.request.Options {
		marker := "  "
		row := fmt.Sprintf("%s%d) %s", marker, i+1, option)
		if i == m.selected && len(m.input) == 0 {
			row = ModePromptStyle.Render(fmt.Sprintf("▸ %d) %s", i+1, option))
		}
		items = append(items, modalItem{lines: wrapLines(inner, row)})
	}

	typed := string(m.input)
	inputRow := ListSelectedStyle.Render("> " + typed + "▏")
	items = append(items, modalItem{lines: wrapLines(inner, inputRow)})

	if m.notice != "" {
		items = append(items, modalItem{lines: wrapLines(inner, HelpDimStyle.Render(m.notice))})
	}

	cursor := len(items) - 1
	if strings.TrimSpace(typed) == "" && len(m.request.Options) > 0 {
		cursor = 1 + m.selected
	}

	return modalSpec{
		title:          "Question from the agent",
		footer:         "1-9 pick · ↑/↓ select · enter send · esc skip",
		items:          items,
		preferredWidth: modalMaxWidth,
		cursor:         cursor,
	}
}

// View renders the modal, or nothing when no question is pending.
func (m QuestionDialogModel) View() string {
	if !m.IsVisible() {
		return ""
	}
	return renderModal(m.width, m.height, m.modalSpec())
}
