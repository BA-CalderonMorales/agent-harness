package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// McpServerConfig represents a single MCP server configuration.
type McpServerConfig struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// LoadFile loads an .mcp.json file, supporting both the top-level mcpServers
// key and top-level server entries directly.
func LoadFile(path string) (map[string]McpServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try top-level mcpServers first.
	var withServers struct {
		McpServers map[string]McpServerConfig `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &withServers); err == nil && len(withServers.McpServers) > 0 {
		for name, cfg := range withServers.McpServers {
			if cfg.Type == "" {
				cfg.Type = "stdio"
			}
			withServers.McpServers[name] = cfg
		}
		return withServers.McpServers, nil
	}

	// Fall back to top-level server entries directly.
	var direct map[string]McpServerConfig
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, fmt.Errorf("invalid mcp config file %s: %w", path, err)
	}
	for name, cfg := range direct {
		if cfg.Type == "" {
			cfg.Type = "stdio"
		}
		direct[name] = cfg
	}
	return direct, nil
}
