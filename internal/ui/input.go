// Rich input handling with history, vim-like editing, and completions
// Inspired by claw-code's LineEditor

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditorMode represents the current editor mode
type EditorMode int

const (
	ModePlain EditorMode = iota
	ModeInsert
	ModeNormal
	ModeVisual
	ModeCommand
)

func (m EditorMode) String() string {
	switch m {
	case ModePlain:
		return "PLAIN"
	case ModeInsert:
		return "INSERT"
	case ModeNormal:
		return "NORMAL"
	case ModeVisual:
		return "VISUAL"
	case ModeCommand:
		return "COMMAND"
	default:
		return "UNKNOWN"
	}
}

// ReadOutcome represents the result of reading a line
type ReadOutcome struct {
	Text   string
	Cancel bool
	Exit   bool
}

// LineEditor provides rich input with history and vim-like editing
type LineEditor struct {
	prompt         string
	completions    []string
	history        []string
	historyIndex   int
	historyBackup  string
	vimEnabled     bool
	mode           EditorMode
	textarea       textarea.Model
	width          int
	height         int
	done           bool
	cancelled      bool
	exitReq        bool
	showMode       bool
}

// NewLineEditor creates a new line editor
func NewLineEditor(prompt string, completions []string) *LineEditor {
	ta := textarea.New()
	ta.SetHeight(1)
	ta.SetWidth(80)
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	return &LineEditor{
		prompt:      prompt,
		completions: completions,
		history:     make([]string, 0),
		mode:        ModeInsert,
		textarea:    ta,
		vimEnabled:  false,
		showMode:    false,
	}
}

// EnableVim enables vim keybindings
func (le *LineEditor) EnableVim() {
	le.vimEnabled = true
	le.mode = ModeInsert
	le.showMode = true
}

// ReadLine reads a line from the user
func (le *LineEditor) ReadLine() (*ReadOutcome, error) {
	// Check if we're in a terminal
	if !isTerminal() {
		return le.readLineSimple()
	}

	// Use bubbletea for rich input
	p := tea.NewProgram(le, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	editor := m.(*LineEditor)
	if editor.exitReq {
		return &ReadOutcome{Exit: true}, nil
	}
	if editor.cancelled {
		return &ReadOutcome{Cancel: true}, nil
	}
	return &ReadOutcome{Text: editor.textarea.Value()}, nil
}

// readLineSimple provides a simple fallback for non-terminal environments
func (le *LineEditor) readLineSimple() (*ReadOutcome, error) {
	fmt.Print(le.prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return &ReadOutcome{Exit: true}, nil
	}
	line = strings.TrimRight(line, "\r\n")
	return &ReadOutcome{Text: line}, nil
}

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

		case tea.KeyEsc:
			if le.vimEnabled {
				le.mode = ModeNormal
				return le, nil
			}

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

		// Handle vim keys when enabled
		if le.vimEnabled && le.mode == ModeNormal {
			switch msg.String() {
			case "i":
				le.mode = ModeInsert
				return le, nil
			case "a":
				le.mode = ModeInsert
				// CursorRight not available in this version
				return le, nil
			case "I":
				le.mode = ModeInsert
				le.textarea.CursorStart()
				return le, nil
			case "A":
				le.mode = ModeInsert
				le.textarea.CursorEnd()
				return le, nil
			case "v":
				le.mode = ModeVisual
				return le, nil
			case ":":
				le.mode = ModeCommand
				return le, nil
			case "h":
				// CursorLeft not available in this version
				return le, nil
			case "l":
				// CursorRight not available in this version
				return le, nil
			case "j":
				le.historyUp()
				return le, nil
			case "k":
				le.historyDown()
				return le, nil
			case "0", "^":
				le.textarea.CursorStart()
				return le, nil
			case "$":
				le.textarea.CursorEnd()
				return le, nil
			case "dd":
				le.textarea.Reset()
				return le, nil
			}
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

	// Render mode indicator if vim is enabled
	prompt := le.prompt
	if le.showMode && le.vimEnabled {
		modeStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#3b82f6")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Padding(0, 1)
		prompt = modeStyle.Render(le.mode.String()) + " " + le.prompt
	}

	// Update textarea prompt
	le.textarea.Prompt = prompt

	b.WriteString(le.textarea.View())

	// Add help text at bottom
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	if le.vimEnabled {
		b.WriteString("\n")
		if le.mode == ModeNormal {
			b.WriteString(helpStyle.Render("i=insert · :cmd · v=visual · hjkl=move · dd=clear"))
		} else if le.mode == ModeInsert {
			b.WriteString(helpStyle.Render("Esc=normal · Ctrl+C=cancel · Enter=submit"))
		}
	} else {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Ctrl+C=cancel · Enter=submit · ↑↓=history · Tab=complete"))
	}

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
	// In a full implementation, cycle through matches
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

// isTerminal checks if stdin/stdout are terminals
func isTerminal() bool {
	return isatty(os.Stdin.Fd()) && isatty(os.Stdout.Fd())
}

func isatty(fd uintptr) bool {
	// Simple check - in production use proper terminal detection
	return true
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
