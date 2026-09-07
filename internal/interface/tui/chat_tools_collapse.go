package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Tool run collapsing: a long-horizon agent turn fires dozens of
// near-identical tool calls, and the flat renderer's wall of
// "✓ bash ..." lines is the #4 reading-pain. Consecutive finalized tool
// messages of the same tool within one turn collapse into a single count
// line ("✓ bash (3) · read (5) — 12s"). Safety guards (non-negotiable):
// an error (or any non-final status) never folds, a running tool renders
// on its own line, and runs never merge across turn boundaries.

// toolRunIsCollapsible reports whether a tool message can participate in
// a collapsed run: only finalized success/complete states. Errors,
// pending/approval states, and running tools force their line visible.
func toolRunIsCollapsible(msg ChatMessage) bool {
	if msg.Role != "tool" {
		return false
	}
	return msg.ToolStatus == ToolStatusSuccess || msg.ToolStatus == ToolStatusComplete
}

// ToggleToolsCollapsed flips the tool-run collapse rendering ('t' key in
// normal mode). Collapsed by default: the wall is the pain, not the
// collapse.
func (m *ChatModel) ToggleToolsCollapsed() {
	m.toolsCollapsed = !m.toolsCollapsed
	m.refreshViewportWithFollow(false)
}

// ToolsCollapsed reports the current collapse state (for tests).
func (m ChatModel) ToolsCollapsed() bool {
	return m.toolsCollapsed
}

// renderToolRunAt renders a collapsed run as a codex-style group: one
// header line per display name present in the run ("✓ Shell"), with
// each call indented beneath it as a sub-list row ("$ git status" or
// the call's detail). Timestamps stay on the header; each sub-row
// carries its status glyph; the run's duration is right-aligned on the
// header (goal 0.3.28 Task 4.1/4.5/4.6).
// renderToolRunAt renders the collapsed run for a width budget —
// nested runs live inside the response bubble.
func (m ChatModel) renderToolRunAt(run []ChatMessage, width int) string {
	// Group consecutive sub-list rows under one header per display
	// name, preserving call order: Grep → "Grep" header + its calls,
	// then Read → "Read File" header + its calls.
	type group struct {
		name  string
		start time.Time
		tag   string // short identifier of the first member (Task 3c)
		rows  []string
	}
	var groups []*group
	var current *group
	for _, msg := range run {
		// Normalize through the display-name mapping so every entry
		// path (registry UserFacingName like "ls", ToolExecutingMsg,
		// session reload) classifies under the same group header —
		// `ls` belongs to "Shell" wherever it came from.
		name := getToolDisplayName(msg.ToolName)
		if current == nil || current.name != name {
			current = &group{name: name, start: msg.ToolStartedAt, tag: shortToolTag(msg.ID)}
			if current.start.IsZero() {
				current.start = msg.Timestamp
			}
			groups = append(groups, current)
		}
		glyph := "✓"
		style := ToolDoneStyle
		switch msg.ToolStatus {
		case ToolStatusRunning:
			glyph, style = "→", ToolRunningStyle
		case ToolStatusError:
			glyph, style = "✗", ToolErrorStyle
		}
		rows := m.toolGroupRowsAt(msg, width)
		for _, r := range rows {
			current.rows = append(current.rows, style.Render(glyph)+" "+r)
		}
	}

	first := run[0]
	// Span: first start to last settle. Messages carry per-call elapsed
	// times when the live path filled them; tests (and legacy data) only
	// set timestamps, so fall back to the timestamp span.
	start := first.ToolStartedAt
	if start.IsZero() {
		start = first.Timestamp
	}
	end := run[len(run)-1].Timestamp
	if last := run[len(run)-1]; !last.ToolStartedAt.IsZero() && last.ToolElapsed > 0 {
		end = last.ToolStartedAt.Add(last.ToolElapsed)
	}
	span := end.Sub(start)

	var b strings.Builder
	for gi, g := range groups {
		dur := ""
		if gi == len(groups)-1 {
			dur = ToolTimeStyle.Render(formatElapsed(span))
		}
		left := fmt.Sprintf("%s %s %s",
			ToolTimeStyle.Render(g.start.Format("15:04:05")),
			ToolDoneStyle.Render("✓"),
			g.name,
		)
		left = ToolDoneStyle.Render(expandCaret(false)) + " " + left
		if dur != "" {
			// The group header carries the first member's short tag
			// (Task 3c): the pointer to the full record in Logs.
			if g.tag != "" {
				dur = dur + "  " + g.tag
			}
			pad := width - lipgloss.Width(left) - lipgloss.Width(dur) - 2
			if pad < 2 {
				pad = 2
			}
			b.WriteString(left + strings.Repeat(" ", pad) + dur)
		} else {
			b.WriteString(left)
		}
		// Indented sub-list of each call beneath the group header, with
		// a blank line between calls: horizontal breathing room keeps
		// consecutive calls visually separate instead of a dense wall
		// (goal 0.3.29 live feedback on grouping).
		for ri, row := range g.rows {
			b.WriteString("\n")
			if ri > 0 {
				b.WriteString("\n")
			}
			b.WriteString(" " + indentBlock(row))
		}
	}
	return b.String()
}

// toolGroupRows renders the sub-list rows for one tool call within a
// group: codex-style summary rows — shell-like tools show `$ <command>`,
// todo shows its checklist, everything else shows the detail target.
// Rows truncate to the run's width budget (width minus the group
// indent): a sub-row wider than its line wraps inside the bubble and
// shifts every row below it (goal 0.3.29 Task 3b).
func (m ChatModel) toolGroupRowsAt(msg ChatMessage, width int) []string {
	if rows := m.todoChecklistRows(msg); rows != nil {
		return rows
	}
	detail := msg.ToolDetail
	if detail == "" {
		detail = msg.ToolName
	}
	// Reserve for the indent, status glyph and spaces the group
	// renderer prepends.
	budget := width - 6
	if budget < 20 {
		budget = 20
	}
	if len(detail) > budget {
		detail = detail[:budget-1] + "…"
	}
	if msg.ToolName == "bash" || msg.ToolName == "BashTool" {
		return []string{"$ " + detail}
	}
	return []string{detail}
}

// toolGroupRows renders at the model's pane width (single standalone
// records created before the width-aware path existed).
func (m ChatModel) toolGroupRows(msg ChatMessage) []string {
	return m.toolGroupRowsAt(msg, m.width)
}

// todoChecklistRows renders a todo-list tool call as a visible
// checklist (goal 0.3.28 Task 4.4): each todo becomes an indented
// checkbox row — ✓ done, → in-progress, ○ pending. Returns nil when
// the message is not a todo call or its input carries no todos.
func (m ChatModel) todoChecklistRows(msg ChatMessage) []string {
	if msg.ToolName != "todo_write" && msg.ToolName != "todo" && msg.ToolName != "TodoTool" {
		return nil
	}
	if msg.ToolInputJSON == "" {
		return nil
	}
	var input struct {
		Todos []struct {
			Text   string `json:"text"`
			Status string `json:"status"`
		} `json:"todos"`
	}
	if err := json.Unmarshal([]byte(msg.ToolInputJSON), &input); err != nil {
		return nil
	}
	if len(input.Todos) == 0 {
		return nil
	}
	rows := make([]string, 0, len(input.Todos))
	for _, td := range input.Todos {
		if td.Text == "" {
			continue
		}
		switch td.Status {
		case "completed", "done":
			rows = append(rows, ToolDoneStyle.Render("✓")+" "+td.Text)
		case "in_progress", "active":
			rows = append(rows, ToolRunningStyle.Render("→")+" "+td.Text)
		default:
			rows = append(rows, HelpDimStyle.Render("○")+" "+td.Text)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}
