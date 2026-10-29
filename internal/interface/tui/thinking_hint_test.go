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

	// Thinking: turn in flight, no prose yet. Honest phase + elapsed
	// clock; no quip words, no star glyph. The phase rides on the Agent
	// header — where the answer bubble will land — not the chrome band.
	m.thinking = true
	m.streaming = true
	m.startTime = time.Now().Add(-1500 * time.Millisecond)
	m.elapsed = 1500 * time.Millisecond
	line := m.liveHeaderStatus(3)
	if !strings.Contains(line, "Thinking") {
		t.Fatalf("thinking status = %q, want the thinking phase", line)
	}
	if !strings.Contains(line, "1.5s") {
		t.Fatalf("thinking status = %q, want elapsed clock", line)
	}
	if strings.Contains(line, "✦") || strings.Contains(line, "✧") {
		t.Fatalf("thinking status = %q, star glyphs are removed", line)
	}
	for _, quip := range []string{"pondering", "brewing", "scheming", "conjuring"} {
		if strings.Contains(line, quip) {
			t.Fatalf("thinking status = %q, quips were removed (phase label is stable)", line)
		}
	}

	// Writing: prose deltas are arriving. The phase changes; the
	// elapsed clock stays the progress signal.
	m.writing = true
	line = m.liveHeaderStatus(1)
	if !strings.Contains(line, "Writing") {
		t.Fatalf("writing status = %q, want the writing phase", line)
	}

	// Running: a tool call in flight names the class that is running;
	// the transcript row carries the command detail.
	tool := ChatMessage{
		ID: "tool-1", Role: "tool", IsTool: true,
		ToolName: "Bash", ToolDisplayName: "Shell", ToolStatus: ToolStatusRunning,
	}
	m.writing = false
	m.currentToolMsg = &tool
	m.completedToolMsgs = nil
	m.turnTools = []turnToolMark{{ToolID: "t1", At: 0}, {ToolID: "t2", At: 1}}
	line = m.liveHeaderStatus(1)
	if !strings.Contains(line, "Running Shell") {
		t.Fatalf("working status = %q, want the running tool class", line)
	}

	// The chrome band carries only the turn track: no spinner, no clock.
	m.loopBeat = beatAsk
	m.loopTrack = true
	m.width = 80
	if band := m.workingStatusLine(0); strings.Contains(band, "Shell") || strings.Contains(band, "⣾") {
		t.Fatalf("band = %q, want only the beat track", band)
	}

	// Idle after the turn closes: hidden again (no stale line).
	m.thinking = false
	m.streaming = false
	m.currentToolMsg = nil
	if line := m.workingStatusLine(0); line != "" {
		t.Fatalf("post-turn status = %q, want empty", line)
	}
}

// TestReasoningPreviewRendersBeforeProse pins the hosted wait state:
// OpenRouter reasoning models can think for most of a minute before the
// first content token. The reasoning tail must be visible under the
// Agent header during that wait, then yield to the answer.
func TestReasoningPreviewRendersBeforeProse(t *testing.T) {
	m := NewChatModel()
	m.width, m.height = 100, 24
	m.resize(100, 24)

	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	// Let the placeholder delay elapse: the bubble materializes on the
	// timer tick, exactly as it does in a live turn.
	m.startTime = time.Now().Add(-2 * time.Second)
	model, _ = m.Update(timerTickMsg{time: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentThinkingMsg{Text: "Checking the repository layout, then the config."})
	m = model.(ChatModel)
	m.elapsed = 3 * time.Second
	m.refreshViewport()

	waitView := SanitizeANSI(m.View())
	if !strings.Contains(waitView, "repository layout") {
		t.Fatalf("wait-state frame does not show the reasoning preview:\n%s", waitView)
	}

	model, _ = m.Update(AgentChunkMsg{Text: "The answer.", Timestamp: time.Now()})
	m = model.(ChatModel)
	m.refreshViewport()
	answerView := SanitizeANSI(m.View())
	if strings.Contains(answerView, "repository layout") {
		t.Fatalf("answer frame still shows the reasoning preview:\n%s", answerView)
	}
	if !strings.Contains(answerView, "The answer.") {
		t.Fatalf("answer frame does not show the streamed prose:\n%s", answerView)
	}
}

// TestLiveTurnIndicatorSitsOnTheAgentHeader pins the placement: while
// a turn cooks, the spinner and phase belong to the Agent header at the
// response bubble, not the chrome band after the loop track.
func TestLiveTurnIndicatorSitsOnTheAgentHeader(t *testing.T) {
	m := NewChatModel()
	m.width, m.height = 100, 24
	m.resize(100, 24)
	m.loopBeat = beatAsk
	m.loopTrack = true
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.elapsed = 400 * time.Millisecond
	m.refreshViewport()

	view := SanitizeANSI(m.View())
	var agentLine, bandLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Agent ") {
			agentLine = line
		}
		if strings.Contains(line, "▸ Ask") {
			bandLine = line
		}
	}
	wantSpinner := stripANSI(workingSpinner(1, false))
	if !strings.Contains(agentLine, "Thinking") || !strings.Contains(agentLine, wantSpinner) {
		t.Fatalf("Agent header = %q, want the live spinner %q and phase", agentLine, wantSpinner)
	}
	if strings.Contains(bandLine, "⣾") || strings.Contains(bandLine, "Thinking") {
		t.Fatalf("band = %q, the loading animation belongs on the Agent header", bandLine)
	}
}

// TestWorkingBandIsOneCompactRow pins the live band layout: one track
// row directly above the composer rule, and the working frame never
// grows the chrome — it occupies the same rows as an idle one.
func TestWorkingBandIsOneCompactRow(t *testing.T) {
	idle := NewChatModel()
	idle.width, idle.height = 80, 20
	idle.SetInput("ready")
	idleRows := len(strings.Split(strings.TrimRight(idle.View(), "\n"), "\n"))

	m := NewChatModel()
	m.width, m.height = 80, 20
	m.SetInput("ready")
	m.SetThinking(true, "Thinking...")
	m.loopBeat = beatAsk
	m.loopTrack = true
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != idleRows {
		t.Fatalf("working view = %d rows, idle = %d (chrome must not grow)", len(lines), idleRows)
	}

	idx := -1
	for i, ln := range lines {
		if strings.Contains(ln, "▸ Ask") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no thinking row in:\n%s", view)
	}
	if idx == 0 || idx == len(lines)-1 {
		t.Fatalf("working row at the pane edge, no room for the composer:\n%s", view)
	}
	if !strings.Contains(lines[idx+1], "─") {
		t.Fatalf("working band is not directly above the composer rule:\n%s", view)
	}
}

// TestWorkingSpinnerCycles pins the live-turn animation contract: the
// wheel advances once per tick, and reduced motion parks it.
func TestWorkingSpinnerCycles(t *testing.T) {
	seen := map[string]bool{}
	for tick := 0; tick < len(workingFrames); tick++ {
		seen[stripANSI(workingSpinner(tick, false))] = true
	}
	if len(seen) != len(workingFrames) {
		t.Fatalf("spinner visited %d of %d frames", len(seen), len(workingFrames))
	}
	still := stripANSI(workingSpinner(0, true))
	for _, tick := range []int{1, 4, 9} {
		if got := stripANSI(workingSpinner(tick, true)); got != still {
			t.Fatalf("reduced-motion spinner(%d) = %q, want %q", tick, got, still)
		}
	}
}
