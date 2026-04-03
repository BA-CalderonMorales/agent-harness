// Rich input handling with contextual awareness
// Seamless conversation flow - the interface disappears

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// ReadOutcome represents the result of reading a line
type ReadOutcome struct {
	Text   string
	Cancel bool
	Exit   bool
}

// LineEditor provides rich input with history and completions
type LineEditor struct {
	prompt         string
	completions    []string
	history        []string
	historyIndex   int
	historyBackup  string
	textarea       textarea.Model
	width          int
	height         int
	done           bool
	cancelled      bool
	exitReq        bool
	termWidth      int
	isTermux       bool
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
type SmartPrompt struct {
	basePrompt      string
	contextProvider func() string
	history         []string
	historyIdx      int
}

// NewSmartPrompt creates a contextual prompt
func NewSmartPrompt() *SmartPrompt {
	return &SmartPrompt{
		basePrompt: "◆",
		history:    make([]string, 0),
		historyIdx: -1,
	}
}

// SetContextProvider sets a function that provides context for the prompt
func (sp *SmartPrompt) SetContextProvider(fn func() string) {
	sp.contextProvider = fn
}

// Render returns the full prompt string
func (sp *SmartPrompt) Render() string {
	context := ""
	if sp.contextProvider != nil {
		context = sp.contextProvider()
	}

	if context != "" {
		return fmt.Sprintf("%s %s ", DimStyle.Render(context), sp.basePrompt)
	}

	return sp.basePrompt + " "
}

// AddToHistory adds an entry to history
func (sp *SmartPrompt) AddToHistory(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	// Avoid duplicates at the end
	if len(sp.history) > 0 && sp.history[len(sp.history)-1] == entry {
		return
	}

	sp.history = append(sp.history, entry)
	sp.historyIdx = -1
}

// HistoryUp moves up in history
func (sp *SmartPrompt) HistoryUp(current string) (string, bool) {
	if len(sp.history) == 0 {
		return "", false
	}

	if sp.historyIdx == -1 {
		// Save current input before going to history
		// (would need to be passed in or stored)
	}

	if sp.historyIdx < len(sp.history)-1 {
		sp.historyIdx++
		return sp.history[len(sp.history)-1-sp.historyIdx], true
	}

	return "", false
}

// HistoryDown moves down in history
func (sp *SmartPrompt) HistoryDown() (string, bool) {
	if sp.historyIdx <= 0 {
		sp.historyIdx = -1
		return "", false
	}

	sp.historyIdx--
	return sp.history[len(sp.history)-1-sp.historyIdx], true
}

// ContextualInput combines input reading with smart prompts
type ContextualInput struct {
	editor     *LineEditor
	prompt     *SmartPrompt
	completions []string
}

// NewContextualInput creates a new contextual input handler
func NewContextualInput(completions []string) *ContextualInput {
	prompt := NewSmartPrompt()
	return &ContextualInput{
		editor:      NewLineEditor(prompt.Render(), completions),
		prompt:      prompt,
		completions: completions,
	}
}

// SetContextProvider sets the context provider for the prompt
func (ci *ContextualInput) SetContextProvider(fn func() string) {
	ci.prompt.SetContextProvider(fn)
	// Update the editor's prompt
	ci.editor.prompt = ci.prompt.Render()
}

// ReadInput reads input with contextual awareness
func (ci *ContextualInput) ReadInput() (*ReadOutcome, error) {
	// Update prompt before reading
	ci.editor.prompt = ci.prompt.Render()
	
	outcome, err := ci.editor.ReadLine()
	if err != nil {
		return nil, err
	}
	
	if outcome != nil && !outcome.Cancel && !outcome.Exit && outcome.Text != "" {
		ci.prompt.AddToHistory(outcome.Text)
	}
	
	return outcome, nil
}
