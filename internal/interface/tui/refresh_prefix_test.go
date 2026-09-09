package tui

import (
	"strings"
	"testing"
	"time"
)

// TestRefreshPrefixReusePinsTranscript pins that structural prefix
// reuse paints byte-identical transcripts to the full-rebuild path:
// three mutations (append, tail change, mid-transcript change) each
// produce a lastPainted that matches a fresh full assembly of the
// same messages.
func TestRefreshPrefixReusePinsTranscript(t *testing.T) {
	m := buildSteadyTranscript(10)

	// Reference: fresh full assembly of the current state.
	full := func(mm ChatModel) string {
		var b strings.Builder
		for i := 0; i < len(mm.messages); {
			rendered, next, _ := mm.appendTurnGroupCached(mm.messages, i)
			b.WriteString(rendered)
			b.WriteString("\n\n")
			i = next
		}
		return b.String()
	}

	// 1. Append a turn: prefix path must equal full rebuild.
	m.messages = append(m.messages, ChatMessage{
		ID: "t-new", Role: "tool", IsTool: true,
		ToolName: "bash", ToolDisplayName: "Shell",
		ToolStatus: ToolStatusSuccess, ToolDetail: "echo new",
		Timestamp: time.Now(), ToolStartedAt: time.Now(),
		ToolElapsed: 100 * time.Millisecond, Turn: 10,
	})
	m.refreshDeferred()
	m.flushDeferredRefresh()
	if got, want := m.lastPainted, full(m); got != want {
		t.Fatalf("append: prefix-reuse transcript diverged")
	}

	// 2. Change the tail (a second append).
	m.messages = append(m.messages, ChatMessage{
		ID:           "a-new",
		Role:         "assistant",
		Content:      "New turn answer prose.",
		Timestamp:    time.Now(),
		ResponseTime: 900 * time.Millisecond,
		Turn:         10,
	})
	m.refreshDeferred()
	m.flushDeferredRefresh()
	if got, want := m.lastPainted, full(m); got != want {
		t.Fatalf("tail change: prefix-reuse transcript diverged")
	}

	// 3. Mid-transcript change (expand an early tool record) — the
	// fallback full-join path.
	m.expandedMessageID = "t-0-0"
	m.refreshDeferred()
	m.flushDeferredRefresh()
	if got, want := m.lastPainted, full(m); got != want {
		t.Fatalf("mid change: prefix-reuse transcript diverged")
	}
	m.expandedMessageID = ""

	// 4. Contract: format stability across a resize width change.
	m.width = 100
	m.viewport.Width = 100
	m.refreshDeferred()
	m.flushDeferredRefresh()
	if got, want := m.lastPainted, full(m); got != want {
		t.Fatalf("resize: prefix-reuse transcript diverged")
	}
}
