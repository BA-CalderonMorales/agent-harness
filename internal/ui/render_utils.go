package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Spinner provides animated progress indication
type Spinner struct {
	frames []string
	index  int
}

// NewSpinner creates a new spinner with Braille frames
func NewSpinner() *Spinner {
	return &Spinner{
		frames: BrailleFrames,
		index:  0,
	}
}

// NewSimpleSpinner creates a spinner with simple ASCII frames
func NewSimpleSpinner() *Spinner {
	return &Spinner{
		frames: []string{"|", "/", "-", "\\"},
		index:  0,
	}
}

// Next returns the next animation frame
func (s *Spinner) Next() string {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	return frame
}

// Current returns the current frame without advancing
func (s *Spinner) Current() string {
	return s.frames[s.index]
}

// ProgressBar renders a progress bar
func ProgressBar(current, total int, width int) string {
	if total <= 0 {
		return ""
	}

	filled := int(float64(current) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percent := int(float64(current) / float64(total) * 100)

	return fmt.Sprintf("[%s] %d%%", bar, percent)
}

// Truncate truncates text with ellipsis
func Truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

// TruncateMiddle truncates in the middle (good for paths)
func TruncateMiddle(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return "..."
	}

	sideLen := (maxLen - 3) / 2
	return text[:sideLen] + "..." + text[len(text)-sideLen:]
}

// WordWrap wraps text to a maximum width
func WordWrap(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)
	return lines
}

// FormatDuration formats a duration in a human-friendly way
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
	}
	return fmt.Sprintf("%.1fm", float64(d)/float64(time.Minute))
}

// FormatBytes formats byte size in a human-friendly way
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// DetectTermux detects if running in Termux environment
func DetectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// UseSimpleRendering returns true if we should use simple ASCII rendering
func UseSimpleRendering() bool {
	return DetectTermux() || os.Getenv("AGENT_HARNESS_SIMPLE_UI") != ""
}
