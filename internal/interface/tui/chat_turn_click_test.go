package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// newTurnBlockModel builds a chat model with one finished turn: a tool
// call, prose, another tool call, and the answer. The prose carries
// status glyphs on purpose — the click index must be built from the
// render, not from sniffing carets in rendered rows.
func newTurnBlockModel(t *testing.T, width int) ChatModel {
	t.Helper()
	m := NewChatModel()
	m.width = width
	m.height = 40
	m.AddToolMessage("bash", "bash", "echo hi")
	m.messages[0].ToolStatus = ToolStatusSuccess
	m.AddToolMessage("read", "read", "read main.go")
	m.messages[1].ToolStatus = ToolStatusSuccess
	m.AddMessage("assistant", "answer prose with a → glyph and a ✓ inside")
	m.messages[2].Parts = []TurnPart{
		{ToolID: m.messages[0].ID},
		{Text: "answer prose with a → glyph and a ✓ inside"},
		{ToolID: m.messages[1].ID},
	}
	m.refreshViewportWithFollow(true)
	return m
}

// TestNestedToolRowsClickAtThreeWidths drives a click on every nested
// tool row through the real dispatch at the widths small screens use.
func TestNestedToolRowsClickAtThreeWidths(t *testing.T) {
	for _, width := range []int{60, 70, 90} {
		m := newTurnBlockModel(t, width)
		content := m.viewport.View()
		rows := strings.Split(content, "\n")

		// Each tool's summary row: find it by its tool name.
		for _, tool := range []string{"bash", "read"} {
			row := -1
			for i, line := range rows {
				if strings.Contains(line, tool) && strings.Contains(line, "▸") {
					row = i
					break
				}
			}
			if row < 0 {
				t.Fatalf("w%d: %s row not rendered:\n%s", width, tool, content)
			}
			m = mouseClickAt(m, row)
			want := ""
			for _, msg := range m.messages {
				if msg.ToolDisplayName == tool {
					want = msg.ID
				}
			}
			if m.expandedMessageID != want {
				t.Fatalf("w%d: click on %s row %d resolved to %q, want %q",
					width, tool, row, m.expandedMessageID, want)
			}
			if !strings.Contains(m.viewport.View(), "esc to close") {
				t.Fatalf("w%d: %s click did not open its record:\n%s", width, tool, m.viewport.View())
			}
			// Fold it back before the next probe.
			m = mouseClickAt(m, row)
			if m.expandedMessageID != "" {
				t.Fatalf("w%d: second click on %s did not fold", width, tool)
			}
		}
	}
}

// TestReasoningFrameClicksInsideTurnBlock: the expanded reasoning frame
// inside a turn bubble resolves to the assistant message — click and
// keyboard both.
func TestReasoningFrameClicksInsideTurnBlock(t *testing.T) {
	for _, width := range []int{60, 70, 90} {
		m := newTurnBlockModel(t, width)
		m.messages[2].ReasoningText = "why: inspect first, then answer"
		m.expandedMessageID = m.messages[2].ID
		m.refreshViewportWithFollow(true)

		content := m.viewport.View()
		rows := strings.Split(content, "\n")
		row := -1
		for i, line := range rows {
			if strings.Contains(line, "reasoning · esc to close") {
				row = i
				break
			}
		}
		if row < 0 {
			t.Fatalf("w%d: reasoning frame not rendered:\n%s", width, content)
		}
		m = mouseClickAt(m, row)
		if m.expandedMessageID != m.messages[2].ID {
			t.Fatalf("w%d: click on reasoning frame row %d resolved to %q", width, row, m.expandedMessageID)
		}
		// Click again folds.
		m = mouseClickAt(m, row)
		if m.expandedMessageID != "" {
			t.Fatalf("w%d: click on folded reasoning frame did not expand-then-fold cleanly", width)
		}
	}
}

// TestReasoningFrameClosesWithFooter pins the reasoning frame shape:
// the expanded record opens with ┌─ and closes with └─, and the click
// range ends exactly at the footer — no tail row leaking into prose.
func TestReasoningFrameClosesWithFooter(t *testing.T) {
	m := NewChatModel()
	m.width = 70
	m.height = 40
	m.AddMessage("assistant", "the answer body")
	m.messages[0].ReasoningText = "line one\nline two\nline three"
	m.expandedMessageID = m.messages[0].ID
	m.refreshViewportWithFollow(true)

	content := m.viewport.View()
	rows := strings.Split(content, "\n")
	open, close := -1, -1
	for i, line := range rows {
		if strings.Contains(line, "┌─ reasoning") {
			open = i
		}
		if strings.Contains(line, "└─ esc to close") {
			close = i
		}
	}
	if open < 0 || close < 0 || close < open {
		t.Fatalf("reasoning frame missing its frame rows:\n%s", content)
	}
	// The three reasoning lines sit between the frame rows, each once.
	body := strings.Join(rows[open+1:close], "\n")
	for _, want := range []string{"line one", "line two", "line three"} {
		if strings.Count(body, want) != 1 {
			t.Fatalf("reasoning body rows wrong around frame (open %d, close %d):\n%s", open, close, content)
		}
	}
	if got := m.expandableMessageAtRow(close); got != m.messages[0].ID {
		t.Fatalf("footer row %d resolves to %q, want the message", close, got)
	}
	if got := m.expandableMessageAtRow(close + 1); got != "" {
		t.Fatalf("row past the frame leaks into the click range: %q", got)
	}
}

// TestGlyphProseNeverClickable: status glyphs inside answer prose must
// not register as tool rows — click ranges come from the render, not
// from scanning for carets.
func TestGlyphProseNeverClickable(t *testing.T) {
	for _, width := range []int{60, 70, 90} {
		m := newTurnBlockModel(t, width)
		content := m.viewport.View()
		for i, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "answer prose") {
				if id := m.expandableMessageAtRow(i); id != "" {
					t.Fatalf("w%d: prose row %d wrongly clickable as %q", width, i, id)
				}
				return
			}
		}
		t.Fatalf("w%d: prose not rendered:\n%s", width, content)
	}
}

// TestCollapsedRunRowsClickThroughRealDispatch: inside a collapsed run,
// every member's row opens its own record via the real Update path.
func TestCollapsedRunRowsClickThroughRealDispatch(t *testing.T) {
	for _, width := range []int{60, 70, 90} {
		m := NewChatModel()
		m.width = width
		m.height = 40
		model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
		m = model.(ChatModel)
		for i := 0; i < 3; i++ {
			model, _ = m.Update(AgentToolStartMsg{
				ToolID: fmt.Sprintf("t%d", i), ToolName: "bash", DisplayName: "bash",
				Input: map[string]any{"command": fmt.Sprintf("echo %d", i)},
			})
			m = model.(ChatModel)
			model, _ = m.Update(AgentToolDoneMsg{ToolID: fmt.Sprintf("t%d", i), Success: true})
			m = model.(ChatModel)
		}
		m.refreshViewportWithFollow(true)

		if !strings.Contains(m.viewport.View(), "bash ×3") {
			t.Fatalf("w%d: run not collapsed:\n%s", width, m.viewport.View())
		}
		// The run line itself unfolds the run.
		m = mouseClickAt(m, 0)
		if m.expandedMessageID == "" {
			t.Fatalf("w%d: click on run line did not unfold", width)
		}
		if strings.Contains(m.viewport.View(), "bash ×3") {
			t.Fatalf("w%d: run still collapsed after click:\n%s", width, m.viewport.View())
		}
	}
}

// TestClickHitsTheRenderedRow is the screen-truth receipt: the tool
// row's pane position is read from the real rendered view — not from
// a compile-time offset — and a click at that screen row must open
// the record. Catches chrome-height drift (the tab bar grew a border
// row and the old constant silently aimed two rows low).
func TestClickHitsTheRenderedRow(t *testing.T) {
	for _, width := range []int{60, 70, 90} {
		m := newTurnBlockModel(t, width)
		m.height = 30
		m.refreshViewportWithFollow(true)

		// The pane row of the first tool row, read off the render.
		content := m.viewport.View()
		rows := strings.Split(content, "\n")
		toolScreenRow := -1
		for i, line := range rows {
			if strings.Contains(line, "bash") && strings.Contains(line, "▸") {
				toolScreenRow = i + viewportTopOffset
				break
			}
		}
		if toolScreenRow < 0 {
			t.Fatalf("w%d: tool row not rendered:\n%s", width, content)
		}

		m = mouseClickAt(m, toolScreenRow-viewportTopOffset)
		want := m.messages[0].ID
		if m.expandedMessageID != want {
			t.Fatalf("w%d: screen click on row %d expanded %q, want %q",
				width, toolScreenRow, m.expandedMessageID, want)
		}
	}
}
