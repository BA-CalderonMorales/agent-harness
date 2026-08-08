package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
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
	if columnWidth > ComposerColumnWidth {
		columnWidth = ComposerColumnWidth
	}
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
			if len(m.suggestions) > 0 && m.suggestionCursor < len(m.suggestions) {
				m.textarea.SetValue(m.suggestions[m.suggestionCursor] + " ")
				m.syncTextareaHeight()
				m.showSuggestions = false
				return m, nil, true
			}
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

		// If a submit is already pending, this Enter is part of a paste stream.
		if m.pendingSubmit {
			m.pasteDetected = true
			m.textarea.InsertString("\n")
			m.syncTextareaHeight()
			return m, m.startSubmitTimer(), true
		}

		// Debounce: start submit timer. If another key arrives before the
		// timer fires, the Enter is treated as a pasted newline.
		if SubmitDebounceDuration <= 0 {
			mm, cmd := m.doSubmit()
			return mm, cmd, true
		}
		m.pendingSubmit = true
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

	// If another key arrives while a submit is pending, the previous
	// Enter was part of a paste stream — cancel the submit and insert
	// the newline that Enter would have represented.
	// Only do this for character keys (runes/space); control keys like
	// Backspace or Escape should simply cancel the pending submit.
	if m.pendingSubmit {
		m.pendingSubmit = false
		m.pendingSubmitGen++
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.textarea.InsertString("\n")
			m.pasteDetected = true
			m.syncTextareaHeight()
		}
	}

	// Update textarea
	lastLen := len(m.textarea.Value())
	var newTA textarea.Model
	var cmd tea.Cmd
	func() {
		defer func() {
			if r := recover(); r != nil {
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

	return m, cmd, false
}
