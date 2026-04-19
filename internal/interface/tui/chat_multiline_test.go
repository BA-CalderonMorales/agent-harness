package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMultilinePasteScenarios covers edge cases for multiline paste handling.
// These are table-driven complements to the Ginkgo paste-detection specs.
func TestMultilinePasteScenarios(t *testing.T) {
	oldDebounce := SubmitDebounceDuration
	SubmitDebounceDuration = 0 // immediate submit for legacy scenarios
	defer func() { SubmitDebounceDuration = oldDebounce }()

	delegate := &testChatDelegate{}

	tests := []struct {
		name           string
		steps          []tea.Msg
		wantDisplay    string
		wantSubmitted  string
		wantMessageLen int
	}{
		{
			name: "bracketed paste two lines short",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "[Pasted text, 2 lines, 3 characters]",
			wantSubmitted:  "a\nb",
			wantMessageLen: 1,
		},
		{
			name: "bracketed paste exactly at char threshold no newline",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", PasteDisplayThreshold)), Paste: true},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    strings.Repeat("x", PasteDisplayThreshold),
			wantSubmitted:  strings.Repeat("x", PasteDisplayThreshold),
			wantMessageLen: 1,
		},
		{
			name: "bracketed paste one above char threshold no newline",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", PasteDisplayThreshold+1)), Paste: true},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "[Pasted text, 201 characters]",
			wantSubmitted:  strings.Repeat("x", PasteDisplayThreshold+1),
			wantMessageLen: 1,
		},
		{
			name: "bracketed paste just under threshold but multiline",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", PasteDisplayThreshold-1) + "\nextra"), Paste: true},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "[Pasted text, 2 lines, 205 characters]",
			wantSubmitted:  strings.Repeat("x", PasteDisplayThreshold-1) + "\nextra",
			wantMessageLen: 1,
		},
		{
			name: "heuristic paste with ctrl+j newline",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("y", PasteHeuristicThreshold+1))},
				tea.KeyMsg{Type: tea.KeyCtrlJ},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tail")},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "[Pasted text, 2 lines, 26 characters]",
			wantSubmitted:  strings.Repeat("y", PasteHeuristicThreshold+1) + "\ntail",
			wantMessageLen: 1,
		},
		{
			name: "manual alt-enter multiline not collapsed",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")},
				tea.KeyMsg{Type: tea.KeyEnter, Alt: true},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "hello\nworld",
			wantSubmitted:  "hello\nworld",
			wantMessageLen: 1,
		},
		{
			name: "ctrl+j on empty input creates newline",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyCtrlJ},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "\n",
			wantSubmitted:  "\n",
			wantMessageLen: 1,
		},
		{
			// Note: the textarea sanitizer normalizes \r to \n, so \r\n becomes \n\n.
			name: "bracketed paste with windows line endings",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line1\r\nline2"), Paste: true},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantDisplay:    "[Pasted text, 3 lines, 12 characters]",
			wantSubmitted:  "line1\n\nline2",
			wantMessageLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := NewChatModel()
			chat.SetDelegate(delegate)
			// reset delegate state
			delegate.submittedText = ""
			delegate.commandText = ""

			for _, step := range tt.steps {
				model, _ := chat.Update(step)
				chat = model.(ChatModel)
			}

			if len(chat.messages) != tt.wantMessageLen {
				t.Fatalf("expected %d messages, got %d", tt.wantMessageLen, len(chat.messages))
			}

			if tt.wantMessageLen > 0 && chat.messages[0].Content != tt.wantDisplay {
				t.Errorf("display: got %q, want %q", chat.messages[0].Content, tt.wantDisplay)
			}

			if delegate.submittedText != tt.wantSubmitted {
				t.Errorf("submitted: got %q, want %q", delegate.submittedText, tt.wantSubmitted)
			}
		})
	}
}
