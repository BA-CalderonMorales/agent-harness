// Package persona defines functional personas that adapt agent behavior,
// system prompts, and security defaults to the user's current role.
package persona

import (
	"fmt"
	"strings"
)

// Persona represents a user role with tailored behavior.
type Persona int

const (
	Developer Persona = iota
	Designer
	PM
	Scientist
	Explorer
)

// String returns the persona identifier.
func (p Persona) String() string {
	switch p {
	case Developer:
		return "developer"
	case Designer:
		return "designer"
	case PM:
		return "pm"
	case Scientist:
		return "scientist"
	case Explorer:
		return "explorer"
	default:
		return "developer"
	}
}

// DisplayName returns a human-friendly name.
func (p Persona) DisplayName() string {
	switch p {
	case Developer:
		return "Developer"
	case Designer:
		return "Designer"
	case PM:
		return "Product Manager"
	case Scientist:
		return "Scientist"
	case Explorer:
		return "Explorer"
	default:
		return "Developer"
	}
}

// Description returns a one-line description of the persona.
func (p Persona) Description() string {
	switch p {
	case Developer:
		return "Code, architecture, debugging — full tool access"
	case Designer:
		return "UI/UX, visuals, styling — read-first, safe exploration"
	case PM:
		return "Requirements, planning, docs — overview and export focused"
	case Scientist:
		return "Data, experiments, analysis — tracked execution"
	case Explorer:
		return "Learning, discovery, browsing — guided exploration"
	default:
		return "General development with full tools"
	}
}

// PromptFragment returns a persona-specific system prompt fragment.
func (p Persona) PromptFragment() string {
	switch p {
	case Developer:
		return `You are in DEVELOPER mode. Focus on writing clean, maintainable code.
- Prefer explicit over implicit
- Handle errors in every script
- Log meaningfully
- Test your changes when possible
- Use the test tool to verify correctness`
	case Designer:
		return `You are in DESIGNER mode. Focus on UI/UX, visual consistency, and user experience.
- Before changing styles, read the existing design system
- Prefer minimal, reversible changes
- Explain visual reasoning
- Respect existing color palettes and spacing conventions`
	case PM:
		return `You are in PRODUCT MANAGER mode. Focus on requirements, planning, and clear communication.
- Break down tasks into actionable steps
- Prioritize user impact
- Document decisions and trade-offs
- Use export to share summaries with stakeholders`
	case Scientist:
		return `You are in SCIENTIST mode. Focus on reproducible analysis and careful experimentation.
- Show your work step by step
- Preserve raw data when possible
- Document assumptions
- Validate results with alternative methods when feasible`
	case Explorer:
		return `You are in EXPLORER mode. Focus on learning and discovery.
- Explain concepts clearly before acting
- Ask clarifying questions when unsure
- Suggest related topics to explore
- Prefer safe, read-only exploration first`
	default:
		return ""
	}
}

// WelcomeGreeting returns a contextual greeting for the persona.
func (p Persona) WelcomeGreeting() string {
	switch p {
	case Developer:
		return "Ready to build something great."
	case Designer:
		return "Ready to craft the experience."
	case PM:
		return "What are we shipping today?"
	case Scientist:
		return "What hypothesis are we testing?"
	case Explorer:
		return "What shall we discover?"
	default:
		return "Ready to help."
	}
}

// DefaultPermissionMode returns the recommended permission mode.
func (p Persona) DefaultPermissionMode() string {
	switch p {
	case Developer:
		return "workspace-write"
	case Designer:
		return "read-only"
	case PM:
		return "workspace-write"
	case Scientist:
		return "interactive"
	case Explorer:
		return "read-only"
	default:
		return "workspace-write"
	}
}

// EmptyStateHint returns a contextual hint for the chat empty state.
func (p Persona) EmptyStateHint() string {
	switch p {
	case Developer:
		return "Describe a feature to build or a bug to fix"
	case Designer:
		return "Describe the UI challenge or component to design"
	case PM:
		return "Describe the requirement or decision to document"
	case Scientist:
		return "Describe the data or experiment to analyze"
	case Explorer:
		return "Ask about anything — I'm here to help you learn"
	default:
		return "Type a message to start"
	}
}

// CommandPriority returns a ranked list of command names most relevant to this persona.
// Used to reorder command palette suggestions.
func (p Persona) CommandPriority() []string {
	switch p {
	case Developer:
		return []string{"/test", "/diff", "/commit", "/branch", "/model", "/export"}
	case Designer:
		return []string{"/export", "/skills", "/glob", "/grep", "/model"}
	case PM:
		return []string{"/export", "/session", "/status", "/config", "/model"}
	case Scientist:
		return []string{"/export", "/status", "/memory", "/model", "/skills"}
	case Explorer:
		return []string{"/help", "/skills", "/agents", "/workspace", "/model"}
	default:
		return nil
	}
}

// All returns all available personas.
func All() []Persona {
	return []Persona{Developer, Designer, PM, Scientist, Explorer}
}

// Parse resolves a string to a Persona.
func Parse(s string) (Persona, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "developer", "dev", "code":
		return Developer, nil
	case "designer", "design", "ui", "ux":
		return Designer, nil
	case "pm", "product", "manager":
		return PM, nil
	case "scientist", "sci", "researcher", "data":
		return Scientist, nil
	case "explorer", "explore", "learn":
		return Explorer, nil
	default:
		return Developer, fmt.Errorf("unknown persona: %q (available: developer, designer, pm, scientist, explorer)", s)
	}
}

// Default returns the default persona.
func Default() Persona {
	return Developer
}
