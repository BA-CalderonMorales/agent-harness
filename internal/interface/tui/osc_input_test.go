package tui

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/term"
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
// term.File identity - tea only enters raw mode when the input implements
// term.File AND the fd is a terminal. The stripped wrapper must keep that
// identity so j/k/Tab stay unbuffered; the raw-mode gate runs on Fd while
// the bytes still flow through the filtered Read.
// ---------------------------------------------------------------------------

func TestOSCStrippingReaderKeepsTerminalIdentity(t *testing.T) {
	f := newOSCStrippingReader(os.Stdin)
	if _, ok := any(f).(term.File); !ok {
		t.Fatal("stripped input no longer satisfies term.File; tea will skip raw mode")
	}
	if f.Fd() != os.Stdin.Fd() {
		t.Fatalf("Fd() = %d, want stdin fd %d", f.Fd(), os.Stdin.Fd())
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing the stdin wrapper must be a no-op, got err: %v", err)
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

	// Non-OSC input (ESC keypresses, CSI sequences, plain text) is
	// byte-transparent regardless of chunking: the filter must never
	// reorder, merge or drop keystrokes.
	properties.Property("non-OSC input is byte-transparent under any chunking", prop.ForAll(
		func(input string, boundary int) bool {
			if boundary < 0 || boundary > len(input) {
				return true
			}
			if strings.Contains(input, "\x1b]") {
				return true // OSC inputs are covered below
			}
			var whole oscStrippingReader
			whole.r = bytes.NewReader([]byte(input))
			if got := drain(t, &whole); got != input {
				return false
			}
			chunks := []string{input[:boundary], input[boundary:]}
			var b bytes.Buffer
			for _, c := range chunks {
				var f oscStrippingReader
				f.r = bytes.NewReader([]byte(c))
				b.WriteString(drain(t, &f))
			}
			return b.String() == input
		},
		gen.AnyString(),
		gen.IntRange(0, 200),
	))

	// Whole-input stripping matches the reference stripper when the
	// input arrives as a single stream (the way terminal replies do).
	properties.Property("single-stream output matches a reference stripper on ANY input", prop.ForAll(
		func(input string) bool {
			var f oscStrippingReader
			f.r = bytes.NewReader([]byte(input))
			got := drain(t, &f)
			want := oscReplyRe.ReplaceAllString(input, "")
			return got == want
		},
		gen.AnyString(),
	))

	// No stripped input may leave a complete OSC reply in the stream.
	properties.Property("complete OSC spans never survive", prop.ForAll(
		func(input string) bool {
			var f oscStrippingReader
			f.r = bytes.NewReader([]byte(input))
			out := drain(t, &f)
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
// ESC keypress regression - the reason the filter exists is OSC replies
// landing in the composer, but ESC keypresses are real input and must
// reach tea. These pin the behavior the TUI depends on: ESC works alone,
// ESC then a key works in any chunking, and no chunking ever panics.
// ---------------------------------------------------------------------------

func TestOSCStrippingReaderDeliversLoneESCAtEndOfStream(t *testing.T) {
	f := &oscStrippingReader{r: bytes.NewReader([]byte("a\x1b"))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), "a\x1b"; got != want {
		t.Fatalf("got %q, want %q: a trailing ESC keypress must survive", got, want)
	}
}

func TestOSCStrippingReaderDeliversESCThenKeyAcrossChunks(t *testing.T) {
	var f oscStrippingReader
	f.r = bytes.NewReader([]byte("\x1b"))
	if got := drain(t, &f); got != "\x1b" {
		t.Fatalf("chunk 1 = %q, want %q", got, "\x1b")
	}
	f.r = bytes.NewReader([]byte("q"))
	if got := drain(t, &f); got != "q" {
		t.Fatalf("chunk 2 = %q, want %q", got, "q")
	}
}

// The exact byte pattern that used to panic: ESC (settings cancel) then
// an arrow key in the same read chunk. 3 input bytes expand to 4 output
// bytes; the writer must spill instead of writing past the buffer.
func TestOSCStrippingReaderNeverPanicsOnESCThenArrowKey(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	p := make([]byte, 3)
	f := &oscStrippingReader{r: bytes.NewReader([]byte("\x1b\x1b[B"))}
	got := ""
	for {
		n, err := f.Read(p)
		got += string(p[:n])
		if err != nil {
			break
		}
	}
	if want := "\x1b\x1b[B"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ESC then a single key byte in a 1-byte read used to panic with
// index out of range [1] with length 1.
func TestOSCStrippingReaderNeverPanicsOnSmallBuffers(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	p := make([]byte, 1)
	f := &oscStrippingReader{r: bytes.NewReader([]byte("\x1b:"))}
	got := ""
	for {
		n, err := f.Read(p)
		got += string(p[:n])
		if err != nil {
			break
		}
	}
	if want := "\x1b:"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// End-to-end regression (the exact reply that was reported as literal text
// in the composer) at the byte layer the app actually consumes: the input
// stream the filter emits must never contain an OSC fragment. This test was
// red - the composer captured "]11;rgb:1919/1aa/1b1b\\" - before the filter
// existed. Kept at the byte layer on purpose: driving the full tea event
// loop through an io.Pipe is racy and adds no coverage the unit and
// property tests above do not already pin.
// ---------------------------------------------------------------------------

func TestOSC11ReplyNeverSurvivesStripping(t *testing.T) {
	reply := "\x1b]11;rgb:1919/1aa/1b1b\x1b\\"
	text := "hey"
	f := &oscStrippingReader{r: bytes.NewReader([]byte(reply + text))}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(out), text; got != want {
		t.Fatalf("stripped stream = %q, want %q: the reply must vanish, the keystrokes must survive", got, want)
	}
	if strings.Contains(string(out), "]11;") {
		t.Fatalf("OSC fragment survived: %q", out)
	}
}
