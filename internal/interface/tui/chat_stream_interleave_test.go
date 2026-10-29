package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// A turn that writes, runs a tool, then writes again must keep that order
// inside one assistant bubble. The loop announces a tool call at its
// position in the stream, and the TUI renders prose and tool rows as the
// parts they actually arrived in. The regression this pins: tool rows
// used to land after the prose that followed them, because the
// streaming bubble materialized before the call was known.
func TestStreamingProseAndToolRowsInterleave(t *testing.T) {
	chat := NewChatModel()
	chat.width = 120
	chat.height = 32
	chat.resize(120, 32)
	chat.focused = true

	model, _ := chat.Update(AgentStartMsg{Timestamp: time.Now()})
	chat = model.(ChatModel)
	// Age the turn past PlaceholderDelay without sleeping.
	chat.startTime = time.Now().Add(-2 * time.Second)

	model, _ = chat.Update(AgentChunkMsg{Text: "Before the call.\n", Timestamp: time.Now()})
	chat = model.(ChatModel)
	model, _ = chat.Update(AgentToolStartMsg{
		ToolID: "t-interleave", ToolName: "bash", DisplayName: "Shell",
		Input: map[string]any{"command": "echo interleave"},
	})
	chat = model.(ChatModel)

	// The call is visible the moment it starts: an LLM gone quiet while
	// the command runs must not leave the row outside the bubble until
	// the next chunk or the completion event.
	assertInOrder(t, "live frame while the call runs", chat.lastPainted, "Before the call.", "echo interleave")
	assertInsideAssistantBubble(t, chat.lastPainted, "echo interleave")
	if got := strings.Count(ansi.Strip(chat.lastPainted), "echo interleave"); got != 1 {
		t.Fatalf("tool row appears %d times while the call runs, want once:\n%s", got, chat.lastPainted)
	}

	model, _ = chat.Update(AgentToolDoneMsg{ToolID: "t-interleave", Success: true})
	chat = model.(ChatModel)

	// When the call completes the row settles in place: the prose that
	// preceded it stays above it, and the row lives inside the assistant
	// bubble — mid-answer is where the call happened, not after the reply.
	assertInOrder(t, "live frame before the second delta", chat.lastPainted, "Before the call.", "echo interleave")
	assertInsideAssistantBubble(t, chat.lastPainted, "echo interleave")
	if got := strings.Count(ansi.Strip(chat.lastPainted), "echo interleave"); got != 1 {
		t.Fatalf("tool row appears %d times in the live frame, want once:\n%s", got, chat.lastPainted)
	}

	// The next delta continues after the call, still in the same bubble.
	model, _ = chat.Update(AgentChunkMsg{Text: "After the call.\n", Timestamp: time.Now()})
	chat = model.(ChatModel)
	chat.startTime = time.Now().Add(-3 * time.Second)
	model, _ = chat.Update(timerTickMsg{time: time.Now()})
	chat = model.(ChatModel)

	live := chat.streamingAssistant()
	if live == nil {
		t.Fatal("no streaming assistant while deltas are arriving")
	}
	assertInterleavedParts(t, "streaming", *live)
	assertInOrder(t, "live frame after the second delta", chat.lastPainted, "Before the call.", "echo interleave", "After the call.")
	assertInsideAssistantBubble(t, chat.lastPainted, "echo interleave")

	// The turn ends: the answer finalizes in place, same order.
	model, _ = chat.Update(AgentDoneMsg{
		FullResponse: "Before the call.\nAfter the call.\n",
		Timestamp:    time.Now(),
	})
	chat = model.(ChatModel)

	final := latestAssistantMessage(t, chat)
	if final.Thinking {
		t.Fatal("finalized assistant message still marked Thinking")
	}
	assertInterleavedParts(t, "final", final)
	assertInOrder(t, "final frame", chat.lastPainted, "Before the call.", "echo interleave", "After the call.")

	rendered := ansi.Strip(chat.renderAssistantMessage(final))
	assertInOrder(t, "final assistant block", rendered, "Before the call.", "echo interleave", "After the call.")
}

// assertInterleavedParts checks the turn's structure: prose, the call,
// then the prose that followed it.
func assertInterleavedParts(t *testing.T, stage string, msg ChatMessage) {
	t.Helper()
	if len(msg.Parts) != 3 {
		t.Fatalf("%s parts = %d (%+v), want 3 (text/tool/text)", stage, len(msg.Parts), msg.Parts)
	}
	if got := strings.TrimSpace(msg.Parts[0].Text); got != "Before the call." {
		t.Fatalf("%s first prose part = %q", stage, got)
	}
	if msg.Parts[1].ToolID != "t-interleave" {
		t.Fatalf("%s tool part = %q, want t-interleave", stage, msg.Parts[1].ToolID)
	}
	if got := strings.TrimSpace(msg.Parts[2].Text); got != "After the call." {
		t.Fatalf("%s last prose part = %q", stage, got)
	}
}

// assertInsideAssistantBubble checks the row carrying detail renders in
// the assistant bubble's gutter, not as a separate block beneath it.
func assertInsideAssistantBubble(t *testing.T, frame, detail string) {
	t.Helper()
	for _, line := range strings.Split(ansi.Strip(frame), "\n") {
		if !strings.Contains(line, detail) {
			continue
		}
		if !strings.HasPrefix(strings.TrimLeft(line, " "), "│") {
			t.Fatalf("tool row is outside the assistant bubble: %q\n%s", line, frame)
		}
		return
	}
	t.Fatalf("frame has no tool row for %q:\n%s", detail, frame)
}

func latestAssistantMessage(t *testing.T, chat ChatModel) ChatMessage {
	t.Helper()
	for i := len(chat.messages) - 1; i >= 0; i-- {
		if chat.messages[i].Role == "assistant" {
			return chat.messages[i]
		}
	}
	t.Fatal("no assistant message in the transcript")
	return ChatMessage{}
}

func assertInOrder(t *testing.T, stage, text string, needles ...string) {
	t.Helper()
	if !appearsInOrder(text, needles...) {
		t.Fatalf("%s lost the order %q:\n%s", stage, needles, text)
	}
}

func appearsInOrder(text string, needles ...string) bool {
	text = ansi.Strip(text)
	at := 0
	for _, needle := range needles {
		next := strings.Index(text[at:], needle)
		if next < 0 {
			return false
		}
		at += next + len(needle)
	}
	return true
}
