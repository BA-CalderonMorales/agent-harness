// Stream rendering for real-time agent output
// Polished animations inspired by Terminal Jarvis ADK and Claude Code

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Kaomoji spinner frames for personality
var KaomojiFrames = []string{
	"┌( >_<)┘",
	"└( >_<)┐",
}

// Braille spinner frames for standard operations
var BrailleFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// StreamRenderer handles real-time output rendering with polished animations
type StreamRenderer struct {
	out           io.Writer
	mu            sync.Mutex
	isThinking    bool
	thinkingStart time.Time
	spinnerIdx    int
	toolStack     []ToolInfo
	kaomojiIdx    int
	lastFrameTime time.Time
}

// ToolInfo tracks active tool execution
type ToolInfo struct {
	ID             string
	Name           string
	Description    string
	StartTime      time.Time
	LatestProgress string
}

// NewStreamRenderer creates a new stream renderer with animation support
func NewStreamRenderer() *StreamRenderer {
	return &StreamRenderer{
		out:           os.Stdout,
		toolStack:     make([]ToolInfo, 0),
		lastFrameTime: time.Now(),
	}
}

// SetOutput sets a custom output writer (for testing)
func (sr *StreamRenderer) SetOutput(w io.Writer) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.out = w
}

// StartThinking shows the agent is thinking with animated kaomoji
func (sr *StreamRenderer) StartThinking(context string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if sr.isThinking {
		return
	}
	
	sr.isThinking = true
	sr.thinkingStart = time.Now()
	sr.kaomojiIdx = 0
	
	// Print thinking indicator with kaomoji
	frame := KaomojiFrames[0]
	if context != "" {
		fmt.Fprintf(sr.out, "\n◆ %s\n   %s %s", 
			DimStyle.Render(context),
			DimStyle.Render(frame),
			DimStyle.Render("thinking..."))
	} else {
		fmt.Fprintf(sr.out, "\n◆ %s\n   %s %s", 
			DimStyle.Render("Processing..."),
			DimStyle.Render(frame),
			DimStyle.Render("thinking..."))
	}
}

// StopThinking stops the thinking indicator and clears the line
func (sr *StreamRenderer) StopThinking() {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if !sr.isThinking {
		return
	}

	sr.isThinking = false
	// Clear the spinner line and the thinking indicator
	fmt.Fprint(sr.out, "\r\033[K\033[1A\r\033[K")
}

// UpdateThinking updates the thinking animation frame
func (sr *StreamRenderer) UpdateThinking() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if !sr.isThinking && len(sr.toolStack) == 0 {
		return
	}
	
	// Only update every 200ms
	if time.Since(sr.lastFrameTime) < 200*time.Millisecond {
		return
	}
	sr.lastFrameTime = time.Now()
	
	sr.kaomojiIdx = (sr.kaomojiIdx + 1) % len(KaomojiFrames)
	frame := KaomojiFrames[sr.kaomojiIdx]
	
	if len(sr.toolStack) > 0 {
		// Update the last tool's progress line
		lastTool := sr.toolStack[len(sr.toolStack)-1]
		progress := lastTool.LatestProgress
		if progress == "" {
			progress = "running..."
		}
		
		// Use \r and \033[K for cleaner single-line updates
		fmt.Fprintf(sr.out, "\r\033[K   %s %s", 
			DimStyle.Render(frame),
			DimStyle.Render(Truncate(progress, 60)))
	} else {
		// Update the kaomoji on the current line
		fmt.Fprintf(sr.out, "\r\033[K   %s %s", 
			DimStyle.Render(frame),
			DimStyle.Render("thinking..."))
	}
}

// PrintAgentOutput prints agent text output
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

// PrintToolStart indicates a tool is starting with visual feedback
func (sr *StreamRenderer) PrintToolStart(toolID, toolName, description string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	action := FormatToolAction(toolName, description)
	toolInfo := ToolInfo{
		ID:          toolID,
		Name:        toolName,
		Description: description,
		StartTime:   time.Now(),
	}
	sr.toolStack = append(sr.toolStack, toolInfo)
	
	// Print tool start with arrow indicator
	fmt.Fprintf(sr.out, "\n→ %s\n", DimStyle.Render(action))
	
	// Show kaomoji spinner on next line
	fmt.Fprintf(sr.out, "   %s %s\n", 
		DimStyle.Render(KaomojiFrames[0]),
		DimStyle.Render("running..."))
}

// HandleProgress updates the latest progress for a tool
func (sr *StreamRenderer) HandleProgress(toolUseID string, data any) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	msg := fmt.Sprintf("%v", data)
	// Truncate and clean message for display
	msg = strings.TrimSpace(msg)
	msg = strings.ReplaceAll(msg, "\n", " ")
	
	for i := range sr.toolStack {
		if sr.toolStack[i].ID == toolUseID {
			sr.toolStack[i].LatestProgress = msg
			break
		}
	}
}

// PrintToolComplete indicates a tool completed successfully
func (sr *StreamRenderer) PrintToolComplete(toolUseID string, result string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Find and remove from stack
	var toolName string
	for i, t := range sr.toolStack {
		if t.ID == toolUseID {
			toolName = t.Name
			sr.toolStack = append(sr.toolStack[:i], sr.toolStack[i+1:]...)
			break
		}
	}
	
	if toolName == "" {
		return
	}
	
	// Clear the spinner line and show success
	action := FormatToolAction(toolName, "")
	fmt.Fprintf(sr.out, "\033[2A\033[K   %s %s\n", 
		SuccessStyle.Render("✓"),
		DimStyle.Render(action))
	
	// Show result summary if significant
	if len(result) > 0 && len(result) < 100 {
		fmt.Fprintf(sr.out, "   %s\n", DimStyle.Render(Truncate(result, 80)))
	}
}

// PrintToolError shows a tool error with clear indication
func (sr *StreamRenderer) PrintToolError(toolUseID string, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Find and remove from stack
	var toolName string
	for i, t := range sr.toolStack {
		if t.ID == toolUseID {
			toolName = t.Name
			sr.toolStack = append(sr.toolStack[:i], sr.toolStack[i+1:]...)
			break
		}
	}
	
	if toolName == "" {
		return
	}
	
	// Clear the spinner line and show error
	action := FormatToolAction(toolName, "")
	fmt.Fprintf(sr.out, "\033[2A\033[K   %s %s\n", 
		ErrorStyle.Render("✗"),
		ErrorStyle.Render(action))
	
	if err != nil {
		fmt.Fprintf(sr.out, "   %s\n", ErrorStyle.Render(err.Error()))
	}
}

// PrintProgress shows ongoing progress with animation
func (sr *StreamRenderer) PrintProgress(message string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	// Update spinner index
	sr.spinnerIdx = (sr.spinnerIdx + 1) % len(BrailleFrames)
	frame := BrailleFrames[sr.spinnerIdx]
	
	fmt.Fprintf(sr.out, "\r   %s %s", 
		DimStyle.Render(frame),
		DimStyle.Render(message))
}

// ClearProgress clears the progress line
func (sr *StreamRenderer) ClearProgress() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	fmt.Fprintf(sr.out, "\r\033[K")
}

// PrintUserMessage shows the user's input with diamond indicator
func (sr *StreamRenderer) PrintUserMessage(text string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	fmt.Fprintf(sr.out, "\n◆ %s\n", text)
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
	
	fmt.Fprintf(sr.out, "   %s %s\n",
		DimStyle.Render("tip:"),
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

// IsThinking returns whether the agent is currently thinking
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

// GetActiveToolCount returns the number of active tools
func (sr *StreamRenderer) GetActiveToolCount() int {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return len(sr.toolStack)
}

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
