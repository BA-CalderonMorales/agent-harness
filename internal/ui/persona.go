// Persona defines the agent's character and voice
// This creates a consistent, friendly, and professional tone

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

// SpinnerVerbs are playful loading messages inspired by Claude Code
var SpinnerVerbs = []string{
	"Thinking",
	"Processing",
	"Exploring",
	"Analyzing",
	"Crafting",
	"Considering",
	"Computing",
	"Synthesizing",
	"Reasoning",
	"Pondering",
	"Evaluating",
	"Constructing",
	"Formulating",
	"Generating",
	"Orchestrating",
	"Deciphering",
	"Contemplating",
	"Examining",
	"Investigating",
	"Designing",
	"Architecting",
	"Sculpting",
	"Weaving",
	"Forging",
	"Curating",
	"Refining",
	"Polishing",
	"Assembling",
	"Composing",
	"Developing",
}

// ActionNouns for tool execution descriptions
var ActionNouns = []string{
	"the code",
	"the files",
	"the structure",
	"the logic",
	"the patterns",
	"the solution",
	"the approach",
	"the details",
	"the architecture",
	"the implementation",
}

// ThinkingIndicators show the agent is working on something complex
var ThinkingIndicators = []string{
	"Let me think about this...",
	"Hmm, let me see...",
	"Working on it...",
	"Give me a moment...",
	"Let me analyze this...",
	"Figuring this out...",
	"Processing your request...",
	"On it...",
}

// GreetingTemplates for contextual welcome messages
var GreetingTemplates = []string{
	"Ready to help you build something great.",
	"Let's make some progress today.",
	"What are we working on?",
	"Here to help you code.",
	"Ready when you are.",
	"Let's get things done.",
}

// CompletionPhrases for when work is done
var CompletionPhrases = []string{
	"Done.",
	"All set.",
	"Finished.",
	"Complete.",
	"Ready.",
	"There you go.",
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

// FormatToolAction creates a human-readable tool description
func FormatToolAction(toolName string, description string) string {
	// Clean up the tool name
	name := strings.TrimPrefix(toolName, "builtin_")
	name = strings.ReplaceAll(name, "_", " ")
	
	if description != "" {
		return fmt.Sprintf("%s %s", name, description)
	}
	
	// Default descriptions based on tool type
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
		return "Searching the web"
	case "web fetch":
		return "Fetching page"
	case "agent":
		return "Delegating to specialist"
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
