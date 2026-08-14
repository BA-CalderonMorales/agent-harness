package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

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
		} else if provider == "fireworks" {
			return "accounts/fireworks/models/llama-v3p3-70b-instruct"
		} else if provider == "nvidia" {
			return "nvidia/nemotron-3.5-lightning"
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
		} else if provider == "fireworks" {
			return "accounts/fireworks/models/deepseek-v4-flash-0731"
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
