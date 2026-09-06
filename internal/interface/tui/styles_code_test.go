package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// The first cut of fitBlockCode hard-wrapped an over-wide token by
// repeatedly stripping a prefix of the wrapped output. Hardwrap returns
// the whole multi-line fold, so its stripped form carries newlines and
// could never match a prefix of the flat token: the loop spun, appending
// the fold once per iteration, and grew a live heap past 10GB until the
// OOM killer took the host. These cases pin the shapes that triggered it
// and the neighbors it must keep working.
func TestFitBlockCodeWrapsOverWideTokensWithoutSpinning(t *testing.T) {
	longToken := strings.Repeat("a", 79)

	cases := map[string]string{
		"plain long token":  longToken,
		"styled long token": "\x1b[31m" + longToken,
		"styled with reset": "\x1b[31m" + longToken + "\x1b[0m",
		"mid-token style":   "aaaa\x1b[1m" + longToken,
		"wide runes":        strings.Repeat("界", 55),
		"emoji pile":        strings.Repeat("🔥", 41),
		"osc fragment":      "]11;rgb:1919/1aa/1b1b\x1b\\" + longToken,
	}

	for name, in := range cases {
		done := make(chan string, 1)
		go func() { done <- fitBlockCode(20, in) }()
		select {
		case got := <-done:
			line := got
			if i := strings.IndexByte(got, '\n'); i >= 0 {
				line = got[:i]
			}
			if w := ansi.StringWidth(line); w > 20 {
				t.Errorf("%s: first line width %d exceeds 20 columns", name, w)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: fitBlockCode did not return (wrap loop is spinning)", name)
		}
	}
}
