package tui

import (
	"strings"
	"testing"
	"time"
)

// TestThinkingHint verifies the prompt-eval progress line only appears
// once the first token has been pending long enough that a slow local
// model (CPU prompt eval) is the likely explanation.
func TestThinkingHint(t *testing.T) {
	if got := thinkingHint(2 * time.Second); got != "" {
		t.Errorf("thinkingHint(2s) = %q, want empty before the threshold", got)
	}
	hint := thinkingHint(6 * time.Second)
	if !strings.Contains(hint, "still thinking") {
		t.Errorf("thinkingHint(6s) = %q, want still-thinking line", hint)
	}
	if !strings.Contains(hint, "minutes") {
		t.Errorf("thinkingHint(6s) = %q, want CPU-model minute hint", hint)
	}
}