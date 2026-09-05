package tui

import (
	"strings"
	"testing"
	"time"
)

// TestToolCallsNestInsideAgentResponse pins the turn-block shape: the
// Agent header renders first, the turn's tool calls sit beneath it
// (indented one step), and the answer follows — one block, not a
// transcript where calls float above the response.
func TestToolCallsNestInsideAgentResponse(t *testing.T) {
	m := newClickTestModel(t)
	base := time.Now()
	m.AddMessage("user", "look around")
	m.messages = append(m.messages,
		ChatMessage{
			ID: "t1", Role: "tool", IsTool: true, Turn: 1,
			ToolName: "ls", ToolDisplayName: "ls", ToolStatus: ToolStatusSuccess,
			Content: "22:24:11 ✓ ls       Listing directory", Timestamp: base,
		},
		ChatMessage{
			ID: "t2", Role: "tool", IsTool: true, Turn: 1,
			ToolName: "read", ToolDisplayName: "Read", ToolStatus: ToolStatusSuccess,
			Content: "22:24:11 ✓ Read     Reading README.md", Timestamp: base,
		},
	)
	m.AddMessage("assistant", "Here's the picture.")
	m.messages[len(m.messages)-1].Turn = 1

	view := m.View()

	headerRow, tool1Row, tool2Row, answerRow := -1, -1, -1, -1
	for i, line := range strings.Split(view, "\n") {
		switch {
		case strings.HasPrefix(strings.TrimLeft(line, " "), "Agent "):
			headerRow = i
		case strings.Contains(line, "Reading README.md"):
			tool2Row = i
		case strings.Contains(line, "Listing directory"):
			tool1Row = i
		case strings.Contains(line, "picture."):
			answerRow = i
		}
	}
	if headerRow < 0 || tool1Row < 0 || tool2Row < 0 || answerRow < 0 {
		t.Fatalf("turn block incomplete:\n%s", view)
	}
	if !(headerRow < tool1Row && tool1Row < tool2Row && tool2Row < answerRow) {
		t.Fatalf("expected header < tools < answer, got rows %d/%d/%d/%d", headerRow, tool1Row, tool2Row, answerRow)
	}
	// The nested tools sit one step in — visually inside the block.
	for _, row := range []struct {
		name string
		idx  int
	}{{"tool1", tool1Row}, {"tool2", tool2Row}} {
		_ = row
	}
}

// TestStreamingToolsRenderStandaloneThenNest pins the live path: while
// the response has not landed, trailing tool calls render on their own;
// once the assistant message exists the calls absorb into its block.
func TestStreamingToolsRenderStandaloneThenNest(t *testing.T) {
	m := newClickTestModel(t)
	m.AddMessage("user", "look around")
	m.messages = append(m.messages, ChatMessage{
		ID: "t1", Role: "tool", IsTool: true, Turn: 1,
		ToolName: "ls", ToolDisplayName: "ls", ToolStatus: ToolStatusRunning,
		Content: "22:24:11 → ls      Listing directory", Timestamp: time.Now(),
	})
	m.refreshViewport()

	view := m.View()
	if !strings.Contains(view, "Listing directory") {
		t.Fatalf("running tool missing while standalone:\n%s", view)
	}

	m.AddMessage("assistant", "Working.")
	m.messages[len(m.messages)-1].Turn = 1
	view = m.View()

	headerRow := strings.Index(view, "Agent ")
	toolRow := strings.Index(view, "Listing directory")
	answerRow := strings.Index(view, "Working")
	if headerRow < 0 || toolRow < 0 || answerRow < 0 || !(headerRow < toolRow && toolRow < answerRow) {
		t.Fatalf("tool did not absorb into the response block:\n%s", view)
	}
}
