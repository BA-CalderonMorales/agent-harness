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
	if !strings.Contains(line, "Working") {
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

	// Working: a tool call in flight does NOT change the line — live
	// feedback removed the class-and-count ("Shell ×2" read as
	// content); the transcript shows what's running, the indicator
	// only says work is happening and for how long.
	tool := ChatMessage{
		ID: "tool-1", Role: "tool", IsTool: true,
		ToolName: "Bash", ToolDisplayName: "Shell", ToolStatus: ToolStatusRunning,
	}
	m.currentToolMsg = &tool
	m.completedToolMsgs = nil
	m.turnTools = []turnToolMark{{ToolID: "t1", At: 0}, {ToolID: "t2", At: 1}}
	line = m.workingStatusLine(1)
	if strings.Contains(line, "Shell") || strings.Contains(line, "×") {
		t.Fatalf("working status = %q, class-and-count was removed (word + elapsed only)", line)
	}
	if !strings.Contains(line, "Working") {
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

// TestWorkingStatusBreathes pins the live status layout: the line gets a
// blank row above and below so it reads as its own band between the
// transcript and the composer, and the working frame never grows the
// chrome — it occupies the same rows as an idle one.
func TestWorkingStatusBreathes(t *testing.T) {
	idle := NewChatModel()
	idle.width, idle.height = 80, 20
	idle.SetInput("ready")
	idleRows := len(strings.Split(strings.TrimRight(idle.View(), "\n"), "\n"))

	m := NewChatModel()
	m.width, m.height = 80, 20
	m.SetInput("ready")
	m.SetThinking(true, "Thinking...")
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != idleRows {
		t.Fatalf("working view = %d rows, idle = %d (chrome must not grow)", len(lines), idleRows)
	}

	idx := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Working") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no Working row in:\n%s", view)
	}
	if idx == 0 || idx == len(lines)-1 {
		t.Fatalf("Working row at the pane edge, no room to breathe:\n%s", view)
	}
	if strings.TrimSpace(lines[idx-1]) != "" || strings.TrimSpace(lines[idx+1]) != "" {
		t.Fatalf("Working row is not flanked by blank rows:\n%s", view)
	}
}

// TestWorkingAnimShimmers pins the in-place animation contract: the
// word never changes, and the bright letter sweeps through it on the
// tick clock. Assertion is on the sweep index, not rendered bytes —
// lipgloss strips ANSI outside a TTY, so color differences are not
// observable here; the sweep order is the behavior.
func TestWorkingAnimShimmers(t *testing.T) {
	word := workingWord()
	if word != "Working" {
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
