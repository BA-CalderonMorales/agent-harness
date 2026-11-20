package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
)

// The journey: the agent asks before running a command. The user reads
// the dialog, approves one command and rejects the next. While a dialog
// is open the keyboard belongs to it: nothing the user types lands in
// the chat composer behind it. The dialog fits a narrow pane and a
// normal one.
func TestJourneyApprovalApproveRejectAndKeyboardOwnership(t *testing.T) {
	for _, size := range []struct{ width, height int }{{40, 15}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			journeyHome(t)
			journeyLocal(t)
			app := bootJourneyApp(t)
			screen := newJourneyScreen(t, wireJourneyTUI(t, app), size.width, size.height)

			// Into the chat, composer focused. Typing reaches the
			// composer here, which is what makes the later check mean
			// something.
			screen.Keys("2")
			screen.Keys("i")
			screen.Keys("draft")
			if got := screen.tuiApp.GetInput(); got != "draft" {
				t.Fatalf("composer = %q before any dialog, want the typed draft", got)
			}
			screen.tuiApp.ClearInput()

			first := askApproval(app, "go test ./...")
			screen.waitFor(t, "Approval required")
			screen.AssertFitsWidth(size.width)
			screen.AssertContains("Approve", "Reject")

			screen.Keys("xyz")
			screen.Keys("a")
			if got := first.wait(t); got != approval.DecisionApprove {
				t.Fatalf("first decision = %v, want approve", got)
			}
			screen.Pump()
			screen.AssertNotContains("Approval required")

			second := askApproval(app, "rm -rf build")
			screen.waitFor(t, "Approval required")
			screen.AssertFitsWidth(size.width)
			screen.Keys("xyz")
			screen.Keys("r")
			if got := second.wait(t); got != approval.DecisionReject {
				t.Fatalf("second decision = %v, want reject", got)
			}
			screen.Pump()
			screen.AssertNotContains("Approval required")

			if got := screen.tuiApp.GetInput(); got != "" {
				t.Fatalf("keys typed into the approval dialog reached the composer: %q", got)
			}
			screen.AssertFitsWidth(size.width)
		})
	}
}

// pendingApproval is an approval request the app is blocked on.
type pendingApproval chan approval.Decision

// askApproval runs the app's own approval request on a goroutine, the
// way a tool call does mid-turn: it sends the request to the TUI and
// blocks until the user decides.
func askApproval(app *App, command string) pendingApproval {
	out := make(pendingApproval, 1)
	go func() {
		decision, _, err := app.requestCommandApproval("bash", command, map[string]any{"command": command})
		if err != nil {
			decision = approval.DecisionPending
		}
		out <- decision
	}()
	return out
}

func (p pendingApproval) wait(t *testing.T) approval.Decision {
	t.Helper()
	select {
	case decision := <-p:
		return decision
	case <-time.After(2 * time.Second):
		t.Fatal("approval request never resolved")
		return approval.DecisionPending
	}
}
