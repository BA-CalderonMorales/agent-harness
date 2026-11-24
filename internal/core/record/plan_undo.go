package record

import "github.com/BA-CalderonMorales/agent-harness/pkg/types"

// maxTodoUndos bounds the proposals waiting for a result. A batch holds a
// handful of calls at most; a flood keeps only the newest.
const maxTodoUndos = 4

// TodoUndo remembers the plan a todo_write proposal replaced, until the
// proposal's result arrives. The proposal shows at once, as the tool's own
// output would, but a rejected one (the repetition guard, a permission denial,
// a validation error) must give the accepted plan back.
//
// Entries are oldest first and form a chain: each Prev is the plan the
// proposal replaced. Seq identifies the proposal; Record.PlanSeq names the
// one whose plan is current. Two proposals may carry the same plan, so the
// plan's content never identifies a proposal.
type TodoUndo struct {
	ID   string `json:"id"`
	Seq  int    `json:"seq"`
	Prev []Todo `json:"prev"`
}

// replacePlan applies a proposed plan and remembers the one it replaced. A
// call without an id can never be matched to its result, so nothing is kept,
// but it still supersedes every pending proposal.
func (r *Record) replacePlan(id string, plan []Todo) {
	r.PlanSeq++
	if id != "" {
		id = boundID(id)
		kept := make([]TodoUndo, 0, len(r.Undo)+1)
		for _, u := range r.Undo {
			if u.ID != id {
				kept = append(kept, u)
			}
		}
		r.Undo = pushNewest(kept, TodoUndo{ID: id, Seq: r.PlanSeq, Prev: r.Todos}, maxTodoUndos)
	}
	r.Todos = plan
}

// settlePlan resolves the proposal a result answers. A success keeps the
// plan. An error removes the proposal from the chain:
//   - if its plan is current, the previous plan comes back and the proposal
//     before it in the chain, if any, becomes current again;
//   - otherwise the next proposal inherits its Prev, so a later rollback
//     skips the rejected plan and reaches the one accepted before it.
func (r *Record) settlePlan(res types.ToolResultBlock) {
	id := boundID(res.ToolUseID)
	for i, u := range r.Undo {
		if u.ID != id {
			continue
		}
		if res.IsError {
			switch {
			case u.Seq == r.PlanSeq:
				r.Todos = u.Prev
				r.PlanSeq = 0
				if i > 0 {
					r.PlanSeq = r.Undo[i-1].Seq
				}
			case i+1 < len(r.Undo):
				r.Undo[i+1].Prev = u.Prev
			}
		}
		r.Undo = append(r.Undo[:i:i], r.Undo[i+1:]...)
		if len(r.Undo) == 0 {
			r.Undo = nil
		}
		return
	}
}
