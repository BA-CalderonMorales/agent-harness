package config

import (
	"os"
	"strconv"
	"time"
)

func (ll *LayeredLoader) extractPermissionToggles(config *LayeredConfig, values map[string]interface{}) {
	applyBoolValue(values, "perm_read", &config.PermRead, &config.PermExplicit)
	applyBoolValue(values, "read", &config.PermRead, &config.PermExplicit)
	applyBoolValue(values, "perm_write", &config.PermWrite, &config.PermExplicit)
	applyBoolValue(values, "write", &config.PermWrite, &config.PermExplicit)
	applyBoolValue(values, "perm_delete", &config.PermDelete, &config.PermExplicit)
	applyBoolValue(values, "delete", &config.PermDelete, &config.PermExplicit)
	applyBoolValue(values, "perm_execute", &config.PermExecute, &config.PermExplicit)
	applyBoolValue(values, "execute", &config.PermExecute, &config.PermExplicit)
}

func (ll *LayeredLoader) applyEnvOverrides(config *LayeredConfig) {
	applyEnvString(firstEnv("AH_PROVIDER", "AGENT_HARNESS_PROVIDER"), &config.Provider)
	applyEnvString(firstEnv("AH_RUNTIME", "AGENT_HARNESS_RUNTIME"), &config.Runtime)
	if firstEnv("AH_MODEL", "AGENT_HARNESS_MODEL") != "" {
		applyEnvString(firstEnv("AH_MODEL", "AGENT_HARNESS_MODEL"), &config.Model)
		// The env pin outranks a resumed session's stored model, the
		// same way EndpointPinned outranks provider defaults.
		config.ModelPinned = true
	}
	applyEnvString(firstEnv("AH_MODEL_PATH", "AGENT_HARNESS_MODEL_PATH"), &config.ModelPath)
	if endpoint := firstEnv("AH_ENDPOINT_URL", "AGENT_HARNESS_ENDPOINT_URL"); endpoint != "" {
		applyEnvString(endpoint, &config.EndpointURL)
		// An env-pinned endpoint survives provider switches: runtime
		// provider mutations must not clobber an explicit endpoint with
		// the provider default, or the wizard's verified connection
		// would lie about what the app will actually use.
		config.EndpointPinned = true
	}
	applyEnvString(firstEnv("AH_API_KEY", "AGENT_HARNESS_API_KEY", "OPENROUTER_API_KEY", "NVIDIA_API_KEY"), &config.APIKey)
	if config.Provider == "nvidia" && config.APIKey == "" {
		// NVIDIA's hosted API uses its own key convention (nvapi-...).
		applyEnvString(firstEnv("NVIDIA_API_KEY"), &config.APIKey)
	}
	applyEnvInt(firstEnv("AH_CONTEXT_LENGTH", "AGENT_HARNESS_CONTEXT_LENGTH"), &config.ContextLength)
	applyEnvFloat(firstEnv("AH_TEMPERATURE", "AGENT_HARNESS_TEMPERATURE"), &config.Temperature)
	applyEnvInt(firstEnv("AH_MAX_TOKENS", "AGENT_HARNESS_MAX_TOKENS"), &config.MaxTokens)
	applyEnvString(firstEnv("AH_WORKSPACE_PATH", "AGENT_HARNESS_WORKSPACE_PATH"), &config.WorkspacePath)
	applyEnvString(firstEnv("AH_LOCAL_SERVER_COMMAND", "AGENT_HARNESS_LOCAL_SERVER_COMMAND"), &config.ServerCommand)
	applyEnvString(firstEnv("AH_PERSONA", "AGENT_HARNESS_PERSONA"), &config.Persona)
	applyEnvString(firstEnv("AH_SESSION_DIR", "AGENT_HARNESS_SESSION_DIR"), &config.SessionDir)
	applyEnvPermissionMode(firstEnv("AH_PERMISSION_MODE", "AGENT_HARNESS_PERMISSION_MODE"), &config.PermissionMode)
	applyEnvString(firstEnv("AH_EXECUTION_MODE", "AGENT_HARNESS_EXECUTION_MODE"), &config.ExecutionMode)
	applyEnvBool(firstEnv("AH_PERM_READ", "AGENT_HARNESS_PERM_READ"), &config.PermRead, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_WRITE", "AGENT_HARNESS_PERM_WRITE"), &config.PermWrite, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_DELETE", "AGENT_HARNESS_PERM_DELETE"), &config.PermDelete, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_EXECUTE", "AGENT_HARNESS_PERM_EXECUTE"), &config.PermExecute, &config.PermExplicit)
	// Timeouts: pinned by env so a provider switch must not recompute them.
	if v := firstEnv("AH_STREAM_IDLE_TIMEOUT", "AGENT_HARNESS_STREAM_IDLE_TIMEOUT"); v != "" {
		config.StreamIdleTimeout = parseEnvDuration(v)
	}
	if v := firstEnv("AH_HTTP_TIMEOUT", "AGENT_HARNESS_HTTP_TIMEOUT"); v != "" {
		config.HTTPTimeout = parseEnvDuration(v)
	}
	config.TimeoutPinned = config.StreamIdleTimeout > 0 || config.HTTPTimeout > 0
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func envSet(names ...string) bool {
	return firstEnv(names...) != ""
}

func stringValue(values map[string]interface{}, key string) (string, bool) {
	v, ok := values[key].(string)
	return v, ok && v != ""
}

func mapValue(values map[string]interface{}, key string) (map[string]interface{}, bool) {
	v, ok := values[key].(map[string]interface{})
	return v, ok
}

func intValue(values map[string]interface{}, key string) (int, bool) {
	switch v := values[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(v)
		return n, err == nil
	default:
		return 0, false
	}
}

func floatValue(values map[string]interface{}, key string) (float64, bool) {
	switch v := values[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func boolValue(values map[string]interface{}, key string) (bool, bool) {
	switch v := values[key].(type) {
	case bool:
		return v, true
	case string:
		n, err := strconv.ParseBool(v)
		return n, err == nil
	default:
		return false, false
	}
}

func applyBoolValue(values map[string]interface{}, key string, target *bool, explicit *bool) {
	if v, ok := boolValue(values, key); ok {
		*target = v
		*explicit = true
	}
}

func applyEnvString(value string, target *string) {
	if value != "" {
		*target = value
	}
}

func applyEnvInt(value string, target *int) {
	if value == "" {
		return
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		*target = parsed
	}
}

func applyEnvFloat(value string, target *float64) {
	if value == "" {
		return
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		*target = parsed
	}
}

func applyEnvBool(value string, target *bool, explicit *bool) {
	if value == "" {
		return
	}
	if parsed, err := strconv.ParseBool(value); err == nil {
		*target = parsed
		*explicit = true
	}
}

func applyEnvPermissionMode(value string, target *PermissionMode) {
	if value == "" {
		return
	}
	if mode, err := ParsePermissionMode(value); err == nil {
		*target = mode
	}
}

// parseEnvDuration accepts Go duration strings ("5m", "1h30m") and falls
// back to plain-integer seconds for shell-friendliness.
func parseEnvDuration(value string) time.Duration {
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}
