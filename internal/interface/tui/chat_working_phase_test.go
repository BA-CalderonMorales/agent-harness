package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The working line names the phase the turn is actually in. These
// tests pin the transitions a user watches for: waiting, writing,
// running a tool, and waiting for approval.
func TestWorkingPhaseFollowsTheLiveTurn(t *testing.T) {
	var m ChatModel
	m.width = 80
	m.thinking = true

	m.thinkingText = "Connecting to local..."
	m.thinkingIsStatus = true
	if got := m.workingPhaseLabel(); got != "Connecting" {
		t.Fatalf("phase = %q, want Connecting", got)
	}

	m.thinkingText = "Thinking..."
	if got := m.workingPhaseLabel(); got != "Thinking" {
		t.Fatalf("phase = %q, want Thinking while the request is in flight", got)
	}

	m.thinkingText = "The user wants the repo layout."
	m.thinkingIsStatus = false
	if got := m.workingPhaseLabel(); got != "Thinking" {
		t.Fatalf("phase = %q, want Thinking while reasoning streams", got)
	}

	m.writing = true
	if got := m.workingPhaseLabel(); got != "Writing" {
		t.Fatalf("phase = %q, want Writing while prose arrives", got)
	}

	m.writing = false
	m.currentToolMsg = &ChatMessage{
		ID: "tool-1", Role: "tool", IsTool: true,
		ToolName: "read", ToolDisplayName: "Read File", ToolStatus: ToolStatusRunning,
	}
	if got := m.workingPhaseLabel(); got != "Running Read File" {
		t.Fatalf("phase = %q, want the running tool class", got)
	}

	m.loopBeat = beatReview
	m.loopTrack = true
	line := m.workingStatusLine(0)
	if !strings.Contains(line, "Review") || !strings.Contains(line, "waiting for you") {
		t.Fatalf("approval line = %q, want the review beat", line)
	}
}

// Esc stops a running turn. The composer hint must say so instead of
// advertising navigation that reads as "nothing will be interrupted".
func TestEscHintStopsDuringATurn(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.focused = true
	m.thinking = true
	if line := m.renderModeLine(); !strings.Contains(line, `"Esc" to stop`) {
		t.Fatalf("live mode line = %q, want the stop hint", line)
	}

	m.thinking = false
	if line := m.renderModeLine(); !strings.Contains(line, `"Esc" to navigate`) {
		t.Fatalf("idle mode line = %q, want the navigate hint", line)
	}
}

// The working band shrinks the viewport while a turn runs. A frame that
// keeps the idle-height scroll offset hides the newest lines under the
// composer — exactly while the answer is arriving.
func TestStreamedTailStaysVisibleAboveTheWorkingBand(t *testing.T) {
	m := NewChatModel()
	m.width, m.height = 100, 20
	for i := 0; i < 12; i++ {
		m.AddMessage("user", fmt.Sprintf("filler line %d", i))
	}
	m.loopTrack = true

	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.startTime = time.Now().Add(-2 * time.Second)
	model, _ = m.Update(timerTickMsg{})
	m = model.(ChatModel)

	model, _ = m.Update(AgentChunkMsg{Text: strings.Repeat("streamed words ", 20) + "STREAM-TAIL", Timestamp: time.Now()})
	m = model.(ChatModel)
	m.startTime = time.Now().Add(-3 * time.Second)
	model, _ = m.Update(timerTickMsg{})
	m = model.(ChatModel)

	view := m.View()
	if !strings.Contains(view, "STREAM-TAIL") {
		t.Fatalf("streamed tail clipped by the working band:\n%s", view)
	}
}

// The live bubble grows a caret while prose is arriving: the answer is
// being written, not waiting. When the turn settles, the caret goes.
func TestStreamingBubbleShowsTheWritingCaret(t *testing.T) {
	m := NewChatModel()
	m.width = 100
	m.thinking = true
	m.streaming = true
	m.writing = true
	m.updateOrCreateStreamingMessage("A partial answer")
	msg := m.streamingAssistant()
	if msg == nil {
		t.Fatal("no streaming assistant")
	}
	if got := m.renderAssistantMessage(*msg); !strings.Contains(got, "▍") {
		t.Fatalf("streaming bubble = %q, want the writing caret", got)
	}

	m.writing = false
	if got := m.renderAssistantMessage(*msg); strings.Contains(got, "▍") {
		t.Fatalf("settled bubble = %q, want no writing caret", got)
	}
}
