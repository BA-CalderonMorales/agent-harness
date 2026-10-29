package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// The markdown renderer pads its output with outer newlines. Left in,
// those rows painted an empty gutter line above and below every bubble:
// a one-line reply looked like a three-row box. Bubbles carry text rows
// only.
func TestBubblesHaveNoBlankBorderRows(t *testing.T) {
	m := NewChatModel()
	m.width = 100
	m.height = 40

	user := ChatMessage{Role: "user", Content: "hello there", Timestamp: time.Now()}
	assistant := ChatMessage{Role: "assistant", Content: "Hello back.", Timestamp: time.Now()}
	live := ChatMessage{Role: "assistant", Content: "streaming words", Timestamp: time.Now(), Thinking: true}

	assistantRendered, _ := m.renderAssistantTracked(assistant, m.width)
	liveRendered, _ := m.renderAssistantTracked(live, m.width)
	for name, rendered := range map[string]string{
		"user":      m.renderUserMessage(user),
		"assistant": assistantRendered,
		"live":      liveRendered,
	} {
		for _, line := range strings.Split(ansi.Strip(rendered), "\n") {
			if strings.TrimSpace(line) == "│" {
				t.Fatalf("%s bubble has a blank border row:\n%s", name, ansi.Strip(rendered))
			}
		}
	}
}

// A live turn before the first token is an Agent header, not an empty
// bubble: the gutter appears only once there is text to sit beside.
func TestLivePlaceholderShowsHeaderWithoutABubble(t *testing.T) {
	m := NewChatModel()
	m.width = 100
	m.height = 40
	m.resize(100, 40)
	model, _ := m.Update(AgentStartMsg{Timestamp: time.Now()})
	m = model.(ChatModel)
	m.elapsed = 300 * time.Millisecond

	placeholder := m.streamingAssistant()
	if placeholder == nil {
		t.Fatal("no live assistant placeholder")
	}
	rendered, _ := m.renderAssistantTracked(*placeholder, m.width)
	rendered = ansi.Strip(rendered)
	if !strings.Contains(rendered, "Agent") {
		t.Fatalf("placeholder missing the Agent header:\n%s", rendered)
	}
	if strings.Contains(rendered, "│") {
		t.Fatalf("waiting placeholder painted a bubble gutter:\n%s", rendered)
	}
}
