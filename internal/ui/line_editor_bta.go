package ui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
	"os"
	"strings"
)

// Init implements tea.Model
func (le LineEditor) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.EnterAltScreen,
	)
}

// Update implements tea.Model
func (le LineEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		le.width = msg.Width
		le.height = msg.Height
		le.textarea.SetWidth(msg.Width - len(le.prompt) - 4)

	case tea.KeyMsg:
		// Handle global keys
		switch msg.Type {
		case tea.KeyCtrlC:
			if le.textarea.Value() == "" {
				le.exitReq = true
				le.done = true
				return le, tea.Quit
			}
			le.cancelled = true
			le.done = true
			return le, tea.Quit

		case tea.KeyCtrlD:
			if le.textarea.Value() == "" {
				le.exitReq = true
				le.done = true
				return le, tea.Quit
			}

		case tea.KeyEnter:
			if !msg.Alt {
				le.done = true
				le.addToHistory(le.textarea.Value())
				return le, tea.Quit
			}
			// Alt+Enter inserts newline
			le.textarea.InsertString("\n")
			return le, nil

		case tea.KeyTab:
			le.handleTabCompletion()
			return le, nil

		case tea.KeyUp:
			le.historyUp()
			return le, nil

		case tea.KeyDown:
			le.historyDown()
			return le, nil
		}
	}

	// Update textarea
	newModel, cmd := le.textarea.Update(msg)
	le.textarea = newModel
	cmds = append(cmds, cmd)

	return le, tea.Batch(cmds...)
}

// View implements tea.Model
func (le LineEditor) View() string {
	if le.done {
		return ""
	}

	var b strings.Builder

	// Update textarea prompt
	le.textarea.Prompt = le.prompt

	b.WriteString(le.textarea.View())

	// Add help text at bottom
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("ctrl+c: cancel • enter: submit • ↑↓: history • tab: complete"))

	return b.String()
}

// handleTabCompletion handles tab completion for slash commands
func (le *LineEditor) handleTabCompletion() {
	value := le.textarea.Value()
	if !strings.HasPrefix(value, "/") {
		return
	}

	// Find matching completions
	prefix := value
	matches := make([]string, 0)
	for _, c := range le.completions {
		if strings.HasPrefix(c, prefix) && c != prefix {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		return
	}

	// Simple: just use first match
	le.textarea.SetValue(matches[0])
	le.textarea.CursorEnd()
}

// historyUp moves up in history
func (le *LineEditor) historyUp() {
	if len(le.history) == 0 {
		return
	}

	if le.historyIndex == 0 {
		return
	}

	// Save current if first time
	if le.historyIndex == -1 {
		le.historyBackup = le.textarea.Value()
	}

	le.historyIndex--
	le.textarea.SetValue(le.history[le.historyIndex])
	le.textarea.CursorEnd()
}

// historyDown moves down in history
func (le *LineEditor) historyDown() {
	if len(le.history) == 0 || le.historyIndex == -1 {
		return
	}

	le.historyIndex++
	if le.historyIndex >= len(le.history) {
		le.historyIndex = -1
		le.textarea.SetValue(le.historyBackup)
		le.textarea.CursorEnd()
		return
	}

	le.textarea.SetValue(le.history[le.historyIndex])
	le.textarea.CursorEnd()
}

// addToHistory adds an entry to history
func (le *LineEditor) addToHistory(entry string) {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return
	}

	// Don't add duplicates
	if len(le.history) > 0 && le.history[len(le.history)-1] == trimmed {
		return
	}

	le.history = append(le.history, trimmed)
	le.historyIndex = -1
}

// isTerminal checks if stdin/stdout are terminals using proper detection
func isTerminal() bool {
	return isatty(os.Stdin.Fd()) && isatty(os.Stdout.Fd())
}

func isatty(fd uintptr) bool {
	_, _, err := term.GetSize(int(fd))
	return err == nil
}

// GetHistory returns the current history
func (le *LineEditor) GetHistory() []string {
	result := make([]string, len(le.history))
	copy(result, le.history)
	return result
}

// SetHistory sets the history
func (le *LineEditor) SetHistory(history []string) {
	le.history = make([]string, len(history))
	copy(le.history, history)
}

// SmartPrompt provides a contextual prompt that adapts to session state
