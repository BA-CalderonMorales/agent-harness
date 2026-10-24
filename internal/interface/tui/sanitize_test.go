package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestSanitizeTerminalText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text is untouched", "hello world", "hello world"},
		{"color is kept", "\x1b[31mred\x1b[0m", "\x1b[31mred\x1b[0m"},
		{"bold is kept", "\x1b[1;33mwarn\x1b[0m", "\x1b[1;33mwarn\x1b[0m"},
		{"clipboard write is dropped", "\x1b]52;c;cGF5bG9hZA==\x07 after", " after"},
		{"hyperlink is dropped", "\x1b]8;;https://evil.example\x1b\\click\x1b]8;;\x1b\\", "click"},
		{"cursor movement is dropped", "a\x1b[2Ab", "ab"},
		{"erase is dropped", "a\x1b[2Jb", "ab"},
		{"private mode is dropped", "\x1b[?25lvisible", "visible"},
		{"device control is dropped", "\x1bPq#0;2;0;0;0\x1b\\data", "data"},
		{"bell is dropped", "ding\x07", "ding"},
		{"carriage return is dropped", "overwrite\r", "overwrite"},
		{"backspace is dropped", "ba\x08d", "bad"},
		{"delete is dropped", "x\x7f", "x"},
		{"eight bit csi is dropped", "a\u009b2Ab", "ab"},
		{"unterminated escape is dropped", "text\x1b[", "text"},
		{"newline and tab survive", "one\ntwo\tthree", "one\ntwo\tthree"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeTerminalText(tt.input); got != tt.want {
				t.Fatalf("SanitizeTerminalText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTerminalTextKeepsNewlinesInBlocks(t *testing.T) {
	input := "first\n\x1b[31mred\x1b[0m\nlast"
	want := "first\n\x1b[31mred\x1b[0m\nlast"
	if got := sanitizeTerminalLines(input); got != want {
		t.Fatalf("sanitizeTerminalLines() = %q, want %q", got, want)
	}
}

func TestChatTranscriptFiltersTerminalSequences(t *testing.T) {
	var m ChatModel
	m.AddMessage("assistant", "before\x1b]52;c;cGF5bG9hZA==\x07after")
	if strings.ContainsRune(m.messages[0].Content, 0x1b) {
		t.Fatalf("message content still carries an escape: %q", m.messages[0].Content)
	}
	if !strings.Contains(m.messages[0].Content, "after") {
		t.Fatalf("message content = %q, want the text kept", m.messages[0].Content)
	}
}

func TestSessionReloadFiltersTerminalSequences(t *testing.T) {
	var m ChatModel
	m.SetMessages([]types.Message{{
		Role:      types.RoleAssistant,
		Timestamp: time.Now(),
		Content:   []types.ContentBlock{types.TextBlock{Text: "hi\x1b[2Jthere"}},
	}})
	for _, msg := range m.messages {
		if strings.ContainsRune(msg.Content, 0x1b) {
			t.Fatalf("reloaded content still carries an escape: %q", msg.Content)
		}
	}
}

func TestToolRowFiltersTerminalSequences(t *testing.T) {
	var m ChatModel
	m.AddOrUpdateToolMessage("t1", "bash", "Shell", "ls\x1b]52;c;cGF5bG9hZA==\x07", ToolStatusRunning)
	if strings.ContainsRune(m.messages[0].ToolDetail, 0x1b) {
		t.Fatalf("tool detail still carries an escape: %q", m.messages[0].ToolDetail)
	}
}
