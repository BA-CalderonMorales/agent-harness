package tui

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestOSCStrippingReaderRemovesBELTerminatedReplies(t *testing.T) {
	f := &oscStrippingReader{r: bytes.NewReader([]byte("hello\x1b]11;rgb:1919/1aa/1b1b\x07world"))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), "helloworld"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOSCStrippingReaderRemovesSTTerminatedReplies(t *testing.T) {
	f := &oscStrippingReader{r: bytes.NewReader([]byte("a\x1b]11;rgb:1919/1aa/1b1b\x1b\\b"))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), "ab"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOSCStrippingReaderKeepsCSISequences(t *testing.T) {
	f := &oscStrippingReader{r: bytes.NewReader([]byte("x\x1b[24;110Ry"))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), "x\x1b[24;110Ry"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOSCStrippingReaderKeepsLoneESCEscapeKeys(t *testing.T) {
	f := &oscStrippingReader{r: bytes.NewReader([]byte("a\x1bb"))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), "a\x1bb"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Chunking invariance - the filter result must not depend on where reads
// happen to split, and must match a reference stripper on the whole input.
// ---------------------------------------------------------------------------

var oscReplyRe = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

func drain(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestOSCFilterChunkingInvariance(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 60
	properties := gopter.NewProperties(parameters)

	properties.Property("stripping is invariant under byte-chunking", prop.ForAll(
		func(input string, boundary int) bool {
			if boundary < 0 || boundary > len(input) {
				return true
			}
			chunks := []string{input[:boundary], input[boundary:]}
			var f1, f2 oscStrippingReader
			f1.r = bytes.NewReader([]byte(input))
			whole := drain(t, &f1)
			var b bytes.Buffer
			for _, c := range chunks {
				f2.r = bytes.NewReader([]byte(c))
				b.WriteString(drain(t, &f2))
			}
			return whole == b.String()
		},
		gen.AnyString(),
		gen.IntRange(0, 200),
	))

	properties.Property("output matches a reference stripper on ANY input", prop.ForAll(
		func(input string) bool {
			var f oscStrippingReader
			f.r = bytes.NewReader([]byte(input))
			got := drain(t, &f)
			want := oscReplyRe.ReplaceAllString(input, "")
			return got == want
		},
		gen.AnyString(),
	))

	properties.Property("OSC spans never survive stripping", prop.ForAll(
		func(input string) bool {
			var f oscStrippingReader
			f.r = bytes.NewReader([]byte(input))
			out := drain(t, &f)
			if strings.Contains(out, "\x1b]") {
				return false
			}
			// No payload fragment of a stripped reply may peek through.
			if m := oscReplyRe.FindString(out); m != "" {
				return false
			}
			return true
		},
		gen.AnyString(),
	))

	properties.TestingRun(t)
}

// ---------------------------------------------------------------------------
// End-to-end regression: the exact OSC 11 reply a terminal sends back must
// never reach the focused composer through the real tea input pipeline.
// This test was red (composer captured "]11;rgb:1919/1aa/1b1b\\") before
// the filter existed.
// ---------------------------------------------------------------------------

func TestOSC11ReplyNeverReachesComposer(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
	}{
		{"whole ST reply", []string{"\x1b]11;rgb:1919/1aa/1b1b\x1b\\"}},
		{"BEL reply", []string{"\x1b]11;rgb:1919/1aa/1b1b\x07"}},
		{"reply split mid-payload", []string{"\x1b]11;rgb:1919/1aa", "1b1b\x1b\\"}},
		{"reply split at ST", []string{"\x1b]11;rgb:1919/1aa/1b1b\x1b", "\\"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chat := NewChatModel()
			chat.Focus()

			r, w := io.Pipe()
			p := tea.NewProgram(chat,
				tea.WithInput(&oscStrippingReader{r: r}),
				tea.WithOutput(io.Discard))

			done := make(chan error, 1)
			go func() { _, err := p.Run(); done <- err }()
			time.Sleep(100 * time.Millisecond) // let the input reader start

			for _, c := range tc.chunks {
				w.Write([]byte(c))
			}
			w.Write([]byte("\r")) // the user's Enter
			w.Close()

			p.Send(tea.Quit())
			if err := <-done; err != nil {
				t.Fatalf("program error: %v", err)
			}

			if got := chat.GetInput(); got != "" {
				t.Fatalf("composer captured %q; want empty", got)
			}
		})
	}
}
