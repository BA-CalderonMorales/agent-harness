package config

import (
	"os"
)

// Config holds runtime configuration.
type Config struct {
	Provider         string // "openrouter" or "anthropic"
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
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		provider = "anthropic"
	}
	model := os.Getenv("AGENT_HARNESS_MODEL")
	if model == "" {
		if provider == "openrouter" {
			model = "anthropic/claude-3.5-sonnet"
		} else {
			model = "claude-3-5-sonnet-20241022"
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
