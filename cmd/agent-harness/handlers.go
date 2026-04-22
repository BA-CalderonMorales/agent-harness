package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/audit"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// handleUserSubmit processes user message submission.
func (app *App) handleUserSubmit(text string, tuiApp *tui.App) {
	// Login wizard intercept
	if app.loginState != loginIdle {
		app.handleLoginStep(text, tuiApp)
		return
	}

	validator := ui.NewTermuxValidator()
	normalizedInput, valid := validator.ValidateInput(text)
	if !valid {
		tuiApp.Send(tui.AgentErrorMsg{Error: fmt.Errorf("invalid input"), Timestamp: time.Now()})
		return
	}

	userMsg := types.Message{
		UUID:      generateUUID(),
		Role:      types.RoleUser,
		Content:   []types.ContentBlock{types.TextBlock{Text: normalizedInput}},
		Timestamp: time.Now(),
	}
	app.session.AddMessage(userMsg)

	app.handleAgentLoopAsync(normalizedInput, tuiApp)
}

// handleLoginStep processes one step of the login wizard.
func (app *App) handleLoginStep(text string, tuiApp *tui.App) {
	switch app.loginState {
	case loginProvider:
		provider := resolveProviderInput(text)
		app.loginProviderTmp = provider
		app.config.Provider = provider
		if provider == "ollama" {
			app.config.APIKey = "ollama"
			app.loginState = loginModel
			tuiApp.AddMessage("system", "Provider: ollama (local)\nEnter model [gemma4:2b]:")
		} else {
			app.loginState = loginAPIKey
			tuiApp.AddMessage("system", sprintf("Provider: %s\nEnter API key (input visible - type carefully):", provider))
		}

	case loginAPIKey:
		key := strings.TrimSpace(text)
		if key == "" {
			tuiApp.AddMessage("system", "API key cannot be empty. Enter API key:")
			return
		}
		app.config.APIKey = key
		app.loginState = loginModel
		tuiApp.RemoveLastUserMessage() // hide key from chat history
		tuiApp.AddMessage("system", "API key received.\nEnter model (or press Enter for default):")

	case loginModel:
		model := strings.TrimSpace(text)
		if model == "" {
			model = getDefaultModel(app.loginProviderTmp)
		}
		app.loginModelTmp = model
		app.config.Model = model
		app.session.Model = model
		app.costTracker.SetModel(model)

		// Save credentials
		credManager := config.NewCredentialManager()
		secureCfg := &config.SecureConfig{
			Provider: app.loginProviderTmp,
			APIKey:   app.config.APIKey,
			Model:    model,
		}
		if err := credManager.SaveSecure(secureCfg); err != nil {
			tuiApp.AddMessage("system", sprintf("[!] Failed to save credentials: %v", err))
		} else {
			tuiApp.AddMessage("system", "Credentials saved.")
		}

		// Recreate LLM client
		app.client = llm.NewHTTPClient(app.config.Provider, app.config.APIKey)
		app.loop = agent.NewLoop(app.client)

		// Update TUI
		tuiApp.SetChatModel(model)
		tuiApp.SetSettings(app.getSettings())
		tuiApp.SetModels(app.getModelItems())

		app.loginState = loginIdle
		tuiApp.AddMessage("system", sprintf("Logged in. Provider: %s | Model: %s", app.loginProviderTmp, model))
	}
}

// resolveProviderInput maps numeric or name input to provider.
func resolveProviderInput(input string) string {
	switch strings.TrimSpace(input) {
	case "2", "openai":
		return "openai"
	case "3", "anthropic":
		return "anthropic"
	case "4", "ollama":
		return "ollama"
	default:
		return "openrouter"
	}
}

// handleUserCommand processes slash commands.
func (app *App) handleUserCommand(command string, tuiApp *tui.App) {
	if result, handled, err := app.cmdRegistry.Handle(command); handled {
		if err != nil {
			tuiApp.AddMessage("system", sprintf("Error: %v", err))
			return
		}
		if commands.IsQuit(result) {
			tuiApp.Send(tui.QuitMsg{})
			return
		}
		if commands.IsReset(result) {
			tuiApp.AddMessage("system", "Agent Harness has been reset. Credentials and sessions deleted.")
			tuiApp.Send(tui.QuitMsg{})
			return
		}
		if result != "" {
			tuiApp.AddMessage("system", result)
		}
	} else {
		tuiApp.AddMessage("system", sprintf("Unknown command: %s", command))
	}
}

// handleAgentLoopAsync runs the full agent loop asynchronously.
func (app *App) handleAgentLoopAsync(input string, tuiApp *tui.App) {
	// PRE-FLIGHT: Check common config issues before calling LLM
	if err := app.validateConfig(); err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	tuiApp.Send(tui.AgentStartMsg{Timestamp: time.Now()})
	// Show connecting state so user knows something is happening
	tuiApp.Send(tui.AgentConnectingMsg{Endpoint: app.config.Provider})

	sysPrompt := app.buildSystemPrompt()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tuiApp.SetAgentCancelFunc(cancel)
	defer tuiApp.SetAgentCancelFunc(nil)

	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController: ctx,
		SubAgentQuery: func(prompt string) (string, error) {
			// Sub-agent runs a single-turn query with fresh context
			subCtx, subCancel := context.WithTimeout(ctx, 60*time.Second)
			defer subCancel()
			req := llm.Request{
				Messages: []types.Message{
					{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: prompt}}, Timestamp: time.Now()},
				},
				SystemPrompt: app.buildSystemPrompt(),
				Model:        app.session.Model,
				MaxTokens:    4096,
			}
			stream, err := app.client.Stream(subCtx, req)
			if err != nil {
				return "", err
			}
			var result strings.Builder
			for event := range stream {
				switch e := event.(type) {
				case types.LLMTextDelta:
					result.WriteString(e.Delta)
				case types.LLMMessageStop:
					// done
				case types.LLMError:
					return result.String(), e.Error
				}
			}
			return result.String(), nil
		},
	}

	canUseTool := app.createToolPermissionFunc(tuiApp)

	params := agent.QueryParams{
		Messages:       app.session.Messages,
		SystemPrompt:   sysPrompt,
		CanUseTool:     canUseTool,
		ToolUseContext: toolCtx,
	}

	stream, err := app.loop.Query(ctx, params)
	if err != nil {
		tuiApp.Send(tui.AgentErrorMsg{Error: err, Timestamp: time.Now()})
		return
	}

	var responseText strings.Builder
	toolCallCount := 0

	for event := range stream {
		switch e := event.(type) {
		case types.StreamMessage:
			for _, block := range e.Message.Content {
				switch b := block.(type) {
				case types.TextBlock:
					tuiApp.Send(tui.AgentChunkMsg{
						Text:      b.Text,
						Timestamp: time.Now(),
					})
					responseText.WriteString(b.Text)
				case types.ToolUseBlock:
					toolCallCount++
					app.handleToolUseStart(b, tuiApp)
				case types.ToolResultBlock:
					tuiApp.Send(tui.AgentToolDoneMsg{
						ToolID:  b.ToolUseID,
						Success: !b.IsError,
						Output:  fmt.Sprintf("%v", b.Content),
					})
				}
			}
			app.session.AddMessage(e.Message)
		case types.StreamError:
			tuiApp.Send(tui.AgentErrorMsg{Error: e.Error, Timestamp: time.Now()})
		}
	}

	app.costTracker.CompleteTurn()

	tuiApp.Send(tui.AgentDoneMsg{
		FullResponse: responseText.String(),
		ToolCalls:    toolCallCount,
		Timestamp:    time.Now(),
	})

	// Auto-save check
	if app.session.Turns%5 == 0 {
		if path, err := app.sessionManager.SaveCurrent(); err == nil {
			tuiApp.Send(tui.StatusMsg{Text: sprintf("Auto-saved to %s", path), Type: "info"})
			tuiApp.RefreshSessions(app.getSessionInfos())
		}
	}
}

// createToolPermissionFunc creates the permission checking function for tools.
func (app *App) createToolPermissionFunc(tuiApp *tui.App) tools.CanUseToolFn {
	return func(toolName string, toolInput map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		t, ok := app.toolRegistry.FindToolByName(toolName)
		if !ok {
			app.logAudit(toolName, toolInput, false, "deny")
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "unknown tool"}, nil
		}

		// Check permission mode first: non-interactive denials before approval UI
		permDecision := app.checkPermissionMode(toolName)
		if permDecision.Behavior == tools.Deny {
			app.logAudit(toolName, toolInput, false, "deny")
			return permDecision, nil
		}

		var decisionStr string

		needsApproval := approval.RequiresApproval(toolName) || permDecision.Behavior == tools.Ask
		if needsApproval {
			cmd := app.extractCommandForDisplay(toolName, toolInput)

			tuiApp.Send(tui.ToolExecutingMsg{
				ToolName: toolName,
				Command:  cmd,
			})

			if app.executionMode == approval.ModeInteractive {
				decision, err := app.requestCommandApproval(toolName, cmd, toolInput)
				if err != nil {
					app.logAudit(toolName, toolInput, false, "deny")
					return tools.PermissionDecision{
						Behavior: tools.Deny,
						Message:  sprintf("Approval failed: %v", err),
					}, nil
				}
				if !decision.IsApproved() {
					app.logAudit(toolName, toolInput, false, "reject")
					return tools.PermissionDecision{
						Behavior: tools.Deny,
						Message:  "Command rejected by user",
					}, nil
				}
				decisionStr = "approve"
			} else {
				decisionStr = "auto"
			}
		}

		for _, allowed := range app.config.AlwaysAllow {
			if allowed == toolName {
				app.logAudit(toolName, toolInput, true, "auto")
				return tools.PermissionDecision{Behavior: tools.Allow}, nil
			}
		}

		permCtx := permissions.EmptyContext()
		result := permissions.Evaluate(t, toolInput, permCtx)
		if result.Behavior == tools.Allow {
			if decisionStr == "" {
				decisionStr = "auto"
			}
			app.logAudit(toolName, toolInput, true, decisionStr)
		} else {
			app.logAudit(toolName, toolInput, false, "deny")
		}
		return result, nil
	}
}

// logAudit records a tool execution to the audit log.
func (app *App) logAudit(toolName string, toolInput map[string]any, approved bool, decision string) {
	if app.auditLogger == nil {
		return
	}
	_ = app.auditLogger.Log(audit.Entry{
		SessionID:      app.session.ID,
		ToolName:       toolName,
		InputHash:      audit.HashInput(toolInput),
		Approved:       approved,
		Decision:       decision,
		Persona:        app.session.Persona,
		PermissionMode: app.config.PermissionMode.String(),
	})
}

// checkPermissionMode checks tool against permission mode and granular settings.
func (app *App) checkPermissionMode(toolName string) tools.PermissionDecision {
	// First check granular permissions (they override mode presets)
	granular := app.checkGranularPermissions(toolName)
	if granular.Behavior != tools.Allow {
		return granular
	}

	switch app.config.PermissionMode {
	case config.PermissionReadOnly:
		if !isReadOnlyTool(toolName) {
			return tools.PermissionDecision{
				Behavior: tools.Deny,
				Message:  sprintf("Permission denied: %s", toolName),
			}
		}
	case config.PermissionWorkspaceWrite:
		if isDangerousTool(toolName) {
			return tools.PermissionDecision{
				Behavior: tools.Ask,
				Message:  sprintf("Confirm: %s", toolName),
			}
		}
	}
	return tools.PermissionDecision{Behavior: tools.Allow}
}

// syncGranularPermissions initializes granular toggles from the active permission mode.
func (app *App) syncGranularPermissions() {
	// If any granular permission is already set (non-zero), don't override
	// (Simple heuristic: if all are false, it's likely first run/not configured)
	if app.config.PermRead || app.config.PermWrite || app.config.PermDelete || app.config.PermExecute {
		return
	}

	switch app.config.PermissionMode {
	case config.PermissionReadOnly:
		app.config.PermRead = true
	case config.PermissionWorkspaceWrite:
		app.config.PermRead = true
		app.config.PermWrite = true
	case config.PermissionDangerFullAccess:
		app.config.PermRead = true
		app.config.PermWrite = true
		app.config.PermDelete = true
		app.config.PermExecute = true
	}
}

// checkGranularPermissions checks individual permission toggles.
func (app *App) checkGranularPermissions(toolName string) tools.PermissionDecision {
	switch toolName {
	case "read", "glob", "grep", "search", "web_fetch", "web_search":
		if !app.config.PermRead {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "Read permission disabled"}
		}
	case "write", "edit":
		if !app.config.PermWrite {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "Write permission disabled"}
		}
	case "delete", "rm", "mv":
		if !app.config.PermDelete {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "Delete permission disabled"}
		}
	case "bash", "shell", "execute_command":
		if !app.config.PermExecute {
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "Execute permission disabled"}
		}
	}
	return tools.PermissionDecision{Behavior: tools.Allow}
}

// handleToolUseStart handles the start of a tool use.
func (app *App) handleToolUseStart(b types.ToolUseBlock, tuiApp *tui.App) {
	tool, ok := app.toolRegistry.FindToolByName(b.Name)
	displayName := b.Name
	activityDesc := ""
	if ok {
		displayName = tool.UserFacingName(b.Input)
		activityDesc = tool.GetActivityDescription(b.Input)
	}

	tuiApp.Send(tui.AgentToolStartMsg{
		ToolID:       b.ID,
		ToolName:     b.Name,
		DisplayName:  displayName,
		ActivityDesc: activityDesc,
		Input:        b.Input,
	})
}

// validateConfig checks pre-flight configuration before calling LLM.
func (app *App) validateConfig() error {
	// Check API key for non-local providers
	if app.config.Provider != "ollama" && app.config.Provider != "local" {
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
type tuiChatDelegate struct {
	app    *App
	tuiApp *tui.App
}

// OnSubmit handles chat submit.
func (d *tuiChatDelegate) OnSubmit(text string) tea.Cmd {
	return func() tea.Msg {
		d.app.handleUserSubmit(text, d.tuiApp)
		return nil
	}
}

// OnCommand handles chat commands.
func (d *tuiChatDelegate) OnCommand(command string) {
	d.app.handleUserCommand(command, d.tuiApp)
}
