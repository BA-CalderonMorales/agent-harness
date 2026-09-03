package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"slices"
	"strings"
)

// updateConfiguration updates configuration options dynamically via /config and re-probes.
func (app *App) updateConfiguration(key, value string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.TrimSpace(value)

	switch key {
	case "provider":
		if value == "" {
			return "", fmt.Errorf("usage: /config provider <local|openai|anthropic|openrouter|ollama>")
		}
		app.config.Provider = value
		app.config.EndpointURL = config.DefaultEndpointForProvider(value)
		// Keep the current model; only fall back to the provider default when unset.
		if app.config.Model == "" {
			app.config.Model = config.DefaultModelForProvider(value)
		}
		app.commitConfigChange()
		app.rebuildLLMClient()
		return sprintf("Provider updated to '%s'\n  Endpoint URL: %s\n  Model: %s\n  Status: Re-probing connection...",
			app.config.Provider, app.config.EndpointURL, app.config.Model), nil

	case "endpoint", "endpoint_url":
		if value == "" {
			return "", fmt.Errorf("usage: /config endpoint <url>")
		}
		app.config.EndpointURL = value
		app.commitConfigChange()
		app.rebuildLLMClient()
		return sprintf("Endpoint URL updated to '%s'\n  Status: Re-probing connection...", app.config.EndpointURL), nil

	case "model":
		if value == "" {
			return "", fmt.Errorf("usage: /config model <model-name>")
		}
		app.config.Model = value
		if app.session != nil {
			app.session.Model = value
		}
		app.commitConfigChange()
		app.rebuildLLMClient()
		return sprintf("Model updated to '%s'\n  Status: Re-probing connection...", app.config.Model), nil

	case "key", "api_key":
		// Literal keys are rejected on purpose: the command text lands in
		// the chat pane and the session file, so a literal would leak the
		// key into history and session exports. Literals go through /login
		// (input hidden); /config key only accepts secret references,
		// which stay safe to display.
		resolved, err := config.ResolveSecret(value)
		if err != nil {
			return "", err
		}
		if resolved == value {
			return "", fmt.Errorf("literal keys are not accepted via /config (they would land in chat history). Use /login for a literal key, or set api_key to a secret://env|file|cmd reference")
		}
		app.config.APIKey = resolved
		app.rebuildLLMClient()
		return "API key updated from secret reference (session-only; keep the reference in your config file for persistence)\n  Status: Re-probing connection...", nil

	case "effort", "reasoning_effort":
		if value == "" {
			value = nextEffort(app.config.Effort)
		} else if !slices.Contains(config.EffortLevels, value) {
			return "", fmt.Errorf("invalid effort '%s'. Available: low, medium, high", value)
		}
		app.config.Effort = value
		app.commitConfigChange()
		app.rebuildLLMClient()
		return sprintf("Reasoning effort updated to '%s'", value), nil

	case "reset":
		app.config.Provider = config.DefaultProvider
		app.config.EndpointURL = config.DefaultEndpointURL
		app.config.Model = config.DefaultModel
		if app.session != nil {
			app.session.Model = config.DefaultModel
		}
		app.commitConfigChange()
		app.rebuildLLMClient()
		return sprintf("Reset configuration to default local provider (%s)\n  Status: Re-probing connection...", config.DefaultEndpointURL), nil

	default:
		return "", fmt.Errorf("unknown config option '%s'. Available options: provider, endpoint, model, key, reset", key)
	}
}

// rebuildLLMClient reconstructs client, updates models, and re-probes provider readiness.
