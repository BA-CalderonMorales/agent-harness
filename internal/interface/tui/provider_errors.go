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
	ProviderErrCredits
	ProviderErrAuth
	ProviderErrModelNotFound
	ProviderErrToolLoop
	ProviderErrAgentLimit
)

// String names the category for diagnostics entries.
func (c ProviderErrorCategory) String() string {
	switch c {
	case ProviderErrToolLoop:
		return "tool-loop"
	case ProviderErrAgentLimit:
		return "agent-limit"
	case ProviderErrTimeout:
		return "timeout"
	case ProviderErrConnection:
		return "connection"
	case ProviderErrRateLimit:
		return "rate-limit"
	case ProviderErrCredits:
		return "credits"
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
	case strings.HasPrefix(s, "tool repetition limit:"):
		return ProviderErrToolLoop
	case strings.Contains(s, "agent turn ended: blocking_limit"), strings.Contains(s, "agent turn ended: max_turns"):
		return ProviderErrAgentLimit
	case strings.Contains(s, "timeout"), strings.Contains(s, "timed out"), strings.Contains(s, "deadline"):
		return ProviderErrTimeout
	case strings.Contains(s, "connection"), strings.Contains(s, "network"), strings.Contains(s, "dial "), strings.Contains(s, "unreachable"):
		return ProviderErrConnection
	case isCreditsError(s):
		return ProviderErrCredits
	case strings.Contains(s, "rate limit"), strings.Contains(s, "quota"), strings.Contains(s, "too many requests"), strings.Contains(s, "429"):
		return ProviderErrRateLimit
	case strings.Contains(s, "authentication"), strings.Contains(s, "api key"), strings.Contains(s, "unauthorized"), strings.Contains(s, "401"):
		return ProviderErrAuth
	case strings.Contains(s, "model") && strings.Contains(s, "not found"), strings.Contains(s, "does not exist"), strings.Contains(s, "404"):
		return ProviderErrModelNotFound
	}
	return ProviderErrUnknown
}

// isCreditsError distinguishes a credit/budget rejection (HTTP 402 or
// OpenRouter's limit_source metadata) from a request-rate limit (429).
// Both can include the word "quota", so credits are checked first.
func isCreditsError(s string) bool {
	for _, phrase := range []string{
		"llm api error 402",
		"insufficient credits",
		"credit balance",
		"add credits",
		"exceeds your available credits",
		"limit_source=openrouter_credits",
		"limit_source=openrouter_in_flight_budget",
		"in_flight_budget_exhausted",
		"weight_exceeds_budget",
	} {
		if strings.Contains(s, phrase) {
			return true
		}
	}
	return false
}

// ProviderErrorFeedback renders the actionable guidance for a category:
// what happened, and the fix in the user's context. Local providers
// never involve API keys — their hints point at the model server.
func ProviderErrorFeedback(category ProviderErrorCategory, errStr string, isLocal bool) string {
	switch category {
	case ProviderErrToolLoop:
		return "[!] " + errStr + "\n\n[>] Review the last tool error and send a corrected path or approach to continue in this session."
	case ProviderErrAgentLimit:
		return "[!] The agent reached a work limit. Your conversation is preserved.\n\n[>] Review the limit notice above, then send a follow-up to continue. Use /limit only when the notice identifies a tool-call budget."
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
		// Keep the provider's own sentence and metadata: "Rate limit
		// exceeded" is the same text for a free-tier daily cap, a busy
		// upstream provider, and a per-minute burst limit. The generic
		// heading alone left the user unable to tell which one it was.
		detail := strings.TrimSpace(errStr)
		if detail == "" {
			detail = "Rate limit or quota exceeded."
		}
		return "[!] " + detail + "\n\n" +
			"[>] Retry after the reset window, or switch models: /model <name>\n" +
			"[>] If the detail names an upstream provider, that endpoint is at capacity; it is not your account\n" +
			"[>] Check your account dashboard for the exact cap"
	case ProviderErrCredits:
		detail := strings.TrimSpace(errStr)
		if detail == "" {
			detail = "The provider rejected this request for credits or budget."
		}
		return "[!] " + detail + "\n\n" +
			"[>] This is a credit or budget rejection, not a request-rate limit\n" +
			"[>] Add credits or switch to another model/provider; free variants still require a positive balance\n" +
			"[>] An in-flight budget rejection clears after running requests settle; honor the Retry-After window"
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
