// Conversation handling for natural greetings and simple interactions
// Makes the agent feel responsive and human on first contact

package agent

import (
	"regexp"
	"strings"
)

// ConversationType categorizes user input
type ConversationType int

const (
	// ConvGreeting is a simple hello/greeting
	ConvGreeting ConversationType = iota
	// ConvQuestion is a question about the agent/capabilities
	ConvQuestion
	// ConvCasual is casual conversation
	ConvCasual
	// ConvTask is a work-related request (should use tools)
	ConvTask
)

// greetingPatterns matches common greeting phrases
var greetingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?i)(hi|hello|hey|howdy|greetings|what's up|sup)[\s!]*$`),
	regexp.MustCompile(`^(?i)(hi|hello|hey)[\s]+(there|harness|agent|assistant)[\s!]*$`),
	regexp.MustCompile(`^(?i)(good\s+(morning|afternoon|evening|day))[\s!]*$`),
	regexp.MustCompile(`^(?i)(yo|hiya|howdy)[\s!]*$`),
}

// questionPatterns matches questions about the agent (must be standalone questions)
var questionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?i)(what can you do|who are you|what are you|how do you work|what can you help with)[?\s]*$`),
	regexp.MustCompile(`^(?i)(help|commands|capabilities)[?\s]*$`),
	regexp.MustCompile(`(?i)^(what do you do|what are your capabilities|how can you help)`),
}

// casualPatterns matches casual conversation
var casualPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?i)(how are you|how's it going|what's up|how do you feel)[?\s]*$`),
	regexp.MustCompile(`(?i)(thank you|thanks|great|awesome|cool|nice|good job)`),
	regexp.MustCompile(`(?i)(tell me a joke|say something|talk to me)`),
}

// taskIndicators suggests the user wants to do actual work
var taskIndicators = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(create|write|edit|modify|fix|debug|build|run|test|refactor|implement|add|remove|delete|find|search|look up|check|analyze|review|explain|help me with|can you)`),
	regexp.MustCompile(`[@\./]`), // File references, @mentions
	regexp.MustCompile(`(?i)(file|folder|directory|repo|repository|code|function|class|method|variable)`),
}

// ClassifyInput determines the type of conversation from user input
func ClassifyInput(input string) ConversationType {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ConvCasual
	}

	// Check for greetings first (exact matches preferred)
	for _, pattern := range greetingPatterns {
		if pattern.MatchString(trimmed) {
			return ConvGreeting
		}
	}

	// Check for questions about capabilities
	for _, pattern := range questionPatterns {
		if pattern.MatchString(trimmed) {
			return ConvQuestion
		}
	}

	// Check for task indicators FIRST (before casual conversation)
	// This ensures that "Fix the bug, thanks!" is treated as a task.
	for _, pattern := range taskIndicators {
		if pattern.MatchString(trimmed) {
			return ConvTask
		}
	}

	// Check for casual conversation
	for _, pattern := range casualPatterns {
		if pattern.MatchString(trimmed) {
			return ConvCasual
		}
	}

	// Default to task for anything else
	return ConvTask
}

// IsConversational returns true if the input is just conversation, not a task
func IsConversational(input string) bool {
	convType := ClassifyInput(input)
	return convType == ConvGreeting || convType == ConvQuestion || convType == ConvCasual
}

// GetGreetingResponse returns an appropriate greeting response
func GetGreetingResponse() string {
	responses := []string{
		"Hello! I'm Harness, your coding assistant. What would you like to work on?",
		"Hi there! Ready to help you build something great.",
		"Hey! I'm here to help with your coding tasks. What's on your mind?",
		"Greetings! What are we working on today?",
		"Hello! Type /help if you need a quick reference, or just tell me what you'd like to do.",
	}
	return responses[0] // Consistent for now, can randomize later
}

// GetCapabilityResponse returns a response explaining what the agent can do
func GetCapabilityResponse() string {
	return `I'm Harness, a coding assistant that can help you with various development tasks.

Here's what I can do:

  • Read and analyze code files
  • Write and edit files in your workspace  
  • Run bash commands and scripts
  • Search through code with grep and glob
  • Fetch web resources and documentation
  • Delegate tasks to specialized sub-agents
  • Help you plan complex changes

I work best when you tell me what you want to achieve in plain language.

Type /help to see available commands, or just start describing what you need.`
}

// GetCasualResponse returns a response for casual conversation
func GetCasualResponse(input string) string {
	inputLower := strings.ToLower(strings.TrimSpace(input))

	if strings.Contains(inputLower, "thank") {
		return "You're welcome! Anything else you'd like to work on?"
	}

	if strings.Contains(inputLower, "how are you") || strings.Contains(inputLower, "how's it going") {
		return "I'm ready to help! What would you like to work on?"
	}

	if strings.Contains(inputLower, "joke") {
		return "Why do programmers prefer dark mode? Because light attracts bugs.\n\nNow, what are we working on?"
	}

	return "I'm here to help with your coding tasks. What would you like to work on?"
}

// ShouldUseTools determines if this input should trigger tool usage
func ShouldUseTools(input string) bool {
	return !IsConversational(input)
}
