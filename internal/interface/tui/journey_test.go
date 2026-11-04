package tui

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// The journey: the user lands on home base, steps into the chat, and
// watches the turn move through its four beats with the human in
// control, then steps back. This is the acceptance test for the whole
// 0.4.0 journey: the loop is visible, the human can redirect, and the
// plain path works at every pane size.

func TestJourneyHomeToChatAndBack(t *testing.T) {
	app := NewApp()
	app.SetLoopTrack(true)
	h := ttui.New(t, app)
	h.Resize(80, 24)

	app.homeModel.SetProjectInfo(ProjectInfo{Name: "agent-harness", Type: "Go", GitBranch: "main"})
	app.homeModel.SetProvider("openrouter")
	app.homeModel.SetStatus("gpt-5.1", "interactive", "builder", 0)
	h.Resize(80, 24)

	h.AssertFitsWidth(80)
	h.AssertContains("⌂", "agent-harness", "openrouter")

	// Step inside: the chat tab.
	h.Press("2")
	h.AssertFitsWidth(80)
	h.AssertContains(`"i" to type a message`)

	// A turn starts: the beat is Ask.
	app.chatModel.AddMessage("user", "fix the flaky test")
	app.chatModel.SetThinking(true, "")
	// The live Agent header exists from the start; the spinner and the
	// delegation marker ride on it while the turn runs.
	app.chatModel.updateOrCreateStreamingMessage("")
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertContains("fix the flaky test")
	h.AssertContains("▸ Ask")
	h.AssertNotContains("▸ Plan", "▸ Review", "▸ Try")

	// The plan is written: the beat moves to Plan.
	app.chatModel.completedToolMsgs = []ChatMessage{{ToolName: "todo_write"}}
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertContains("▸ Plan")

	// A risky step needs the human: Review, and the frame says so.
	app.approvalDialog.Show(approval.NewApprovalRequest(approval.CommandInfo{
		ID: "cmd-1", ToolName: "bash", DisplayName: "bash", Command: "go test ./...",
	}))
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertContains("Approval required")
	h.AssertContains("go test ./...")
	if app.currentLoopBeat() != beatReview {
		t.Fatalf("beat = %q, want review while the approval is open", app.currentLoopBeat())
	}

	// The human approves: the beat moves to Try.
	app.approvalDialog.Hide()
	block := ToolUseBlock{ID: "t1", Name: "bash"}
	app.chatModel.currentTool = &block
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertContains("▸ Try")
	h.AssertNotContains("waiting for you")

	// A delegation is a flight between places.
	app.chatModel.currentTool = &ToolUseBlock{ID: "t2", Name: "agent"}
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertContains("✈ delegating")

	// The turn ends: the track goes away.
	app.chatModel.thinking = false
	app.chatModel.currentTool = nil
	app.chatModel.completedToolMsgs = nil
	app.chatModel.dropPlaceholderIfEmpty()
	h.Resize(80, 24)
	h.AssertFitsWidth(80)
	h.AssertNotContains("▸ Ask", "▸ Plan", "▸ Review", "▸ Try", "✈ delegating")

	// Back to home base.
	h.Press("1")
	h.AssertFitsWidth(80)
	h.AssertContains("⌂", "agent-harness")
}

func TestJourneyStaysOnAPhonePane(t *testing.T) {
	app := NewApp()
	app.SetLoopTrack(true)
	h := ttui.New(t, app)
	h.Resize(40, 15)
	app.homeModel.SetProjectInfo(ProjectInfo{Name: "agent-harness", Type: "Go", GitBranch: "main"})
	app.homeModel.SetProvider("openrouter")
	app.homeModel.SetStatus("gpt-5.1", "interactive", "builder", 0)
	h.Resize(40, 15)

	h.AssertFitsWidth(40)
	h.AssertContains("Agent-Harness")

	// The chat needs a little more height than Home: the composer and
	// its band are non-negotiable, and the beat row is the first thing
	// to yield on a pane that short.
	app.activeView = viewChat
	app.chatModel.AddMessage("user", "fix it")
	app.chatModel.SetThinking(true, "")
	h.Resize(40, 24)
	h.AssertFitsWidth(40)
	h.AssertContains("▸ Ask")

	app.approvalDialog.Show(approval.NewApprovalRequest(approval.CommandInfo{
		ID: "cmd-1", ToolName: "bash", DisplayName: "bash", Command: "go test",
	}))
	h.Resize(40, 24)
	h.AssertFitsWidth(40)
	h.AssertContains("Approval required", "go test")
}
