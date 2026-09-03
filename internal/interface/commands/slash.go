// Comprehensive slash command system inspired by claw-code

package commands

import (
	"fmt"
	"sort"
	"strings"
)

// SlashCommand represents a parsed slash command
type SlashCommand struct {
	Name string
	Args string
	Raw  string
}

// SlashHandler is a function that handles a slash command
type SlashHandler func(args string) (string, error)

// SlashRegistry holds all slash commands
type SlashRegistry struct {
	commands     map[string]SlashHandler
	help         map[string]string
	featureFlags map[string]string
}

// NewSlashRegistry creates a new slash command registry
func NewSlashRegistry() *SlashRegistry {
	return &SlashRegistry{
		commands:     make(map[string]SlashHandler),
		help:         make(map[string]string),
		featureFlags: make(map[string]string),
	}
}

// Register registers a slash command
func (sr *SlashRegistry) Register(name, description string, handler SlashHandler) {
	sr.commands[name] = handler
	sr.help[name] = description
}

// FeatureFlag marks a command as not yet available.
// The command will appear in /help under "Coming Soon" and will return an
// informational message when invoked.
func (sr *SlashRegistry) FeatureFlag(name, description string) {
	sr.featureFlags[name] = description
}

// IsFeatureFlagged returns true if the named command is feature-flagged.
func (sr *SlashRegistry) IsFeatureFlagged(name string) bool {
	_, ok := sr.featureFlags[name]
	return ok
}

// FeatureFlagMessage returns the "coming soon" message for a flagged command.
func (sr *SlashRegistry) FeatureFlagMessage(name string) string {
	desc := sr.featureFlags[name]
	var b strings.Builder
	b.WriteString(fmt.Sprintf("/%s — coming soon\n", name))
	b.WriteString(fmt.Sprintf("  %s\n\n", desc))
	b.WriteString("  This command is planned for an upcoming release.\n")
	b.WriteString("  We are working to stabilize it with full test coverage.\n\n")
	b.WriteString("  In the meantime, OpenCode (https://opencode.ai) offers\n")
	b.WriteString("  similar functionality for coding agent workflows.\n\n")
	b.WriteString("  Type /help to see all available commands.")
	return b.String()
}

// Handle handles a slash command
func (sr *SlashRegistry) Handle(input string) (string, bool, error) {
	if !strings.HasPrefix(input, "/") {
		return "", false, nil
	}

	cmd := ParseSlashCommand(input)

	if sr.IsFeatureFlagged(cmd.Name) {
		return sr.FeatureFlagMessage(cmd.Name), true, nil
	}

	handler, exists := sr.commands[cmd.Name]
	if !exists {
		suggestions := sr.findSimilar(cmd.Name)
		if len(suggestions) > 0 {
			return fmt.Sprintf("Unknown command: /%s\nDid you mean: %s?", cmd.Name, strings.Join(suggestions, ", ")), true, nil
		}
		return fmt.Sprintf("Unknown command: /%s\nType /help for available commands", cmd.Name), true, nil
	}

	result, err := handler(cmd.Args)
	return result, true, err
}

// ParseSlashCommand parses a slash command from input
func ParseSlashCommand(input string) SlashCommand {
	input = strings.TrimPrefix(input, "/")

	// Handle edge case where user types "/ " (slash followed by space)
	input = strings.TrimLeft(input, " ")

	if input == "" {
		return SlashCommand{Name: "", Raw: ""}
	}
	parts := strings.SplitN(input, " ", 2)

	cmd := SlashCommand{
		Name: parts[0],
		Raw:  input,
	}

	if len(parts) > 1 {
		cmd.Args = strings.TrimSpace(parts[1])
	}

	return cmd
}

// findSimilar finds similar command names for suggestions
func (sr *SlashRegistry) findSimilar(name string) []string {
	var suggestions []string
	for cmdName := range sr.commands {
		if strings.HasPrefix(cmdName, name) || strings.HasPrefix(name, cmdName) {
			suggestions = append(suggestions, "/"+cmdName)
		}
	}
	sort.Strings(suggestions)
	return suggestions
}

// GetHelp returns formatted help text
func (sr *SlashRegistry) GetHelp() string {
	var lines []string
	lines = append(lines, "Available commands:")
	lines = append(lines, "")

	categories := []struct {
		name string
		cmds []string
	}{
		{"Core", []string{"help", "clear", "compact", "version", "quit", "workspace", "init", "current-model"}},
		{"Session", []string{"status", "session", "steer"}},
		{"Model", []string{"model", "models", "provider"}},
		{"Settings", []string{"permissions", "config", "login", "logout", "settings"}},
		{"Git", []string{"branch", "pr"}},
		{"Output", []string{"cost", "export"}},
		{"Tools", []string{"agents", "skills", "audit", "plan", "memory"}},
	}

	categorized := make(map[string]bool)
	for _, cat := range categories {
		var catLines []string
		for _, cmd := range cat.cmds {
			categorized[cmd] = true
			if desc, exists := sr.help[cmd]; exists {
				catLines = append(catLines, fmt.Sprintf("  /%-15s %s", cmd, desc))
			}
		}
		if len(catLines) > 0 {
			lines = append(lines, cat.name+":")
			lines = append(lines, catLines...)
			lines = append(lines, "")
		}
	}

	var other []string
	for cmd := range sr.commands {
		if !categorized[cmd] && cmd != "exit" {
			other = append(other, cmd)
		}
	}
	sort.Strings(other)
	if len(other) > 0 {
		lines = append(lines, "Other:")
		for _, cmd := range other {
			if desc, exists := sr.help[cmd]; exists {
				lines = append(lines, fmt.Sprintf("  /%-15s %s", cmd, desc))
			}
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// GetCompletions returns all command names for tab completion
func (sr *SlashRegistry) GetCompletions() []string {
	completions := make([]string, 0, len(sr.commands))
	for name := range sr.commands {
		if name == "exit" {
			continue
		}
		completions = append(completions, "/"+name)
	}
	sort.Strings(completions)
	return completions
}

// CommandInfo represents metadata for a command in the command palette or UI.
type CommandInfo struct {
	Command     string
	Args        string
	Description string
	Category    string
}

// GetCommandInfos returns structured command metadata for all visible registered commands.
func (sr *SlashRegistry) GetCommandInfos() []CommandInfo {
	var infos []CommandInfo
	for name, desc := range sr.help {
		if name == "exit" {
			continue
		}
		infos = append(infos, CommandInfo{
			Command:     "/" + name,
			Description: desc,
			Category:    guessCategory(name),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Command < infos[j].Command
	})
	return infos
}

func guessCategory(name string) string {
	switch name {
	case "help", "status", "clear", "compact", "session", "reset", "quit":
		return "Session"
	case "model", "current-model":
		return "Model"
	case "cost", "export", "diff":
		return "Output"
	case "branch", "pr", "worktree":
		return "Git"
	case "agents", "skills", "plan", "memory", "init":
		return "Tools"
	case "permissions", "config", "login", "logout", "settings":
		return "Settings"
	default:
		return "System"
	}
}
func (sr *SlashRegistry) GetCompletionDescriptions() map[string]string {
	descriptions := make(map[string]string, len(sr.help))
	for name, description := range sr.help {
		if name == "exit" {
			continue
		}
		descriptions["/"+name] = description
	}
	return descriptions
}
func IsQuit(result string) bool {
	return result == "__QUIT__"
}
