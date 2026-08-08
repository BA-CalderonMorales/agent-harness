package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// getModelItems returns available models for TUI.
func (app *App) getModelItems() []tui.ModelItem {
	provider := app.config.Provider
	if provider == "" {
		provider = config.DefaultProvider
	}

	return getModelsForProvider(provider, app.session.Model)
}

// getModelsForProvider returns models appropriate for the provider.
func getModelsForProvider(provider, currentModel string) []tui.ModelItem {
	switch provider {
	case "openai":
		return []tui.ModelItem{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4o"},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4o-mini"},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4-turbo"},
		}
	case "anthropic":
		return []tui.ModelItem{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic", ContextLen: 200000, IsDefault: currentModel == "claude-3-5-sonnet-20241022"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", ContextLen: 200000, IsDefault: currentModel == "claude-3-opus-20240229"},
		}
	case "ollama":
		return []tui.ModelItem{
			{ID: "gemma4:2b", Name: "Gemma 4 E2B (Fast)", Provider: "ollama", ContextLen: 128000, IsDefault: currentModel == "gemma4:2b"},
			{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Provider: "ollama", ContextLen: 128000, IsDefault: currentModel == "llama3.2:3b"},
		}
	case "local":
		return []tui.ModelItem{
			{ID: config.DefaultModel, Name: "Ornith 1.0 9B GGUF", Provider: "local", ContextLen: config.DefaultContextLength, IsDefault: currentModel == config.DefaultModel},
			{ID: "local-model", Name: "OpenAI-compatible local model", Provider: "local", ContextLen: config.DefaultContextLength, IsDefault: currentModel == "local-model"},
		}
	default:
		return []tui.ModelItem{
			{ID: "nvidia/nemotron-3-super-120b-a12b:free", Name: "Nemotron 3 Super 120B (free)", Provider: "openrouter", ContextLen: 128000, IsDefault: currentModel == "nvidia/nemotron-3-super-120b-a12b:free"},
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", ContextLen: 200000, IsDefault: currentModel == "anthropic/claude-3.5-sonnet"},
			{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", ContextLen: 128000, IsDefault: currentModel == "openai/gpt-4o"},
		}
	}
}

// getProjectInfo returns project metadata for the home dashboard.
