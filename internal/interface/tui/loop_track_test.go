package tui

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
)

// toolUseBlockForTest is a running tool call.
func toolUseBlockForTest() ToolUseBlock {
	return ToolUseBlock{ID: "t1", Name: "bash"}
}

// The beat track reads live state. These tests pin the reading, so the
// track can never claim the turn is further along than it is.

func TestCurrentLoopBeatReadsTheLiveState(t *testing.T) {
	t.Run("no turn means no beat", func(t *testing.T) {
		app := NewApp()
		if beat := app.currentLoopBeat(); beat != "" {
			t.Fatalf("beat = %q, want none when no turn is running", beat)
		}
	})

	t.Run("thinking is ask", func(t *testing.T) {
		app := NewApp()
		app.chatModel.thinking = true
		if beat := app.currentLoopBeat(); beat != beatAsk {
			t.Fatalf("beat = %q, want ask", beat)
		}
	})

	t.Run("a written plan is plan", func(t *testing.T) {
		app := NewApp()
		app.chatModel.thinking = true
		app.chatModel.completedToolMsgs = []ChatMessage{{ToolName: "todo_write"}}
		if beat := app.currentLoopBeat(); beat != beatPlan {
			t.Fatalf("beat = %q, want plan", beat)
		}
	})

	t.Run("an open approval is review", func(t *testing.T) {
		app := NewApp()
		app.chatModel.thinking = true
		app.chatModel.completedToolMsgs = []ChatMessage{{ToolName: "todo_write"}}
		app.approvalDialog.Show(approval.NewApprovalRequest(approval.CommandInfo{ID: "1", ToolName: "bash"}))
		if beat := app.currentLoopBeat(); beat != beatReview {
			t.Fatalf("beat = %q, want review", beat)
		}
	})

	t.Run("running tools after a plan is try", func(t *testing.T) {
		app := NewApp()
		app.chatModel.thinking = true
		app.chatModel.completedToolMsgs = []ChatMessage{{ToolName: "todo_write"}}
		block := toolUseBlockForTest()
		app.chatModel.currentTool = &block
		if beat := app.currentLoopBeat(); beat != beatTry {
			t.Fatalf("beat = %q, want try", beat)
		}
	})

	t.Run("tools before a plan are still ask", func(t *testing.T) {
		app := NewApp()
		app.chatModel.thinking = true
		block := toolUseBlockForTest()
		app.chatModel.currentTool = &block
		if beat := app.currentLoopBeat(); beat != beatAsk {
			t.Fatalf("beat = %q, want ask while the model is still working it out", beat)
		}
	})
}

func TestLoopTrackLineShowsEveryBeat(t *testing.T) {
	line := loopTrackLine(beatPlan, 80)
	for _, want := range []string{"Ask", "Plan", "Review", "Try"} {
		if !strings.Contains(line, want) {
			t.Fatalf("track %q is missing %q", line, want)
		}
	}
	if !strings.Contains(line, "▸ Plan") {
		t.Fatalf("track %q does not mark the current beat", line)
	}
}

func TestLoopTrackLineCollapsesOnANarrowPane(t *testing.T) {
	line := loopTrackLine(beatReview, 24)
	if strings.Contains(line, "Ask") {
		t.Fatalf("track %q should collapse to the current beat on a narrow pane", line)
	}
	if !strings.Contains(line, "Review") {
		t.Fatalf("track %q is missing the current beat", line)
	}
}

func TestReviewBeatSaysItWaitsForYou(t *testing.T) {
	line := loopTrackLine(beatReview, 80)
	if !strings.Contains(line, "waiting for you") {
		t.Fatalf("track %q does not say the turn is waiting for the human", line)
	}
}

func TestLoopTrackIsEmptyWithoutABeat(t *testing.T) {
	if line := loopTrackLine("", 80); line != "" {
		t.Fatalf("track = %q, want nothing without a beat", line)
	}
}

func TestWorkingLineShowsTheTrack(t *testing.T) {
	var m ChatModel
	m.width = 80
	m.thinking = true
	m.loopBeat = beatReview

	line := m.workingStatusLine(0)
	if !strings.Contains(line, "Review") {
		t.Fatalf("working line = %q, want the beat track", line)
	}
}
