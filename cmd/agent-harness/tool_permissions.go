package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/audit"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"strings"
)

// createToolPermissionFunc creates the permission checking function for tools.
func (app *App) createToolPermissionFunc(tuiApp *tui.App) tools.CanUseToolFn {
	return func(toolName string, toolInput map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		auditEvent := func(event tools.ToolAuditEvent) error {
			return app.logAudit(event)
		}
		checkpoint := func() (tools.PermissionDecision, error) {
			cmd := app.extractCommandForDisplay(toolName, toolInput)
			if tuiApp != nil {
				tuiApp.Send(tui.ToolExecutingMsg{
					ToolName: toolName,
					Command:  cmd,
				})
			}

			if app.executionMode == approval.ModeInteractive {
				decision, err := app.requestCommandApproval(toolName, cmd, toolInput)
				if err != nil {
					return tools.PermissionDecision{
						Behavior: tools.Deny,
						Message:  sprintf("Approval failed: %v", err),
					}, nil
				}
				if !decision.IsApproved() {
					return tools.PermissionDecision{
						Behavior: tools.Deny,
						Message:  "Command rejected by user",
					}, nil
				}
			}
			return tools.PermissionDecision{
				Behavior:     tools.Allow,
				UpdatedInput: toolInput,
			}, nil
		}

		makeDecision := func(behavior tools.DecisionBehavior, message string) tools.PermissionDecision {
			return tools.PermissionDecision{
				Behavior:     behavior,
				UpdatedInput: toolInput,
				Message:      message,
				Checkpoint:   checkpoint,
				Audit:        auditEvent,
			}
		}

		// Explicit deny rules are the first global-policy layer.
		for _, denied := range app.config.AlwaysDeny {
			if denied == toolName {
				return makeDecision(tools.Deny, "denied by always_deny"), nil
			}
		}

		toolDef, ok := app.toolRegistry.FindToolByName(toolName)
		if !ok {
			return makeDecision(tools.Deny, "unknown tool"), nil
		}

		for _, allowed := range app.config.AlwaysAllow {
			if allowed == toolName {
				return makeDecision(tools.Allow, ""), nil
			}
		}

		permDecision := app.checkPermissionMode(toolName)
		if permDecision.Behavior == tools.Deny {
			return makeDecision(tools.Deny, permDecision.Message), nil
		}

		needsApproval := permDecision.Behavior == tools.Ask ||
			approval.RequiresApproval(toolName) ||
			strings.HasPrefix(toolName, "mcp_") ||
			(toolDef.Capabilities.IsDestructive != nil && toolDef.Capabilities.IsDestructive(toolInput))
		if needsApproval {
			return makeDecision(tools.Ask, permDecision.Message), nil
		}
		return makeDecision(tools.Allow, ""), nil
	}
}

// logAudit records a tool execution to the audit log.
func (app *App) logAudit(event tools.ToolAuditEvent) error {
	if app.auditLogger == nil {
		return fmt.Errorf("audit logger unavailable")
	}

	sessionID := ""
	personaName := ""
	if app.session != nil {
		sessionID = app.session.ID
		personaName = app.session.Persona
	}
	permissionMode := ""
	if app.config != nil {
		permissionMode = app.config.PermissionMode.String()
	}
	errorText := ""
	if event.Err != nil {
		errorText = event.Err.Error()
	}

	return app.auditLogger.Log(audit.Entry{
		SessionID:      sessionID,
		Event:          event.Event,
		ToolCallID:     event.ToolCallID,
		ToolName:       event.ToolName,
		InputHash:      audit.HashInput(event.Input),
		Approved:       event.Behavior == tools.Allow,
		Decision:       string(event.Behavior),
		DurationMillis: event.DurationMillis,
		Error:          errorText,
		Persona:        personaName,
		PermissionMode: permissionMode,
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
