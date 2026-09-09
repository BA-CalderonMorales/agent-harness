package tui

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestLiveToolRowInBubbleAfterPlaceholder pins the live-visibility
// contract (goal 0.3.29): once a turn's bubble exists, a tool call
// that starts mid-turn nests inside the bubble immediately — not on
// the next chunk. An LLM gone quiet while a command runs must leave
// the call visible in the transcript.
func TestLiveToolRowInBubbleAfterPlaceholder(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.placeholderPending = false // the tick fired; bubble materialized
	m.startTime = time.Now().Add(-2 * time.Second)
	model, _ = m.Update(AgentChunkMsg{Text: "Investigating the issue now", Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-live", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo hello"}})
	m = model.(ChatModel)
	m.refreshDeferred()
	m.flushDeferredRefresh()

	msg := m.streamingAssistant()
	if msg == nil {
		t.Fatal("no streaming assistant")
	}
	found := false
	for _, p := range msg.Parts {
		if p.ToolID == "t-live" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tool call not part of the streaming bubble's parts: %+v", msg.Parts)
	}
	if !strings.Contains(m.lastPainted, "echo hello") {
		t.Fatalf("running command not visible in the transcript")
	}
}

// TestLiveToolRowMaterializesPlaceholder pins the worst case: a tool
// that starts INSIDE the placeholder window (fast model, instant
// command, no chunk yet). The streaming assistant must materialize at
// that moment and the running row must nest inside it — the user must
// never watch the working line spin over an empty transcript.
func TestLiveToolRowMaterializesPlaceholder(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	// No chunk, placeholder still pending — tool starts immediately.
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-early", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo early"}})
	m = model.(ChatModel)
	m.refreshDeferred()
	m.flushDeferredRefresh()

	if m.currentStreamingAssistantID == "" {
		t.Fatal("streaming assistant did not materialize on tool start")
	}
	msg := m.streamingAssistant()
	found := false
	for _, p := range msg.Parts {
		if p.ToolID == "t-early" {
			found = true
		}
	}
	if !found {
		t.Fatalf("early tool call not part of the bubble's parts: %+v", msg.Parts)
	}
	if !strings.Contains(m.lastPainted, "echo early") {
		t.Fatalf("early running command not visible in the transcript")
	}
	if !strings.Contains(m.lastPainted, "→") {
		t.Fatalf("running glyph (→) not shown for the in-flight call")
	}
}

// TestLiveToolRowFinalizesInPlace pins the completion flip: when the
// call settles, the running row inside the bubble flips to its
// settled state (✓ glyph, elapsed) without leaving the bubble.
func TestLiveToolRowFinalizesInPlace(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.placeholderPending = false
	m.startTime = time.Now().Add(-2 * time.Second)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-live", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo hello"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t-live", Success: true})
	m = model.(ChatModel)
	m.refreshDeferred()
	m.flushDeferredRefresh()

	if !strings.Contains(m.lastPainted, "echo hello") {
		t.Fatal("finalized command dropped from the transcript")
	}
	// The turn finalized (streaming assistant cleared by design), so
	// assert on the settled transcript: the ✓ glyph replaces → for the
	// call, and the row stays inside the turn block.
	if strings.Contains(m.lastPainted, "→ echo hello") {
		t.Fatal("settled call still renders the running glyph")
	}
	if !strings.Contains(m.lastPainted, "✓") {
		t.Fatal("settled call missing the ✓ glyph")
	}
}

func TestLiveToolStartIsExactlyOnce(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	for i := 0; i < 2; i++ {
		model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
		m = model.(ChatModel)
		// Keep the same turn active for the duplicate notification.
		if i == 0 {
			model, _ = m.Update(AgentToolStartMsg{ToolID: "same", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo once"}})
			m = model.(ChatModel)
			model, _ = m.Update(AgentToolStartMsg{ToolID: "same", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo once"}})
			m = model.(ChatModel)
			break
		}
	}
	count := 0
	for _, msg := range m.messages {
		if msg.IsTool && msg.ID == "same" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate start created %d transcript rows, want 1", count)
	}
	if got := strings.Count(m.lastPainted, "echo once"); got != 1 {
		t.Fatalf("duplicate start painted command %d times, want 1:\n%s", got, m.lastPainted)
	}
}

func TestProviderErrorSettlesLiveTool(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-error", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo fail"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentErrorMsg{Error: errors.New("provider stopped")})
	m = model.(ChatModel)
	for _, msg := range m.messages {
		if msg.ID == "t-error" {
			if msg.ToolStatus != ToolStatusError {
				t.Fatalf("tool status after provider error = %q, want error", msg.ToolStatus)
			}
		}
	}
	if strings.Contains(m.lastPainted, "→ echo fail") {
		t.Fatalf("provider error left a running tool row:\n%s", m.lastPainted)
	}
	if !strings.Contains(m.lastPainted, "✗") {
		t.Fatalf("provider error did not render an error glyph:\n%s", m.lastPainted)
	}
}

func TestProviderErrorIgnoresLateToolResult(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-error-late", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo fail"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentErrorMsg{Error: errors.New("provider stopped")})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolDoneMsg{ToolID: "t-error-late", Success: true})
	m = model.(ChatModel)

	for _, msg := range m.messages {
		if msg.ID == "t-error-late" && msg.ToolStatus != ToolStatusError {
			t.Fatalf("late result changed provider-error tool status to %q", msg.ToolStatus)
		}
	}
}

func TestCancelSettlesLiveTool(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	model, _ = m.Update(AgentToolStartMsg{ToolID: "t-cancel", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": "echo cancel"}})
	m = model.(ChatModel)
	model, _ = m.Update(AgentCancelMsg{})
	m = model.(ChatModel)
	for _, msg := range m.messages {
		if msg.ID == "t-cancel" && msg.ToolStatus != ToolStatusError {
			t.Fatalf("cancelled tool status = %q, want error", msg.ToolStatus)
		}
	}
	if strings.Contains(m.lastPainted, "→ echo cancel") {
		t.Fatalf("cancel left a running tool row:\n%s", m.lastPainted)
	}
}

func TestAdjacentToolClickRangesUseBlockOffsets(t *testing.T) {
	for _, width := range []int{80, 120, 160} {
		m := NewChatModel()
		m.width = width
		m.height = 40
		m.messages = []ChatMessage{
			{ID: "tool-a", Role: "tool", IsTool: true, ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, ToolStartedAt: time.Now(), ToolElapsed: time.Second, ToolDetail: "echo a"},
			{ID: "tool-b", Role: "tool", IsTool: true, ToolName: "bash", ToolDisplayName: "bash", ToolStatus: ToolStatusSuccess, ToolStartedAt: time.Now(), ToolElapsed: time.Second, ToolDetail: "echo b"},
		}
		m.refreshViewportWithFollow(true)
		if len(m.clickIndex) != 1 {
			t.Fatalf("width %d: got %d click ranges for adjacent collapsed run, want 1", width, len(m.clickIndex))
		}
		if m.clickIndex[0].start != 0 {
			t.Fatalf("width %d: first click range starts at %d, want 0", width, m.clickIndex[0].start)
		}

		m.messages = append(m.messages,
			ChatMessage{ID: "plain", Role: "user", Content: "separates blocks"},
			ChatMessage{ID: "tool-c", Role: "tool", IsTool: true, ToolName: "read", ToolDisplayName: "read", ToolStatus: ToolStatusSuccess, ToolStartedAt: time.Now(), ToolElapsed: time.Second, ToolDetail: "file"},
		)
		m.refreshViewportWithFollow(true)
		if len(m.clickIndex) != 2 || m.clickIndex[1].start <= m.clickIndex[0].start {
			t.Fatalf("width %d: click range lost its offset after adjacent block: %+v", width, m.clickIndex)
		}
	}
}
