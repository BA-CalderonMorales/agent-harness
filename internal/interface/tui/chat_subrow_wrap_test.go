package tui

import (
	"strings"
	"testing"
	"time"
)

// visualWidth strips ANSI and counts visible columns.
func visualWidth(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

// TestToolRowNestedWidth pins goal 0.3.29 Task 3b: a tool row rendered
// inside the turn bubble truncates to the bubble's inner budget — never
// the pane width baked in at record creation. Before the fix the row
// was composed at m.width and shipped over-wide inside the bubble,
// wrapping its duration onto a second line.
func TestToolRowNestedWidth(t *testing.T) {
	for _, pane := range []int{100, 120, 160} {
		pane := pane
		t.Run(itoa(pane), func(t *testing.T) {
			m := NewChatModel()
			m.width = pane
			m.viewport.Width = pane
			m.height = 40
			model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
			m = model.(ChatModel)
			long := strings.Repeat("x", 200) + " | tail -1"
			model, _ = m.Update(AgentToolStartMsg{ToolID: "t0", ToolName: "bash", DisplayName: "bash", Input: map[string]any{"command": long}})
			m = model.(ChatModel)
			model, _ = m.Update(AgentToolDoneMsg{ToolID: "t0", Success: true})
			m = model.(ChatModel)
			model, _ = m.Update(AgentChunkMsg{Text: "work done", Timestamp: time.Now()})
			m = model.(ChatModel)
			model, _ = m.Update(AgentDoneMsg{FullResponse: "", Timestamp: time.Now()})
			m = model.(ChatModel)

			// The turn block nests rows at innerWidth = pane - 8.
			inner := pane - 8
			for i := range m.messages {
				if m.messages[i].IsTool {
					row, _ := m.renderCollapsedMessageAt(m.messages, i, true, inner)
					if w := visualWidth(row); w > inner {
						t.Fatalf("pane=%d: nested tool row width %d exceeds bubble budget %d", pane, w, inner)
					}
				}
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
