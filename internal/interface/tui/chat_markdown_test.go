package tui

import (
	"strings"
	"testing"
)

// The chat renders on the user's own terminal background. The markdown
// pipeline must therefore never emit a background-color escape, and
// must wrap at the bubble width.
func TestRenderMarkdownNeverPaintsBackground(t *testing.T) {
	content := strings.Join([]string{
		"# Heading One",
		"",
		"Body with **bold**, *italics*, `inline code` and a [link](https://example.com/path).",
		"",
		"> quoted thought",
		"",
		"- first item",
		"- second item",
		"",
		"```go",
		"func main() { println(\"hi\") }",
		"```",
	}, "\n")

	for _, width := range []int{40, 80, 120} {
		out := renderMarkdown(content, width)
		if strings.Contains(out, "\x1b[48;") {
			t.Fatalf("width %d: output paints a background:\n%q", width, out)
		}
		for _, line := range strings.Split(out, "\n") {
			stripped := stripANSI(line)
			if len([]rune(stripped)) > width+2 {
				t.Fatalf("width %d: unwrapped line %q exceeds width", width, stripped)
			}
		}
		// Structure assertions run on ANSI-stripped text: glamour wraps
		// individual words in SGR runs, so raw ContainSubstring lies.
		strippedOut := stripANSI(out)
		if !strings.Contains(strippedOut, "Heading One") || !strings.Contains(strippedOut, "first item") {
			t.Fatalf("width %d: structure lost:\n%s", width, out)
		}
	}
}

// stripANSI lives in settings_layout_test.go (shared test helper).

func TestRenderMarkdownPlainTextFallback(t *testing.T) {
	out := renderMarkdown("", 80)
	if out != "" {
		t.Fatalf("empty content = %q, want empty", out)
	}
}
