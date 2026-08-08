package commands

import (
	"fmt"
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
