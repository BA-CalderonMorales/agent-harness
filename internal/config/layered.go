// Layered configuration system inspired by claw-code
// Supports user, project, and local config layers with precedence

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigSource represents the source of a configuration entry
type ConfigSource int

const (
	SourceUser ConfigSource = iota
	SourceProject
	SourceLocal
)

func (s ConfigSource) String() string {
	switch s {
	case SourceUser:
		return "user"
	case SourceProject:
		return "project"
	case SourceLocal:
		return "local"
	default:
		return "unknown"
	}
}

// ConfigEntry represents a single configuration file entry
type ConfigEntry struct {
	Source ConfigSource
	Path   string
}

// LayeredConfig holds merged configuration from all sources
type LayeredConfig struct {
	merged         map[string]interface{}
	loadedEntries  []ConfigEntry
	Provider       string
	APIKey         string
	Model          string
	PermissionMode PermissionMode
	ExecutionMode  string // "interactive" or "yolo"
	AlwaysAllow    []string
	AlwaysDeny     []string
	McpServers     map[string]McpServerConfig
	CustomEnv      map[string]string

	// Granular permissions (override PermissionMode when set)
	PermRead    bool // Allow read/search tools
	PermWrite   bool // Allow write/edit tools
	PermDelete  bool // Allow delete/remove tools
	PermExecute bool // Allow execute/bash tools
}

// PermissionMode controls what tools can do
type PermissionMode int

const (
	PermissionReadOnly PermissionMode = iota
	PermissionWorkspaceWrite
	PermissionDangerFullAccess
)

func (p PermissionMode) String() string {
	switch p {
	case PermissionReadOnly:
		return "read-only"
	case PermissionWorkspaceWrite:
		return "workspace-write"
	case PermissionDangerFullAccess:
		return "danger-full-access"
	default:
		return "unknown"
	}
}

func (p PermissionMode) Description() string {
	switch p {
	case PermissionReadOnly:
		return "Only read/search tools can run automatically"
	case PermissionWorkspaceWrite:
		return "Editing tools can modify files in the workspace"
	case PermissionDangerFullAccess:
		return "All tools can run without additional sandbox limits"
	default:
		return "Unknown permission mode"
	}
}

// ParsePermissionMode parses a permission mode string
func ParsePermissionMode(s string) (PermissionMode, error) {
	switch s {
	case "read-only", "readonly", "plan":
		return PermissionReadOnly, nil
	case "workspace-write", "workspace", "auto", "accept-edits":
		return PermissionWorkspaceWrite, nil
	case "danger-full-access", "danger", "dont-ask", "full":
		return PermissionDangerFullAccess, nil
	default:
		return PermissionReadOnly, fmt.Errorf("unknown permission mode: %s", s)
	}
}

// McpServerConfig represents an MCP server configuration
type McpServerConfig struct {
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// LayeredLoader handles loading and merging configuration layers
type LayeredLoader struct {
	cwd        string
	configHome string
}

// NewLayeredLoader creates a new layered config loader
func NewLayeredLoader(cwd string) *LayeredLoader {
	configHome := defaultConfigHome()
	return &LayeredLoader{
		cwd:        cwd,
		configHome: configHome,
	}
}

// defaultConfigHome returns the default config home directory
func defaultConfigHome() string {
	if env := os.Getenv("AGENT_HARNESS_CONFIG_HOME"); env != "" {
		return env
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-harness")
	}
	return ".agent-harness"
}

// Discover returns all configuration file entries in precedence order
func (ll *LayeredLoader) Discover() []ConfigEntry {
	entries := []ConfigEntry{
		// User-level configs
		{Source: SourceUser, Path: filepath.Join(ll.configHome, "settings.json")},
		// Project-level configs
		{Source: SourceProject, Path: filepath.Join(ll.cwd, ".agent-harness", "settings.json")},
		// Local configs (gitignored)
		{Source: SourceLocal, Path: filepath.Join(ll.cwd, ".agent-harness", "settings.local.json")},
	}
	return entries
}

// Load loads and merges all configuration layers
func (ll *LayeredLoader) Load() (*LayeredConfig, error) {
	config := &LayeredConfig{
		merged:        make(map[string]interface{}),
		loadedEntries: make([]ConfigEntry, 0),
		McpServers:    make(map[string]McpServerConfig),
		CustomEnv:     make(map[string]string),
	}

	for _, entry := range ll.Discover() {
		data, err := ll.loadFile(entry.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to load %s: %w", entry.Path, err)
		}

		ll.deepMerge(config.merged, data)
		config.loadedEntries = append(config.loadedEntries, entry)
	}

	// Extract values from merged config
	ll.extractValues(config)

	return config, nil
}

// loadFile loads a single JSON config file
func (ll *LayeredLoader) loadFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return result, nil
}

// deepMerge recursively merges source into target
func (ll *LayeredLoader) deepMerge(target, source map[string]interface{}) {
	for key, value := range source {
		if targetValue, exists := target[key]; exists {
			// If both are maps, merge recursively
			targetMap, targetIsMap := targetValue.(map[string]interface{})
			sourceMap, sourceIsMap := value.(map[string]interface{})
			if targetIsMap && sourceIsMap {
				ll.deepMerge(targetMap, sourceMap)
				continue
			}
		}
		// Otherwise, overwrite
		target[key] = value
	}
}

// extractValues extracts typed values from the merged config
func (ll *LayeredLoader) extractValues(config *LayeredConfig) {
	if v, ok := config.merged["provider"].(string); ok {
		config.Provider = v
	}
	if v, ok := config.merged["api_key"].(string); ok {
		config.APIKey = v
	}
	if v, ok := config.merged["model"].(string); ok {
		config.Model = v
	}
	if v, ok := config.merged["permission_mode"].(string); ok {
		if mode, err := ParsePermissionMode(v); err == nil {
			config.PermissionMode = mode
		}
	}
	if v, ok := config.merged["execution_mode"].(string); ok {
		config.ExecutionMode = v
	}
	if v, ok := config.merged["always_allow"].([]interface{}); ok {
		config.AlwaysAllow = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				config.AlwaysAllow = append(config.AlwaysAllow, s)
			}
		}
	}
	if v, ok := config.merged["always_deny"].([]interface{}); ok {
		config.AlwaysDeny = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				config.AlwaysDeny = append(config.AlwaysDeny, s)
			}
		}
	}

	// Extract MCP servers
	if mcpServers, ok := config.merged["mcpServers"].(map[string]interface{}); ok {
		for name, serverData := range mcpServers {
			if serverMap, ok := serverData.(map[string]interface{}); ok {
				config.McpServers[name] = parseMcpServerConfig(serverMap)
			}
		}
	}

	// Extract custom env
	if env, ok := config.merged["env"].(map[string]interface{}); ok {
		config.CustomEnv = make(map[string]string)
		for k, v := range env {
			if s, ok := v.(string); ok {
				config.CustomEnv[k] = s
			}
		}
	}
}

func parseMcpServerConfig(data map[string]interface{}) McpServerConfig {
	config := McpServerConfig{}
	if v, ok := data["transport"].(string); ok {
		config.Transport = v
	}
	if v, ok := data["command"].(string); ok {
		config.Command = v
	}
	if v, ok := data["url"].(string); ok {
		config.URL = v
	}
	if args, ok := data["args"].([]interface{}); ok {
		config.Args = make([]string, 0, len(args))
		for _, arg := range args {
			if s, ok := arg.(string); ok {
				config.Args = append(config.Args, s)
			}
		}
	}
	if env, ok := data["env"].(map[string]interface{}); ok {
		config.Env = make(map[string]string)
		for k, v := range env {
			if s, ok := v.(string); ok {
				config.Env[k] = s
			}
		}
	}
	return config
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
		"model":           config.Model,
		"permission_mode": config.PermissionMode.String(),
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
func (lc *LayeredConfig) GetConfigReport() string {
	var result string

	result += "Configuration\n"
	result += fmt.Sprintf("  Provider         %s\n", lc.Provider)
	result += fmt.Sprintf("  Model            %s\n", lc.Model)
	result += fmt.Sprintf("  Permission mode  %s\n", lc.PermissionMode.String())
	result += "\n"

	result += "Loaded from\n"
	for _, entry := range lc.loadedEntries {
		result += fmt.Sprintf("  %s\n", entry.Path)
	}

	if len(lc.McpServers) > 0 {
		result += "\nMCP Servers\n"
		for name := range lc.McpServers {
			result += fmt.Sprintf("  %s\n", name)
		}
	}

	return result
}

// GetPermissionReport returns a formatted permission mode report
func (lc *LayeredConfig) GetPermissionReport() string {
	modes := []struct {
		name        string
		description string
		current     bool
	}{
		{"read-only", "Read/search tools only", lc.PermissionMode == PermissionReadOnly},
		{"workspace-write", "Edit files inside the workspace", lc.PermissionMode == PermissionWorkspaceWrite},
		{"danger-full-access", "Unrestricted tool access", lc.PermissionMode == PermissionDangerFullAccess},
	}

	result := "Permissions\n"
	result += fmt.Sprintf("  Active mode      %s\n", lc.PermissionMode.String())
	result += fmt.Sprintf("  Effect           %s\n", lc.PermissionMode.Description())
	result += "\n"
	result += "Modes\n"

	for _, mode := range modes {
		marker := "○ available"
		if mode.current {
			marker = "● current"
		}
		result += fmt.Sprintf("  %-18s %-11s %s\n", mode.name, marker, mode.description)
	}

	result += "\n"
	result += "Next\n"
	result += "  /permissions              Show the current mode\n"
	result += "  /permissions <mode>       Switch modes for subsequent tool calls\n"

	return result
}
