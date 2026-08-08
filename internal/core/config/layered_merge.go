package config

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
)

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

// extractValues extracts typed values from the merged config.
// Environment variables take precedence over file values.
func (ll *LayeredLoader) extractValues(config *LayeredConfig) {
	if v, ok := stringValue(config.merged, "provider"); ok {
		config.Provider = v
	}
	if v, ok := stringValue(config.merged, "api_key"); ok {
		config.APIKey = v
	}
	if v, ok := stringValue(config.merged, "model"); ok {
		config.Model = v
	}
	if v, ok := stringValue(config.merged, "runtime"); ok {
		config.Runtime = v
	}
	if v, ok := stringValue(config.merged, "model_path"); ok {
		config.ModelPath = v
	}
	if v, ok := stringValue(config.merged, "endpoint_url"); ok {
		config.EndpointURL = v
	}
	if v, ok := intValue(config.merged, "context_length"); ok {
		config.ContextLength = v
	}
	if v, ok := floatValue(config.merged, "temperature"); ok {
		config.Temperature = v
	}
	if v, ok := intValue(config.merged, "max_tokens"); ok {
		config.MaxTokens = v
	}
	if v, ok := stringValue(config.merged, "workspace_path"); ok {
		config.WorkspacePath = v
	}
	if v, ok := stringValue(config.merged, "local_server_command"); ok {
		config.ServerCommand = v
	}
	if v, ok := stringValue(config.merged, "persona"); ok {
		config.Persona = v
	}
	if v, ok := stringValue(config.merged, "reasoning_effort"); ok {
		if v == "low" || v == "medium" || v == "high" {
			config.Effort = v
		}
	}
	if v, ok := stringValue(config.merged, "session_dir"); ok {
		config.SessionDir = v
	}

	if v, ok := stringValue(config.merged, "permission_mode"); ok {
		if mode, err := ParsePermissionMode(v); err == nil {
			config.PermissionMode = mode
		}
	}
	if permissions, ok := mapValue(config.merged, "permissions"); ok {
		if v, ok := stringValue(permissions, "mode"); ok {
			if mode, err := ParsePermissionMode(v); err == nil {
				config.PermissionMode = mode
			}
		}
		ll.extractPermissionToggles(config, permissions)
	}
	ll.extractPermissionToggles(config, config.merged)

	if v, ok := stringValue(config.merged, "execution_mode"); ok {
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

	ll.applyEnvOverrides(config)
	// When the provider comes from the environment but the endpoint does
	// not, the endpoint follows the provider's default. Otherwise a
	// file-pinned endpoint (e.g. a local llama.cpp URL in agent-harness.yml)
	// would keep sending provider traffic - and its API key - to the wrong
	// host.
	if config.EndpointURL == "" || (envSet("AH_PROVIDER", "AGENT_HARNESS_PROVIDER") && !envSet("AH_ENDPOINT_URL", "AGENT_HARNESS_ENDPOINT_URL")) {
		config.EndpointURL = DefaultEndpointForProvider(config.Provider)
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

	if config.Model == "" {
		config.Model = DefaultModelForProvider(config.Provider)
	}
	if config.EndpointURL == "" {
		config.EndpointURL = DefaultEndpointForProvider(config.Provider)
	}
}

func parseMcpServerConfig(data map[string]interface{}) mcp.McpServerConfig {
	config := mcp.McpServerConfig{}
	if v, ok := data["type"].(string); ok {
		config.Type = v
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

// SaveSettings writes an explicit set of values to a configuration layer,
// deep-merging with any existing file so unrelated keys survive. Unlike Save,
// it persists only the keys the caller passes, keeping user settings a delta
// over the tracked project defaults instead of a frozen snapshot.
