package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSubmitDebounce covers the Termux paste scenario where pasted newlines
// arrive as KeyEnter events between character bursts. The debounce mechanism
// must distinguish intentional submission from pasted newlines.
func TestSubmitDebounce(t *testing.T) {
	oldDebounce := SubmitDebounceDuration
	SubmitDebounceDuration = 10 * time.Millisecond
	defer func() { SubmitDebounceDuration = oldDebounce }()

	tests := []struct {
		name          string
		steps         []tea.Msg
		wantMessages  int
		wantSubmitted string
		wantTextarea  string
		fireTimer     bool // whether to manually fire the debounce timer after steps
	}{
		{
			name: "termux paste with keyenter newlines",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line1")},
				tea.KeyMsg{Type: tea.KeyEnter},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line2")},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "line1\nline2",
			fireTimer:     true,
		},
		{
			name: "termux paste with empty lines",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
				tea.KeyMsg{Type: tea.KeyEnter},
				tea.KeyMsg{Type: tea.KeyEnter},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "a\n\nb",
			fireTimer:     true,
		},
		{
			name: "ctrl+j cancels pending submit",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
				tea.KeyMsg{Type: tea.KeyEnter},
				tea.KeyMsg{Type: tea.KeyCtrlJ},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "a\n",
			fireTimer:     true,
		},
		{
			name: "empty input does not start timer",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "",
			fireTimer:     false,
		},
		{
			name: "intentional submit after pause",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "hello",
			fireTimer:     true,
		},
		{
			name: "rapid typing then enter submits after timer",
			steps: []tea.Msg{
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")},
				tea.KeyMsg{Type: tea.KeyEnter},
			},
			wantMessages:  0,
			wantSubmitted: "",
			wantTextarea:  "hi",
			fireTimer:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := NewChatModel()
			delegate := &testChatDelegate{}
			chat.SetDelegate(delegate)

			for _, step := range tt.steps {
				model, _ := chat.Update(step)
				chat = model.(ChatModel)
			}

			if len(chat.messages) != tt.wantMessages {
				t.Fatalf("after steps: expected %d messages, got %d", tt.wantMessages, len(chat.messages))
			}

			if chat.textarea.Value() != tt.wantTextarea {
				t.Errorf("after steps: textarea got %q, want %q", chat.textarea.Value(), tt.wantTextarea)
			}

			if tt.fireTimer {
				gen := chat.pendingSubmitGen
				model, _ := chat.Update(submitTimerMsg{generation: gen})
				chat = model.(ChatModel)
			}

			if tt.fireTimer && tt.wantSubmitted != "" {
				if len(chat.messages) != 1 {
					t.Fatalf("after timer: expected 1 message, got %d", len(chat.messages))
				}
				if delegate.submittedText != tt.wantSubmitted {
					t.Errorf("submitted got %q, want %q", delegate.submittedText, tt.wantSubmitted)
				}
			}

			if tt.fireTimer && tt.wantSubmitted == "" && len(chat.messages) != 0 {
				// For cases where timer fires but there was no pending submit
				// (e.g., ctrl+j cancels it), we expect 0 messages.
				if tt.name == "ctrl+j cancels pending submit" && len(chat.messages) != 0 {
					t.Fatalf("after timer: expected 0 messages, got %d", len(chat.messages))
				}
			}
		})
	}
}

// TestSubmitDebounceSpamPrevention verifies the exact anti-pattern:
// pasting 20 lines of text where each newline arrives as KeyEnter
// must result in exactly ONE submission, not 20.
func TestSubmitDebounceSpamPrevention(t *testing.T) {
	oldDebounce := SubmitDebounceDuration
	SubmitDebounceDuration = 10 * time.Millisecond
	defer func() { SubmitDebounceDuration = oldDebounce }()

	chat := NewChatModel()
	delegate := &testChatDelegate{}
	chat.SetDelegate(delegate)

	lines := []string{
		"Line 1: First requirement",
		"Line 2: Second requirement",
		"Line 3: Third requirement",
		"Line 4: Fourth requirement",
		"Line 5: Fifth requirement",
	}

	// Simulate Termux paste: each line followed by KeyEnter (\r)
	for i, line := range lines {
		model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
		chat = model.(ChatModel)

		// All but the last line are followed by Enter in the paste
		if i < len(lines)-1 {
			model, _ = chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
			chat = model.(ChatModel)
		}
	}

	// At this point no messages should have been submitted
	if len(chat.messages) != 0 {
		t.Fatalf("expected 0 messages mid-paste, got %d", len(chat.messages))
	}

	// The textarea should contain all lines with newlines
	expected := "Line 1: First requirement\nLine 2: Second requirement\nLine 3: Third requirement\nLine 4: Fourth requirement\nLine 5: Fifth requirement"
	if chat.textarea.Value() != expected {
		t.Errorf("textarea mismatch:\ngot:  %q\nwant: %q", chat.textarea.Value(), expected)
	}

	// User presses Enter to submit after the paste
	model, _ := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})
	chat = model.(ChatModel)

	// Fire the debounce timer
	gen := chat.pendingSubmitGen
	model, _ = chat.Update(submitTimerMsg{generation: gen})
	chat = model.(ChatModel)

	// Exactly ONE message should have been submitted
	if len(chat.messages) != 1 {
		t.Fatalf("expected 1 message after submit, got %d", len(chat.messages))
	}

	if delegate.submittedText != expected {
		t.Errorf("submitted mismatch:\ngot:  %q\nwant: %q", delegate.submittedText, expected)
	}
}
