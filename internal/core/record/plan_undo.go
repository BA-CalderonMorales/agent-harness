package record

import (
	"reflect"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// maxTodoUndos bounds the proposals waiting for a result. A batch holds a
// handful of calls at most; a flood keeps only the newest.
const maxTodoUndos = 4

// TodoUndo remembers the plan a todo_write proposal replaced, until the
// proposal's result arrives. The proposal shows at once, as the tool's own
// output would, but a rejected one (the repetition guard, a permission denial,
// a validation error) must give the accepted plan back.
type TodoUndo struct {
	ID   string `json:"id"`
	Prev []Todo `json:"prev"`
	Plan []Todo `json:"plan"`
}

// replacePlan applies a proposed plan and remembers the one it replaced. A
// call without an id can never be matched to its result, so nothing is kept.
func (r *Record) replacePlan(id string, plan []Todo) {
	if id != "" {
		id = boundID(id)
		kept := make([]TodoUndo, 0, len(r.Undo)+1)
		for _, u := range r.Undo {
			if u.ID != id {
				kept = append(kept, u)
			}
		}
		r.Undo = pushNewest(kept, TodoUndo{ID: id, Prev: r.Todos, Plan: plan}, maxTodoUndos)
	}
	r.Todos = plan
}

// settlePlan resolves the proposal a result answers. A success keeps the
// plan. An error restores the previous one, but only while the proposal is
// still the current plan: a later accepted plan is never rolled back.
func (r *Record) settlePlan(res types.ToolResultBlock) {
	id := boundID(res.ToolUseID)
	for i, u := range r.Undo {
		if u.ID != id {
			continue
		}
		if res.IsError && reflect.DeepEqual(r.Todos, u.Plan) {
			r.Todos = u.Prev
		}
		r.Undo = append(r.Undo[:i:i], r.Undo[i+1:]...)
		if len(r.Undo) == 0 {
			r.Undo = nil
		}
		return
	}
}
