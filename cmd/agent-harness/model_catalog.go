package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
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

// applyModelContext sets config.ContextLength from the provider's own
// advertised context_length for the model, falling back to the static
// catalog when the live list is unreachable or silent. The catalog is
// the honest offline default; the live value wins whenever the endpoint
// states one, so the context bar never shows a smaller window than the
// model actually has.
func (app *App) applyModelContext(model string) {
	if c, ok := app.client.(*llm.HTTPClient); ok && c != nil {
		if infos, err := c.ListModelsDetailed(); err == nil {
			for _, info := range infos {
				if info.ID == model && info.ContextLength > 0 {
					app.config.ContextLength = info.ContextLength
					return
				}
			}
		}
	}
	applyCatalogContext(app.config, model)
}

// ensureModelFitsProvider resets a model that cannot exist on the
// active provider to that provider's default. A provider switch (env
// override, saved config) strands the old model otherwise: the mode
// line read "glm-5p3-flash · local" — a Fireworks model pointed at a
// local server.
//
// Two checks: hosted providers compare against the static catalog
// (offline, deterministic); local providers compare against the live
// /v1/models list, because any model name is legitimate on a local
// server and only the endpoint knows what it serves. An unreachable or
// silent endpoint keeps the current model — no evidence, no reset.
// Returns the previous model when a reset happened.
func (app *App) ensureModelFitsProvider(cfg *config.LayeredConfig, sessionModel string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	model := cfg.Model
	if model == "" && sessionModel != "" {
		model = sessionModel
	}
	if model == "" {
		return "", false
	}

	if !config.IsLocalProvider(cfg.Provider) {
		for _, item := range getModelsForProvider(cfg.Provider, model) {
			if item.ID == model {
				return "", false
			}
		}
	} else {
		if c, ok := app.client.(*llm.HTTPClient); ok && c != nil {
			infos, err := c.ListModelsDetailed()
			if err != nil || len(infos) == 0 {
				return "", false // endpoint silent: no evidence, no reset
			}
			known := false
			for _, info := range infos {
				if info.ID == model {
					known = true
					break
				}
			}
			if !known {
				previous := model
				cfg.Model = config.DefaultModel
				if cfg.Model == previous {
					return "", false
				}
				return previous, true
			}
			return "", false
		}
		return "", false
	}

	previous := model
	cfg.Model = config.DefaultModelForProvider(cfg.Provider)
	if cfg.Model == previous {
		return "", false
	}
	return previous, true
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
		// Context values audited against the live /v1/models response
		// (2026-09): glm-5p3-flash and glm-5p3 advertise context_length
		// 1048576 — the old 131072 entries understated the window 8x
		// and starved the context bar. llama-v3p3-70b-instruct and
		// mixtral-8x22b-instruct no longer appear in the live serving
		// list; dead entries that 404 on selection were dropped.
		return []tui.ModelItem{
			{ID: "accounts/fireworks/models/glm-5p3-flash", Name: "GLM 5.3 Flash", Provider: "fireworks", ContextLen: 1048576, IsDefault: currentModel == "accounts/fireworks/models/glm-5p3-flash"},
			{ID: "accounts/fireworks/models/glm-5p3", Name: "GLM 5.3", Provider: "fireworks", ContextLen: 1048576, IsDefault: currentModel == "accounts/fireworks/models/glm-5p3"},
			{ID: "accounts/fireworks/models/deepseek-v4-flash-0731", Name: "DeepSeek V4 Flash", Provider: "fireworks", ContextLen: 1048576, IsDefault: currentModel == "accounts/fireworks/models/deepseek-v4-flash-0731"},
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
