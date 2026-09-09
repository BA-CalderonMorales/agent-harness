package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// buildSteadyTranscript builds a realistic marathon transcript: T
// turns, each with 3 tool calls (all bash) and a prose answer, all
// finalized (the steady state between turns).
func buildSteadyTranscript(turns int) ChatModel {
	m := NewChatModel()
	m.width = 120
	m.viewport.Width = 120
	m.height = 40
	m.focused = true
	for turn := 0; turn < turns; turn++ {
		for tl := 0; tl < 3; tl++ {
			id := fmt.Sprintf("t-%d-%d", turn, tl)
			m.messages = append(m.messages, ChatMessage{
				ID: id, Role: "tool", IsTool: true,
				ToolName: "bash", ToolDisplayName: "Shell",
				ToolStatus:    ToolStatusSuccess,
				ToolDetail:    fmt.Sprintf("echo step-%d-%d && ls -la", turn, tl),
				Timestamp:     time.Now().Add(-time.Duration(turns-turn) * time.Minute),
				ToolStartedAt: time.Now().Add(-time.Duration(turns-turn) * time.Minute),
				ToolElapsed:   250 * time.Millisecond,
				Turn:          turn,
			})
		}
		m.messages = append(m.messages, ChatMessage{
			ID:           fmt.Sprintf("a-%d", turn),
			Role:         "assistant",
			Content:      fmt.Sprintf("Turn %d completed the work: files checked, steps verified, and the summary is here. This prose gives the bubble something realistic to wrap.", turn),
			Timestamp:    time.Now().Add(-time.Duration(turns-turn) * time.Minute),
			ResponseTime: 1200 * time.Millisecond,
			Turn:         turn,
		})
	}
	m.refreshViewportWithFollow(true)
	return m
}

// TestBenchBuildSanity builds the transcript once so the benchmark
// helper stays covered by `go test` (benchmarks don't run in CI).
func TestBenchBuildSanity(t *testing.T) {
	m := buildSteadyTranscript(5)
	if len(m.messages) != 20 {
		t.Fatalf("built %d messages, want 20", len(m.messages))
	}
	if !strings.Contains(m.lastPainted, "step-0-0") {
		t.Fatal("transcript missing first tool row")
	}
}

// BenchmarkRefreshSteady measures a full refreshViewportWithFollow on
// an unchanged transcript — the per-frame cost the tick timer pays.
func BenchmarkRefreshSteady(b *testing.B) {
	m := buildSteadyTranscript(50) // 200 messages
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.refreshViewportWithFollow(false)
	}
}

// BenchmarkRefreshSteadyLarge scales the transcript to 500 turns
// (2000 messages) to expose the O(n) growth.
func BenchmarkRefreshSteadyLarge(b *testing.B) {
	m := buildSteadyTranscript(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.refreshViewportWithFollow(false)
	}
}
