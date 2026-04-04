// Test suite for the command approval system
// Validates interactive and yolo mode behaviors

package approval

import (
	"context"
	"testing"
	"time"
)

// mockApprovalHandler is a test double for ApprovalHandler
type mockApprovalHandler struct {
	lastRequest    *ApprovalRequest
	respondWith    Decision
	showedCommands []CommandInfo
	onCancelCalled bool
}

func (m *mockApprovalHandler) RequestApproval(req *ApprovalRequest) error {
	m.lastRequest = req
	// Auto-respond after a short delay to avoid blocking tests
	go func() {
		time.Sleep(10 * time.Millisecond)
		req.Respond(m.respondWith)
	}()
	return nil
}

func (m *mockApprovalHandler) ShowCommand(cmd CommandInfo) {
	m.showedCommands = append(m.showedCommands, cmd)
}

func (m *mockApprovalHandler) OnCancel() {
	m.onCancelCalled = true
}

// ---------------------------------------------------------------------------
// Mode Tests
// ---------------------------------------------------------------------------

func TestParseExecutionMode(t *testing.T) {
	tests := []struct {
		input    string
		expected ExecutionMode
		wantErr  bool
	}{
		{"interactive", ModeInteractive, false},
		{"ask", ModeInteractive, false},
		{"confirm", ModeInteractive, false},
		{"yolo", ModeYolo, false},
		{"auto", ModeYolo, false},
		{"trust", ModeYolo, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := ParseExecutionMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExecutionMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if mode != tt.expected {
				t.Errorf("ParseExecutionMode(%q) = %v, want %v", tt.input, mode, tt.expected)
			}
		})
	}
}

func TestExecutionModeString(t *testing.T) {
	tests := []struct {
		mode     ExecutionMode
		expected string
	}{
		{ModeInteractive, "interactive"},
		{ModeYolo, "yolo"},
		{ExecutionMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("ExecutionMode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Decision Tests
// ---------------------------------------------------------------------------

func TestDecisionIsApproved(t *testing.T) {
	tests := []struct {
		decision Decision
		approved bool
	}{
		{DecisionPending, false},
		{DecisionApprove, true},
		{DecisionReject, false},
		{DecisionApproveAll, true},
		{DecisionRejectAll, false},
	}

	for _, tt := range tests {
		t.Run(tt.decision.String(), func(t *testing.T) {
			if got := tt.decision.IsApproved(); got != tt.approved {
				t.Errorf("Decision.IsApproved() = %v, want %v", got, tt.approved)
			}
		})
	}
}

func TestDecisionIsFinal(t *testing.T) {
	tests := []struct {
		decision Decision
		final    bool
	}{
		{DecisionPending, false},
		{DecisionApprove, true},
		{DecisionReject, true},
		{DecisionApproveAll, true},
		{DecisionRejectAll, true},
	}

	for _, tt := range tests {
		t.Run(tt.decision.String(), func(t *testing.T) {
			if got := tt.decision.IsFinal(); got != tt.final {
				t.Errorf("Decision.IsFinal() = %v, want %v", got, tt.final)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager Tests - Interactive Mode
// ---------------------------------------------------------------------------

func TestManager_InteractiveMode_Approve(t *testing.T) {
	handler := &mockApprovalHandler{respondWith: DecisionApprove}
	manager := NewManager(ModeInteractive, handler)

	cmd := CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	}

	decision, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}

	if decision != DecisionApprove {
		t.Errorf("CheckApproval() = %v, want %v", decision, DecisionApprove)
	}

	if handler.lastRequest == nil {
		t.Error("Expected handler.RequestApproval to be called")
	}
}

func TestManager_InteractiveMode_Reject(t *testing.T) {
	handler := &mockApprovalHandler{respondWith: DecisionReject}
	manager := NewManager(ModeInteractive, handler)

	cmd := CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "rm -rf /",
	}

	decision, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Errorf("Did not expect error for simple reject: %v", err)
	}

	if decision.IsApproved() {
		t.Errorf("Expected command to be rejected, got %v", decision)
	}

	if decision != DecisionReject {
		t.Errorf("Expected DecisionReject, got %v", decision)
	}
}

func TestManager_InteractiveMode_ApproveAll(t *testing.T) {
	handler := &mockApprovalHandler{respondWith: DecisionApproveAll}
	manager := NewManager(ModeInteractive, handler)

	cmd := CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	}

	// First call should request approval
	decision, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}

	if decision != DecisionApprove {
		t.Errorf("CheckApproval() = %v, want %v", decision, DecisionApprove)
	}

	// Second call with same command should auto-approve
	handler.lastRequest = nil
	decision2, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}

	if decision2 != DecisionApprove {
		t.Errorf("Second CheckApproval() = %v, want %v", decision2, DecisionApprove)
	}

	if handler.lastRequest != nil {
		t.Error("Expected second call to not request approval (auto-approved)")
	}
}

// ---------------------------------------------------------------------------
// Manager Tests - Yolo Mode
// ---------------------------------------------------------------------------

func TestManager_YoloMode_AutoApprove(t *testing.T) {
	handler := &mockApprovalHandler{}
	manager := NewManager(ModeYolo, handler)

	cmd := CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "echo hello",
	}

	decision, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}

	if decision != DecisionApprove {
		t.Errorf("CheckApproval() = %v, want %v", decision, DecisionApprove)
	}

	if handler.lastRequest != nil {
		t.Error("Expected handler.RequestApproval to NOT be called in yolo mode")
	}

	if len(handler.showedCommands) != 1 {
		t.Error("Expected handler.ShowCommand to be called in yolo mode")
	}
}

func TestManager_YoloMode_ShowsCommand(t *testing.T) {
	handler := &mockApprovalHandler{}
	manager := NewManager(ModeYolo, handler)

	commands := []CommandInfo{
		{ID: "1", ToolName: "bash", Command: "echo hello"},
		{ID: "2", ToolName: "write", Command: "file.txt"},
		{ID: "3", ToolName: "edit", Command: "other.txt"},
	}

	for _, cmd := range commands {
		manager.CheckApproval(cmd)
	}

	if len(handler.showedCommands) != 3 {
		t.Errorf("Expected 3 commands to be shown, got %d", len(handler.showedCommands))
	}
}

// ---------------------------------------------------------------------------
// Manager Tests - Mode Switching
// ---------------------------------------------------------------------------

func TestManager_SetMode(t *testing.T) {
	handler := &mockApprovalHandler{}
	manager := NewManager(ModeInteractive, handler)

	if manager.GetMode() != ModeInteractive {
		t.Errorf("Initial mode = %v, want %v", manager.GetMode(), ModeInteractive)
	}

	manager.SetMode(ModeYolo)

	if manager.GetMode() != ModeYolo {
		t.Errorf("After SetMode, mode = %v, want %v", manager.GetMode(), ModeYolo)
	}

	// Verify behavior changed
	cmd := CommandInfo{ID: "test", ToolName: "bash", Command: "echo test"}
	decision, err := manager.CheckApproval(cmd)
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}

	if decision != DecisionApprove {
		t.Errorf("In yolo mode, decision = %v, want %v", decision, DecisionApprove)
	}
}

// ---------------------------------------------------------------------------
// Manager Tests - Cancel
// ---------------------------------------------------------------------------

func TestManager_CancelAll(t *testing.T) {
	handler := &mockApprovalHandler{}
	manager := NewManager(ModeInteractive, handler)

	// Create a pending request
	req := NewApprovalRequest(CommandInfo{ID: "test", ToolName: "bash", Command: "echo test"})
	manager.pending["test"] = req

	// Cancel all
	manager.CancelAll()

	// Check that onCancel was called
	if !handler.onCancelCalled {
		t.Error("Expected handler.OnCancel to be called")
	}

	// Check that request was removed from pending
	if len(manager.pending) != 0 {
		t.Errorf("Expected pending to be empty, got %d items", len(manager.pending))
	}
}

// ---------------------------------------------------------------------------
// Command Pattern Memory Tests
// ---------------------------------------------------------------------------

func TestManager_RejectedPattern_Remembered(t *testing.T) {
	handler := &mockApprovalHandler{respondWith: DecisionRejectAll}
	manager := NewManager(ModeInteractive, handler)

	cmd := CommandInfo{
		ID:       "test-1",
		ToolName: "bash",
		Command:  "rm -rf /",
	}

	// First call - reject all
	_, _ = manager.CheckApproval(cmd)

	// Second call with same command should be auto-rejected
	handler.respondWith = DecisionApprove // This shouldn't matter
	decision2, err := manager.CheckApproval(cmd)

	if err == nil {
		t.Error("Expected error for previously rejected pattern")
	}

	if decision2.IsApproved() {
		t.Error("Expected previously rejected command to stay rejected")
	}
}

// ---------------------------------------------------------------------------
// Approval Request Tests
// ---------------------------------------------------------------------------

func TestApprovalRequest_Respond(t *testing.T) {
	req := NewApprovalRequest(CommandInfo{ID: "test", ToolName: "bash"})

	go func() {
		req.Respond(DecisionApprove)
	}()

	select {
	case decision := <-req.Response:
		if decision != DecisionApprove {
			t.Errorf("Response = %v, want %v", decision, DecisionApprove)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for response")
	}
}

func TestApprovalRequest_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := &ApprovalRequest{
		Command:  CommandInfo{ID: "test", ToolName: "bash"},
		Response: make(chan Decision, 1),
		Context:  ctx,
	}

	cancel() // Cancel context immediately

	select {
	case <-req.Context.Done():
		// Expected behavior
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected context to be cancelled")
	}
}

// ---------------------------------------------------------------------------
// FormatCommandForDisplay Tests
// ---------------------------------------------------------------------------

func TestFormatCommandForDisplay(t *testing.T) {
	tests := []struct {
		toolName string
		command  string
		want     string
	}{
		{"bash", "echo hello", "echo hello"},
		{"bash", "", "[bash]"},
		{"shell", "ls -la", "ls -la"},
		{"write", "file.txt", "file.txt"},
		{"", "", "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.toolName+"_"+tt.command, func(t *testing.T) {
			got := FormatCommandForDisplay(tt.toolName, tt.command)
			if got != tt.want {
				t.Errorf("FormatCommandForDisplay(%q, %q) = %q, want %q",
					tt.toolName, tt.command, got, tt.want)
			}
		})
	}
}

func TestFormatCommandForDisplay_LongCommand(t *testing.T) {
	longCmd := "echo " + string(make([]byte, 200))
	for i := range longCmd[5:] {
		longCmd = longCmd[:5] + string('a'+byte(i%26)) + longCmd[6:]
	}

	result := FormatCommandForDisplay("other", longCmd)
	if len(result) > 103 {
		t.Errorf("Expected truncated command, got length %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// RequiresApproval Tests
// ---------------------------------------------------------------------------

func TestRequiresApproval(t *testing.T) {
	tests := []struct {
		toolName string
		requires bool
	}{
		{"bash", true},
		{"shell", true},
		{"write", true},
		{"edit", true},
		{"delete", true},
		{"rm", true},
		{"read", false},
		{"glob", false},
		{"grep", false},
		{"search", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			got := RequiresApproval(tt.toolName)
			if got != tt.requires {
				t.Errorf("RequiresApproval(%q) = %v, want %v",
					tt.toolName, got, tt.requires)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkCheckApproval_YoloMode(b *testing.B) {
	manager := NewManager(ModeYolo, &mockApprovalHandler{})
	cmd := CommandInfo{ID: "bench", ToolName: "bash", Command: "echo test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CheckApproval(cmd)
	}
}

func BenchmarkCheckApproval_CachedPattern(b *testing.B) {
	manager := NewManager(ModeInteractive, &mockApprovalHandler{})
	manager.approvedPatterns["echo test"] = true
	cmd := CommandInfo{ID: "bench", ToolName: "bash", Command: "echo test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CheckApproval(cmd)
	}
}
