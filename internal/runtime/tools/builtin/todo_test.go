package builtin

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

func todoInput(todos ...map[string]any) map[string]any {
	entries := make([]any, 0, len(todos))
	for _, td := range todos {
		entries = append(entries, td)
	}
	return map[string]any{"todos": entries}
}

// TestTodoWriteRejectsUnknownStatus pins the fix for a plan that lied: an
// unrecognized status used to be accepted and rendered as pending, so a
// step the agent believed was done read as not started. The error must
// name the allowed values.
func TestTodoWriteRejectsUnknownStatus(t *testing.T) {
	input := todoInput(map[string]any{"text": "unify the frame", "status": "finished"})
	result := TodoWriteTool.ValidateInput(input, tools.Context{})
	if result.Valid {
		t.Fatal("unknown status was accepted")
	}
	for _, want := range todoStatuses {
		if !strings.Contains(result.Message, want) {
			t.Errorf("rejection does not list %q: %s", want, result.Message)
		}
	}
}

// TestTodoWriteAcceptsAnInProgressStep pins the state the checklist could
// never show before: the schema forbade in_progress, so the plan had no
// way to say which step was running.
func TestTodoWriteAcceptsAnInProgressStep(t *testing.T) {
	input := todoInput(
		map[string]any{"text": "read the modal code", "status": "done"},
		map[string]any{"text": "unify the frame", "status": "in_progress"},
		map[string]any{"text": "notify reviewers", "status": "cancelled"},
	)
	if result := TodoWriteTool.ValidateInput(input, tools.Context{}); !result.Valid {
		t.Fatalf("valid plan rejected: %s", result.Message)
	}
}

func TestTodoWriteRequiresText(t *testing.T) {
	input := todoInput(map[string]any{"status": "pending"})
	result := TodoWriteTool.ValidateInput(input, tools.Context{})
	if result.Valid {
		t.Fatal("a todo with no text was accepted")
	}
}

// TestTodoWriteEchoesThePlan pins that the model can read back the plan it
// just wrote: the checklist is what the user reads, and a plan the model
// cannot see is one it lets drift.
func TestTodoWriteEchoesThePlan(t *testing.T) {
	result, err := TodoWriteTool.Call(todoInput(
		map[string]any{"text": "read the modal code", "status": "done"},
		map[string]any{"text": "unify the frame", "status": "in_progress"},
	), tools.Context{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := result.Data.(string)
	if !strings.Contains(data, "Updated 2 todos") {
		t.Fatalf("summary does not report the count: %q", data)
	}
	if !strings.Contains(data, "[in_progress] unify the frame") {
		t.Fatalf("summary does not carry each item's state: %q", data)
	}
}

// TestTodoWriteDerivesAMissingID pins that an id the checklist never shows
// is not worth blocking on.
func TestTodoWriteDerivesAMissingID(t *testing.T) {
	if result := TodoWriteTool.ValidateInput(todoInput(map[string]any{"text": "one step", "status": "pending"}), tools.Context{}); !result.Valid {
		t.Fatalf("a todo without an id was rejected: %s", result.Message)
	}
}
