package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The loop track.
//
// A turn has four beats, and the user should be able to see which one
// they are in: Ask (the model is working out what it needs), Plan (it
// wrote a plan), Review (it is waiting for the human), and Try (it is
// executing).
//
// The track is derived from state the app already has — thinking,
// streaming, the running tool, the tool calls this turn, and the
// approval dialog. Nothing here introduces a second model of the turn;
// the track is a reading of what is already true.

// The beats of a turn.
const (
	beatAsk    = "ask"
	beatPlan   = "plan"
	beatReview = "review"
	beatTry    = "try"
)

// loopBeats is the order the beats happen in.
var loopBeats = []string{beatAsk, beatPlan, beatReview, beatTry}

// beatLabel is the word shown for a beat.
func beatLabel(beat string) string {
	switch beat {
	case beatAsk:
		return "Ask"
	case beatPlan:
		return "Plan"
	case beatReview:
		return "Review"
	case beatTry:
		return "Try"
	default:
		return ""
	}
}

// currentLoopBeat reads the beat from the live state. An empty string
// means no turn is in flight.
func (a App) currentLoopBeat() string {
	chat := a.chatModel
	if !chat.thinking && !chat.streaming {
		return ""
	}
	// The human is the beat: while an approval is open, nothing else
	// matters.
	if a.approvalDialog.IsVisible() {
		return beatReview
	}
	hasPlan := chat.turnHasPlan()
	if hasPlan && chat.currentTool != nil {
		return beatTry
	}
	if hasPlan {
		return beatPlan
	}
	return beatAsk
}

// turnHasPlan reports whether this turn wrote a plan. The plan tools
// are the ones the agent uses to lay out steps before it changes
// anything.
func (m ChatModel) turnHasPlan() bool {
	for _, msg := range m.completedToolMsgs {
		switch msg.ToolName {
		case "todo_write", "plan", "TodoWriteTool", "PlanTool":
			return true
		}
	}
	return false
}

// loopTrackLine renders the beat track: every beat, with the current one
// marked. On a narrow pane the track collapses to the current beat, so
// the line never wraps.
func loopTrackLine(beat string, width int) string {
	if beat == "" {
		return ""
	}
	label := beatLabel(beat)
	if label == "" {
		return ""
	}
	if width < 34 {
		return ModePromptStyle.Render("▸ " + label)
	}
	parts := make([]string, 0, len(loopBeats))
	for _, candidate := range loopBeats {
		candidateLabel := beatLabel(candidate)
		if candidate == beat {
			parts = append(parts, ModePromptStyle.Render("▸ "+candidateLabel))
			continue
		}
		parts = append(parts, HelpDimStyle.Render(candidateLabel))
	}
	track := strings.Join(parts, HelpDimStyle.Render(" · "))
	if lipgloss.Width(track) > width {
		return ModePromptStyle.Render("▸ " + label)
	}
	if beat == beatReview {
		// The human is the beat: say so, so a waiting turn never reads
		// as a stalled one.
		track += HelpDimStyle.Render(" · waiting for you")
	}
	return track
}
