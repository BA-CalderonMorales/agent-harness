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
		{name: "capped", input: "1\n2\n3\n4\n5", rows: 4, area: 1 + ComposerTopPadding + 4 + ComposerBottomPadding + 1},
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
