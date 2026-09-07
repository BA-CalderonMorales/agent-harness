package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Latency benchmarks for the composer and frame path at marathon
// transcript scale (the 0.3.27 session slow-down fix).
//
// History: before the group render cache + deferred refresh, every
// agent event and every timer tick rebuilt the whole transcript
// through lipgloss — O(n) per frame, quadratic per session. The probe
// numbers that drove the fix (10k events, bulk-built transcript):
//
//	first full render            ~0.8s   (one-time, was timeout)
//	per streaming frame          ~0.3ms  (was ~40ms and climbing)
//	keystroke while streaming    ~0.43ms (flat in n)
//	idle frame                   ~0.32ms (flat in n)
//
// Run: go test ./internal/interface/tui/ -bench BenchmarkComposerKeystroke -benchmem

// benchBulkTranscript builds a synthetic n-event session in bulk (the
// SetMessages shape): user turns, assistant answers, tool rows. Bulk
// because the fix targets frame cost at scale, not append speed —
// live sessions grow one message at a time at human pace.
func benchBulkTranscript(b *testing.B, n int) *ChatModel {
	//nolint
	chat := NewChatModel()
	chat.width = 100
	chat.height = 40
	chat.Focus()
	turn := 0
	for i := 0; i < n; i++ {
		switch i % 4 {
		case 0:
			turn++
			chat.messages = append(chat.messages, ChatMessage{
				ID: fmt.Sprintf("u-%d", i), Role: "user",
				Content: fmt.Sprintf("question %d about the config", i), Timestamp: time.Now(),
			})
		case 1, 2:
			var c strings.Builder
			c.WriteString(fmt.Sprintf("### Answer %d\n\n", i))
			c.WriteString(strings.Repeat("The parser walks tokens in order and updates state. ", 12))
			chat.messages = append(chat.messages, ChatMessage{
				ID: fmt.Sprintf("a-%d", i), Role: "assistant",
				Content: c.String(), Timestamp: time.Now(),
			})
		default:
			chat.messages = append(chat.messages, ChatMessage{
				ID: fmt.Sprintf("t-%d", i), Role: "tool", IsTool: true, Turn: turn,
				ToolName: "bash", ToolDisplayName: "Shell",
				ToolStatus: ToolStatusSuccess, ToolDetail: "go test ./...",
				Content: "go test ./...", Timestamp: time.Now(),
			})
		}
	}
	chat.refreshViewport() // warm: pay the first render outside the loop
	return &chat
}

// streamBurst simulates one streaming frame: a chunk, a timer tick,
// and the View that BubbleTea would paint.
func streamBurst(chat *ChatModel) {
	m, _ := chat.Update(AgentChunkMsg{Text: "chunk "})
	m, _ = m.(ChatModel).Update(timerTickMsg{time: time.Now()})
	_ = m.(ChatModel).View()
}

// BenchmarkComposerKeystroke100 measures one composer keystroke
// (Update+View) on a 100-event transcript.
func BenchmarkComposerKeystroke100(b *testing.B) {
	chat := benchBulkTranscript(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		_ = m.(ChatModel).View()
	}
}

// BenchmarkComposerKeystroke10000: the marathon case — if this is
// flat against the 100-event number, keystroke latency no longer
// scales with session length.
func BenchmarkComposerKeystroke10000(b *testing.B) {
	chat := benchBulkTranscript(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		_ = m.(ChatModel).View()
	}
}

// BenchmarkStreamFrame10000: one streaming frame (chunk + tick + View)
// at 10k events. Before the fix this cost O(transcript) per frame;
// now only the streaming tail re-renders.
func BenchmarkStreamFrame10000(b *testing.B) {
	chat := benchBulkTranscript(b, 10000)
	// Capture the returned model: Update has a value receiver, so the
	// streaming/timer state AgentStartMsg enables only persists on the
	// returned value. Without this the benchmarked frames would be
	// no-ops and the numbers meaningless.
	started, _ := chat.Update(AgentStartMsg{})
	*chat = started.(ChatModel)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		streamBurst(chat)
	}
}
