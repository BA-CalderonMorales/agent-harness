package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"time"
)

// requestCommandApproval requests user approval for a command.
func (app *App) requestCommandApproval(toolName, command string, toolInput map[string]any) (approval.Decision, error) {
	if app.tuiApp == nil {
		return approval.DecisionReject, fmt.Errorf("TUI not available")
	}

	cmdID := generateUUID()
	isDestructive := checkDestructive(toolName, command)

	preview := app.buildCommandPreview(toolName, toolInput)

	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:            cmdID,
		ToolName:      toolName,
		DisplayName:   toolName,
		Command:       command,
		Description:   approval.FormatCommandForDisplay(toolName, command),
		Preview:       preview,
		IsDestructive: isDestructive,
		Timestamp:     time.Now(),
	})

	app.tuiApp.Send(tui.ApprovalRequestMsg{Request: req})

	select {
	case decision := <-req.Response:
		return decision, nil
	case <-req.Context.Done():
		return approval.DecisionReject, req.Context.Err()
	}
}

// checkDestructive determines if a command is destructive.
