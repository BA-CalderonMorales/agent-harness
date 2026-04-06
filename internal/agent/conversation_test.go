package agent

import (
	"testing"
)

func TestClassifyInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ConversationType
	}{
		// Greetings
		{"simple hello", "Hello", ConvGreeting},
		{"hello with exclamation", "Hello!", ConvGreeting},
		{"hi there", "Hi there", ConvGreeting},
		{"hey harness", "Hey Harness", ConvGreeting},
		{"good morning", "Good morning", ConvGreeting},
		{"good afternoon", "Good afternoon", ConvGreeting},
		{"what's up", "What's up", ConvGreeting},
		{"yo", "Yo", ConvGreeting},

		// Questions about capabilities
		{"what can you do", "What can you do?", ConvQuestion},
		{"who are you", "Who are you?", ConvQuestion},
		{"what are you", "What are you?", ConvQuestion},
		{"help", "Help", ConvQuestion},
		{"help me with", "Help me with this error", ConvTask},

		// Casual conversation
		{"how are you", "How are you?", ConvCasual},
		{"thanks", "Thanks!", ConvCasual},
		{"thank you", "Thank you", ConvCasual},
		{"great job", "Great job!", ConvCasual},

		// Tasks (should use tools)
		{"create file", "Create a file called test.txt", ConvTask},
		{"fix bug", "Fix the bug in main.go", ConvTask},
		{"edit file", "Edit the file to add a function", ConvTask},
		{"search code", "Search for all occurrences of 'TODO'", ConvTask},
		{"run command", "Run the test suite", ConvTask},
		{"explain code", "Explain this code to me", ConvTask},
		{"help with", "Help me with this error", ConvTask},
		{"analyze file", "Can you analyze @file.go", ConvTask},

		// Edge cases
		{"empty string", "", ConvCasual},
		{"just spaces", "   ", ConvCasual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyInput(tt.input)
			if got != tt.expected {
				t.Errorf("ClassifyInput(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsConversational(t *testing.T) {
	conversational := []string{
		"Hello",
		"Hi there!",
		"Good morning",
		"What's up?",
		"How are you?",
		"Thanks!",
		"What can you do?",
		"Who are you?",
	}

	taskBased := []string{
		"Create a file",
		"Fix the bug",
		"Edit main.go",
		"Search for TODOs",
		"Run the tests",
		"Explain this code",
		"Help me debug this",
	}

	for _, input := range conversational {
		if !IsConversational(input) {
			t.Errorf("IsConversational(%q) = false, want true", input)
		}
	}

	for _, input := range taskBased {
		if IsConversational(input) {
			t.Errorf("IsConversational(%q) = true, want false", input)
		}
	}
}

func TestShouldUseTools(t *testing.T) {
	shouldNotUseTools := []string{
		"Hello",
		"Hi!",
		"Good morning",
		"What can you do?",
		"Thanks!",
	}

	shouldUseTools := []string{
		"Create a file",
		"Fix the bug",
		"Edit main.go",
		"Search for TODOs",
	}

	for _, input := range shouldNotUseTools {
		if ShouldUseTools(input) {
			t.Errorf("ShouldUseTools(%q) = true, want false", input)
		}
	}

	for _, input := range shouldUseTools {
		if !ShouldUseTools(input) {
			t.Errorf("ShouldUseTools(%q) = false, want true", input)
		}
	}
}

func TestGetGreetingResponse(t *testing.T) {
	response := GetGreetingResponse()
	if response == "" {
		t.Error("GetGreetingResponse() returned empty string")
	}
	// Should mention Harness and offer help
	if !testContainsAny(response, []string{"Harness", "help", "assistant"}) {
		t.Errorf("GetGreetingResponse() = %q, should mention Harness or help", response)
	}
}

func TestGetCapabilityResponse(t *testing.T) {
	response := GetCapabilityResponse()
	if response == "" {
		t.Error("GetCapabilityResponse() returned empty string")
	}
	// Should mention key capabilities
	keyTerms := []string{"read", "write", "edit", "bash", "search", "help"}
	found := 0
	for _, term := range keyTerms {
		if testContains(response, term) {
			found++
		}
	}
	if found < 3 {
		t.Errorf("GetCapabilityResponse() should mention multiple capabilities, got: %q", response)
	}
}

func TestGetCasualResponse(t *testing.T) {
	// Test thank you response
	thanks := GetCasualResponse("Thank you!")
	if thanks == "" {
		t.Error("GetCasualResponse('Thank you!') returned empty string")
	}

	// Test how are you response
	howAreYou := GetCasualResponse("How are you?")
	if howAreYou == "" {
		t.Error("GetCasualResponse('How are you?') returned empty string")
	}

	// Test joke request
	joke := GetCasualResponse("Tell me a joke")
	if joke == "" {
		t.Error("GetCasualResponse('Tell me a joke') returned empty string")
	}
}

// Helper functions
func testContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && testContainsAt(s, substr, 0))
}

func testContainsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func testContainsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if testContains(s, substr) {
			return true
		}
	}
	return false
}
