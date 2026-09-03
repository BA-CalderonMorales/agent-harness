package tui

import (
	"strings"
	"testing"
)

func TestNormalizeSoftBreaks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "model hard-wrap collapses into one paragraph",
			in:   "This repo is **agent-harness**, a terminal-based AI coding agent\nthat connects to LLM providers.",
			want: "This repo is **agent-harness**, a terminal-based AI coding agent that connects to LLM providers.",
		},
		{
			name: "paragraph break survives",
			in:   "First paragraph.\n\nSecond paragraph.",
			want: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name: "fenced code keeps newlines",
			in:   "Before:\n```go\nfmt.Println(\"a\")\nfmt.Println(\"b\")\n```\nAfter.",
			want: "Before:\n```go\nfmt.Println(\"a\")\nfmt.Println(\"b\")\n```\nAfter.",
		},
		{
			name: "table rows keep newlines",
			in:   "See:\n\n| a | b |\n|---|---|\n| 1 | 2 |",
			want: "See:\n\n| a | b |\n|---|---|\n| 1 | 2 |",
		},
		{
			name: "list items keep newlines",
			in:   "Items:\n- one\n- two",
			want: "Items:\n- one\n- two",
		},
		{
			name: "prose after list stays separate then joins own paragraph",
			in:   "- one\n- two\nTrailing prose\ncontinues here.",
			want: "- one\n- two\nTrailing prose continues here.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeSoftBreaks(c.in); got != c.want {
				t.Errorf("normalizeSoftBreaks() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRenderMarkdownParagraphSpacing(t *testing.T) {
	out := renderMarkdown("First paragraph.\n\nSecond paragraph.", 60)
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected multi-line output, got %q", out)
	}
}
