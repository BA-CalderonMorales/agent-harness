package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveSettings writes an explicit set of values to a configuration layer,
// deep-merging with any existing file so unrelated keys survive. Unlike Save,
// it persists only the keys the caller passes, keeping user settings a delta
// over the tracked project defaults instead of a frozen snapshot.
func (ll *LayeredLoader) SaveSettings(source ConfigSource, values map[string]interface{}) error {
	var path string
	switch source {
	case SourceUser:
		path = filepath.Join(ll.configHome, "settings.json")
	case SourceProject:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.json")
	case SourceLocal:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.local.json")
	}

	merged := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &merged); err != nil {
			return fmt.Errorf("failed to parse existing config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config %s: %w", path, err)
	}
	ll.deepMerge(merged, values)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(path, append(jsonData, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Save saves the configuration to a specific layer
func (ll *LayeredLoader) Save(source ConfigSource, config *LayeredConfig) error {
	var path string
	switch source {
	case SourceUser:
		path = filepath.Join(ll.configHome, "settings.json")
	case SourceProject:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.json")
	case SourceLocal:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.local.json")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Build config data
	data := map[string]interface{}{
		"provider":        config.Provider,
		"runtime":         config.Runtime,
		"model":           config.Model,
		"model_path":      config.ModelPath,
		"endpoint_url":    config.EndpointURL,
		"context_length":  config.ContextLength,
		"temperature":     config.Temperature,
		"max_tokens":      config.MaxTokens,
		"workspace_path":  config.WorkspacePath,
		"permission_mode": config.PermissionMode.String(),
		"perm_read":       config.PermRead,
		"perm_write":      config.PermWrite,
		"perm_delete":     config.PermDelete,
		"perm_execute":    config.PermExecute,
		"persona":         config.Persona,
	}
	if config.ServerCommand != "" {
		data["local_server_command"] = config.ServerCommand
	}
	if config.Theme != "" {
		data["theme"] = config.Theme
	}

	if len(config.AlwaysAllow) > 0 {
		data["always_allow"] = config.AlwaysAllow
	}
	if len(config.AlwaysDeny) > 0 {
		data["always_deny"] = config.AlwaysDeny
	}
	if len(config.McpServers) > 0 {
		data["mcpServers"] = config.McpServers
	}
	if len(config.CustomEnv) > 0 {
		data["env"] = config.CustomEnv
	}

	// Marshal with indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(path, append(jsonData, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetConfigReport returns a formatted report of the current configuration
