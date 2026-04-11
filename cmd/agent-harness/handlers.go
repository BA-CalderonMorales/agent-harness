package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// handleUserSubmit processes user message submission.
func (app *App) handleUserSubmit(text string, tuiApp *tui.App) {
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
	tuiApp.Send(tui.AgentStartMsg{Timestamp: time.Now()})

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
			return tools.PermissionDecision{Behavior: tools.Deny, Message: "unknown tool"}, nil
		}

		if approval.RequiresApproval(toolName) {
			cmd := app.extractCommandForDisplay(toolName, toolInput)

			tuiApp.Send(tui.ToolExecutingMsg{
				ToolName: toolName,
				Command:  cmd,
			})

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
		}

		if decision := app.checkPermissionMode(toolName); decision.Behavior != tools.Allow {
			return decision, nil
		}

		for _, allowed := range app.config.AlwaysAllow {
			if allowed == toolName {
				return tools.PermissionDecision{Behavior: tools.Allow}, nil
			}
		}

		permCtx := permissions.EmptyContext()
		return permissions.Evaluate(t, toolInput, permCtx), nil
	}
}

// checkPermissionMode checks tool against permission mode.
func (app *App) checkPermissionMode(toolName string) tools.PermissionDecision {
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
