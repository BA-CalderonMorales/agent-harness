package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileConfig represents settings loaded from disk.
type FileConfig struct {
	Provider         string            `json:"provider,omitempty"`
	Model            string            `json:"model,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	Verbose          bool              `json:"verbose,omitempty"`
	AlwaysAllow      []string          `json:"always_allow,omitempty"`
	AlwaysDeny       []string          `json:"always_deny,omitempty"`
	McpServers       map[string]McpServer `json:"mcp_servers,omitempty"`
	CustomEnv        map[string]string `json:"custom_env,omitempty"`
}

// McpServer describes an MCP server configuration.
type McpServer struct {
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// LoadFile reads configuration from a JSON file.
func LoadFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{}, nil
		}
		return nil, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes configuration to a JSON file.
func (c *FileConfig) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness", "config.json")
}
