package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// turnOf builds a chat model whose transcript is one turn made of the given
// tool calls, so derivation can be exercised without a live session.
func turnOf(tools ...ChatMessage) ChatModel {
	m := newEmptyChatTestModel()
	m.messages = nil
	for i, tool := range tools {
		tool.Turn = 1
		tool.IsTool = true
		tool.ID = "t" + string(rune('0'+i))
		m.messages = append(m.messages, tool)
	}
	return m
}

func promptsOf(steps []NextStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Prompt)
	}
	return out
}

func hasPrompt(steps []NextStep, want string) bool {
	for _, s := range steps {
		if s.Prompt == want {
			return true
		}
	}
	return false
}

// TestNextStepsLeadWithAnEditedTurn pins the ordering that makes the list
// useful: after edits, reviewing and verifying come before committing.
func TestNextStepsLeadWithAnEditedTurn(t *testing.T) {
	steps := turnOf(ChatMessage{ToolName: "edit", ToolStatus: ToolStatusSuccess}).deriveNextSteps()
	if len(steps) == 0 {
		t.Fatal("no suggestions for a turn that edited files")
	}
	if steps[0].Prompt != "/diff" {
		t.Fatalf("first suggestion = %q, want the review step first", steps[0].Prompt)
	}
	if !hasPrompt(steps, "commit these changes with a clear message") {
		t.Fatalf("no commit step after edits: %v", promptsOf(steps))
	}
	for _, s := range steps {
		if s.Why == "" {
			t.Fatalf("suggestion %q has no reason; an unjustified suggestion is noise", s.Prompt)
		}
	}
}

// TestNextStepsPrioritizeAnOpenPlan pins that the plan wins: when items are
// still open, the most specific statement of what is left leads.
func TestNextStepsPrioritizeAnOpenPlan(t *testing.T) {
	openPlan := `{"todos":[{"text":"a","status":"done"},{"text":"b","status":"pending"}]}`
	steps := turnOf(
		ChatMessage{ToolName: "todo_write", ToolStatus: ToolStatusSuccess, ToolInputJSON: openPlan},
		ChatMessage{ToolName: "edit", ToolStatus: ToolStatusSuccess},
	).deriveNextSteps()

	if steps[0].Prompt != "continue with the next step" {
		t.Fatalf("first suggestion = %q, want the open plan to lead", steps[0].Prompt)
	}
	if !strings.Contains(steps[0].Why, "1 item") {
		t.Fatalf("reason does not report the open count: %q", steps[0].Why)
	}
}

// TestNextStepsFollowAFailedTool pins that a failure redirects the list to
// diagnosis instead of the happy path.
func TestNextStepsFollowAFailedTool(t *testing.T) {
	steps := turnOf(ChatMessage{ToolName: "bash", ToolStatus: ToolStatusError}).deriveNextSteps()
	if len(steps) == 0 || !strings.Contains(steps[0].Prompt, "failed") {
		t.Fatalf("a failed tool did not lead with a fix: %v", promptsOf(steps))
	}
}

// TestNextStepsOfferActionAfterReadOnlyWork pins the read-only case: the
// investigation is done, so the list pushes toward acting on it.
func TestNextStepsOfferActionAfterReadOnlyWork(t *testing.T) {
	steps := turnOf(
		ChatMessage{ToolName: "read", ToolStatus: ToolStatusSuccess},
		ChatMessage{ToolName: "grep", ToolStatus: ToolStatusSuccess},
	).deriveNextSteps()
	if !hasPrompt(steps, "go ahead and make that change") {
		t.Fatalf("read-only turn did not suggest acting: %v", promptsOf(steps))
	}
}

// TestNextStepsBeforeAnyTurnAreStarters pins that an empty transcript does
// not guess at the user's goal; it offers orientation instead.
func TestNextStepsBeforeAnyTurnAreStarters(t *testing.T) {
	steps := newEmptyChatTestModel().deriveNextSteps()
	if len(steps) == 0 {
		t.Fatal("no suggestions before the first turn")
	}
	if steps[0].Prompt != "/status" {
		t.Fatalf("starter = %q, want an orientation command", steps[0].Prompt)
	}
}

// TestNextStepsHaveNoDuplicates pins that a prompt appears once: a
// suggestion list that repeats itself reads as padding.
func TestNextStepsHaveNoDuplicates(t *testing.T) {
	all := [][]NextStep{
		turnOf(ChatMessage{ToolName: "edit", ToolStatus: ToolStatusSuccess}).deriveNextSteps(),
		turnOf(ChatMessage{ToolName: "bash", ToolStatus: ToolStatusError}).deriveNextSteps(),
		turnOf(ChatMessage{ToolName: "read", ToolStatus: ToolStatusSuccess}).deriveNextSteps(),
		newEmptyChatTestModel().deriveNextSteps(),
	}
	for _, steps := range all {
		seen := map[string]bool{}
		for _, s := range steps {
			if seen[s.Prompt] {
				t.Fatalf("duplicate prompt %q in %v", s.Prompt, promptsOf(steps))
			}
			seen[s.Prompt] = true
		}
	}
}

// TestNextStepsPickerRunsTheSelection pins the interaction: the chosen step
// closes the modal and is handed back once.
func TestNextStepsPickerRunsTheSelection(t *testing.T) {
	p := NewNextSteps()
	p.Open(100, 30, []NextStep{{Prompt: "first"}, {Prompt: "second"}})

	if _, closed := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}); closed {
		t.Fatal("navigation closed the picker")
	}
	chosen, closed := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !closed || chosen == nil {
		t.Fatalf("enter did not select: closed=%v chosen=%v", closed, chosen)
	}
	if chosen.Prompt != "second" {
		t.Fatalf("selected %q, want the cursor's entry", chosen.Prompt)
	}
	if p.IsShowing() {
		t.Fatal("picker stayed open after a selection")
	}
}

// TestNextStepsPickerDigitsPickOutright pins that a three-item list does not
// need two keystrokes for the first item.
func TestNextStepsPickerDigitsPickOutright(t *testing.T) {
	p := NewNextSteps()
	p.Open(100, 30, []NextStep{{Prompt: "first"}, {Prompt: "second"}})

	chosen, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if chosen == nil || chosen.Prompt != "first" {
		t.Fatalf("digit did not pick outright: %v", chosen)
	}
}

// TestNextStepsPickerWrapsAtTheEdges pins the modal family's navigation
// grammar: the cursor never dead-ends at either end of the list.
func TestNextStepsPickerWrapsAtTheEdges(t *testing.T) {
	p := NewNextSteps()
	p.Open(100, 30, []NextStep{{Prompt: "first"}, {Prompt: "second"}})

	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if p.cursor != 1 {
		t.Fatalf("k from the top landed on %d, want the wrap to the last entry", p.cursor)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if p.cursor != 0 {
		t.Fatalf("j from the bottom landed on %d, want the wrap to the first entry", p.cursor)
	}
}

// TestNextStepsPickerFitsShortPanes pins the family invariant for this
// overlay too.
func TestNextStepsPickerFitsShortPanes(t *testing.T) {
	steps := []NextStep{
		{Prompt: "/diff", Why: "review the change before it drifts"},
		{Prompt: "run the tests that cover what you just changed", Why: "verify the change"},
		{Prompt: "commit these changes with a clear message", Why: "make the work durable"},
	}
	for _, pane := range [][2]int{{100, 30}, {100, 14}, {60, 10}, {40, 8}} {
		p := NewNextSteps()
		p.Open(pane[0], pane[1], steps)
		assertFitsPane(t, "nextSteps", p.View(), pane[0], pane[1])
	}
}
