package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Reduced motion holds the frame still. The information the animated
// element carries stays; only the movement goes.

func TestWorkingIndicatorIsStillWithReducedMotion(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	var m ChatModel
	m.reducedMotion = true
	m.thinking = true

	first := m.workingStatusLine(0)
	for _, tick := range []int{1, 3, 7, 11} {
		if got := m.workingStatusLine(tick); got != first {
			t.Fatalf("workingStatusLine(%d) = %q, want the still line %q", tick, got, first)
		}
	}
}

func TestWorkingIndicatorMovesWithoutReducedMotion(t *testing.T) {
	// The shimmer is a color difference, so the styles have to carry
	// color for the difference to be visible.
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	var m ChatModel
	m.thinking = true

	first := m.workingStatusLine(0)
	moved := false
	for _, tick := range []int{1, 2, 3, 4, 5, 6} {
		if m.workingStatusLine(tick) != first {
			moved = true
		}
	}
	if !moved {
		t.Fatal("workingStatusLine never changed; the shimmer is not animating")
	}
}

func TestReducedMotionStillReportsElapsedTime(t *testing.T) {
	var m ChatModel
	m.reducedMotion = true
	m.thinking = true
	m.elapsed = 4 * time.Second

	line := m.workingStatusLine(0)
	if !containsText(line, "4") {
		t.Fatalf("workingStatusLine = %q, want the elapsed time to stay visible", line)
	}
}

func containsText(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) && indexOfText(haystack, needle) >= 0
}

func indexOfText(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
