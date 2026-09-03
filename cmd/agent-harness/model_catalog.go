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

// applyCatalogContext syncs config.ContextLength with the catalog budget for
// the selected model, so the context bar and loop limits match the model's
// real window. Models unknown to the catalog keep the current setting.
func applyCatalogContext(cfg *config.LayeredConfig, model string) {
	for _, item := range getModelsForProvider(cfg.Provider, model) {
		if item.ID == model && item.ContextLen > 0 {
			cfg.ContextLength = item.ContextLen
			return
		}
	}
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
	case "flm":
		return []tui.ModelItem{
			{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Provider: "flm", ContextLen: 131072, IsDefault: currentModel == "llama3.2:3b"},
			{ID: "qwen3.5:4b", Name: "Qwen 3.5 4B", Provider: "flm", ContextLen: 131072, IsDefault: currentModel == "qwen3.5:4b"},
			{ID: "gpt-oss:20b", Name: "GPT-OSS 20B", Provider: "flm", ContextLen: 131072, IsDefault: currentModel == "gpt-oss:20b"},
			{ID: "gemma3:4b", Name: "Gemma 3 4B (Vision)", Provider: "flm", ContextLen: 131072, IsDefault: currentModel == "gemma3:4b"},
		}
	case "nvidia":
		return []tui.ModelItem{
			{ID: "nvidia/nemotron-3.5-lightning-30b-a3b", Name: "Nemotron 3.5 Lightning 30B (thinking)", Provider: "nvidia", ContextLen: 128000, IsDefault: currentModel == "nvidia/nemotron-3.5-lightning-30b-a3b"},
			{ID: "nvidia/nemotron-3-super-120b-a12b", Name: "Nemotron 3 Super 120B", Provider: "nvidia", ContextLen: 128000, IsDefault: currentModel == "nvidia/nemotron-3-super-120b-a12b"},
			{ID: "nvidia/llama-3.1-nemotron-ultra-253b-v1", Name: "Llama Nemotron Ultra 253B", Provider: "nvidia", ContextLen: 128000, IsDefault: currentModel == "nvidia/llama-3.1-nemotron-ultra-253b-v1"},
		}
	case "local":
		return []tui.ModelItem{
			{ID: config.DefaultModel, Name: "Ornith 1.0 9B GGUF", Provider: "local", ContextLen: config.DefaultContextLength, IsDefault: currentModel == config.DefaultModel},
			{ID: "local-model", Name: "OpenAI-compatible local model", Provider: "local", ContextLen: config.DefaultContextLength, IsDefault: currentModel == "local-model"},
		}
	case "fireworks":
		return []tui.ModelItem{
			{ID: "accounts/fireworks/models/glm-5p3-flash", Name: "GLM 5.3 Flash", Provider: "fireworks", ContextLen: 131072, IsDefault: currentModel == "accounts/fireworks/models/glm-5p3-flash"},
			{ID: "accounts/fireworks/models/glm-5p3", Name: "GLM 5.3", Provider: "fireworks", ContextLen: 131072, IsDefault: currentModel == "accounts/fireworks/models/glm-5p3"},
			{ID: "accounts/fireworks/models/llama-v3p3-70b-instruct", Name: "Llama 3.3 70B Instruct", Provider: "fireworks", ContextLen: 128000, IsDefault: currentModel == "accounts/fireworks/models/llama-v3p3-70b-instruct"},
			{ID: "accounts/fireworks/models/deepseek-v4-flash-0731", Name: "DeepSeek V4 Flash", Provider: "fireworks", ContextLen: 128000, IsDefault: currentModel == "accounts/fireworks/models/deepseek-v4-flash-0731"},
			{ID: "accounts/fireworks/models/mixtral-8x22b-instruct", Name: "Mixtral 8x22B Instruct", Provider: "fireworks", ContextLen: 65536, IsDefault: currentModel == "accounts/fireworks/models/mixtral-8x22b-instruct"},
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
