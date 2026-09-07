package tui

import (
	"strings"
	"testing"
	"time"
)

// TestThinkingLineRemoved: the "still thinking" progress line and the
// ✦/✧ badge glyph were removed (live 0.3.28 finding — the hint read as
// a duplicate user-visible line and left a stale transcript row; the
// star read as decoration rather than signal). The animated header
// elapsed clock plus the rotating quip are the only wait-state signal
// before the first token.
func TestThinkingLineRemoved(t *testing.T) {
	m := ChatModel{}
	for _, d := range []time.Duration{2 * time.Second, 6 * time.Second, 5 * time.Minute} {
		badge := m.thinkingBadge(int(d.Seconds()) * 4)
		if strings.Contains(badge, "still thinking") {
			t.Errorf("badge(%s) contains removed 'still thinking' line: %q", d, badge)
		}
		if strings.Contains(badge, "✦") || strings.Contains(badge, "✧") {
			t.Errorf("badge(%s) contains removed star glyph: %q", d, badge)
		}
	}
}
