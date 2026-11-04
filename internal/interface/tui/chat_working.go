package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The agent working indicator: a status line that states what the agent
// is doing while a turn runs — Codex-parity for the "is it doing
// anything?" confusion (0.3.28 dogfood finding). The live phase rides
// on the Agent header, at the bubble the user is waiting for; the chrome
// band above the composer carries only the turn's beat track.
//
// States:
//   thinking  — turn in flight, no prose yet (animated spinner + elapsed)
//   writing   — assistant prose is arriving (the bubble grows above)
//   running   — a tool call is in flight (the row shows the detail)
//   review    — an approval is open and the turn waits for the human
//   idle      — nothing; the line is hidden entirely

// workingFrames is the live turn's spinner. It exists to prove the
// frame is moving while the elapsed clock proves the turn is
// progressing. Eight frames at the 250ms tick is a calm two-second
// wheel, the same glyph family the rest of the TUI uses.
var workingFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// workingSpinner renders the wheel. Reduced motion keeps the state but
// parks the wheel on one frame.
func workingSpinner(tick int, reducedMotion bool) string {
	if reducedMotion {
		return InfoStyle.Render("•")
	}
	return InfoStyle.Render(workingFrames[tick%len(workingFrames)])
}

// workingPhaseLabel names what the turn is doing right now. The
// transcript carries the detail (the command, the file, the answer
// arriving); this line only has to say which phase the agent is in.
//
// Derived from existing state only: the running tool row, whether text
// deltas are flowing, the reasoning preview, and the approval beat the
// app already computes. No second model of the turn.
func (m ChatModel) workingPhaseLabel() string {
	switch {
	case m.loopBeat == beatReview:
		return "Waiting for approval"
	case m.currentToolMsg != nil && m.currentToolMsg.ToolStatus == ToolStatusRunning:
		if name := strings.TrimSpace(m.currentToolMsg.ToolDisplayName); name != "" {
			return "Running " + name
		}
		return "Running tool"
	case m.writing && !m.placeholderPending:
		// Buffered deltas are not visible prose yet: the header still
		// says Thinking until the bubble actually shows the text.
		return "Writing"
	case strings.HasPrefix(m.thinkingText, "Connecting"):
		return "Connecting"
	default:
		return "Thinking"
	}
}

// liveHeaderStatus renders the live turn's phase for the Agent header:
// the spinner, what the turn is doing, and how long it has run. Waiting
// for the human is not work — the review beat says so without a wheel.
func (m ChatModel) liveHeaderStatus(tick int) string {
	label := HelpDimStyle.Render(m.workingPhaseLabel())
	if m.loopBeat == beatReview {
		return label
	}
	phase := workingSpinner(tick, m.reducedMotion) + " " + label
	// A delegation is a flight between places: the sub-agent left, and
	// the user should see that rather than guess from a spinner.
	if m.delegationInFlight() {
		phase += HelpDimStyle.Render(" · ") + ModePromptStyle.Render("✈ delegating")
	}
	return phase + HelpDimStyle.Render(" · ") + ToolTimeStyle.Render(formatElapsed(m.elapsed))
}

// workingStatusLine renders the chrome band above the composer. It
// carries only the turn's beat track: the live spinner and the clock
// moved to the Agent header, where the response is being cooked.
// Empty string means idle — the band is hidden and takes no vertical
// space (the caller collapses it, so composer geometry is unchanged
// when nothing runs).
func (m ChatModel) workingStatusLine(_ int) string {
	if !m.loopTrack || (!m.thinking && !m.streaming) {
		return ""
	}
	return loopTrackLine(m.loopBeat, m.width)
}

// workingStatusHeight reports how many rows the beat band occupies in
// the current state (0 when idle) so View can budget vertical space
// without a hidden-but-reserved row. Live, it is a single track row.
func (m ChatModel) workingStatusHeight() int {
	if m.workingStatusLine(0) == "" {
		return 0
	}
	return 1
}

// renderWorkingStatus renders the status line pinned to the pane width
// (left-aligned, dim) for placement above the composer rule. The caller
// wraps it in blank rows so the line reads as its own band between the
// transcript and the composer instead of being pressed against both.
// Empty when idle.
func (m ChatModel) renderWorkingStatus(tick int, width int) string {
	line := m.workingStatusLine(tick)
	if line == "" {
		return ""
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(line)
}
