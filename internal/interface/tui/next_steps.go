// Suggested next steps.
//
// After a turn, the hardest question is usually "what now?" — the agent
// stopped, the transcript is long, and the logical next move is buried in
// what just happened. This proposes that move: a short list of actions
// derived from the turn that just ran, most relevant first.
//
// Derivation is deterministic on purpose. The signals are things the user
// just watched happen — files were edited, a tool failed, plan items are
// still open — so a suggestion can be traced back to its reason, and it
// costs no extra round trip. The reason is shown next to each action, not
// hidden behind it: a suggestion the reader cannot justify is noise.

package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// NextStep is one suggested action: the message it submits, and the
// observable reason it is being suggested.
type NextStep struct {
	Prompt string
	Why    string
}

// mutateTools are the tool calls that changed the workspace. bash is
// deliberately absent: a shell call may have been a test run, a read, or an
// edit, and guessing wrong would put "review the change" in front of
// someone who only ran grep.
var mutateTools = map[string]bool{
	"edit":          true,
	"write":         true,
	"notebook_edit": true,
}

// readTools are the tool calls that only looked at the workspace.
var readTools = map[string]bool{
	"read": true, "grep": true, "glob": true, "ls": true,
	"ls_recursive": true, "find": true, "search_transcript": true,
}

// deriveNextSteps proposes what to act on next, given the most recent turn.
func (m ChatModel) deriveNextSteps() []NextStep {
	lastTurn := 0
	for _, msg := range m.messages {
		if msg.Turn > lastTurn {
			lastTurn = msg.Turn
		}
	}
	if lastTurn == 0 {
		return starterNextSteps()
	}

	var edited, failed, looked bool
	openTodos := 0
	for _, msg := range m.messages {
		if msg.Turn != lastTurn || !msg.IsTool {
			continue
		}
		if msg.ToolStatus == ToolStatusError {
			failed = true
		}
		if mutateTools[msg.ToolName] {
			edited = true
		}
		if readTools[msg.ToolName] {
			looked = true
		}
		if msg.ToolName == "todo_write" {
			openTodos += openTodoCount(msg.ToolInputJSON)
		}
	}

	if openTodos > 0 {
		// The plan is the most specific statement of what is left, so it
		// leads. The count is on an item that still needs doing.
		return dedupeNextSteps([]NextStep{
			{Prompt: "continue with the next step", Why: fmt.Sprintf("%s still open in the plan", pluralizeItems(openTodos))},
			{Prompt: "/diff", Why: "review what has changed so far"},
			{Prompt: "summarize where this stands and what is left", Why: "re-orient before continuing"},
		})
	}

	if failed {
		return dedupeNextSteps([]NextStep{
			{Prompt: "the last command failed - explain why and fix it", Why: "a tool errored in the last turn"},
			{Prompt: "show the recent diagnostics for that failure", Why: "check the Logs tab for the detail"},
			{Prompt: "/diff", Why: "see what state the workspace is in"},
		})
	}

	if edited {
		return dedupeNextSteps([]NextStep{
			{Prompt: "/diff", Why: "review the change before it drifts"},
			{Prompt: "run the tests that cover what you just changed", Why: "verify the change before moving on"},
			{Prompt: "commit these changes with a clear message", Why: "the work is done; make it durable"},
		})
	}

	if looked {
		return dedupeNextSteps([]NextStep{
			{Prompt: "go ahead and make that change", Why: "the investigation is done - act on it"},
			{Prompt: "plan this out with todos first", Why: "more than one step, so make the plan explicit"},
			{Prompt: "/diff", Why: "check whether the workspace already has changes"},
		})
	}

	return dedupeNextSteps([]NextStep{
		{Prompt: "/diff", Why: "see what is uncommitted"},
		{Prompt: "summarize where this stands", Why: "re-orient on the current state"},
		{Prompt: "/help", Why: "the full command list"},
	})
}

// starterNextSteps is what to offer before any work has happened - the
// first thing to do, not a guess at the user's goal.
func starterNextSteps() []NextStep {
	return []NextStep{
		{Prompt: "/status", Why: "check the provider, model, and permissions"},
		{Prompt: "/diff", Why: "see what is uncommitted in this workspace"},
		{Prompt: "/help", Why: "the full command list"},
	}
}

// openTodoCount counts plan items that still need doing.
func openTodoCount(inputJSON string) int {
	if strings.TrimSpace(inputJSON) == "" {
		return 0
	}
	var input struct {
		Todos []struct {
			Status string `json:"status"`
		} `json:"todos"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return 0
	}
	open := 0
	for _, td := range input.Todos {
		if td.Status != "done" && td.Status != "completed" && td.Status != "cancelled" {
			open++
		}
	}
	return open
}

func pluralizeItems(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// dedupeNextSteps drops repeated prompts, keeping order and the first
// reason offered for each.
func dedupeNextSteps(steps []NextStep) []NextStep {
	seen := make(map[string]bool, len(steps))
	out := make([]NextStep, 0, len(steps))
	for _, step := range steps {
		if seen[step.Prompt] {
			continue
		}
		seen[step.Prompt] = true
		out = append(out, step)
	}
	return out
}

// ---------------------------------------------------------------------------
// The picker
// ---------------------------------------------------------------------------

// NextStepsModel is the overlay that presents the suggestions. It wears the
// shared modal frame, so it scrolls and pads like every other overlay.
type NextStepsModel struct {
	visible bool
	width   int
	height  int
	steps   []NextStep
	cursor  int
}

// NewNextSteps creates the picker.
func NewNextSteps() NextStepsModel {
	return NextStepsModel{}
}

// Open populates and shows the picker.
func (m *NextStepsModel) Open(width, height int, steps []NextStep) {
	m.width = width
	m.height = height
	m.steps = steps
	m.cursor = 0
	m.visible = true
}

// Close hides the picker.
func (m *NextStepsModel) Close() { m.visible = false }

// IsShowing reports whether the picker is visible.
func (m NextStepsModel) IsShowing() bool { return m.visible }

// Update handles a key. It returns the chosen step once, then closes.
func (m *NextStepsModel) Update(msg tea.Msg) (chosen *NextStep, closed bool) {
	if !m.visible {
		return nil, false
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	switch key.String() {
	case "esc", "q":
		m.visible = false
		return nil, true
	case "j", "down", "tab":
		// Wrapping navigation, like the rest of the modal family: the
		// cursor never dead-ends at an edge.
		m.cursor = (m.cursor + 1) % len(m.steps)
		return nil, false
	case "k", "up":
		if m.cursor == 0 {
			m.cursor = len(m.steps) - 1
		} else {
			m.cursor--
		}
		return nil, false
	case "enter", " ":
		return m.selectCurrent()
	}

	// 1-9 pick an entry outright: a three-item list should not need two
	// keystrokes.
	if n := len(key.String()); n == 1 && key.String()[0] >= '1' && key.String()[0] <= '9' {
		if idx := int(key.String()[0] - '1'); idx < len(m.steps) {
			m.cursor = idx
			return m.selectCurrent()
		}
	}
	return nil, false
}

func (m *NextStepsModel) selectCurrent() (*NextStep, bool) {
	if m.cursor < 0 || m.cursor >= len(m.steps) {
		return nil, false
	}
	step := m.steps[m.cursor]
	m.visible = false
	return &step, true
}

// View renders the picker through the shared modal frame.
func (m NextStepsModel) View() string {
	if !m.visible {
		return ""
	}

	items := make([]modalItem, 0, len(m.steps))
	for i, step := range m.steps {
		marker, style := IndicatorUnselected, HelpDimStyle
		if i == m.cursor {
			marker, style = IndicatorSelected, InfoStyle
		}
		items = append(items, modalItem{lines: []string{
			style.Render(fmt.Sprintf("%s%d. %s", marker, i+1, step.Prompt)),
			HelpDimStyle.Render("     " + step.Why),
		}})
	}
	if len(items) == 0 {
		items = append(items, modalItem{lines: []string{HelpDimStyle.Render("Nothing to suggest yet.")}})
	}

	return renderModal(m.width, m.height, modalSpec{
		title:          "Suggested next steps",
		hint:           "Derived from the turn you just ran. Enter runs the selected one.",
		items:          items,
		footer:         "j/k: navigate  Enter: run  Esc: cancel",
		preferredWidth: modalMaxWidth,
		cursor:         m.cursor,
	})
}
