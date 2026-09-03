package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// getDefaultModel returns default model for provider.
func getDefaultModel(provider string) string {
	return config.DefaultModelForProvider(provider)
}

// getMemoryInfo formats system prompt and session memory context.
