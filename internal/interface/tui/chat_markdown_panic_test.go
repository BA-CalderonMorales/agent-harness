package tui

import (
	"strings"
	"testing"
)

// Reproduces the glamour panic from the diag log:
// "index out of range [-2] (width=108 content=\"consider what it means to be in this harnes\")"
func TestGlamourPlainProsePanic(t *testing.T) {
	cases := []struct {
		width int
		text  string
	}{
		{108, "consider what it means to be in this harnes"},
		{108, "/mode"},
		{108, "talk to me about this codebase"},
		{106, "Yo! What are we working on today?"},
		{106, strings.Repeat("word ", 300)},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			// renderMarkdown recovers internally; a panic means the
			// recovery path itself is fine but we want to know it fired.
			got := renderMarkdown(c.text, c.width)
			if got == "" {
				t.Fatal("empty render")
			}
		})
	}
}
