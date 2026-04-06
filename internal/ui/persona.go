// Persona defines the agent's character and voice
// Professional, concise, and helpful - no fluff, no cringe

package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// SpinnerVerbs are concise loading messages
var SpinnerVerbs = []string{
	"Analyzing",
	"Processing",
	"Computing",
	"Evaluating",
	"Checking",
	"Building",
	"Searching",
	"Reading",
	"Writing",
	"Updating",
}

// ActionNouns for tool execution descriptions
var ActionNouns = []string{
	"files",
	"code",
	"structure",
	"dependencies",
	"configuration",
}

// ThinkingIndicators show the agent is working - brief and professional
var ThinkingIndicators = []string{
	"Thinking...",
	"Working...",
	"Processing...",
}

// GreetingTemplates for contextual welcome messages - professional only
var GreetingTemplates = []string{
	"Ready to help you build something great.",
	"What are we working on?",
	"Here to help you code.",
	"Ready when you are.",
	"Let's get things done.",
}

// CompletionPhrases for when work is done - brief and clean
var CompletionPhrases = []string{
	"Done.",
	"Complete.",
	"Ready.",
	"Finished.",
}

// StartupMessages - brief acknowledgments (no emojis, no cringe)
var StartupMessages = []string{
	"Analyzing request...",
	"Processing...",
	"Working on it...",
}

// GetRandomSpinnerVerb returns a random loading verb
func GetRandomSpinnerVerb() string {
	return SpinnerVerbs[rand.Intn(len(SpinnerVerbs))]
}

// GetRandomThinkingIndicator returns a random thinking phrase
func GetRandomThinkingIndicator() string {
	return ThinkingIndicators[rand.Intn(len(ThinkingIndicators))]
}

// GetRandomGreeting returns a random greeting
func GetRandomGreeting() string {
	return GreetingTemplates[rand.Intn(len(GreetingTemplates))]
}

// GetRandomCompletion returns a random completion phrase
func GetRandomCompletion() string {
	return CompletionPhrases[rand.Intn(len(CompletionPhrases))]
}

// GetRandomStartupMessage returns a brief startup acknowledgment
func GetRandomStartupMessage() string {
	return StartupMessages[rand.Intn(len(StartupMessages))]
}

// FormatToolAction creates a human-readable tool description
func FormatToolAction(toolName string, description string) string {
	// Clean up the tool name
	name := strings.TrimPrefix(toolName, "builtin_")
	name = strings.ReplaceAll(name, "_", " ")

	if description != "" {
		return fmt.Sprintf("%s %s", name, description)
	}

	// Default descriptions based on tool type - concise and clear
	switch name {
	case "read":
		return "Reading file"
	case "write":
		return "Writing file"
	case "edit":
		return "Editing file"
	case "bash":
		return "Running command"
	case "glob":
		return "Finding files"
	case "grep":
		return "Searching code"
	case "web search":
		return "Searching web"
	case "web fetch":
		return "Fetching page"
	case "agent":
		return "Delegating task"
	default:
		return fmt.Sprintf("Using %s", name)
	}
}

// FormatFileAction creates context-aware file operation descriptions
func FormatFileAction(action, path string) string {
	// Get just the filename for brevity
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	switch action {
	case "read":
		return fmt.Sprintf("Reading %s", filename)
	case "write":
		return fmt.Sprintf("Creating %s", filename)
	case "edit":
		return fmt.Sprintf("Updating %s", filename)
	case "delete":
		return fmt.Sprintf("Removing %s", filename)
	default:
		return fmt.Sprintf("%s %s", action, filename)
	}
}

// TimeOfDayGreeting returns a time-appropriate greeting
func TimeOfDayGreeting() string {
	hour := time.Now().Hour()

	switch {
	case hour < 6:
		return "Working late?"
	case hour < 12:
		return "Good morning."
	case hour < 18:
		return "Good afternoon."
	default:
		return "Good evening."
	}
}

// PersonaName is the agent's identity
const PersonaName = "Harness"

// FormatAgentPrefix returns the agent's name as a prefix
func FormatAgentPrefix() string {
	return DimStyle.Render("Harness")
}

// FormatUserPrefix returns the user prefix
func FormatUserPrefix() string {
	return UserStyle.Render("You")
}
