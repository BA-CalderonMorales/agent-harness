package tui

import (
	"fmt"
	"strings"
)

// ProviderErrorCategory is the normalized shape of a provider failure.
// Every provider surfaces the same five failure modes in its own
// dialect — the classifier maps the dialect to the category, and the
// feedback names the fix in the user's context (local server vs
// hosted API).
type ProviderErrorCategory int

const (
	ProviderErrUnknown ProviderErrorCategory = iota
	ProviderErrTimeout
	ProviderErrConnection
	ProviderErrRateLimit
	ProviderErrAuth
	ProviderErrModelNotFound
)

// String names the category for diagnostics entries.
func (c ProviderErrorCategory) String() string {
	switch c {
	case ProviderErrTimeout:
		return "timeout"
	case ProviderErrConnection:
		return "connection"
	case ProviderErrRateLimit:
		return "rate-limit"
	case ProviderErrAuth:
		return "auth"
	case ProviderErrModelNotFound:
		return "model-not-found"
	}
	return "unknown"
}

// ClassifyProviderError maps a raw provider error string to its
// category. Matching is dialect-tolerant: providers phrase the same
// failure differently ("connection refused", "network unreachable",
// "dial tcp"), so the patterns stay loose.
func ClassifyProviderError(errStr string) ProviderErrorCategory {
	s := strings.ToLower(errStr)
	switch {
	case strings.Contains(s, "timeout"), strings.Contains(s, "timed out"), strings.Contains(s, "deadline"):
		return ProviderErrTimeout
	case strings.Contains(s, "connection"), strings.Contains(s, "network"), strings.Contains(s, "dial "), strings.Contains(s, "unreachable"):
		return ProviderErrConnection
	case strings.Contains(s, "rate limit"), strings.Contains(s, "quota"), strings.Contains(s, "too many requests"), strings.Contains(s, "429"):
		return ProviderErrRateLimit
	case strings.Contains(s, "authentication"), strings.Contains(s, "api key"), strings.Contains(s, "unauthorized"), strings.Contains(s, "401"):
		return ProviderErrAuth
	case strings.Contains(s, "model") && strings.Contains(s, "not found"), strings.Contains(s, "does not exist"), strings.Contains(s, "404"):
		return ProviderErrModelNotFound
	}
	return ProviderErrUnknown
}

// ProviderErrorFeedback renders the actionable guidance for a category:
// what happened, and the fix in the user's context. Local providers
// never involve API keys — their hints point at the model server.
func ProviderErrorFeedback(category ProviderErrorCategory, errStr string, isLocal bool) string {
	switch category {
	case ProviderErrTimeout:
		return "[!] Model timed out. The model may be overloaded or unresponsive.\n\n" +
			"[>] Try switching models: type /model <name> or press Tab to go to Settings\n" +
			"[>] Popular alternatives: claude-3-5-sonnet, gpt-4o, deepseek-chat"
	case ProviderErrConnection:
		if isLocal {
			return "[!] Connection error. The local model server is not responding.\n\n" +
				"[>] Start it (e.g. llama-server or ollama) and verify the endpoint:\n" +
				"[>] Tab → Settings → Endpoint URL"
		}
		return "[!] Connection error. Check your internet connection and API key.\n\n" +
			"[>] Verify settings: /config or Tab → Settings\n" +
			"[>] Check API key: /config"
	case ProviderErrRateLimit:
		return "[!] Rate limit or quota exceeded.\n\n" +
			"[>] Try a different model: /model <name>\n" +
			"[>] Check your account at your provider's dashboard"
	case ProviderErrAuth:
		if isLocal {
			return "[!] Local provider does not use API keys.\n\n" +
				"[>] Verify the endpoint and model in: Tab → Settings"
		}
		return "[!] Authentication failed. Your API key may be invalid.\n\n" +
			"[>] Update API key: Tab → Settings → Provider\n" +
			"[>] Check /config for current settings"
	case ProviderErrModelNotFound:
		return "[!] Model not found or unavailable.\n\n" +
			"[>] List available models: /model (with no args)\n" +
			"[>] Check supported models: /models or see docs/supported_models.md"
	}
	return fmt.Sprintf("[!] Error: %s\n\n"+
		"[>] If the model isn't responding, try: /model <name>\n"+
		"[>] Or switch models via: Tab → Settings", errStr)
}
