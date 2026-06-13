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
	if !strings.Contains(view, "model: test-model") {
		t.Fatalf("composer meta line is not visible in rendered view")
	}

	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	tail := strings.Join(lines[maxLayoutInt(0, len(lines)-8):], "\n")
	if !strings.Contains(tail, "ready") || !strings.Contains(tail, "model: test-model") {
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
		{name: "empty", rows: 1, area: 5},
		{name: "single line", input: "hello", rows: 1, area: 5},
		{name: "two lines", input: "hello\nworld", rows: 2, area: 6},
		{name: "capped", input: "1\n2\n3\n4\n5", rows: 4, area: 8},
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
