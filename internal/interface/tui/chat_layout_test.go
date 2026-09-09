package tui

import (
	"strings"
	"testing"
)

func TestComposerStaysVisibleAtBottom(t *testing.T) {
	chat := NewChatModel()
	chat.width = 80
	chat.height = 24
	chat.SetModel("test-model")
	chat.SetInput("ready")

	for i := 0; i < 60; i++ {
		chat.AddMessage("assistant", strings.Repeat("history ", 8))
	}

	view := chat.View()
	if !strings.Contains(view, "ready") {
		t.Fatalf("composer input is not visible in rendered view")
	}
	if !strings.Contains(view, "effort") {
		t.Fatalf("composer mode line is not visible in rendered view")
	}

	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	tail := strings.Join(lines[maxLayoutInt(0, len(lines)-8):], "\n")
	if !strings.Contains(tail, "ready") || !strings.Contains(tail, "effort") {
		t.Fatalf("composer is not anchored near the bottom; tail=%q", tail)
	}
}

func TestInputAreaHeightTracksVisibleRows(t *testing.T) {
	chat := NewChatModel()

	cases := []struct {
		name  string
		input string
		rows  int
		area  int
	}{
		{name: "empty", rows: 1, area: 1 + ComposerTopPadding + 1 + ComposerBottomPadding + 1},
		{name: "single line", input: "hello", rows: 1, area: 1 + ComposerTopPadding + 1 + ComposerBottomPadding + 1},
		{name: "two lines", input: "hello\nworld", rows: 2, area: 1 + ComposerTopPadding + 2 + ComposerBottomPadding + 1},
		{name: "eight lines", input: "1\n2\n3\n4\n5\n6\n7\n8", rows: 8, area: 1 + ComposerTopPadding + 8 + ComposerBottomPadding + 1},
		{name: "capped", input: "1\n2\n3\n4\n5\n6\n7\n8\n9", rows: MaxInputRows, area: 1 + ComposerTopPadding + MaxInputRows + ComposerBottomPadding + 1 + 1}, // +1 overflow marker row
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chat.SetInput(tc.input)
			if got := chat.inputRows(); got != tc.rows {
				t.Fatalf("inputRows() = %d, want %d", got, tc.rows)
			}
			if got := chat.textarea.Height(); got != tc.rows {
				t.Fatalf("textarea.Height() = %d, want %d", got, tc.rows)
			}
			// The solid block hugs the text: border + top padding + rows +
			// the mode line row below the block.
			if got := chat.inputAreaHeight(); got != tc.area {
				t.Fatalf("inputAreaHeight() = %d, want %d", got, tc.area)
			}
		})
	}
}

func maxLayoutInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestOneLineComposerLeavesTranscriptRoom(t *testing.T) {
	chat := NewChatModel()
	chat.width = 92
	chat.height = 18
	chat.SetModel("nex-agi/nex-n2-pro:free")
	chat.SetInput("ready")

	view := chat.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if got, wantMax := len(lines), 18; got > wantMax {
		t.Fatalf("rendered lines = %d, want <= %d\n%s", got, wantMax, view)
	}
	if got, want := chat.inputAreaHeight(), 1+ComposerTopPadding+1+ComposerBottomPadding+1; got != want {
		t.Fatalf("one-line input area height = %d, want %d", got, want)
	}
	tail := strings.Join(lines[maxLayoutInt(0, len(lines)-10):], "\n")
	if !strings.Contains(tail, "ready") {
		t.Fatalf("composer input not near bottom; tail=%q", tail)
	}
}

func TestMultilineComposerHasStablePadding(t *testing.T) {
	chat := NewChatModel()
	chat.width = 80
	chat.height = 20
	chat.SetInput("one\ntwo\nthree")

	if got, want := chat.inputRows(), 3; got != want {
		t.Fatalf("inputRows() = %d, want %d", got, want)
	}
	if got, want := chat.inputAreaHeight(), 1+ComposerTopPadding+3+ComposerBottomPadding+1; got != want {
		t.Fatalf("multi-line input area height = %d, want %d", got, want)
	}
	view := chat.View()
	for _, line := range []string{"one", "two", "three"} {
		if !strings.Contains(view, line) {
			t.Fatalf("multi-line composer missing %q\n%s", line, view)
		}
	}
}

func TestWrappedLineGrowsComposerRows(t *testing.T) {
	chat := NewChatModel()
	chat.width = 80
	chat.height = 24
	chat.SetInput("one")
	// A long single sentence that soft-wraps across the editor width must
	// count as multiple visual rows, not one: the composer grows so the
	// wrapped tail is visible instead of hiding below the fold.
	longText := "The quick brown fox jumps over the lazy dog while the sun sets slowly behind the distant mountains."
	chat.SetInput(longText)

	if got := chat.inputRows(); got <= 1 {
		t.Fatalf("wrapped long line inputRows() = %d, want > 1", got)
	}
	if got := chat.textarea.Height(); got != chat.inputRows() {
		t.Fatalf("textarea.Height() = %d, want %d", got, chat.inputRows())
	}
}

func TestComposerFitsShortPane(t *testing.T) {
	// On a short terminal the composer must give way to fixed chrome and
	// the viewport floor instead of overflowing off the bottom. The editor
	// shrinks and bubbles scrolls internally; the mode line stays visible.
	chat := NewChatModel()
	chat.width = 80
	chat.height = 16
	chat.SetModel("test-model")

	// A draft long enough to want the full 8-row cap.
	draft := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10"
	chat.SetInput(draft)

	maxRows := chat.maxComposerRows()
	if got := chat.inputRows(); got > maxRows {
		t.Fatalf("inputRows() = %d exceeds pane budget %d", got, maxRows)
	}

	view := chat.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > chat.height {
		t.Fatalf("rendered lines = %d, want <= %d\n%s", len(lines), chat.height, view)
	}
	if !strings.Contains(view, "effort") {
		t.Fatalf("mode line not visible on short pane\n%s", view)
	}

	// Meanwhile the one-line composer is unaffected by the short pane:
	// it keeps its natural single-row height plus chrome.
	chat.SetInput("hi")
	if got, want := chat.inputRows(), 1; got != want {
		t.Fatalf("short-pane one-line composer inputRows() = %d, want %d", got, want)
	}
}

func TestWrappedLineFitsShortPane(t *testing.T) {
	// A long single line on a narrow short pane: the wrap count is capped
	// by the pane budget, never exceeding it even though the raw text
	// would need more rows than the pane can show.
	chat := NewChatModel()
	chat.width = 40
	chat.height = 16
	chat.SetModel("test-model")

	longText := "The quick brown fox jumps over the lazy dog while the sun sets slowly behind the distant mountains forever and ever amen."
	chat.SetInput(longText)

	maxRows := chat.maxComposerRows()
	if got := chat.inputRows(); got > maxRows {
		t.Fatalf("inputRows() = %d exceeds pane budget %d", got, maxRows)
	}

	view := chat.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > chat.height {
		t.Fatalf("rendered lines = %d, want <= %d\n%s", len(lines), chat.height, view)
	}
	if !strings.Contains(view, "effort") {
		t.Fatalf("mode line not visible on narrow short pane\n%s", view)
	}
}

func TestStatusLineStaysQuietAtNarrowWidth(t *testing.T) {
	chat := NewChatModel()
	chat.width = 42
	chat.height = 16
	chat.SetModel("openrouter/nex-agi/nex-n2-pro:free")

	view := chat.View()
	if strings.Contains(view, "Auto-saved") {
		t.Fatalf("status line should not show noisy persistence metadata\n%s", view)
	}
	if !strings.Contains(view, "effort") {
		t.Fatalf("mode line should stay visible at narrow width\n%s", view)
	}
}
