package ui

import (
	"testing"
)

func TestTermuxValidator_ValidateInput(t *testing.T) {
	validator := NewTermuxValidator()

	tests := []struct {
		name       string
		input      string
		wantValid  bool
		wantOutput string
	}{
		{"simple text", "Hello", true, "Hello"},
		{"text with spaces", "  Hello World  ", true, "Hello World"},
		{"empty string", "", false, ""},
		{"only spaces", "   ", false, ""},
		{"greeting with punctuation", "Hello!", true, "Hello!"},
		{"unicode text", "Hello 世界", true, "Hello 世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := validator.ValidateInput(tt.input)
			if valid != tt.wantValid {
				t.Errorf("ValidateInput() valid = %v, want %v", valid, tt.wantValid)
			}
			if valid && got != tt.wantOutput {
				t.Errorf("ValidateInput() output = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

func TestTermuxValidator_IsGreeting(t *testing.T) {
	validator := NewTermuxValidator()

	greetings := []string{
		"Hello",
		"Hi",
		"Hey",
		"Good morning",
		"Good afternoon",
		"Good evening",
		"What's up",
		"Yo",
		"Hiya",
	}

	nonGreetings := []string{
		"Create a file",
		"Fix the bug",
		"High five",
		"Hello create",
		"Hello world project",
	}

	for _, g := range greetings {
		if !validator.IsGreeting(g) {
			t.Errorf("IsGreeting(%q) = false, want true", g)
		}
	}

	for _, ng := range nonGreetings {
		if validator.IsGreeting(ng) {
			t.Errorf("IsGreeting(%q) = true, want false", ng)
		}
	}
}

func TestTermuxValidator_IsSimpleQuestion(t *testing.T) {
	validator := NewTermuxValidator()

	questions := []string{
		"What can you do?",
		"Who are you?",
		"What are you?",
		"Help",
		"What do you do?",
	}

	nonQuestions := []string{
		"Create a file",
		"Help me with this",
		"What is the weather?",
	}

	for _, q := range questions {
		if !validator.IsSimpleQuestion(q) {
			t.Errorf("IsSimpleQuestion(%q) = false, want true", q)
		}
	}

	for _, nq := range nonQuestions {
		if validator.IsSimpleQuestion(nq) {
			t.Errorf("IsSimpleQuestion(%q) = true, want false", nq)
		}
	}
}

func TestNormalizeTermuxInput(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello  .", "Hello."},             // Samsung double-space
		{"Line1\r\nLine2", "Line1\nLine2"}, // Windows line endings
		{"Line1\rLine2", "Line1\nLine2"},   // Old Mac line endings
		{"Hello\n\n\n", "Hello"},           // Trailing newlines
		{"  Hello  ", "  Hello  "},         // No change needed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTermuxInput(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeTermuxInput(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsTermuxInputIssue(t *testing.T) {
	tests := []struct {
		input       string
		wantIssue   bool
		wantMessage string
	}{
		{"", true, "empty"},
		{"   ", true, "empty"},
		{"Hello", false, ""},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true, "repeated"},
		{"Hello\x00World", true, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(10, len(tt.input))], func(t *testing.T) {
			hasIssue, msg := IsTermuxInputIssue(tt.input)
			if hasIssue != tt.wantIssue {
				t.Errorf("IsTermuxInputIssue(%q) = %v, want %v", tt.input, hasIssue, tt.wantIssue)
			}
			if tt.wantIssue && msg == "" {
				t.Error("IsTermuxInputIssue() returned empty message for detected issue")
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
