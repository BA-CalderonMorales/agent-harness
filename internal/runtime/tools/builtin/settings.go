package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// SettingsTool manages user configuration.
var SettingsTool = tools.NewTool(tools.Tool{
	Name:        "settings",
	Description: "Read or update agent-harness settings (model, provider, etc.).",
	InputSchema: func() map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"get", "set"},
					"description": "Whether to read or write settings",
				},
				"key": map[string]any{"type": "string"},
				"value": map[string]any{
					"type":        "string",
					"description": "Value to set (only for action=set)",
				},
			},
			"required": []string{"action"},
		}
	},
	Capabilities: tools.CapabilityFlags{
		IsEnabled:         func() bool { return true },
		IsConcurrencySafe: func(map[string]any) bool { return true },
		IsReadOnly:        func(map[string]any) bool { return false },
	},
	ValidateInput: func(input map[string]any, ctx tools.Context) tools.ValidationResult {
		action := getString(input, "action")
		if action != "get" && action != "set" {
			return tools.ValidationResult{Valid: false, Message: "action must be 'get' or 'set'"}
		}
		return tools.ValidationResult{Valid: true}
	},
	CheckPermissions: func(input map[string]any, ctx tools.Context) tools.PermissionDecision {
		return tools.PermissionDecision{Behavior: tools.Allow, UpdatedInput: input}
	},
	Call: func(input map[string]any, ctx tools.Context, canUse tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
		action := getString(input, "action")
		key := getString(input, "key")
		configPath := defaultConfigPath()

		data := loadSettingsMap(configPath)

		if action == "get" {
			if key == "" {
				pretty, _ := json.MarshalIndent(data, "", "  ")
				return tools.ToolResult{Data: string(pretty)}, nil
			}
			val, ok := data[key]
			if !ok {
				return tools.ToolResult{Data: fmt.Sprintf("Setting '%s' not found", key)}, nil
			}
			return tools.ToolResult{Data: fmt.Sprintf("%s = %v", key, val)}, nil
		}

		if action == "set" {
			value := getString(input, "value")
			data[key] = value
			if err := saveSettingsMap(configPath, data); err != nil {
				return tools.ToolResult{}, err
			}
			return tools.ToolResult{Data: fmt.Sprintf("Set %s = %s", key, value)}, nil
		}

		return tools.ToolResult{}, fmt.Errorf("unknown action")
	},
	MapResult: func(result any, toolUseID string) types.ToolResultBlock {
		return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
	},
	UserFacingName: func(map[string]any) string { return "settings" },
})

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness", "config.json")
}

func loadSettingsMap(path string) map[string]any {
	data := make(map[string]any)
	b, err := os.ReadFile(path)
	if err != nil {
		return data
	}
	_ = json.Unmarshal(b, &data)
	return data
}

func saveSettingsMap(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
