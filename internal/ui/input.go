// Rich input handling with contextual awareness
// Seamless conversation flow - the interface disappears

package ui

import (
	"bufio"
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"strings"
)

// ReadOutcome represents the result of reading a line
type ReadOutcome struct {
	Text   string
	Cancel bool
	Exit   bool
}

// LineEditor provides rich input with history and completions
type LineEditor struct {
	prompt        string
	completions   []string
	history       []string
	historyIndex  int
	historyBackup string
	textarea      textarea.Model
	width         int
	height        int
	done          bool
	cancelled     bool
	exitReq       bool
	termWidth     int
	isTermux      bool
}

// NewLineEditor creates a new line editor
func NewLineEditor(prompt string, completions []string) *LineEditor {
	ta := textarea.New()
	ta.SetHeight(1)
	ta.SetWidth(80)
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	// Detect Termux environment
	isTermux := DetectTermux()

	return &LineEditor{
		prompt:      prompt,
		completions: completions,
		history:     make([]string, 0),
		textarea:    ta,
		termWidth:   80,
		isTermux:    isTermux,
	}
}

// ReadLine reads a line from the user
func (le *LineEditor) ReadLine() (*ReadOutcome, error) {
	// Check if we're in a terminal
	if !isTerminal() {
		return le.readLineSimple()
	}

	// For Termux/mobile, use simple line reading for better compatibility
	if le.isTermux {
		return le.readLineTermux()
	}

	// Use bubbletea for rich input on desktop
	p := tea.NewProgram(le, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	// type assert to value type (bubbletea returns value, not pointer)
	editor, ok := m.(LineEditor)
	if !ok {
		// try pointer type as fallback
		if editorPtr, ok := m.(*LineEditor); ok {
			editor = *editorPtr
		} else {
			return nil, fmt.Errorf("unexpected model type: %T", m)
		}
	}
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

// readLineTermux provides Termux-optimized input
func (le *LineEditor) readLineTermux() (*ReadOutcome, error) {
	// Print prompt without special characters that might render weird
	fmt.Print(le.prompt)

	reader := bufio.NewReader(os.Stdin)
	var input strings.Builder

	for {
		ch, _, err := reader.ReadRune()
		if err != nil {
			return &ReadOutcome{Exit: true}, nil
		}

		switch ch {
		case '\n', '\r':
			line := input.String()
			le.addToHistory(line)
			// Move to new line and show input with diamond indicator
			fmt.Println()
			fmt.Printf("◆ %s\n", line)
			return &ReadOutcome{Text: line}, nil
		case '\x03': // Ctrl+C
			if input.Len() == 0 {
				return &ReadOutcome{Exit: true}, nil
			}
			fmt.Println("^C")
			return &ReadOutcome{Cancel: true}, nil
		case '\x04': // Ctrl+D
			if input.Len() == 0 {
				return &ReadOutcome{Exit: true}, nil
			}
		case '\x7f', '\b': // Backspace
			if input.Len() > 0 {
				str := input.String()
				input.Reset()
				// Remove last rune
				runes := []rune(str)
				if len(runes) > 0 {
					input.WriteString(string(runes[:len(runes)-1]))
					// Clear line and rewrite
					fmt.Print("\r\033[K") // Clear to end of line
					fmt.Print(le.prompt + input.String())
				}
			}
		case '\x09': // Tab
			// Simple tab completion for slash commands
			value := input.String()
			if strings.HasPrefix(value, "/") {
				for _, c := range le.completions {
					if strings.HasPrefix(c, value) && c != value {
						// Complete to this command
						input.Reset()
						input.WriteString(c)
						fmt.Print("\r\033[K")
						fmt.Print(le.prompt + c)
						break
					}
				}
			}
		case '\x1b': // Escape sequence (arrow keys, etc)
			// Try to read the rest of the escape sequence
			next, _, err := reader.ReadRune()
			if err != nil {
				continue
			}
			if next == '[' {
				// CSI sequence
				cmd, _, err := reader.ReadRune()
				if err != nil {
					continue
				}
				switch cmd {
				case 'A': // Up arrow - history up
					if len(le.history) > 0 && le.historyIndex < len(le.history)-1 {
						le.historyIndex++
						le.historyBackup = input.String()
						input.Reset()
						idx := len(le.history) - 1 - le.historyIndex
						input.WriteString(le.history[idx])
						fmt.Print("\r\033[K")
						fmt.Print(le.prompt + input.String())
					}
				case 'B': // Down arrow - history down
					if le.historyIndex > 0 {
						le.historyIndex--
						input.Reset()
						idx := len(le.history) - 1 - le.historyIndex
						input.WriteString(le.history[idx])
					} else if le.historyIndex == 0 {
						le.historyIndex = -1
						input.Reset()
						input.WriteString(le.historyBackup)
					}
					fmt.Print("\r\033[K")
					fmt.Print(le.prompt + input.String())
				}
			}
		default:
			// Regular character
			input.WriteRune(ch)
			fmt.Print(string(ch))
		}
	}
}
