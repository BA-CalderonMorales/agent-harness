// Stream rendering for real-time agent output
// Polished animations inspired by Terminal Jarvis ADK and Claude Code

package ui

import (
	"fmt"
	"io"
	"os"
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
	// Layout: blank line, "◆ <context>", "   <frame> <status>" (no trailing newline on last line)
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
	// Clear the thinking indicator lines
	// StartThinking prints: newline, "◆ <context>", newline, "   <frame> <status>"
	// We need to clear those lines and return to the start
	fmt.Fprint(sr.out, "\r\033[K")        // Clear current line (spinner line)
	fmt.Fprint(sr.out, "\033[1A\r\033[K") // Move up and clear the ◆ line
	fmt.Fprint(sr.out, "\033[1A\r\033[K") // Move up and clear the blank line
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

	// Build status text - keep it short to avoid line wrapping
	var status string
	if len(sr.toolStack) > 0 {
		lastTool := sr.toolStack[len(sr.toolStack)-1]
		status = lastTool.LatestProgress
		if status == "" {
			status = "running..."
		}
	} else {
		status = "thinking..."
	}

	// Get terminal width for truncation
	width, _, _ := GetTerminalSize()
	maxStatusLen := width - 15 // Reserve space for frame and padding
	if maxStatusLen < 20 {
		maxStatusLen = 20
	}

	// Truncate status to fit on one line
	status = Truncate(status, maxStatusLen)

	// IMPORTANT: Check toolStack FIRST before isThinking
	// PrintToolStart prints a trailing newline, so cursor is on line AFTER spinner
	// We need to move up one line to get to the spinner line
	// For thinking spins: StartThinking prints NO trailing newline, cursor is ON spinner line
	// We just need \r to go to start of line
	if len(sr.toolStack) > 0 {
		// Tool spinner - move up to the spinner line, update, then add newline back
		fmt.Fprintf(sr.out, "\033[1A\r\033[K   %s %s\n",
			DimStyle.Render(frame),
			DimStyle.Render(status))
	} else if sr.isThinking {
		// Thinking spinner - already on the right line
		fmt.Fprintf(sr.out, "\r\033[K   %s %s",
			DimStyle.Render(frame),
			DimStyle.Render(status))
	}
}
