package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

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
