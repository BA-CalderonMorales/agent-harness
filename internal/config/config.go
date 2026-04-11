package config

import (
	"os"
)

// Config holds runtime configuration.
type Config struct {
	Provider         string // "openrouter", "openai", or "anthropic"
	APIKey           string
	Model            string
	WorkingDirectory string
	Verbose          bool
}

// Load reads configuration from environment variables.
func Load() Config {
	provider := "openrouter"
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		provider = "openai"
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		provider = "anthropic"
	}
	// Check for local/ollama provider
	if os.Getenv("OLLAMA_HOST") != "" || os.Getenv("AGENT_HARNESS_PROVIDER") == "ollama" {
		provider = "ollama"
		apiKey = "ollama" // Ollama doesn't require a real API key
	}
	model := os.Getenv("AGENT_HARNESS_MODEL")
	if model == "" {
		switch provider {
		case "openrouter":
			model = "nvidia/nemotron-3-super-120b-a12b:free"
		case "openai":
			model = "gpt-4o"
		case "anthropic":
			model = "claude-3-5-sonnet-20241022"
		case "ollama":
			model = "gemma4:2b"
		}
	}
	wd, _ := os.Getwd()
	return Config{
		Provider:         provider,
		APIKey:           apiKey,
		Model:            model,
		WorkingDirectory: wd,
		Verbose:          os.Getenv("AGENT_HARNESS_VERBOSE") == "1",
	}
}
