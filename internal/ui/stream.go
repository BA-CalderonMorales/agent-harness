// Stream rendering for real-time agent output
// Creates that "streaming" feel even with discrete events

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// StreamRenderer handles real-time output rendering
type StreamRenderer struct {
	out        io.Writer
	mu         sync.Mutex
	isThinking bool
	lastLine   string
	spinner    *Spinner
	toolStack  []string
}

// NewStreamRenderer creates a new stream renderer
func NewStreamRenderer() *StreamRenderer {
	return &StreamRenderer{
		out:       os.Stdout,
		spinner:   NewSpinner(),
		toolStack: make([]string, 0),
	}
}

// SetOutput sets a custom output writer (for testing)
func (sr *StreamRenderer) SetOutput(w io.Writer) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.out = w
}

// StartThinking shows the agent is thinking/working with animated spinner
func (sr *StreamRenderer) StartThinking(context string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if sr.isThinking {
		return
	}
	
	sr.isThinking = true
	
	// Show animated thinking indicator with spinner
	frame := sr.spinner.Next()
	indicator := GetRandomThinkingIndicator()
	if context != "" {
		fmt.Fprintf(sr.out, "\n%s %s\n", DimStyle.Render(frame), DimStyle.Render(context))
	} else {
		fmt.Fprintf(sr.out, "\n%s %s\n", DimStyle.Render(frame), DimStyle.Render(indicator))
	}
}

// StopThinking stops the thinking indicator
func (sr *StreamRenderer) StopThinking() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.isThinking = false
}

// PrintAgentOutput prints agent text output with natural flow
func (sr *StreamRenderer) PrintAgentOutput(text string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	sr.isThinking = false
	
	// Split into lines and render each
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			fmt.Fprintln(sr.out)
		}
		if line != "" {
			fmt.Fprint(sr.out, line)
		}
	}
}

// PrintToolStart indicates a tool is starting
func (sr *StreamRenderer) PrintToolStart(toolName, description string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	action := FormatToolAction(toolName, description)
	sr.toolStack = append(sr.toolStack, toolName)
	
	// Compact, professional tool indication
	fmt.Fprintf(sr.out, "\n%s %s\n", 
		DimStyle.Render("→"),
		DimStyle.Render(action))
}

// PrintToolComplete indicates a tool completed successfully
func (sr *StreamRenderer) PrintToolComplete(toolName string, result string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Pop from stack
	if len(sr.toolStack) > 0 {
		sr.toolStack = sr.toolStack[:len(sr.toolStack)-1]
	}
	
	// Only show completion for significant operations
	if len(result) > 100 || strings.Contains(result, "error") {
		status := SuccessStyle.Render("✓")
		if strings.Contains(strings.ToLower(result), "error") {
			status = ErrorStyle.Render("✗")
		}
		fmt.Fprintf(sr.out, "  %s\n", status)
	}
}

// PrintToolError shows a tool error
func (sr *StreamRenderer) PrintToolError(toolName string, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Pop from stack
	if len(sr.toolStack) > 0 {
		sr.toolStack = sr.toolStack[:len(sr.toolStack)-1]
	}
	
	fmt.Fprintf(sr.out, "  %s %s\n", 
		ErrorStyle.Render("✗"),
		ErrorStyle.Render(err.Error()))
}

// PrintProgress shows ongoing progress for long operations
func (sr *StreamRenderer) PrintProgress(message string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	frame := sr.spinner.Next()
	fmt.Fprintf(sr.out, "\r%s %s", 
		DimStyle.Render(frame),
		DimStyle.Render(message))
}

// ClearProgress clears the progress line
func (sr *StreamRenderer) ClearProgress() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	fmt.Fprintf(sr.out, "\r\033[K") // Clear line
}

// PrintUserMessage shows the user's input (for echo)
func (sr *StreamRenderer) PrintUserMessage(text string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Compact user indicator
	fmt.Fprintf(sr.out, "\n%s %s\n", 
		UserStyle.Render("◆"),
		UserStyle.Render(text))
}

// PrintSeparator prints a subtle separator
func (sr *StreamRenderer) PrintSeparator() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	width := 40
	if w, _, err := GetTerminalSize(); err == nil && w > 0 {
		width = w / 2
		if width > 60 {
			width = 60
		}
	}
	
	sep := strings.Repeat("─", width)
	fmt.Fprintf(sr.out, "\n%s\n", DimStyle.Render(sep))
}

// PrintSuggestion prints a helpful suggestion
func (sr *StreamRenderer) PrintSuggestion(text string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	fmt.Fprintf(sr.out, "  %s %s\n",
		DimStyle.Render("💡"),
		DimStyle.Render(text))
}

// Flush ensures all output is written
func (sr *StreamRenderer) Flush() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if f, ok := sr.out.(*os.File); ok {
		f.Sync()
	}
}

// IsThinking returns whether the agent is currently "thinking"
func (sr *StreamRenderer) IsThinking() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.isThinking
}

// HasActiveTools returns whether tools are currently running
func (sr *StreamRenderer) HasActiveTools() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return len(sr.toolStack) > 0
}

// GetTerminalSize attempts to get terminal dimensions
func GetTerminalSize() (width, height int, err error) {
	// Simplified - in a full implementation, use term.GetSize
	return 80, 24, nil
}

// AnimatedText provides typewriter-like text output
type AnimatedText struct {
	out    io.Writer
	delay  time.Duration
}

// NewAnimatedText creates animated text output
func NewAnimatedText() *AnimatedText {
	return &AnimatedText{
		out:   os.Stdout,
		delay: 1 * time.Millisecond, // Very fast, almost imperceptible
	}
}

// Print prints text with a subtle animation effect
func (at *AnimatedText) Print(text string) {
	// For now, just print directly
	// In a full implementation, this could animate
	fmt.Fprint(at.out, text)
}

// Sprint returns the animated string
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
