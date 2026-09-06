package tui

import (
	"fmt"
	"strings"
	"testing"
)

// benchTranscript builds a synthetic 500-message conversation with
// representative content: short user turns and long markdown-heavy
// assistant answers (the shape of real marathon sessions).
func benchTranscript(n int) *ChatModel {
	//nolint
	chat := NewChatModel()
	chat.width = 100
	chat.height = 40
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			chat.AddMessage("user", fmt.Sprintf("question %d about the config", i))
			continue
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("### Answer %d\n\n", i))
		b.WriteString("Here is the explanation with **bold** and `inline code`.\n\n")
		b.WriteString(strings.Repeat("The parser walks tokens in order and updates state. ", 12))
		b.WriteString("\n\n```go\nfunc handle(w *Writer) error {\n\treturn w.Flush()\n}\n```\n")
		chat.AddMessage("assistant", b.String())
	}
	return &chat
}

// BenchmarkChatRepaint500 measures one full transcript repaint at 500
// messages. Before the render cache this re-ran glamour for every
// message every frame; after, completed messages hit the cache and only
// the streaming tail re-renders.
//
// Run: go test ./internal/interface/tui/ -bench BenchmarkChatRepaint -benchmem
func BenchmarkChatRepaint500(b *testing.B) {
	chat := benchTranscript(500)
	chat.refreshViewport() // warm: first build pays the uncached cost
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chat.refreshViewportWithFollow(true)
	}
}

// BenchmarkChatRepaint500Cold forces the uncached path by clearing the
// cache each iteration — the "before" number for comparison.
func BenchmarkChatRepaint500Cold(b *testing.B) {
	chat := benchTranscript(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderCacheStore = newRenderCache()
		chat.refreshViewportWithFollow(true)
	}
}

// TestRenderCacheByteIdentical: cached output must be byte-identical to
// the uncached render across widths and content shapes (golden check
// for the no-visual-diff constraint).
func TestRenderCacheByteIdentical(t *testing.T) {
	contents := []string{
		"plain text answer",
		"# Heading\n\nParagraph with **bold**, *italic*, and `code`.\n",
		"```go\nfunc main() {}\n```\ntrailing prose",
		"line one\nline two\nline three",
		strings.Repeat("long flowing paragraph text. ", 50),
		"unicode: café naïve 日本語 ✓",
	}
	// Empty and whitespace-only content never reach the render paths:
	// renderMarkdown early-returns them unchanged, before the cache.
	for _, empty := range []string{"", "   \n\t  "} {
		if got := renderMarkdown(empty, 80); got != empty {
			t.Fatalf("empty-content early return changed: %q -> %q", empty, got)
		}
	}
	widths := []int{40, 60, 80, 100, 120}
	for _, w := range widths {
		for _, content := range contents {
			want := renderMarkdownUncached(content, w)
			renderCacheStore = newRenderCache()
			got := renderMarkdown(content, w)
			if got != want {
				t.Fatalf("cached != uncached at width %d, content %q:\n got %q\nwant %q", w, content, got, want)
			}
			// Second call must hit the cache and return the same bytes.
			got2 := renderMarkdown(content, w)
			if got2 != want {
				t.Fatalf("cache hit diverged at width %d: %q vs %q", w, got2, want)
			}
		}
	}
}

// TestRenderCacheWidthInvalidation: re-rendering the same content at a
// new width replaces the old entry — the stale width's output must
// never be served again at the new width.
func TestRenderCacheWidthInvalidation(t *testing.T) {
	renderCacheStore = newRenderCache()
	content := strings.Repeat("wrap this text across the bubble width, please ", 10)
	a := renderMarkdown(content, 40)
	b := renderMarkdown(content, 80)
	if _, ok := renderCacheStore.get(content, 40); ok {
		t.Fatalf("stale width-40 entry survived a width-80 re-render")
	}
	if got, _ := renderCacheStore.get(content, 80); got != b {
		t.Fatalf("width-80 entry missing or wrong")
	}
	_ = a // width-40 output was correct at render time; entry now replaced
}

// TestRenderCacheBounded: LRU eviction keeps the map within capacity.
func TestRenderCacheBounded(t *testing.T) {
	renderCacheStore = newRenderCache()
	for i := 0; i < renderCacheCapacity+100; i++ {
		renderCacheStore.put(fmt.Sprintf("content-%d", i), 80, "out")
	}
	if got := len(renderCacheStore.ents); got > renderCacheCapacity {
		t.Fatalf("cache grew to %d entries, cap %d", got, renderCacheCapacity)
	}
}

// TestRenderCachePanicRecoveryComposes: the panic-recovery path in
// renderMarkdown must still return the raw content when the renderer
// panics, even through the cache wrapper (renderer eviction composes).
func TestRenderCachePanicRecoveryComposes(t *testing.T) {
	renderCacheStore = newRenderCache()
	// Feed content designed to stress the renderer; recovery returns
	// input on panic. Either way cache and uncached must agree.
	content := "[![img](x)](y) <not-a-tag> | broken | table |\n--- | --- |\n| a"
	want := renderMarkdownUncached(content, 80)
	got := renderMarkdown(content, 80)
	if got != want {
		t.Fatalf("recovery path diverged:\n got %q\nwant %q", got, want)
	}
}
