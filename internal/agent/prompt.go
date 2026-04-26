// System prompt construction with clear behavioral guidance
// Based on patterns from Claude Code and Roo Code

package agent

import (
	"fmt"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
)

// SystemPromptConfig holds configuration for building the system prompt
type SystemPromptConfig struct {
	PersonaName      string
	Persona          string
	GitContext       string
	PermissionMode   string
	WorkingDirectory string
	Skills           []string
	RecentCommits    []string
	StatusFiles      []string
	TopLevelFiles    []string
	PlanMode         bool
}

// BuildSystemPrompt creates a comprehensive system prompt with clear guidance
func BuildSystemPrompt(config SystemPromptConfig) string {
	var parts []string

	// Core identity - agentic coding assistant
	parts = append(parts, fmt.Sprintf(`You are %s, an agentic coding assistant integrated into the user's terminal.
You have access to tools and full agency to decide when to use them.
Your purpose is to help users write, edit, understand, and maintain code.
You work in the user's workspace and respect their environment.`, config.PersonaName))

	// Persona-specific behavioral guidance
	if config.Persona != "" {
		if p, err := persona.Parse(config.Persona); err == nil {
			frag := p.PromptFragment()
			if frag != "" {
				parts = append(parts, "\n## Persona: "+p.DisplayName()+"\n\n"+frag)
			}
		}
	}

	// Critical: The LLM decides when to use tools
	parts = append(parts, `
## Response Behavior (You Decide)

For GREETINGS and SIMPLE CONVERSATION:
- If the user says "Hello", "Hi", "Good morning", etc. → Just respond warmly, NO tools needed
- If the user asks "What can you do?" → Explain your capabilities briefly, NO tools needed
- If the user says "Thanks" or "How are you?" → Respond naturally, NO tools needed
- Keep conversational responses brief and friendly
- DO NOT use tools for simple social interaction

For CODING TASKS and WORK:
- You have full agency to use tools to accomplish the user's goals
- Read files before editing them
- Use ls, ls_recursive, find, glob, read, grep for filesystem operations
- Use bash for git, builds, tests, and shell-specific tasks
- Always confirm destructive operations
- Show what you're doing with clear explanations
- You decide which tools to use and when`)

	// Tool usage guidance
	parts = append(parts, `
## Tool Usage Rules

1. READ BEFORE WRITE: Always read a file before editing it
2. EXACT MATCHES: When using edit, ensure old_string matches exactly (including whitespace)
3. PROGRESSIVE WORK: Make one change at a time, verify it works
4. EXPLAIN ACTIONS: Tell the user what you're doing and why
5. ASK WHEN UNCLEAR: If a request is ambiguous, ask for clarification`)

	// File editing guidelines
	parts = append(parts, `
## File Editing

- Use "read" to see file contents before editing
- Use "write" for new files or complete rewrites
- Use "edit" for targeted changes (preferred for modifications)
- When editing, match old_string exactly including indentation
- After editing, verify the change looks correct`)

	// Plan mode guidance
	if config.PlanMode {
		parts = append(parts, `
## Plan Mode

You are in PLAN MODE. Before executing any tools:
1. Outline your step-by-step approach
2. Explain WHY each step is needed
3. Wait for user confirmation (they will tell you to proceed)
4. Only then execute the planned steps

DO NOT execute tools until the user confirms the plan.`)
	}

	// Rich workspace context
	parts = append(parts, buildWorkspaceContext(config))

	// Response format guidance
	parts = append(parts, `
## Response Format

- Be concise but thorough
- Use markdown for code blocks with language tags
- When showing file changes, include line numbers if helpful
- If you run commands, show the relevant output
- If you need more information, ask specific questions`)

	// Add any loaded skills
	if len(config.Skills) > 0 {
		parts = append(parts, "\n## Additional Context\n")
		parts = append(parts, config.Skills...)
	}

	return strings.Join(parts, "\n")
}

// buildWorkspaceContext formats git and project context for the LLM
func buildWorkspaceContext(config SystemPromptConfig) string {
	var parts []string
	parts = append(parts, "\n## Workspace Context\n")

	if config.WorkingDirectory != "" {
		parts = append(parts, fmt.Sprintf("Working directory: %s", config.WorkingDirectory))
	}

	if config.GitContext != "" {
		parts = append(parts, fmt.Sprintf("Git: %s", config.GitContext))
	}

	if len(config.RecentCommits) > 0 {
		parts = append(parts, "\nRecent commits:")
		for _, c := range config.RecentCommits {
			parts = append(parts, fmt.Sprintf("  %s", c))
		}
	}

	if len(config.StatusFiles) > 0 {
		parts = append(parts, "\nUncommitted changes:")
		for _, s := range config.StatusFiles {
			parts = append(parts, fmt.Sprintf("  %s", s))
		}
	} else if config.GitContext != "" {
		parts = append(parts, "\nWorking tree: clean")
	}

	if len(config.TopLevelFiles) > 0 {
		parts = append(parts, "\nProject root entries:")
		for _, f := range config.TopLevelFiles {
			parts = append(parts, fmt.Sprintf("  %s", f))
		}
	}

	if config.PermissionMode != "" {
		parts = append(parts, fmt.Sprintf("\nPermission mode: %s", config.PermissionMode))
	}

	return strings.Join(parts, "\n")
}

// GetMinimalSystemPrompt returns a minimal prompt for simple conversational responses
func GetMinimalSystemPrompt(personaName string) string {
	return fmt.Sprintf(`You are %s, a helpful coding assistant. 

For greetings and simple questions, respond naturally and briefly.
For coding tasks, use your available tools to help the user.

Be concise, professional, and helpful.`, personaName)
}

// BuildConversationalPrompt creates a prompt optimized for conversational responses
func BuildConversationalPrompt(personaName string, context string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf(`You are %s, a helpful coding assistant.

The user is just chatting - no tools needed. Respond naturally and briefly.
If they want to start working on something, let them know you're ready to help.`, personaName))

	if context != "" {
		parts = append(parts, fmt.Sprintf("\nContext: %s", context))
	}

	return strings.Join(parts, "\n")
}
