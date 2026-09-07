package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestProbePhases isolates per-append refresh cost growth (skipped by
// default via -short; a diagnostic aid for future latency work, not an
// invariant). Run: go test ./internal/interface/tui/ -run TestProbePhases -v
func TestProbePhases(t *testing.T) {
	if testing.Short() {
		t.Skip("latency probe is a benchmark aid, not an invariant")
	}
	chat := NewChatModel()
	chat.width = 100
	chat.height = 40
	chat.Focus()
	for i := 0; i < 3000; i++ {
		s := time.Now()
		if i%3 == 0 {
			chat.AddMessage("user", fmt.Sprintf("q %d", i))
		} else {
			chat.AddMessage("assistant", strings.Repeat("The parser walks tokens. ", 12))
		}
		switch i {
		case 99, 499, 999, 1999, 2999:
			t.Logf("append %d: %v", i+1, time.Since(s))
		}
	}
}
