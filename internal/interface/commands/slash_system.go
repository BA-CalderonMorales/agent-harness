package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
)

// humanSize renders a byte count for the /cleanup listing.
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
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

// CleanupHandler lists saved sessions by size and age, or deletes them
// with confirmation (goal 0.3.28 Task 8). Deletion goes through the
// caller-provided function (SessionManager.DeleteSession — never raw
// os.Remove) so the audit trail and the active-session guard hold.
// Forms:
//
//	/cleanup            — list sessions with size/age, no deletion
//	/cleanup <id>...    — dry-run preview of what would be deleted
//	/cleanup <id>... --confirm — delete the named sessions
//	/cleanup --all --confirm   — delete every non-active session
func CleanupHandler(list func() []CleanupSession, del func(id string) error) SlashHandler {
	return func(args string) (string, error) {
		confirm := false
		all := false
		var ids []string
		for _, tok := range strings.Fields(args) {
			switch tok {
			case "--confirm", "-y":
				confirm = true
			case "--all", "-a":
				all = true
			default:
				ids = append(ids, tok)
			}
		}

		sessions := list()
		if len(sessions) == 0 {
			return "No saved sessions to clean up.", nil
		}

		if !confirm {
			var b strings.Builder
			b.WriteString("Saved sessions (oldest first):\n")
			for _, s := range sessions {
				b.WriteString(fmt.Sprintf("  %s  %6s  %d msgs  %s\n",
					s.ID, humanSize(s.SizeBytes), s.MessageCount, s.UpdatedAt.Format("2006-01-02 15:04")))
			}
			if all {
				b.WriteString("\nRerun with --confirm to delete ALL non-active sessions.")
			} else if len(ids) > 0 {
				b.WriteString("\nRerun with --confirm to delete the listed sessions.")
			} else {
				b.WriteString("\n/cleanup <id>... --confirm to delete, or /cleanup --all --confirm for everything non-active.")
			}
			return b.String(), nil
		}

		var targets []string
		if all {
			for _, s := range sessions {
				targets = append(targets, s.ID)
			}
		} else {
			targets = ids
		}
		if len(targets) == 0 {
			return "cleanup: nothing to delete (name session IDs, or use --all).", nil
		}

		var deleted, failed int
		var b strings.Builder
		for _, id := range targets {
			if err := del(id); err != nil {
				failed++
				b.WriteString(fmt.Sprintf("  ✗ %s: %v\n", id, err))
			} else {
				deleted++
			}
		}
		return fmt.Sprintf("cleanup: deleted %d, failed %d\n%s", deleted, failed, b.String()), nil
	}
}

// CleanupSession is one row of the /cleanup listing: the metadata the
// sessions view already carries, plus the on-disk size the metadata
// cache tracks.
type CleanupSession struct {
	ID           string
	UpdatedAt    time.Time
	MessageCount int
	SizeBytes    int64
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
		if args == "list" {
			if listPersonas == nil {
				return "", fmt.Errorf("persona listing is not available")
			}
			return listPersonas(), nil
		}

		if getPersona == nil || setPersona == nil {
			return "", fmt.Errorf("persona switching is not available")
		}

		// Bare /persona cycles: the next behavior mode wraps, "list"
		// remains the catalog.
		if args == "" {
			current := getPersona()
			next := NextInList(personaNames(), current)
			if next == "" {
				return "", fmt.Errorf("no personas available")
			}
			args = next
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

// personaNames lists the behavior modes in catalog order.
func personaNames() []string {
	names := make([]string, 0, len(persona.All()))
	for _, p := range persona.All() {
		names = append(names, p.String())
	}
	return names
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
