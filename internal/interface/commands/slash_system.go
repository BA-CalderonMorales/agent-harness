package commands

import (
	"fmt"
	"strings"
)

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
