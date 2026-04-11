package commands

import (
	"fmt"
	"strings"
)

// Command represents a slash command available in the REPL.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Handler     func(args []string) (string, error)
}

// Registry holds all registered slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
	}
}

// Find looks up a command by name.
func (r *Registry) Find(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// All returns all unique commands.
func (r *Registry) All() []Command {
	seen := make(map[string]bool)
	var out []Command
	for _, cmd := range r.commands {
		if seen[cmd.Name] {
			continue
		}
		seen[cmd.Name] = true
		out = append(out, cmd)
	}
	return out
}

// Parse extracts a command name and args from user input.
func Parse(input string) (name string, args []string, ok bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", nil, false
	}
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return "", nil, false
	}
	return parts[0], parts[1:], true
}

// BuiltInCommands returns the standard slash commands.
func BuiltInCommands() []Command {
	return []Command{
		{
			Name:        "clear",
			Description: "Clear the conversation history",
			Handler: func(args []string) (string, error) {
				return "history_cleared", nil
			},
		},
		{
			Name:        "compact",
			Description: "Manually trigger context compaction",
			Handler: func(args []string) (string, error) {
				return "compact_triggered", nil
			},
		},
		{
			Name:        "model",
			Description: "Switch the active model",
			Usage:       "/model <name>",
			Handler: func(args []string) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: /model <name>")
				}
				return fmt.Sprintf("model_changed:%s", args[0]), nil
			},
		},
		{
			Name:        "help",
			Description: "Show available commands",
			Handler: func(args []string) (string, error) {
				return "help_displayed", nil
			},
		},
		{
			Name:        "exit",
			Aliases:     []string{"quit"},
			Description: "Exit the application",
			Handler: func(args []string) (string, error) {
				return "exit_requested", nil
			},
		},
	}
}

// WorkspaceHandler returns a slash handler for showing workspace info
func WorkspaceHandler(infoFunc func() string) func(string) (string, error) {
	return func(args string) (string, error) {
		return infoFunc(), nil
	}
}
