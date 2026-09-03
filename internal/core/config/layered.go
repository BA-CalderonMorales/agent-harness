// Layered configuration system inspired by claw-code
// Supports user, project, and local config layers with precedence

package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	yaml "go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"strings"
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
	merged        map[string]interface{}
	loadedEntries []ConfigEntry
	Provider      string
	APIKey        string
	Model         string
	// ModelPinned is true when AH_MODEL pinned the model in the
	// environment: the pin outranks a resumed session's stored model,
	// exactly like EndpointPinned outranks provider defaults.
	ModelPinned    bool
	EndpointPinned bool // endpoint came from AH_ENDPOINT_URL; survives provider switches
	Runtime        string
	ModelPath      string
	EndpointURL    string
	ContextLength  int
	Temperature    float64
	MaxTokens      int
	WorkspacePath  string
	ServerCommand  string
	PermissionMode PermissionMode
	ExecutionMode  string // "interactive" or "yolo"
	AlwaysAllow    []string
	AlwaysDeny     []string
	McpServers     map[string]mcp.McpServerConfig
	CustomEnv      map[string]string

	// Granular permissions (override PermissionMode when set)
	PermRead     bool // Allow read/search tools
	PermWrite    bool // Allow write/edit tools
	PermDelete   bool // Allow delete/remove tools
	PermExecute  bool // Allow execute/bash tools
	PermExplicit bool

	// Persona determines the agent's behavioral mode
	Persona string

	// Effort is the reasoning effort level used per request (low, medium, high)
	Effort string

	// StreamIdleTimeout is the stream-idle watchdog window; 0 means the
	// provider default applies.
	StreamIdleTimeout time.Duration
	// HTTPTimeout is the HTTP client timeout; 0 means the provider default
	// applies.
	HTTPTimeout time.Duration
	// TimeoutPinned is true when an environment override pins the timeouts
	// so provider switches must not recompute them.
	TimeoutPinned bool

	// SessionDir overrides the default session storage directory
	SessionDir string
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
	return ConfigHome()
}

// Discover returns all configuration file entries in precedence order.
// Later entries override earlier ones: project defaults < user settings <
// local overrides. The user settings layer sits above project defaults so a
// provider/model chosen in a previous session survives restarts even when a
// tracked agent-harness.yml pins its own values.
func (ll *LayeredLoader) Discover() []ConfigEntry {
	entries := []ConfigEntry{
		// Project-level configs
		{Source: SourceProject, Path: filepath.Join(ll.cwd, "agent-harness.yml")},
		{Source: SourceProject, Path: filepath.Join(ll.cwd, ".agent-harness.yml")},
		{Source: SourceProject, Path: filepath.Join(ll.cwd, ".agent-harness", "settings.json")},
		// User-level configs
		{Source: SourceUser, Path: filepath.Join(ll.configHome, "settings.json")},
		{Source: SourceUser, Path: filepath.Join(ll.configHome, ".mcp.json")},
		{Source: SourceProject, Path: filepath.Join(ll.cwd, ".mcp.json")},
		// Local configs (gitignored, highest precedence)
		{Source: SourceLocal, Path: filepath.Join(ll.cwd, ".agent-harness", "settings.local.json")},
	}
	return entries
}

// Load loads and merges all configuration layers
func (ll *LayeredLoader) Load() (*LayeredConfig, error) {
	config := &LayeredConfig{
		merged:        make(map[string]interface{}),
		loadedEntries: make([]ConfigEntry, 0),
		Provider:      DefaultProvider,
		Runtime:       DefaultRuntime,
		ContextLength: DefaultContextLength,
		Temperature:   DefaultTemperature,
		MaxTokens:     DefaultMaxTokens,
		ServerCommand: DefaultLocalServerCommand,
		McpServers:    make(map[string]mcp.McpServerConfig),
		CustomEnv:     make(map[string]string),
	}

	for _, entry := range ll.Discover() {
		if strings.HasSuffix(entry.Path, ".mcp.json") {
			servers, err := mcp.LoadFile(entry.Path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("failed to load %s: %w", entry.Path, err)
			}
			for name, cfg := range servers {
				config.McpServers[name] = cfg
			}
			config.loadedEntries = append(config.loadedEntries, entry)
			continue
		}

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

// loadFile loads a single JSON or YAML config file.
func (ll *LayeredLoader) loadFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	switch filepath.Ext(path) {
	case ".yml", ".yaml":
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	default:
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	return result, nil
}
