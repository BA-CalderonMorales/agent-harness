package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// validateConfig checks pre-flight configuration before calling LLM.
func (app *App) validateConfig() error {
	// Check API key for non-local providers
	if !config.IsLocalProvider(app.config.Provider) {
		if app.config.APIKey == "" {
			return fmt.Errorf("no API key configured. Run setup or set AGENT_HARNESS_API_KEY / OPENROUTER_API_KEY")
		}
	}
	// Check model is set
	if app.session.Model == "" {
		return fmt.Errorf("no model selected. Use /model <name> to select a model")
	}
	return nil
}

// tuiChatDelegate connects TUI chat to the app.
