package tui

import (
	"testing"
)

// TestCompactCommandForWidthNeverPanics pins the slice-bounds panics that
// crashed the app when a long tool command (e.g. the raw input map of
// ls_recursive) flowed through the width-aware truncator: cmd[:97] on a
// 95-byte string and a negative index into the path base name.
func TestCompactCommandForWidthNeverPanics(t *testing.T) {
	longInput := "map[depth:3 path:/mnt/c/Users/bacm6/OneDrive/Desktop/Life/repositories/working/agent-harness]"
	cases := []struct {
		cmd    string
		maxLen int
	}{
		{longInput, 100}, // raw tool input map, len 95 -> was cmd[:97]
		{longInput, 40},  // compact path with a short budget
		{"run task ./scripts/deep/deeper/deepest/builder --verbose", 20},
		{"cat /a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z", 12},
		{"short", 2},
		{"short", 3},
		{"short", 4},
		{"fits fine", 50},
		{"", 10},
		{"abc", 0},
		{"abc", -5},
	}
	for _, c := range cases {
		out := compactCommandForWidth(c.cmd, c.maxLen)
		if len(out) > len(c.cmd) {
			t.Fatalf("compactCommandForWidth(%q, %d) expanded to %d bytes", c.cmd, c.maxLen, len(out))
		}
		if c.maxLen > 0 && c.maxLen <= 3 && len(out) > c.maxLen {
			t.Fatalf("compactCommandForWidth(%q, %d) = %q exceeds the hard cap", c.cmd, c.maxLen, out)
		}
	}
}
