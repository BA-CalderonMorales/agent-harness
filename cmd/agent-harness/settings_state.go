package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// getSettings returns current settings for TUI grouped by logical section.
func (app *App) getSettings() []tui.Setting {
	return []tui.Setting{
		// Provider & Connection
		{Key: "provider", Label: "Provider", Value: app.config.Provider, Category: "Provider & Connection", Description: "API provider (openai, anthropic, openrouter, ollama, nvidia, local).", Type: "choice", Options: []string{"local", "openai", "anthropic", "openrouter", "ollama", "nvidia"}},
		{Key: "endpoint_url", Label: "Endpoint URL", Value: app.config.EndpointURL, Category: "Provider & Connection", Description: "OpenAI-compatible API base URL (e.g. http://localhost:8080/v1 or https://openrouter.ai/api/v1).", Type: "string"},
		{Key: "runtime", Label: "Runtime", Value: app.config.Runtime, Category: "Provider & Connection", Description: "Local runtime such as llama.cpp or ollama.", Type: "string"},

		// Model & Agent Behavior
		{Key: "model", Label: "Model", Value: app.session.Model, Category: "Model & Agent Behavior", Description: "The AI model to use for this session. Saved as default.", Type: "string"},
		{Key: "persona", Label: "Persona", Value: app.session.Persona, Category: "Model & Agent Behavior", Description: "Agent behavior mode (developer, designer, pm, scientist, explorer).", Type: "choice", Options: []string{"developer", "designer", "pm", "scientist", "explorer"}},
		{Key: "context_length", Label: "Context Length", Value: fmt.Sprintf("%d", app.config.ContextLength), Category: "Model & Agent Behavior", Description: "Model context window in tokens.", Type: "number"},
		{Key: "max_tokens", Label: "Max Tokens", Value: fmt.Sprintf("%d", app.config.MaxTokens), Category: "Model & Agent Behavior", Description: "Maximum response tokens per turn.", Type: "number"},
		{Key: "temperature", Label: "Temperature", Value: fmt.Sprintf("%.2f", app.config.Temperature), Category: "Model & Agent Behavior", Description: "Sampling temperature (0.0-2.0). Lower = more deterministic.", Type: "number"},
		{Key: "reasoning_effort", Label: "Reasoning Effort", Value: app.config.Effort, Category: "Model & Agent Behavior", Description: "Reasoning effort sent per request (low, medium, high).", Type: "choice", Options: config.EffortLevels},

		// Workspace & Permissions
		{Key: "permissions", Label: "Permission Mode", Value: app.config.PermissionMode.String(), Category: "Workspace & Permissions", Description: "Tool permission level.", Type: "choice", Options: []string{"read-only", "workspace-write", "danger-full-access"}},
		{Key: "execution_mode", Label: "Execution Mode", Value: app.executionMode.String(), Category: "Workspace & Permissions", Description: "Command approval mode (interactive or yolo).", Type: "choice", Options: []string{"interactive", "yolo"}},
		{Key: "perm_read", Label: "Allow Read", Value: "", Category: "Workspace & Permissions", Description: "Allow read/search tools.", Type: "bool", BoolValue: app.config.PermRead},
		{Key: "perm_write", Label: "Allow Write", Value: "", Category: "Workspace & Permissions", Description: "Allow write/edit tools.", Type: "bool", BoolValue: app.config.PermWrite},
		{Key: "perm_delete", Label: "Allow Delete", Value: "", Category: "Workspace & Permissions", Description: "Allow delete/remove tools.", Type: "bool", BoolValue: app.config.PermDelete},
		{Key: "perm_execute", Label: "Allow Execute", Value: "", Category: "Workspace & Permissions", Description: "Allow bash/execute tools.", Type: "bool", BoolValue: app.config.PermExecute},

		// System & Storage
		{Key: "session_dir", Label: "Session Directory", Value: app.config.SessionDir, Category: "System & Storage", Description: "Directory for session log storage (~/.agent-harness/sessions).", Type: "string"},
	}
}

// getModelItems returns available models for TUI.
