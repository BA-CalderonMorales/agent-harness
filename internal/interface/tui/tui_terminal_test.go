package tui

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// The terminal harness renders the real model and checks what a terminal
// would receive: nothing wider than the pane, and the same frame every
// run for the overlays that own the keyboard.

func TestQuestionDialogGolden(t *testing.T) {
	app := NewApp()
	h := ttui.New(t, app)
	h.Resize(80, 24)

	req := NewQuestionRequest(nil, "Which environment should the deploy target?", []string{"staging", "production"})
	h.Send(QuestionRequestMsg{Request: req})

	h.AssertFitsWidth(80)
	h.AssertContains("Which environment should the deploy target?", "staging", "production")

	ttui.AssertGolden(t, "question-dialog", h.Text())
}

func TestApprovalDialogGolden(t *testing.T) {
	app := NewApp()
	h := ttui.New(t, app)
	h.Resize(80, 24)

	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:          "cmd-1",
		ToolName:    "bash",
		DisplayName: "bash",
		Command:     "go test ./...",
		Description: "go test ./...",
	})
	h.Send(ApprovalRequestMsg{Request: req})

	h.AssertFitsWidth(80)
	h.AssertContains("go test ./...", "Approve")

	ttui.AssertGolden(t, "approval-dialog", h.Text())
}

func TestHomeFitsEveryPaneSize(t *testing.T) {
	for _, size := range [][2]int{{40, 15}, {80, 24}, {120, 30}} {
		app := NewApp()
		h := ttui.New(t, app)
		h.Resize(size[0], size[1])

		// The tab bar abbreviates to fit the pane, so assert on the
		// stable part of the frame instead of a tab label.
		h.AssertFitsWidth(size[0])
		h.AssertContains("Agent-Harness")
	}
}

func TestChatFitsEveryPaneSize(t *testing.T) {
	for _, size := range [][2]int{{40, 15}, {80, 24}, {120, 30}} {
		app := NewApp()
		h := ttui.New(t, app)
		h.Resize(size[0], size[1])
		app.activeView = viewChat
		app.chatModel.AddMessage("user", "list the files")
		// Re-render: the harness records a frame per message, so the
		// mutation above needs one more update to be visible.
		h.Resize(size[0], size[1])

		h.AssertFitsWidth(size[0])
		if size[1] >= 24 {
			// On a very short pane the transcript scrolls and the
			// message leaves the visible window; the fit check is the
			// invariant that matters there.
			h.AssertContains("list the files")
		}
	}
}
