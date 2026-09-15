package config

// dropNoOpValues removes entries that carry no information from a parsed
// configuration layer before it is merged.
//
// Without it, a layer that pins a non-value overwrites the layer below:
// a stale settings.json carrying context_length 0 blanked the project's
// 8192 window (every session then ran with no context or output budget),
// and empty endpoint/model strings blanked the project's provider wiring.
// Zero is not a valid context window or output budget, and an empty
// string is not a valid path, model, or key.
//
// Booleans are kept (false is meaningful, e.g. perm_read), and so is the
// tagline (a blank tagline deliberately hides the Home line).
func dropNoOpValues(data map[string]interface{}) {
	for _, key := range []string{"context_length", "max_tokens"} {
		if v, ok := intValue(data, key); ok && v <= 0 {
			delete(data, key)
		}
	}
	for _, key := range []string{
		"provider", "model", "runtime", "model_path", "endpoint_url",
		"workspace_path", "local_server_command", "api_key", "theme",
		"session_dir", "persona", "execution_mode", "permission_mode",
		"reasoning_effort",
	} {
		if v, ok := data[key].(string); ok && v == "" {
			delete(data, key)
		}
	}
}
