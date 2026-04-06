// Termux-specific UI handling and input validation
// Ensures reliable input/output on Android/Termux environments

package ui

import (
	"os"
	"strings"
	"unicode"
)

// TermuxValidator provides input validation for Termux environments
type TermuxValidator struct {
	maxInputLength int
}

// NewTermuxValidator creates a new validator with sensible defaults
func NewTermuxValidator() *TermuxValidator {
	return &TermuxValidator{
		maxInputLength: 10000, // Reasonable limit for terminal input
	}
}

// ValidateInput checks if input is valid and safe
func (tv *TermuxValidator) ValidateInput(input string) (string, bool) {
	// Trim whitespace
	trimmed := strings.TrimSpace(input)

	// Check for empty input
	if trimmed == "" {
		return "", false
	}

	// Check length limit
	if len(trimmed) > tv.maxInputLength {
		return trimmed[:tv.maxInputLength], true // Truncate but accept
	}

	// Clean up any problematic characters that might come from mobile keyboards
	cleaned := tv.sanitizeInput(trimmed)

	return cleaned, true
}

// sanitizeInput removes or normalizes problematic characters
func (tv *TermuxValidator) sanitizeInput(input string) string {
	var result strings.Builder

	for _, r := range input {
		switch {
		// Keep printable ASCII
		case r >= 32 && r < 127:
			result.WriteRune(r)
		// Keep common Unicode punctuation and symbols
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			result.WriteRune(r)
		// Keep Unicode letters and numbers
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			result.WriteRune(r)
		// Replace various whitespace with space
		case unicode.IsSpace(r):
			result.WriteRune(' ')
		// Skip control characters and other non-printable
		default:
			// Skip
		}
	}

	return result.String()
}

// IsGreeting checks if input is a simple greeting
func (tv *TermuxValidator) IsGreeting(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))

	// Must be exact match or greeting + simple name (e.g., "Hello Harness")
	greetings := []string{
		"hello", "hi", "hey", "howdy", "greetings",
		"yo", "hiya", "what's up", "sup",
		"good morning", "good afternoon", "good evening",
	}

	// Common follow-ups that indicate a greeting (not a task)
	greetingFollowups := []string{
		"there", "harness", "agent", "assistant", "bot",
		"friend", "pal", "buddy", "mate",
		"everyone", "all", "folks", "guys",
		"again", "back",
	}

	for _, g := range greetings {
		// Exact match
		if lower == g {
			return true
		}
		// Greeting + simple follow-up (e.g., "Hello there", "Hi Harness")
		if strings.HasPrefix(lower, g+" ") {
			rest := strings.ToLower(strings.TrimSpace(lower[len(g)+1:]))
			// Check if rest is a simple greeting follow-up (max 2 words)
			words := strings.Fields(rest)
			if len(words) <= 2 {
				for _, followup := range greetingFollowups {
					if rest == followup {
						return true
					}
				}
				// Also accept single word that's likely a name (no verbs)
				if len(words) == 1 && !isLikelyVerb(words[0]) {
					return true
				}
			}
		}
	}

	return false
}

// isLikelyVerb checks if a word is likely a verb (indicating a task, not a greeting)
func isLikelyVerb(word string) bool {
	commonVerbs := []string{
		"create", "make", "write", "edit", "fix", "debug", "build", "run",
		"test", "add", "remove", "delete", "search", "find", "look", "get",
		"show", "tell", "explain", "help", "check", "update", "change",
		"implement", "refactor", "move", "copy", "generate", "convert",
	}
	word = strings.ToLower(word)
	for _, verb := range commonVerbs {
		if word == verb || strings.HasPrefix(word, verb) {
			return true
		}
	}
	return false
}

// IsSimpleQuestion checks if input is a simple question about capabilities
func (tv *TermuxValidator) IsSimpleQuestion(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))

	// These should be standalone questions, not task requests
	patterns := []string{
		"what can you do",
		"who are you",
		"what are you",
		"what do you do",
		"how do you work",
		"what are your capabilities",
		"how can you help",
	}

	// "Help" alone is a question, but "Help me" is a task request
	if lower == "help" || lower == "help?" {
		return true
	}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

// GetTermuxInfo returns information about the Termux environment
func GetTermuxInfo() map[string]string {
	info := make(map[string]string)

	info["TERMUX_VERSION"] = os.Getenv("TERMUX_VERSION")
	info["TERMUX_APP_PID"] = os.Getenv("TERMUX_APP_PID")
	info["HOME"] = os.Getenv("HOME")
	info["PREFIX"] = os.Getenv("PREFIX")

	// Detect if we're actually in Termux
	isTermux := DetectTermux()
	if isTermux {
		info["detected"] = "true"
	} else {
		info["detected"] = "false"
	}

	return info
}

// IsTermuxInputIssue checks for common Termux input problems
func IsTermuxInputIssue(input string) (bool, string) {
	// Check for empty or whitespace-only input
	if strings.TrimSpace(input) == "" {
		return true, "Input appears to be empty"
	}

	// Check for excessive repeated characters (might indicate stuck key)
	if hasExcessiveRepeats(input, 10) {
		return true, "Input has excessive repeated characters"
	}

	// Check for null bytes or control characters
	if strings.Contains(input, "\x00") {
		return true, "Input contains null bytes"
	}

	return false, ""
}

// hasExcessiveRepeats checks if a character repeats too many times in a row
func hasExcessiveRepeats(s string, threshold int) bool {
	if len(s) < threshold {
		return false
	}

	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			count++
			if count >= threshold {
				return true
			}
		} else {
			count = 1
		}
	}

	return false
}

// NormalizeTermuxInput normalizes input specifically for Termux quirks
func NormalizeTermuxInput(input string) string {
	// Handle Samsung keyboard double-space period
	input = strings.ReplaceAll(input, "  .", ".")

	// Normalize line endings
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	// Trim trailing newlines
	input = strings.TrimRight(input, "\n")

	return input
}
