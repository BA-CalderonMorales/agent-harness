package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Navigate mode must show the reciprocal "i" to type hint — the visible
// cue that `i` re-enters typing mode.
func TestModeLineNavigateShowsIHint(t *testing.T) {
	chat := NewChatModel()
	chat.width = 80
	chat.focused = false

	line := chat.renderModeLine()
	if !strings.Contains(line, `"i" to type`) {
		t.Fatalf("navigate mode line missing i-hint: %q", line)
	}
	if strings.Contains(line, `"Esc" to navigate`) {
		t.Fatalf("navigate mode line must not show Esc hint: %q", line)
	}
}

// Focused (typing) mode shows the Esc hint and not the i-hint.
func TestModeLineFocusedShowsEscHint(t *testing.T) {
	chat := NewChatModel()
	chat.width = 80
	chat.focused = true

	line := chat.renderModeLine()
	if !strings.Contains(line, `"Esc" to navigate`) {
		t.Fatalf("focused mode line missing Esc hint: %q", line)
	}
	if strings.Contains(line, `"i" to type`) {
		t.Fatalf("focused mode line must not show i-hint: %q", line)
	}
}

// Mobile panes enter the composer by tap, not `i`.
func TestModeLineMobileShowsTapHint(t *testing.T) {
	chat := NewChatModel()
	chat.width = 40 // mobile pane width
	chat.focused = false

	line := chat.renderModeLine()
	if isMobilePane(chat.width) {
		if !strings.Contains(line, "tap to type") {
			t.Fatalf("mobile navigate line missing tap hint: %q", line)
		}
	}
}

// Narrow width truncates without wrapping and the mode bit survives.
func TestModeLineNarrowTruncatesNoWrap(t *testing.T) {
	chat := NewChatModel()
	chat.width = 24
	chat.focused = false

	line := chat.renderModeLine()
	for _, l := range strings.Split(line, "\n") {
		if w := lipgloss.Width(l); w > chat.width {
			t.Fatalf("mode line wrapped/overflow: width %d > %d, line %q", w, chat.width, l)
		}
	}
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "navigate") {
		t.Fatalf("mode bit lost on truncation: %q", plain)
	}
}

// Whole-conversation copy: in order, labeled, first and last present.
func TestCopyConversationIncludesFirstAndLast(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "first question")
	chat.AddMessage("assistant", "middle answer")
	chat.AddMessage("user", "last question")

	text, n := chat.CopyConversation()
	if n != 3 {
		t.Fatalf("CopyConversation counted %d messages, want 3", n)
	}
	if !strings.Contains(text, "[user]\nfirst question") {
		t.Fatalf("first message missing/misordered: %q", text)
	}
	if !strings.Contains(text, "[user]\nlast question") {
		t.Fatalf("last message missing: %q", text)
	}
	if !strings.Contains(text, "[assistant]\nmiddle answer") {
		t.Fatalf("assistant label missing: %q", text)
	}
	if strings.Index(text, "first question") > strings.Index(text, "last question") {
		t.Fatalf("conversation out of order: %q", text)
	}
}

// Tool records include their detail in the whole-conversation copy.
func TestCopyConversationIncludesToolDetail(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "run it")
	chat.AddMessage("tool", "tool body")
	for i := range chat.messages {
		if chat.messages[i].Role == "tool" {
			chat.messages[i].IsTool = true
			chat.messages[i].ToolDetail = "$ ls"
		}
	}

	text, n := chat.CopyConversation()
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	if !strings.Contains(text, "$ ls") {
		t.Fatalf("tool detail missing: %q", text)
	}
}

// Empty conversation copies nothing — the caller flashes an error.
func TestCopyConversationEmpty(t *testing.T) {
	chat := NewChatModel()
	if text, n := chat.CopyConversation(); text != "" || n != 0 {
		t.Fatalf("empty conversation copied %q (%d)", text, n)
	}
}

// Message content must be copied verbatim (same contract as
// CopyRecord): leading whitespace can be semantically meaningful
// (indentation-based code blocks), so only the emptiness check trims.
func TestCopyConversationPreservesWhitespace(t *testing.T) {
	chat := NewChatModel()
	chat.AddMessage("user", "  indented\n    block  ")
	chat.AddMessage("assistant", "\ttabbed start\nend  ")

	text, n := chat.CopyConversation()
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	if !strings.Contains(text, "[user]\n  indented\n    block  \n\n") {
		t.Fatalf("user content was trimmed: %q", text)
	}
	// The final message is not followed by the "\n\n" separator
	// (single trailing trim on the assembled transcript), but its own
	// whitespace must be intact.
	if !strings.HasSuffix(text, "[assistant]\n\ttabbed start\nend  ") {
		t.Fatalf("assistant content was trimmed: %q", text)
	}
}
