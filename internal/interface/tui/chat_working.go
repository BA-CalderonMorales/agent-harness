package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The agent working indicator: a status line above the composer rule
// that states what the agent is doing while a turn runs — Codex-parity
// for the "is it doing anything?" confusion (0.3.28 dogfood finding).
// Renders in the chat chrome, never in the transcript: the transcript
// is the record, this is the live weather report.
//
// States:
//   thinking — turn in flight, no tool running (animated word + elapsed)
//   working  — tool call(s) in flight (class, count, elapsed)
//   idle     — nothing; the line is hidden entirely

// workingVerbs is the word the status line animates in place. The word
// never changes — letters shimmer under it instead — so the line reads
// as one stable state, not a ticker (live feedback on 0.3.29 develop:
// the rotating quips read as content, not progress).
var workingVerbs = []string{"working"}

// workingWord returns the (single) verb for the status line.
func workingWord() string { return workingVerbs[0] }

// workingBrightIndex returns which letter of the word is highlighted on
// the given tick — the sweep position of the in-place shimmer. Exposed
// for tests: rendered color is a TTY concern (lipgloss strips ANSI in
// non-TTY contexts), the sweep order is the behavior.
func workingBrightIndex(tick, wordLen int) int {
	if wordLen <= 0 {
		return 0
	}
	return tick % wordLen
}

// workingAnim renders the in-place shimmer for the word: one letter at
// a time brightens, sweeping through the word on the tick clock. The
// elapsed clock is the real progress signal; the shimmer just proves
// the frame is live.
func workingAnim(word string, tick int) string {
	bright := workingBrightIndex(tick, len([]rune(word)))
	var b strings.Builder
	for i, r := range word {
		if i == bright {
			b.WriteString(InfoStyle.Render(string(r)))
		} else {
			b.WriteString(HelpDimStyle.Render(string(r)))
		}
	}
	return b.String()
}

// workingStatusLine renders the agent status for the current model
// state. Empty string means idle — the line is hidden and takes no
// vertical space (the caller collapses it, so composer geometry is
// unchanged when nothing runs).
//
// Derived from existing state only: thinking/streaming (turn in
// flight), currentToolMsg (a tool is executing), completedToolMsgs +
// turnTools (how many calls this turn has made). No new fields, no new
// refreshViewport path — the line animates on the existing timer tick
// because View reads it fresh each frame.
func (m ChatModel) workingStatusLine(tick int) string {
	if !m.thinking && !m.streaming {
		return ""
	}
	elapsed := formatElapsed(m.elapsed)
	if m.currentToolMsg != nil {
		// Working: tool call(s) in flight. Count every call this turn
		// has made (completed + the one running) so the line answers
		// "how far along is it".
		n := len(m.completedToolMsgs)
		if n == 0 {
			n = len(m.turnTools)
		}
		if n == 0 {
			n = 1
		}
		class := m.currentToolMsg.ToolDisplayName
		if class == "" {
			class = m.currentToolMsg.ToolName
		}
		return fmt.Sprintf("%s %s · %s ×%d · %s",
			SuccessStyle.Render("✻"),
			workingAnim(workingWord(), tick),
			class, n, elapsed)
	}
	// Thinking: no tool running. The animated word + elapsed clock.
	return fmt.Sprintf("%s %s · %s",
		SuccessStyle.Render("✻"),
		workingAnim(workingWord(), tick),
		elapsed)
}

// workingStatusHeight reports how many rows the status line occupies in
// the current state (0 when idle) so View can budget vertical space
// without a hidden-but-reserved row.
func (m ChatModel) workingStatusHeight() int {
	if m.workingStatusLine(0) == "" {
		return 0
	}
	return 1
}

// renderWorkingStatus renders the status line pinned to the pane width
// (left-aligned, dim) for placement above the composer rule. Empty when
// idle.
func (m ChatModel) renderWorkingStatus(tick int, width int) string {
	line := m.workingStatusLine(tick)
	if line == "" {
		return ""
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(line)
}
