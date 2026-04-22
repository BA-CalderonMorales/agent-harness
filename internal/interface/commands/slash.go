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

	// Track which commands have been categorized
	categorized := make(map[string]bool)
	for _, cat := range categories {
		lines = append(lines, cat.name+":")
		for _, cmd := range cat.cmds {
			categorized[cmd] = true
			if desc, exists := sr.help[cmd]; exists {
				lines = append(lines, fmt.Sprintf("  /%-15s %s", cmd, desc))
			}
		}
		lines = append(lines, "")
	}

	// Catch any registered commands not in the hardcoded categories
	// (skip hidden aliases like "exit")
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

// ClearHandler clears the session and optionally the TUI chat.
// When clearChatFn is provided, it receives the confirmation message to display;
// the handler returns an empty string to avoid double-adding the message.
func ClearHandler(clearFn func() error, clearChatFn func(string)) SlashHandler {
	return func(args string) (string, error) {
		if err := clearFn(); err != nil {
			return "", err
		}
		if clearChatFn != nil {
			clearChatFn("Session cleared.")
			return "", nil
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

// CommitHandler stages changes and commits with the given message.
func CommitHandler(commitFn func(message string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			return "Usage: /commit <message>\nStages all changes and commits.", nil
		}
		return commitFn(args)
	}
}

// BranchHandler handles git branch operations.
func BranchHandler(listFn func() (string, error), createFn func(string) (string, error), switchFn func(string) (string, error), deleteFn func(string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		parts := strings.SplitN(args, " ", 2)
		subcmd := parts[0]
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}
		switch subcmd {
		case "create":
			if name == "" {
				return "Usage: /branch create <name>", nil
			}
			return createFn(name)
		case "switch":
			if name == "" {
				return "Usage: /branch switch <name>", nil
			}
			return switchFn(name)
		case "delete":
			if name == "" {
				return "Usage: /branch delete <name>", nil
			}
			return deleteFn(name)
		default:
			return fmt.Sprintf("Unknown branch command: %s\nUsage: /branch [list|create <name>|switch <name>|delete <name>]", subcmd), nil
		}
	}
}

// PlanHandler toggles plan mode.
func PlanHandler(getMode func() bool, setMode func(bool) string) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			if getMode() {
				return setMode(false), nil
			}
			return setMode(true), nil
		}
		switch args {
		case "on":
			return setMode(true), nil
		case "off":
			return setMode(false), nil
		default:
			return "Usage: /plan [on|off]\nToggles plan mode. In plan mode the agent outlines steps before executing.", nil
		}
	}
}

// PRHandler handles pull request operations via gh CLI.
func PRHandler(createFn func(title, body string) (string, error), listFn func() (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		// /pr create "title" [body]
		if strings.HasPrefix(args, "create ") {
			rest := strings.TrimPrefix(args, "create ")
			if rest == "" {
				return "Usage: /pr create \"title\" [body]", nil
			}
			// Simple parsing: first quoted string is title, rest is body
			if strings.HasPrefix(rest, "\"") {
				end := strings.Index(rest[1:], "\"")
				if end == -1 {
					return "Usage: /pr create \"title\" [body]", nil
				}
				title := rest[1 : end+1]
				body := strings.TrimSpace(rest[end+2:])
				return createFn(title, body)
			}
			// Unquoted: first word is title
			parts := strings.SplitN(rest, " ", 2)
			title := parts[0]
			body := ""
			if len(parts) > 1 {
				body = parts[1]
			}
			return createFn(title, body)
		}
		return "Usage: /pr [list|create \"title\" [body]]", nil
	}
}

// InitHandler scaffolds a new project with standard files.
func InitHandler(initFn func(projectType string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		projectType := args
		if projectType == "" {
			projectType = "generic"
		}
		return initFn(projectType)
	}
}

// MemoryHandler shows system prompt and context state.
func MemoryHandler(getMemory func() string) SlashHandler {
	return func(args string) (string, error) {
		return getMemory(), nil
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

// WorktreeHandler handles git worktree commands
func WorktreeHandler(listFn func() (string, error), addFn func(path, branch string) (string, error), removeFn func(path string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			return listFn()
		}
		parts := strings.SplitN(args, " ", 2)
		switch parts[0] {
		case "add":
			if len(parts) < 2 {
				return "Usage: /worktree add <path> [branch]", nil
			}
			addParts := strings.SplitN(parts[1], " ", 2)
			path := addParts[0]
			branch := ""
			if len(addParts) > 1 {
				branch = addParts[1]
			}
			return addFn(path, branch)
		case "remove":
			if len(parts) < 2 {
				return "Usage: /worktree remove <path>", nil
			}
			return removeFn(parts[1])
		default:
			return "Usage: /worktree [list|add <path> [branch]|remove <path>]", nil
		}
	}
}

// AgentsHandler handles agent-related commands
func AgentsHandler(handleFn func(args string) string) SlashHandler {
	return func(args string) (string, error) {
		return handleFn(args), nil
	}
}

// TestHandler handles running project tests
func TestHandler(runFn func() (string, error)) SlashHandler {
	return func(args string) (string, error) {
		return runFn()
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

// LogoutHandler handles logout - clears credentials from memory and storage.
func LogoutHandler(logoutFn func() error) SlashHandler {
	return func(args string) (string, error) {
		if err := logoutFn(); err != nil {
			return "", err
		}
		return "Logged out. Credentials cleared from memory and storage. Run /login to authenticate.", nil
	}
}

// AuditHandler shows recent audit entries.
func AuditHandler(getAudit func() string) SlashHandler {
	return func(args string) (string, error) {
		return getAudit(), nil
	}
}

// PersonaHandler handles persona switching.
func PersonaHandler(getPersona func() string, setPersona func(string) error, listPersonas func() string) SlashHandler {
	return func(args string) (string, error) {
		if args == "" || args == "list" {
			if listPersonas == nil {
				return "", fmt.Errorf("persona listing is not available")
			}
			return listPersonas(), nil
		}

		if getPersona == nil || setPersona == nil {
			return "", fmt.Errorf("persona switching is not available")
		}

		previous := getPersona()
		if err := setPersona(args); err != nil {
			return "", err
		}
		current := getPersona()

		return fmt.Sprintf(`Persona updated
  Previous         %s
  Current          %s
  Tip              Personality and tool hints updated for this session`, previous, current), nil
	}
}

// LoginHandler handles login - starts the login wizard.
func LoginHandler(startLoginFn func() error) SlashHandler {
	return func(args string) (string, error) {
		if err := startLoginFn(); err != nil {
			return "", err
		}
		return "", nil
	}
}

// IsReset checks if the result is a reset command
func IsReset(result string) bool {
	return result == "__RESET__"
}

// SteerHandler queues a message for the current chat turn without interrupting
// the agent. The queued message is auto-submitted after the turn completes.
func SteerHandler(queueFn func(string)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			return "Usage: /steer <message>\nQueue a message for the current chat without interrupting the agent.", nil
		}
		queueFn(args)
		return "", nil
	}
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
