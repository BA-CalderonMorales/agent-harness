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
	commands map[string]SlashHandler
	help     map[string]string
}

// NewSlashRegistry creates a new slash command registry
func NewSlashRegistry() *SlashRegistry {
	return &SlashRegistry{
		commands: make(map[string]SlashHandler),
		help:     make(map[string]string),
	}
}

// Register registers a slash command
func (sr *SlashRegistry) Register(name, description string, handler SlashHandler) {
	sr.commands[name] = handler
	sr.help[name] = description
}

// Handle handles a slash command
func (sr *SlashRegistry) Handle(input string) (string, bool, error) {
	if !strings.HasPrefix(input, "/") {
		return "", false, nil
	}

	// Parse the command
	cmd := ParseSlashCommand(input)

	handler, exists := sr.commands[cmd.Name]
	if !exists {
		// Try to find similar commands
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
		// Simple similarity: starts with same prefix or contains substring
		if strings.HasPrefix(cmdName, name) || strings.HasPrefix(name, cmdName) {
			suggestions = append(suggestions, "/"+cmdName)
		}
	}
	return suggestions
}

// GetHelp returns formatted help text
func (sr *SlashRegistry) GetHelp() string {
	var lines []string
	lines = append(lines, "Available commands:")
	lines = append(lines, "")

	// Group commands by category (slice for deterministic order)
	categories := []struct {
		name string
		cmds []string
	}{
		{"Session", []string{"help", "status", "clear", "compact", "session", "reset", "quit", "workspace"}},
		{"Model", []string{"model", "current-model"}},
		{"Settings", []string{"permissions", "config"}},
		{"Output", []string{"cost", "diff", "export", "version"}},
		{"Tools", []string{"agents", "skills"}},
	}

	for _, cat := range categories {
		lines = append(lines, cat.name+":")
		for _, cmd := range cat.cmds {
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
			continue // hide alias from completions
		}
		completions = append(completions, "/"+name)
	}
	sort.Strings(completions)
	return completions
}

// Built-in command handlers

// HelpHandler returns the help text
func HelpHandler(registry *SlashRegistry) SlashHandler {
	return func(args string) (string, error) {
		if args != "" {
			// Specific command help
			if desc, exists := registry.help[args]; exists {
				return fmt.Sprintf("/%s - %s", args, desc), nil
			}
			return fmt.Sprintf("Unknown command: /%s", args), nil
		}
		return registry.GetHelp(), nil
	}
}

// StatusHandler returns status information
func StatusHandler(getStatus func() string) SlashHandler {
	return func(args string) (string, error) {
		return getStatus(), nil
	}
}

// ClearHandler clears the session and optionally the TUI chat
func ClearHandler(clearFn func() error, clearChatFn func()) SlashHandler {
	return func(args string) (string, error) {
		if err := clearFn(); err != nil {
			return "", err
		}
		if clearChatFn != nil {
			clearChatFn()
		}
		return "Session cleared.", nil
	}
}

// CompactHandler compacts the session
func CompactHandler(compactFn func() (string, error)) SlashHandler {
	return func(args string) (string, error) {
		return compactFn()
	}
}

// CostHandler returns cost information
func CostHandler(getCost func() string) SlashHandler {
	return func(args string) (string, error) {
		return getCost(), nil
	}
}

// CurrentModelHandler shows the current model
func CurrentModelHandler(getModel func() string) SlashHandler {
	return func(args string) (string, error) {
		return fmt.Sprintf("Current model: %s", getModel()), nil
	}
}

// ModelHandler handles model switching
func ModelHandler(getModel func() string, setModel func(string) error, listModels func() []string) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			current := getModel()
			models := listModels()
			result := fmt.Sprintf("Model\n  Current          %s\n\nAvailable models:\n", current)
			for _, m := range models {
				marker := "  "
				if m == current {
					marker = "● "
				}
				result += fmt.Sprintf("%s%s\n", marker, m)
			}
			return result, nil
		}

		previous := getModel()
		if err := setModel(args); err != nil {
			return "", err
		}

		return fmt.Sprintf(`Model updated
  Previous         %s
  Current          %s
  Preserved        Conversation context maintained
  Tip              Existing conversation context stayed attached`, previous, args), nil
	}
}

// PermissionsHandler handles permission mode switching
func PermissionsHandler(getMode func() string, setMode func(string) error, getReport func() string) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			return getReport(), nil
		}

		previous := getMode()
		if err := setMode(args); err != nil {
			return "", err
		}

		return fmt.Sprintf(`Permissions updated
  Previous mode    %s
  Active mode      %s
  Applies to       Subsequent tool calls in this session
  Tip              Run /permissions to review all available modes`, previous, args), nil
	}
}

// ConfigHandler shows configuration
func ConfigHandler(getConfig func() string) SlashHandler {
	return func(args string) (string, error) {
		return getConfig(), nil
	}
}

// ExportHandler exports the session
func ExportHandler(exportFn func(path string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		path, err := exportFn(args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Export\n  Result           wrote transcript\n  File             %s", path), nil
	}
}

// DiffHandler shows git diff
func DiffHandler(getDiff func() string) SlashHandler {
	return func(args string) (string, error) {
		diff := getDiff()
		if diff == "" {
			return "No changes detected in workspace.", nil
		}
		return diff, nil
	}
}

// VersionHandler returns version information
func VersionHandler(version, buildInfo string) SlashHandler {
	return func(args string) (string, error) {
		result := fmt.Sprintf("agent-harness %s", version)
		if buildInfo != "" {
			result += "\n" + buildInfo
		}
		return result, nil
	}
}

// MemoryHandler shows memory information
func MemoryHandler(getMemory func() string) SlashHandler {
	return func(args string) (string, error) {
		return getMemory(), nil
	}
}

// AgentsHandler handles agent-related commands
func AgentsHandler(handleFn func(args string) string) SlashHandler {
	return func(args string) (string, error) {
		return handleFn(args), nil
	}
}

// SkillsHandler handles skill-related commands
func SkillsHandler(handleFn func(args string) string) SlashHandler {
	return func(args string) (string, error) {
		return handleFn(args), nil
	}
}

// SessionHandler handles session commands
func SessionHandler(listSessions func() string, loadSession func(id string) error) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listSessions(), nil
		}
		if strings.HasPrefix(args, "load ") {
			id := strings.TrimPrefix(args, "load ")
			if err := loadSession(id); err != nil {
				return "", err
			}
			return fmt.Sprintf("Session loaded: %s", id), nil
		}
		return "Usage: /session [list|load <id>]", nil
	}
}

// ResetHandler handles resetting agent harness
func ResetHandler(resetFn func() error) SlashHandler {
	return func(args string) (string, error) {
		if args != "--confirm" && args != "-y" {
			return "reset: WARNING - this will delete your encrypted credentials and ALL session history. This action cannot be undone. Rerun with /reset --confirm to proceed.", nil
		}
		if err := resetFn(); err != nil {
			return "", err
		}
		return "__RESET__", nil
	}
}

// IsReset checks if the result is a reset command
func IsReset(result string) bool {
	return result == "__RESET__"
}

// QuitHandler handles quitting
func QuitHandler() SlashHandler {
	return func(args string) (string, error) {
		return "__QUIT__", nil
	}
}

// IsQuit checks if the result is a quit command
func IsQuit(result string) bool {
	return result == "__QUIT__"
}
