package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// View renders the chat.
func (m *ChatModel) SetThinking(thinking bool, text string) {
	m.thinking = thinking
	m.thinkingText = text
	if text == "" {
		m.thinkingText = "Thinking..."
	}

	// Start/stop timer based on thinking state
	if thinking {
		m.startTime = time.Now()
		m.timerRunning = true
		m.elapsed = 0
		m.chunkCount = 0
		m.syncTextareaHeight()
	} else {
		m.timerRunning = false
	}
}

// SetThinkingText updates the live reasoning preview without resetting
// the thinking timer. No repaint here: reasoning deltas can arrive
// dozens of times a second, and the streaming repaints batch onto the
// turn timer's tick (timerTickMsg) — one full-transcript render 4x a
// second instead of one per delta. Real deltas clear the status
// sentinel: what follows is model reasoning, not loop status.
func (m *ChatModel) SetThinkingText(text string) {
	m.thinkingText = text
	m.thinkingIsStatus = false
}

// startTimer returns a command that ticks every 100ms to update elapsed time
func (m *ChatModel) startTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return timerTickMsg{time: t}
	})
}

// timerTickMsg is sent on each timer tick
type timerTickMsg struct {
	time time.Time
}

// recordHistory appends a submission to the input history and resets
// navigation state (the next ↑ starts from the newest entry). Duplicate
// consecutive entries collapse — pressing ↑ twice through identical
// re-prompts reads as noise otherwise.
func (m *ChatModel) recordHistory(entry string) {
	if entry == "" {
		return
	}
	m.historyNavigating = false
	m.historyCursor = 0
	m.historyDraft = ""
	if n := len(m.inputHistory); n > 0 && m.inputHistory[n-1] == entry {
		return
	}
	m.inputHistory = append(m.inputHistory, entry)
	// Bound the history: a marathon session must not grow without
	// limit (the harness has an OOM history). 500 entries of bounded
	// composer text is ample recall depth.
	const inputHistoryLimit = 500
	if len(m.inputHistory) > inputHistoryLimit {
		m.inputHistory = m.inputHistory[len(m.inputHistory)-inputHistoryLimit:]
	}
}

// doSubmit performs the actual message submission after the debounce window.
func (m ChatModel) doSubmit() (model ChatModel, cmd tea.Cmd) {
	// Defensive: recover from any panic during submission to prevent the
	// entire TUI from crashing.
	defer func() {
		if r := recover(); r != nil {
			diag.Panic("tui.chat_submit", r)
			model = m
			cmd = nil
			m.pasteDetected = false
			m.textarea.SetValue("")
		}
	}()

	input := m.textarea.Value()
	if input == "" {
		model = m
		return
	}

	// Collapsed pastes expand back to their full content here, after
	// display formatting: the transcript shows the collapsed form, the
	// model receives the material.
	input = m.expandPasteTokens(input)
	m.clearPendingPastes()

	// Handle slash commands
	trimmed := strings.TrimSpace(input)
	m.recordHistory(trimmed)
	if strings.HasPrefix(trimmed, "/") {
		m.AddMessage("user", trimmed)
		if m.delegate != nil {
			m.delegate.OnCommand(trimmed)
		}
		m.pasteDetected = false
		m.textarea.SetValue("")
		m.syncTextareaHeight()
		m.refreshViewportFollow()
		model = m
		cmd = func() tea.Msg {
			return UserCommandMsg{Command: trimmed}
		}
		return
	} else {
		// Regular message: pastes keep a bounded preview in the
		// transcript — the head identifies the content, the marker is
		// honest about the remainder.
		displayText := input
		if m.pasteDetected {
			displayText = pastePreview(input)
		}
		m.AddMessage("user", displayText)
		if m.delegate != nil {
			cmd = m.delegate.OnSubmit(input)
			m.textarea.SetValue("")
			m.syncTextareaHeight()
			m.pasteDetected = false
			m.refreshViewportFollow()
			model = m
			return
		}
	}

	m.pasteDetected = false
	m.textarea.SetValue("")
	m.clearPendingPastes()
	m.syncTextareaHeight()
	m.refreshViewportFollow()
	model = m
	return
}

// startSubmitTimer returns a command that fires after SubmitDebounceDuration.
func (m *ChatModel) startSubmitTimer() tea.Cmd {
	m.pendingSubmitGen++
	gen := m.pendingSubmitGen
	return tea.Tick(SubmitDebounceDuration, func(t time.Time) tea.Msg {
		return submitTimerMsg{generation: gen}
	})
}

// renderStatusLine renders the thinking/streaming status with timer
