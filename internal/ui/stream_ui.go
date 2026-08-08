package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// GetTerminalSize attempts to get terminal dimensions
func GetTerminalSize() (width, height int, err error) {
	// Simplified - in a full implementation, use term.GetSize
	return 80, 24, nil
}

// AnimatedText provides typewriter-like text output
type AnimatedText struct {
	out   io.Writer
	delay time.Duration
}

// NewAnimatedText creates animated text output
func NewAnimatedText() *AnimatedText {
	return &AnimatedText{
		out:   os.Stdout,
		delay: 1 * time.Millisecond,
	}
}

// Print prints text (no animation for now)
func (at *AnimatedText) Print(text string) {
	fmt.Fprint(at.out, text)
}

// Sprint returns the string
func (at *AnimatedText) Sprint(text string) string {
	return text
}

// InteractivePrompt provides a smart, contextual prompt
type InteractivePrompt struct {
	basePrompt      string
	contextProvider func() string
	history         []string
	historyIdx      int
}

// NewInteractivePrompt creates a smart prompt
func NewInteractivePrompt(base string) *InteractivePrompt {
	return &InteractivePrompt{
		basePrompt: base,
		history:    make([]string, 0),
		historyIdx: -1,
	}
}

// SetContextProvider sets a function that provides context for the prompt
func (ip *InteractivePrompt) SetContextProvider(fn func() string) {
	ip.contextProvider = fn
}

// Render renders the full prompt with context
func (ip *InteractivePrompt) Render() string {
	context := ""
	if ip.contextProvider != nil {
		context = ip.contextProvider()
	}

	if context != "" {
		return fmt.Sprintf("%s %s ", DimStyle.Render(context), ip.basePrompt)
	}

	return ip.basePrompt + " "
}

// AddToHistory adds an entry to history
func (ip *InteractivePrompt) AddToHistory(entry string) {
	if entry = strings.TrimSpace(entry); entry == "" {
		return
	}

	// Avoid duplicates at the end
	if len(ip.history) > 0 && ip.history[len(ip.history)-1] == entry {
		return
	}

	ip.history = append(ip.history, entry)
	ip.historyIdx = -1
}

// HistoryUp moves up in history
func (ip *InteractivePrompt) HistoryUp() (string, bool) {
	if len(ip.history) == 0 {
		return "", false
	}

	if ip.historyIdx < len(ip.history)-1 {
		ip.historyIdx++
		return ip.history[len(ip.history)-1-ip.historyIdx], true
	}

	return "", false
}

// HistoryDown moves down in history
func (ip *InteractivePrompt) HistoryDown() (string, bool) {
	if ip.historyIdx <= 0 {
		ip.historyIdx = -1
		return "", false
	}

	ip.historyIdx--
	return ip.history[len(ip.history)-1-ip.historyIdx], true
}
