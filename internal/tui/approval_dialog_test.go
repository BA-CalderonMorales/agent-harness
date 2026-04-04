// Test suite for the approval dialog component

package tui

import (
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/approval"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Initialization Tests
// ---------------------------------------------------------------------------

func TestNewApprovalDialog(t *testing.T) {
	dialog := NewApprovalDialog()

	if dialog.visible {
		t.Error("New dialog should not be visible")
	}

	if dialog.request != nil {
		t.Error("New dialog should not have a request")
	}

	if len(dialog.options) != 4 {
		t.Errorf("Expected 4 options, got %d", len(dialog.options))
	}
}

func TestDefaultApprovalOptions(t *testing.T) {
	options := defaultApprovalOptions()

	expectedOptions := []struct {
		label       string
		key         string
		decision    approval.Decision
		isDangerous bool
	}{
		{"Approve", "a", approval.DecisionApprove, false},
		{"Approve All", "A", approval.DecisionApproveAll, false},
		{"Reject", "r", approval.DecisionReject, true},
		{"Reject + Suggest", "R", approval.DecisionRejectAll, true},
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d", len(expectedOptions), len(options))
	}

	for i, expected := range expectedOptions {
		opt := options[i]
		if opt.Label != expected.label {
			t.Errorf("Option %d: Label = %q, want %q", i, opt.Label, expected.label)
		}
		if opt.Key != expected.key {
			t.Errorf("Option %d: Key = %q, want %q", i, opt.Key, expected.key)
		}
		if opt.Decision != expected.decision {
			t.Errorf("Option %d: Decision = %v, want %v", i, opt.Decision, expected.decision)
		}
		if opt.IsDangerous != expected.isDangerous {
			t.Errorf("Option %d: IsDangerous = %v, want %v", i, opt.IsDangerous, expected.isDangerous)
		}
	}
}

// ---------------------------------------------------------------------------
// Show/Hide Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_Show(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})

	dialog.Show(req)

	if !dialog.IsVisible() {
		t.Error("Expected dialog to be visible after Show()")
	}

	if dialog.request != req {
		t.Error("Expected request to be set")
	}

	if dialog.selected != 0 {
		t.Errorf("Expected selected to be 0, got %d", dialog.selected)
	}
}

func TestApprovalDialog_Hide(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})

	dialog.Show(req)
	dialog.Hide()

	if dialog.IsVisible() {
		t.Error("Expected dialog to be hidden after Hide()")
	}

	if dialog.request != nil {
		t.Error("Expected request to be cleared after Hide()")
	}
}

// ---------------------------------------------------------------------------
// Navigation Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_Update_Navigation(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})
	dialog.Show(req)

	// Test down navigation
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	if dialog.selected != 1 {
		t.Errorf("After Down, selected = %d, want 1", dialog.selected)
	}

	// Test wrap around
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	if dialog.selected != 0 {
		t.Errorf("After wrapping, selected = %d, want 0", dialog.selected)
	}

	// Test up navigation
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyUp})
	if dialog.selected != 3 {
		t.Errorf("After Up, selected = %d, want 3", dialog.selected)
	}
}

// ---------------------------------------------------------------------------
// Selection Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_Update_Enter(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})
	dialog.Show(req)

	// Select second option (Approve All)
	dialog, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyDown})

	resultChan := make(chan approval.Decision, 1)
	go func() {
		select {
		case decision := <-req.Response:
			resultChan <- decision
		case <-time.After(100 * time.Millisecond):
			resultChan <- approval.DecisionPending
		}
	}()

	dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Execute the command
	if cmd != nil {
		msg := cmd()
		if approvalMsg, ok := msg.(ApprovalDecisionMsg); ok {
			if approvalMsg.Decision != approval.DecisionApproveAll {
				t.Errorf("Expected ApproveAll decision, got %v", approvalMsg.Decision)
			}
		} else {
			t.Error("Expected ApprovalDecisionMsg")
		}
	}

	// Check that dialog is hidden
	if dialog.IsVisible() {
		t.Error("Expected dialog to be hidden after selection")
	}
}

func TestApprovalDialog_Update_Escape(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})
	dialog.Show(req)

	dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Execute the command
	if cmd != nil {
		msg := cmd()
		if approvalMsg, ok := msg.(ApprovalDecisionMsg); ok {
			if approvalMsg.Decision != approval.DecisionReject {
				t.Errorf("Expected Reject decision, got %v", approvalMsg.Decision)
			}
		} else {
			t.Error("Expected ApprovalDecisionMsg")
		}
	}

	// Check that dialog is hidden
	if dialog.IsVisible() {
		t.Error("Expected dialog to be hidden after escape")
	}
}

func TestApprovalDialog_Update_DirectKey(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})
	dialog.Show(req)

	// Press 'a' to approve directly
	dialog, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// Execute the command
	if cmd != nil {
		msg := cmd()
		if approvalMsg, ok := msg.(ApprovalDecisionMsg); ok {
			if approvalMsg.Decision != approval.DecisionApprove {
				t.Errorf("Expected Approve decision, got %v", approvalMsg.Decision)
			}
		} else {
			t.Error("Expected ApprovalDecisionMsg")
		}
	}

	// Check that dialog is hidden
	if dialog.IsVisible() {
		t.Error("Expected dialog to be hidden after key press")
	}
}

// ---------------------------------------------------------------------------
// Notification Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_ShowNotification(t *testing.T) {
	dialog := NewApprovalDialog()

	dialog.ShowNotification("Command executed: echo hello")

	if !dialog.IsShowingNotification() {
		t.Error("Expected notification to be showing")
	}

	if dialog.notification != "Command executed: echo hello" {
		t.Errorf("Notification = %q, want %q", dialog.notification, "Command executed: echo hello")
	}
}

func TestApprovalDialog_NotificationTimeout(t *testing.T) {
	dialog := NewApprovalDialog()

	// Show notification with very short duration by manipulating the time
	dialog.ShowNotification("Test")
	dialog.notificationUntil = time.Now().Add(-1 * time.Millisecond) // Already expired

	if dialog.IsShowingNotification() {
		t.Error("Expected notification to be expired")
	}
}

// ---------------------------------------------------------------------------
// Window Size Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_Update_WindowSize(t *testing.T) {
	dialog := NewApprovalDialog()

	dialog, _ = dialog.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Window size is stored when dialog is visible or for future use
	// The test verifies the update is processed without error
}

// ---------------------------------------------------------------------------
// View Tests
// ---------------------------------------------------------------------------

func TestApprovalDialog_View_NotVisible(t *testing.T) {
	dialog := NewApprovalDialog()

	view := dialog.View()

	if view != "" {
		t.Error("Expected empty view when not visible")
	}
}

func TestApprovalDialog_View_Visible(t *testing.T) {
	dialog := NewApprovalDialog()
	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	})
	dialog.Show(req)
	dialog.width = 80
	dialog.height = 24

	view := dialog.View()

	if view == "" {
		t.Error("Expected non-empty view when visible")
	}
}

// ---------------------------------------------------------------------------
// Helper Function Tests
// ---------------------------------------------------------------------------

func TestWrapText(t *testing.T) {
	tests := []struct {
		input     string
		maxWidth  int
		wantLines int
	}{
		{"short text", 100, 1},
		{"line one\nline two", 100, 2},
		{"very long text that needs wrapping in the middle somewhere", 20, 3},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 10)], func(t *testing.T) {
			got := wrapText(tt.input, tt.maxWidth)
			lines := 1
			for _, c := range got {
				if c == '\n' {
					lines++
				}
			}
			if lines != tt.wantLines {
				t.Errorf("wrapText(%q, %d) produced %d lines, want %d",
					tt.input, tt.maxWidth, lines, tt.wantLines)
			}
		})
	}
}

func TestWrapText_EdgeCases(t *testing.T) {
	// Zero width
	result := wrapText("test", 0)
	if result != "test" {
		t.Errorf("wrapText with 0 width should return original, got %q", result)
	}

	// Negative width
	result = wrapText("test", -1)
	if result != "test" {
		t.Errorf("wrapText with negative width should return original, got %q", result)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
