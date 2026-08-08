package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"strings"
)

// handleLoginStep processes one step of the login wizard.
func (app *App) handleLoginStep(text string, tuiApp *tui.App) {
	switch app.loginState {
	case loginProvider:
		provider := resolveProviderInput(text)
		app.loginProviderTmp = provider
		app.config.Provider = provider
		app.config.EndpointURL = config.DefaultEndpointForProvider(provider)
		if config.IsLocalProvider(provider) {
			app.config.APIKey = provider
			app.loginState = loginModel
			tuiApp.AddMessage("system", sprintf("Provider: %s (local)\nEnter model [%s]:", provider, getDefaultModel(provider)))
		} else {
			app.loginState = loginAPIKey
			tuiApp.AddMessage("system", sprintf("Provider: %s\nEnter API key (input visible - type carefully):", provider))
		}

	case loginAPIKey:
		key := strings.TrimSpace(text)
		if key == "" {
			tuiApp.AddMessage("system", "API key cannot be empty. Enter API key:")
			return
		}
		app.config.APIKey = key
		app.loginState = loginModel
		tuiApp.RemoveLastUserMessage() // hide key from chat history
		tuiApp.AddMessage("system", "API key received.\nEnter model (or press Enter for default):")

	case loginModel:
		model := strings.TrimSpace(text)
		if model == "" {
			model = getDefaultModel(app.loginProviderTmp)
		}
		app.loginModelTmp = model
		app.config.Model = model
		app.session.Model = model
		app.costTracker.SetModel(model)
		app.commitConfigChange()

		if config.IsLocalProvider(app.loginProviderTmp) {
			tuiApp.AddMessage("system", "Local provider configured.")
		} else {
			credManager := config.NewCredentialManager()
			secureCfg := &config.SecureConfig{
				Provider: app.loginProviderTmp,
				APIKey:   app.config.APIKey,
				Model:    model,
			}
			if err := credManager.SaveSecure(secureCfg); err != nil {
				tuiApp.AddMessage("system", sprintf("[!] Failed to save credentials: %v", err))
			} else {
				tuiApp.AddMessage("system", "Credentials saved.")
			}
		}

		// Recreate LLM client
		app.client = llm.NewHTTPClientWithBaseURL(app.config.Provider, app.config.APIKey, app.config.EndpointURL)
		app.loop = agent.NewLoop(app.client)

		// Update TUI
		tuiApp.SetChatModel(model)
		tuiApp.SetSettings(app.getSettings())
		tuiApp.SetModels(app.getModelItems())

		app.loginState = loginIdle
		tuiApp.AddMessage("system", sprintf("Logged in. Provider: %s | Model: %s", app.loginProviderTmp, model))
	}
}

// resolveProviderInput maps numeric or name input to provider.
func resolveProviderInput(input string) string {
	switch strings.TrimSpace(input) {
	case "2", "openai":
		return "openai"
	case "3", "anthropic":
		return "anthropic"
	case "4", "openrouter":
		return "openrouter"
	case "1", "local", "llama.cpp", "llama":
		return "local"
	case "5", "ollama":
		return "ollama"
	default:
		return "local"
	}
}

// handleUserCommand processes slash commands.
