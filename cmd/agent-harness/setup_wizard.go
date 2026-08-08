package main

import (
	"bufio"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"os"
	"strings"
)

// interactiveSetup guides user through initial configuration.
func (app *App) interactiveSetup(credManager *config.CredentialManager) error {
	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("  Welcome to Agent Harness"))
	fmt.Println()
	fmt.Println("  Let's get you set up.")
	fmt.Println()

	provider := promptProvider()
	app.config.Provider = provider

	if config.IsLocalProvider(provider) {
		app.config.APIKey = provider
		fmt.Printf("  %s uses a local OpenAI-compatible endpoint - no API key required\n", provider)
	} else {
		apiKey := promptAPIKey(provider)
		if apiKey == "" {
			return errf("API key cannot be empty")
		}
		app.config.APIKey = apiKey
		fmt.Println("  " + ui.RenderSuccess("API key received"))
	}

	model := promptModel(provider)
	app.config.Model = model

	if config.IsLocalProvider(provider) {
		return nil
	}

	fmt.Println()
	fmt.Println("  Credentials will be encrypted.")
	fmt.Println()

	secureCfg := &config.SecureConfig{
		Provider: app.config.Provider,
		APIKey:   app.config.APIKey,
		Model:    app.config.Model,
	}

	if err := credManager.SaveSecure(secureCfg); err != nil {
		return errf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + ui.RenderSuccess("Credentials saved"))
	fmt.Println()

	return nil
}

// promptProvider prompts user for API provider.
func promptProvider() string {
	fmt.Println("  Choose an API provider:")
	fmt.Println("    1) Local OpenAI-compatible (llama.cpp)")
	fmt.Println("    2) OpenAI")
	fmt.Println("    3) Anthropic")
	fmt.Println("    4) OpenRouter")
	fmt.Println("    5) Ollama")
	fmt.Print("  Enter choice (1-5) [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "2":
		return "openai"
	case "3":
		return "anthropic"
	case "4":
		return "openrouter"
	case "5":
		return "ollama"
	default:
		return "local"
	}
}

// promptAPIKey prompts user for API key.
func promptAPIKey(provider string) string {
	fmt.Printf("  Enter your %s API key: ", provider)
	apiKey, err := config.PromptPassword("")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(apiKey)
}

// promptModel prompts user for model selection.
func promptModel(provider string) string {
	defaultModel := getDefaultModel(provider)

	fmt.Printf("  Model [%s]: ", defaultModel)
	reader := bufio.NewReader(os.Stdin)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)

	model = resolveModelInput(model, provider)

	if model != "" {
		return model
	}
	return defaultModel
}

// getDefaultModel returns default model for provider.
func getDefaultModel(provider string) string {
	return config.DefaultModelForProvider(provider)
}

// resolveModelInput resolves numeric or empty model input.
func resolveModelInput(input, provider string) string {
	switch input {
	case "1":
		if provider == "openai" {
			return "gpt-4o"
		} else if provider == "anthropic" {
			return "claude-3-5-sonnet-20241022"
		} else if provider == "ollama" {
			return "gemma4:2b"
		} else if provider == "local" {
			return config.DefaultModel
		}
		return "nvidia/nemotron-3-super-120b-a12b:free"
	case "2":
		if provider == "openai" {
			return "gpt-4o-mini"
		} else if provider == "anthropic" {
			return "claude-3-opus-20240229"
		} else if provider == "ollama" {
			return "llama3.2:3b"
		} else if provider == "local" {
			return "local-model"
		}
		return "anthropic/claude-3.5-sonnet"
	case "3":
		if provider == "openai" {
			return "gpt-4-turbo"
		}
		return "openai/gpt-4o"
	default:
		return input
	}
}

// getMemoryInfo formats system prompt and session memory context.
