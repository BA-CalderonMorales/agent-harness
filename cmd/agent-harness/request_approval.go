package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"time"
)

// requestCommandApproval requests user approval for a command.
// Returns the decision and, for rejections, the optional free-text note
// the user attached ("Reject + Suggest") — the agent reads it as the
// deny reason and can adapt instead of retrying blind.
func (app *App) requestCommandApproval(toolName, command string, toolInput map[string]any) (approval.Decision, string, error) {
	if app.tuiApp == nil {
		return approval.DecisionReject, "", fmt.Errorf("TUI not available")
	}

	// "Approve All" memory: the exact command was already approved this
	// session — don't ask again.
	if app.approvedCommands[command] {
		app.tuiApp.Send(tui.StatusMsg{
			Text: sprintf("Auto-approved (Approve All): %s", truncateRunes(command, 60)),
			Type: "info",
		})
		return approval.DecisionApprove, "", nil
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
		if decision == approval.DecisionApproveAll {
			app.approvedCommands[command] = true
			app.tuiApp.Send(tui.StatusMsg{
				Text: "Same command won't ask again this session (Approve All).",
				Type: "info",
			})
			return approval.DecisionApprove, "", nil
		}
		return decision, req.Note, nil
	case <-req.Context.Done():
		return approval.DecisionReject, "", req.Context.Err()
	}
}

// truncateRunes shortens s to at most n runes with an ellipsis.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// checkDestructive determines if a command is destructive.
