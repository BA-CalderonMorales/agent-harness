package ui

import (
	"fmt"
	"strings"
)

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
	editor      *LineEditor
	prompt      *SmartPrompt
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
