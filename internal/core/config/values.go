package config

import (
	"os"
	"strconv"
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
	applyEnvString(firstEnv("AH_MODEL", "AGENT_HARNESS_MODEL"), &config.Model)
	applyEnvString(firstEnv("AH_MODEL_PATH", "AGENT_HARNESS_MODEL_PATH"), &config.ModelPath)
	applyEnvString(firstEnv("AH_ENDPOINT_URL", "AGENT_HARNESS_ENDPOINT_URL"), &config.EndpointURL)
	applyEnvString(firstEnv("AH_API_KEY", "AGENT_HARNESS_API_KEY"), &config.APIKey)
	applyEnvInt(firstEnv("AH_CONTEXT_LENGTH", "AGENT_HARNESS_CONTEXT_LENGTH"), &config.ContextLength)
	applyEnvFloat(firstEnv("AH_TEMPERATURE", "AGENT_HARNESS_TEMPERATURE"), &config.Temperature)
	applyEnvInt(firstEnv("AH_MAX_TOKENS", "AGENT_HARNESS_MAX_TOKENS"), &config.MaxTokens)
	applyEnvString(firstEnv("AH_WORKSPACE_PATH", "AGENT_HARNESS_WORKSPACE_PATH"), &config.WorkspacePath)
	applyEnvString(firstEnv("AH_LOCAL_SERVER_COMMAND", "AGENT_HARNESS_LOCAL_SERVER_COMMAND"), &config.ServerCommand)
	applyEnvString(firstEnv("AH_PERSONA", "AGENT_HARNESS_PERSONA"), &config.Persona)
	applyEnvPermissionMode(firstEnv("AH_PERMISSION_MODE", "AGENT_HARNESS_PERMISSION_MODE"), &config.PermissionMode)
	applyEnvString(firstEnv("AH_EXECUTION_MODE", "AGENT_HARNESS_EXECUTION_MODE"), &config.ExecutionMode)
	applyEnvBool(firstEnv("AH_PERM_READ", "AGENT_HARNESS_PERM_READ"), &config.PermRead, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_WRITE", "AGENT_HARNESS_PERM_WRITE"), &config.PermWrite, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_DELETE", "AGENT_HARNESS_PERM_DELETE"), &config.PermDelete, &config.PermExplicit)
	applyEnvBool(firstEnv("AH_PERM_EXECUTE", "AGENT_HARNESS_PERM_EXECUTE"), &config.PermExecute, &config.PermExplicit)
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
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
