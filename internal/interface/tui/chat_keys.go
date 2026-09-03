package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// resize applies a terminal resize to the chat layout.
func (m *ChatModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.syncTextareaHeight()

	headerHeight := 2
	separatorHeight := 1
	inputHeight := m.inputAreaHeight()
	vpHeight := height - inputHeight - headerHeight - separatorHeight
	if vpHeight < 5 {
		vpHeight = 5
	}

	m.viewport.Width = width
	m.viewport.Height = vpHeight
	columnWidth := width
	textareaWidth := columnWidth - 8
	if textareaWidth < 20 {
		textareaWidth = 20
	}
	m.textarea.SetWidth(textareaWidth)

	m.refreshViewport()
}

// handleKeys processes a key message while the chat is focused.
// It returns the (possibly mutated) model, the accumulated command, and
// whether the key was fully handled (the caller must not continue normal
// message processing when true).
func (m ChatModel) handleKeys(msg tea.KeyMsg) (ChatModel, tea.Cmd, bool) {
	if !m.focused {
		return m, nil, true
	}

	// Detect bracketed paste from terminal
	if msg.Paste {
		m.pasteDetected = true
	}

	// Inline suggestion navigation
	if m.showSuggestions {
		switch msg.String() {
		case "down", "j":
			if m.suggestionCursor < len(m.suggestions)-1 {
				m.suggestionCursor++
				m.syncSuggestionOffset()
			}
			return m, nil, true
		case "up", "k":
			if m.suggestionCursor > 0 {
				m.suggestionCursor--
				m.syncSuggestionOffset()
			}
			return m, nil, true
		case "enter":
			// An exact match means the user typed the whole command:
			// submit it instead of completing to the same string (which
			// used to swallow the Enter and leave the command unrun).
			if len(m.suggestions) > 0 && m.suggestionCursor < len(m.suggestions) &&
				m.suggestions[m.suggestionCursor] != m.textarea.Value() {
				m.textarea.SetValue(m.suggestions[m.suggestionCursor] + " ")
				m.syncTextareaHeight()
				m.showSuggestions = false
				return m, nil, true
			}
			// Partial completion or empty list: fall through so Enter
			// submits normally.
		case "tab":
			if len(m.suggestions) > 0 {
				m.textarea.SetValue(m.suggestions[0] + " ")
				m.syncTextareaHeight()
				m.showSuggestions = false
				return m, nil, true
			}
		case "esc":
			m.showSuggestions = false
			return m, nil, true
		case "ctrl+c":
			m.showSuggestions = false
			return m, nil, true
		}
	}

	// Trigger inline suggestions when "/" is typed in empty input
	if msg.String() == "/" && m.textarea.Value() == "" {
		m.showSuggestions = true
		m.suggestions = m.filterSuggestions("/")
		m.suggestionCursor = 0
		m.textarea.InsertString("/")
		m.syncTextareaHeight()
		return m, nil, true
	}

	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt {
			// Multi-line input
			m.textarea.InsertString("\n")
			m.syncTextareaHeight()
			return m, nil, true
		}

		m.showSuggestions = false

		input := m.textarea.Value()
		if input == "" {
			return m, nil, true
		}

		// If a submit is already pending, this Enter is either a pasted
		// newline (machine-speed burst) or a human double-tap (submit
		// now). The burst threshold tells them apart.
		if m.pendingSubmit {
			if time.Since(m.pendingAt) < PasteBurstThreshold {
				m.pasteDetected = true
				m.textarea.InsertString("\n")
				m.syncTextareaHeight()
				return m, m.startSubmitTimer(), true
			}
			m.pendingSubmit = false
			mm, cmd := m.doSubmit()
			return mm, cmd, true
		}

		// Debounce: start submit timer. Keys arriving inside the window
		// are classified by burst speed (see PasteBurstThreshold).
		if SubmitDebounceDuration <= 0 {
			mm, cmd := m.doSubmit()
			return mm, cmd, true
		}
		m.pendingSubmit = true
		m.pendingAt = time.Now()
		return m, m.startSubmitTimer(), true

	case tea.KeyCtrlC:
		if m.textarea.Value() != "" {
			m.textarea.SetValue("")
			m.pasteDetected = false
			m.pendingSubmit = false
			m.pendingSubmitGen++
			m.syncTextareaHeight()
		}
		return m, nil, true

	case tea.KeyCtrlJ:
		// Treat Ctrl+J (line feed) as newline insertion.
		// This preserves pasted newlines from terminals that send
		// raw LF instead of bracketed paste events.
		if m.pendingSubmit {
			m.pendingSubmit = false
			m.pendingSubmitGen++
		}
		m.textarea.InsertString("\n")
		m.syncTextareaHeight()
		return m, nil, true
	}

	// If another key arrives while a submit is pending, decide by what
	// the key is and how fast it arrived:
	//   - printable keystroke from a machine-speed burst: paste
	//     continuation — the Enter was a pasted newline; cancel the
	//     submit and keep the paste intact.
	//   - printable keystroke from a human (slower than the burst
	//     threshold): a fast typist starting their next message. Enter's
	//     contract (submit) wins — flush the pending submit now and let
	//     the keystroke flow into the fresh composer below. The old
	//     behavior cancelled the submit and inserted a phantom newline,
	//     eating the keystroke and silently dropping the message.
	//   - anything else (Esc, Backspace, Ctrl+J): cancel the pending
	//     submit; the key then does its normal work.
	var pendingCmd tea.Cmd
	if m.pendingSubmit {
		pasteBurst := time.Since(m.pendingAt) < PasteBurstThreshold
		switch {
		case msg.Paste || (pasteBurst && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace)):
			m.pendingSubmit = false
			m.pendingSubmitGen++
			m.textarea.InsertString("\n")
			m.pasteDetected = true
			m.syncTextareaHeight()
		case msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace:
			mm, submitCmd := m.doSubmit()
			m = mm
			m.pendingSubmit = false
			pendingCmd = submitCmd
		default:
			m.pendingSubmit = false
			m.pendingSubmitGen++
		}
	}

	// Update textarea
	lastLen := len(m.textarea.Value())
	var newTA textarea.Model
	var cmd tea.Cmd
	func() {
		defer func() {
			if r := recover(); r != nil {
				diag.Panic("tui.textarea", r)
				fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] textarea.Update: %v\n", r)
			}
		}()
		newTA, cmd = m.textarea.Update(msg)
	}()
	m.textarea = newTA
	m.syncTextareaHeight()

	// Heuristic paste detection for terminals without bracketed paste
	if !msg.Paste && len(m.textarea.Value())-lastLen > PasteHeuristicThreshold {
		m.pasteDetected = true
	}
	// Reset paste flag if input was cleared
	if len(m.textarea.Value()) == 0 {
		m.pasteDetected = false
	}

	// Refresh suggestions if showing
	if m.showSuggestions {
		val := m.textarea.Value()
		if !strings.HasPrefix(val, "/") || strings.Contains(val, " ") {
			m.showSuggestions = false
		} else {
			m.suggestions = m.filterSuggestions(val)
			m.suggestionCursor = 0
		}
	}

	if pendingCmd != nil {
		if cmd != nil {
			return m, tea.Batch(pendingCmd, cmd), false
		}
		return m, pendingCmd, false
	}
	return m, cmd, false
}
