// Command approval system for agent-harness
// Provides interactive and yolo execution modes with transparent command visibility

package approval

import (
	"context"
	"fmt"
	"time"
)

// ExecutionMode determines how commands are approved
type ExecutionMode int

const (
	// ModeInteractive prompts user for each command (default)
	ModeInteractive ExecutionMode = iota
	// ModeYolo auto-approves commands but shows what is happening
	ModeYolo
)

func (m ExecutionMode) String() string {
	switch m {
	case ModeInteractive:
		return "interactive"
	case ModeYolo:
		return "yolo"
	default:
		return "unknown"
	}
}

// ParseExecutionMode parses an execution mode string
func ParseExecutionMode(s string) (ExecutionMode, error) {
	switch s {
	case "interactive", "ask", "confirm":
		return ModeInteractive, nil
	case "yolo", "auto", "trust":
		return ModeYolo, nil
	default:
		return ModeInteractive, fmt.Errorf("unknown execution mode: %s", s)
	}
}

// Decision represents a user's decision on a command approval
type Decision int

const (
	// DecisionPending means no decision has been made yet
	DecisionPending Decision = iota
	// DecisionApprove allows this command to execute
	DecisionApprove
	// DecisionReject blocks this command
	DecisionReject
	// DecisionApproveAll allows this and all similar commands
	DecisionApproveAll
	// DecisionRejectAll blocks this and suggests an alternative
	DecisionRejectAll
)

func (d Decision) String() string {
	switch d {
	case DecisionPending:
		return "pending"
	case DecisionApprove:
		return "approve"
	case DecisionReject:
		return "reject"
	case DecisionApproveAll:
		return "approve-all"
	case DecisionRejectAll:
		return "reject-all"
	default:
		return "unknown"
	}
}

// IsApproved returns true if the decision allows execution
func (d Decision) IsApproved() bool {
	return d == DecisionApprove || d == DecisionApproveAll
}

// IsFinal returns true if the decision is final (not pending)
func (d Decision) IsFinal() bool {
	return d != DecisionPending
}

// CommandInfo contains details about a command awaiting approval
type CommandInfo struct {
	ID            string
	ToolName      string
	DisplayName   string
	Command       string // The actual command being executed
	Description   string // Human-readable description of what it does
	IsDestructive bool
	Timestamp     time.Time
}

// ApprovalRequest represents a request for user approval
type ApprovalRequest struct {
	Command  CommandInfo
	Response chan Decision
	Context  context.Context
}

// NewApprovalRequest creates a new approval request
func NewApprovalRequest(cmd CommandInfo) *ApprovalRequest {
	return &ApprovalRequest{
		Command:  cmd,
		Response: make(chan Decision, 1),
		Context:  context.Background(),
	}
}

// Respond sends a decision response
func (r *ApprovalRequest) Respond(d Decision) {
	select {
	case r.Response <- d:
	default:
	}
}

// Manager handles command approval flow
type Manager struct {
	mode             ExecutionMode
	handler          ApprovalHandler
	pending          map[string]*ApprovalRequest
	approvedPatterns map[string]bool // Patterns that are auto-approved
	rejectedPatterns map[string]bool // Patterns that are auto-rejected
}

// ApprovalHandler is called when approval is needed
// Implementations should display the command and collect user input
type ApprovalHandler interface {
	// RequestApproval asks the user for approval
	// Returns a channel that will receive the decision
	RequestApproval(req *ApprovalRequest) error

	// ShowCommand displays what command is running (for yolo mode)
	ShowCommand(cmd CommandInfo)

	// OnCancel is called when the user cancels (ESC key)
	OnCancel()
}

// NewManager creates a new approval manager
func NewManager(mode ExecutionMode, handler ApprovalHandler) *Manager {
	return &Manager{
		mode:             mode,
		handler:          handler,
		pending:          make(map[string]*ApprovalRequest),
		approvedPatterns: make(map[string]bool),
		rejectedPatterns: make(map[string]bool),
	}
}

// SetMode changes the execution mode
func (m *Manager) SetMode(mode ExecutionMode) {
	m.mode = mode
}

// GetMode returns the current execution mode
func (m *Manager) GetMode() ExecutionMode {
	return m.mode
}

// CheckApproval determines if a command should be executed
// Returns the decision and true if the command should proceed
func (m *Manager) CheckApproval(cmd CommandInfo) (Decision, error) {
	// Check if this pattern was previously approved/rejected
	if m.approvedPatterns[cmd.Command] {
		return DecisionApprove, nil
	}
	if m.rejectedPatterns[cmd.Command] {
		return DecisionReject, fmt.Errorf("command pattern was previously rejected")
	}

	// In yolo mode, auto-approve but show what's happening
	if m.mode == ModeYolo {
		if m.handler != nil {
			m.handler.ShowCommand(cmd)
		}
		return DecisionApprove, nil
	}

	// In interactive mode, request approval
	if m.handler == nil {
		return DecisionReject, fmt.Errorf("no approval handler configured")
	}

	req := NewApprovalRequest(cmd)
	m.pending[cmd.ID] = req

	if err := m.handler.RequestApproval(req); err != nil {
		delete(m.pending, cmd.ID)
		return DecisionReject, err
	}

	// Wait for response
	select {
	case decision := <-req.Response:
		delete(m.pending, cmd.ID)

		// Handle special decisions
		switch decision {
		case DecisionApproveAll:
			m.approvedPatterns[cmd.Command] = true
			return DecisionApprove, nil
		case DecisionRejectAll:
			m.rejectedPatterns[cmd.Command] = true
			return DecisionReject, fmt.Errorf("command rejected by user")
		}

		return decision, nil
	case <-req.Context.Done():
		delete(m.pending, cmd.ID)
		return DecisionReject, req.Context.Err()
	}
}

// CancelAll cancels all pending approvals
func (m *Manager) CancelAll() {
	for id, req := range m.pending {
		req.Respond(DecisionReject)
		delete(m.pending, id)
	}
	if m.handler != nil {
		m.handler.OnCancel()
	}
}

// RequiresApproval returns true if a tool requires user approval
func RequiresApproval(toolName string) bool {
	// Tools that can modify the system require approval
	dangerousTools := []string{"bash", "shell", "write", "edit", "delete", "rm"}
	for _, t := range dangerousTools {
		if t == toolName {
			return true
		}
	}
	return false
}

// FormatCommandForDisplay formats a command for user display
func FormatCommandForDisplay(toolName, command string) string {
	// Clean up the command display
	if command == "" {
		return fmt.Sprintf("[%s]", toolName)
	}

	// For shell commands, show the actual command
	if toolName == "bash" || toolName == "shell" {
		return command
	}

	// For other tools, show the tool name and truncated details
	if len(command) > 100 {
		return command[:97] + "..."
	}
	return command
}
