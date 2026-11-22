package record

import (
	"reflect"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func twoCalls(todo types.Message, bash types.Message) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: append(append([]types.ContentBlock(nil), todo.Content...), bash.Content...)}
}

// An assistant proposes a plan that marks everything done next to a call the
// repetition guard rejects. Both calls come back as errors, so the accepted
// plan stays current and its unfinished items stay pending.
func TestRejectedTodoProposalDoesNotReplaceTheAcceptedPlan(t *testing.T) {
	accepted := [][2]string{{"write tests", "pending"}, {"update docs", "pending"}}
	proposal := twoCalls(
		todoCall("t2", [2]string{"write tests", "done"}, [2]string{"update docs", "done"}),
		use("b2", "bash", map[string]any{"command": "make test"}),
	)
	rejected := types.Message{Role: types.RoleUser, Content: []types.ContentBlock{
		types.ToolResultBlock{ToolUseID: "t2", Content: "The repetition guard stopped this batch.", IsError: true},
		types.ToolResultBlock{ToolUseID: "b2", Content: "The repetition guard stopped this batch.", IsError: true},
	}}
	r := fold(
		one(user("ship the settings change")),
		one(todoCall("t1", accepted...)), one(result("t1", "ok", false)),
		one(proposal), one(rejected),
	)
	want := []Todo{{Text: "write tests", Status: "pending"}, {Text: "update docs", Status: "pending"}}
	if !reflect.DeepEqual(r.Todos, want) {
		t.Fatalf("todos = %+v, want the accepted plan %+v", r.Todos, want)
	}
	if len(r.Pending) < 2 || !reflect.DeepEqual(r.Pending[:2], []string{"write tests", "update docs"}) {
		t.Fatalf("pending = %v, want both unfinished items", r.Pending)
	}
	if len(r.Undo) != 0 {
		t.Fatalf("undo = %+v, want the settled proposal forgotten", r.Undo)
	}
}

// An accepted proposal keeps showing, and once its result arrives nothing is
// left to undo. A later accepted plan is never rolled back by an earlier
// proposal's late rejection.
func TestAcceptedTodoProposalStaysAndALateRejectionCannotRollBackALaterPlan(t *testing.T) {
	r := fold(
		one(user("ship it")),
		one(todoCall("t1", [2]string{"write tests", "pending"})),
		one(result("t1", "ok", false)),
	)
	want := []Todo{{Text: "write tests", Status: "pending"}}
	if !reflect.DeepEqual(r.Todos, want) || len(r.Undo) != 0 {
		t.Fatalf("todos = %+v undo = %+v, want the plan kept and nothing to undo", r.Todos, r.Undo)
	}

	r = fold(
		one(user("ship it")),
		one(todoCall("t1", [2]string{"a", "pending"})),
		one(todoCall("t2", [2]string{"b", "pending"})),
		one(result("t2", "ok", false)),
		one(result("t1", "rejected", true)),
	)
	if want := []Todo{{Text: "b", Status: "pending"}}; !reflect.DeepEqual(r.Todos, want) {
		t.Fatalf("todos = %+v, want the later accepted plan kept", r.Todos)
	}
}

// A call without an id can never be matched to its result, so it applies and
// nothing is kept to undo.
func TestTodoProposalWithoutAnIDAppliesImmediately(t *testing.T) {
	r := fold(one(user("ship it")), one(todoCall("", [2]string{"write tests", "pending"})))
	if len(r.Todos) != 1 {
		t.Fatalf("todos = %+v, want the id-less plan applied", r.Todos)
	}
}

// Two proposals rejected in one batch: the rollback must reach the plan that
// was accepted before either, whichever order the rejections arrive in.
func TestTwoRejectedProposalsRollBackToTheLastAcceptedPlan(t *testing.T) {
	accepted := []Todo{{Text: "p", Status: "pending"}}
	for name, order := range map[string][2]string{"oldest first": {"t2", "t3"}, "newest first": {"t3", "t2"}} {
		t.Run(name, func(t *testing.T) {
			r := fold(
				one(user("ship it")),
				one(todoCall("t1", [2]string{"p", "pending"})), one(result("t1", "ok", false)),
				one(todoCall("t2", [2]string{"a", "pending"})),
				one(todoCall("t3", [2]string{"b", "pending"})),
				one(result(order[0], "rejected", true)),
				one(result(order[1], "rejected", true)),
			)
			if !reflect.DeepEqual(r.Todos, accepted) {
				t.Fatalf("todos = %+v, want the accepted plan %+v", r.Todos, accepted)
			}
		})
	}
}

// Two proposals can carry the same plan. Identity is the proposal, not its
// content: rejecting the older one while the newer is current changes nothing.
func TestRejectingAnOlderIdenticalProposalKeepsTheCurrentOne(t *testing.T) {
	r := fold(
		one(user("ship it")),
		one(todoCall("t1", [2]string{"p", "pending"})), one(result("t1", "ok", false)),
		one(todoCall("t2", [2]string{"a", "pending"})),
		one(todoCall("t3", [2]string{"a", "pending"})),
		one(result("t3", "ok", false)),
		one(result("t2", "rejected", true)),
	)
	if want := []Todo{{Text: "a", Status: "pending"}}; !reflect.DeepEqual(r.Todos, want) {
		t.Fatalf("todos = %+v, want the accepted identical plan %+v", r.Todos, want)
	}
}
