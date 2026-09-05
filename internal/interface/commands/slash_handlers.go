package commands

import (
	"fmt"
	"strconv"
	"strings"
)

// Built-in command handlers

// HelpHandler returns the help text
func HelpHandler(registry *SlashRegistry) SlashHandler {
	return func(args string) (string, error) {
		if args != "" {
			if desc, exists := registry.help[args]; exists {
				return fmt.Sprintf("/%s - %s", args, desc), nil
			}
			if registry.IsFeatureFlagged(args) {
				return registry.FeatureFlagMessage(args), nil
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

// LimitHandler manages the session-scoped tool-call ceiling (the /limit
// knob). No args reports the current ceiling; a number sets it for this
// session only; anything else is rejected honestly.
func LimitHandler(getLimit func() int, setLimit func(int) error) SlashHandler {
	const defaultLimit = 15
	const maxLimit = 100
	return func(args string) (string, error) {
		if args == "" {
			current := getLimit()
			if current <= 0 {
				current = defaultLimit
			}
			return fmt.Sprintf("Tool call limit: %d (default %d). Use /limit <n> to change it for this session.", current, defaultLimit), nil
		}
		n, err := strconv.Atoi(args)
		if err != nil || n < 1 {
			return fmt.Sprintf("Tool call limit must be a number between 1 and %d; got %q.", maxLimit, args), nil
		}
		if n > maxLimit {
			return fmt.Sprintf("Tool call limit capped at %d; got %d.", maxLimit, n), nil
		}
		if err := setLimit(n); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tool call limit set to %d for this session.", n), nil
	}
}

// NextInList returns the entry after current, wrapping — the /theme
// cycle semantics, shared by every selector-style command. An unknown
// or absent current wraps to the first entry.
func NextInList(list []string, current string) string {
	if len(list) == 0 {
		return ""
	}
	for i, item := range list {
		if item == current {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// ModelHandler handles model switching: bare /model cycles to the next
// model in the provider's list — browsing stays on /models.
func ModelHandler(getModel func() string, setModel func(string) error, listModels func() []string) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			current := getModel()
			next := NextInList(listModels(), current)
			if next == "" {
				return "", fmt.Errorf("no models available — /models picks from the full list")
			}
			if next == current {
				// One model, already current: a no-op with a notice, not
				// an error — cycling is never a failure.
				return fmt.Sprintf("Only one model available — already on %s", current), nil
			}
			if err := setModel(next); err != nil {
				return "", err
			}
			return fmt.Sprintf(`Model cycled
  Previous         %s
  Current          %s
  Preserved        Conversation context maintained`, current, next), nil
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

// ConfigHandler shows or updates configuration
func ConfigHandler(getConfig func() string, setConfig func(key, value string) (string, error)) SlashHandler {
	return func(args string) (string, error) {
		if args == "" {
			return getConfig(), nil
		}
		parts := strings.SplitN(args, " ", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = strings.TrimSpace(parts[1])
		}
		if key == "set" && val != "" {
			subParts := strings.SplitN(val, " ", 2)
			key = subParts[0]
			val = ""
			if len(subParts) > 1 {
				val = strings.TrimSpace(subParts[1])
			}
		}
		if setConfig == nil {
			return "", fmt.Errorf("configuration modification not supported")
		}
		return setConfig(key, val)
	}
}

// SettingsHandler returns a switch to settings command
func SettingsHandler() SlashHandler {
	return func(args string) (string, error) {
		return "__SETTINGS__", nil
	}
}

// IsSettings checks if the result is a settings tab command
func IsSettings(result string) bool {
	return result == "__SETTINGS__"
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
