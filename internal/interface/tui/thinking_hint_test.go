package tui

import (
	"strings"
	"testing"
	"time"
)

// TestWorkingIndicatorStates pins the working-indicator state machine
// (goal 0.3.29 Task 4): thinking→working→idle, elapsed clock, hidden
// when idle. Replaces the removed thinking badge tests.
func TestWorkingIndicatorStates(t *testing.T) {
	m := ChatModel{width: 100, height: 24}

	// Idle: line hidden entirely.
	if line := m.workingStatusLine(0); line != "" {
		t.Fatalf("idle status = %q, want empty", line)
	}
	if h := m.workingStatusHeight(); h != 0 {
		t.Fatalf("idle height = %d, want 0 (no reserved row)", h)
	}

	// Thinking: turn in flight, no tool. Animated word + elapsed clock;
	// no quip words, no star glyph.
	m.thinking = true
	m.streaming = true
	m.startTime = time.Now().Add(-1500 * time.Millisecond)
	m.elapsed = 1500 * time.Millisecond
	line := m.workingStatusLine(3)
	if !strings.Contains(line, "working") {
		t.Fatalf("thinking status = %q, want the animated word", line)
	}
	if !strings.Contains(line, "1.5s") {
		t.Fatalf("thinking status = %q, want elapsed clock", line)
	}
	if strings.Contains(line, "✦") || strings.Contains(line, "✧") {
		t.Fatalf("thinking status = %q, star glyphs are removed", line)
	}
	for _, quip := range []string{"pondering", "brewing", "scheming", "conjuring"} {
		if strings.Contains(line, quip) {
			t.Fatalf("thinking status = %q, quips were removed (word animates in place)", line)
		}
	}

	// Working: tool call in flight — class + count + elapsed.
	tool := ChatMessage{
		ID: "tool-1", Role: "tool", IsTool: true,
		ToolName: "Bash", ToolDisplayName: "Shell", ToolStatus: ToolStatusRunning,
	}
	m.currentToolMsg = &tool
	m.completedToolMsgs = nil
	m.turnTools = []turnToolMark{{ToolID: "t1", At: 0}, {ToolID: "t2", At: 1}}
	line = m.workingStatusLine(1)
	if !strings.Contains(line, "Shell ×2") {
		t.Fatalf("working status = %q, want class and count", line)
	}
	if !strings.Contains(line, "working") {
		t.Fatalf("working status = %q, want the animated word", line)
	}

	// Idle after the turn closes: hidden again (no stale line).
	m.thinking = false
	m.streaming = false
	m.currentToolMsg = nil
	if line := m.workingStatusLine(0); line != "" {
		t.Fatalf("post-turn status = %q, want empty", line)
	}
}

// TestWorkingAnimShimmers pins the in-place animation contract: the
// word never changes, and the bright letter sweeps through it on the
// tick clock. Assertion is on the sweep index, not rendered bytes —
// lipgloss strips ANSI outside a TTY, so color differences are not
// observable here; the sweep order is the behavior.
func TestWorkingAnimShimmers(t *testing.T) {
	word := workingWord()
	if word != "working" {
		t.Fatalf("word = %q, want the single stable word", word)
	}
	n := len([]rune(word))
	seen := map[int]bool{}
	for tick := 0; tick < 2*n; tick++ {
		if got := workingBrightIndex(tick, n); got != tick%n {
			t.Fatalf("brightIndex(%d) = %d, want %d", tick, got, tick%n)
		}
		seen[workingBrightIndex(tick, n)] = true
	}
	if len(seen) != n {
		t.Fatalf("shimmer visited %d of %d letters over a full sweep", len(seen), n)
	}
	// Text is stable across ticks even though the highlight moves.
	if stripANSI(workingAnim(word, 0)) != word || stripANSI(workingAnim(word, 3)) != word {
		t.Fatalf("anim text changed across ticks; word must animate in place")
	}
}
