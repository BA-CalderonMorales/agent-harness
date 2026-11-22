package record

import (
	"reflect"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Tool names the record understands. Every other tool is ignored.
const (
	toolRead  = "read"
	toolWrite = "write"
	toolEdit  = "edit"
	toolBash  = "bash"
	toolTodo  = "todo_write"
)

var todoStatuses = map[string]bool{"pending": true, "in_progress": true, "done": true, "cancelled": true}

// Fold builds a record from a transcript: Apply over every message, starting
// from the zero Record.
func Fold(msgs []types.Message) Record {
	var r Record
	for _, m := range msgs {
		r = Apply(r, m)
	}
	return r
}

// Apply returns the record after one more message. It takes and returns
// values and never mutates its input. A message that changes nothing the
// record tracks returns the input unchanged.
func Apply(r Record, msg types.Message) Record {
	next := clone(r)

	var text strings.Builder
	var uses []types.ToolUseBlock
	var results []types.ToolResultBlock
	for _, block := range msg.Content {
		switch b := block.(type) {
		case types.TextBlock:
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(b.Text)
		case types.ToolUseBlock:
			uses = append(uses, b)
		case types.ToolResultBlock:
			results = append(results, b)
		}
	}

	switch msg.Role {
	case types.RoleUser:
		next.applyUserText(text.String())
	case types.RoleAssistant:
		if strings.TrimSpace(text.String()) != "" {
			next.Replied = true
		}
	}
	// Uses first, then results, so a call and its result in one message
	// still pair up.
	for _, use := range uses {
		next.applyUse(use)
	}
	for _, res := range results {
		next.applyResult(res)
	}

	next.Pending = next.pending()
	if reflect.DeepEqual(next, r) {
		return r
	}
	next.V = Version
	return next
}

func (r *Record) applyUserText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if r.Goal == "" {
		r.Goal = clip(text, MaxGoal)
	}
	for _, c := range constraintsIn(text) {
		r.Constraints = moveToEnd(r.Constraints, c, MaxConstraints)
	}
	r.LastUser = clip(text, MaxPendingLen)
	r.Replied = false
}

func (r *Record) applyUse(use types.ToolUseBlock) {
	switch use.Name {
	case toolRead, toolWrite, toolEdit:
		path, _ := use.Input["file_path"].(string)
		if strings.TrimSpace(path) != "" {
			r.open(use.ID, use.Name, clipTail(path, maxOpenCallArgLen))
		}
	case toolBash:
		cmd, _ := use.Input["command"].(string)
		if strings.TrimSpace(cmd) != "" {
			r.open(use.ID, use.Name, clip(joinCommandLines(cmd), maxOpenCallArgLen))
		}
	case toolTodo:
		if todos, ok := todosFrom(use.Input); ok {
			r.replacePlan(use.ID, todos)
		}
	}
}

func (r *Record) open(id, tool, arg string) {
	if id == "" {
		return
	}
	r.Open = pushNewest(r.Open, OpenCall{ID: boundID(id), Tool: tool, Arg: arg}, maxOpenCalls)
}

func (r *Record) applyResult(res types.ToolResultBlock) {
	if res.IsError && isDenial(res.Content) {
		r.Denied = moveToEnd(r.Denied, clip(res.Content, MaxDeniedLen), MaxDenied)
	}
	r.settlePlan(res)
	idx := -1
	for i, call := range r.Open {
		if call.ID == boundID(res.ToolUseID) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	call := r.Open[idx]
	r.Open = append(r.Open[:idx], r.Open[idx+1:]...)
	if len(r.Open) == 0 {
		r.Open = nil
	}

	switch call.Tool {
	case toolRead, toolWrite, toolEdit:
		if !res.IsError {
			r.touch(call.Arg, call.Tool)
		}
	case toolBash:
		r.ran(call.Arg, res)
	}
}

func (r *Record) touch(path, op string) {
	path = clipTail(path, MaxPathLen)
	touch := FileTouch{Path: path, Op: op, N: 1}
	kept := make([]FileTouch, 0, len(r.Files)+1)
	for _, f := range r.Files {
		if f.Path == path {
			touch.N = f.N + 1
			continue
		}
		kept = append(kept, f)
	}
	r.Files = pushNewest(kept, touch, MaxFiles)
}

func (r *Record) ran(fullCmd string, res types.ToolResultBlock) {
	run := CmdRun{Cmd: clip(fullCmd, MaxCmdLen), Failed: res.IsError}
	if res.IsError {
		run.Err = clip(res.Content, MaxErrLen)
	}
	r.Cmds = pushNewest(r.Cmds, run, MaxCmds)
	if res.IsError {
		// A later failure retires earlier evidence for the same command.
		r.Verified = removeString(r.Verified, run.Cmd)
		return
	}
	if isCheckCommand(fullCmd) {
		r.Verified = moveToEnd(r.Verified, run.Cmd, MaxVerified)
	}
}

// pending lists unfinished todos, then the last user message when no
// assistant text has answered it.
func (r Record) pending() []string {
	var out []string
	for _, t := range r.Todos {
		if t.Status != "done" && t.Status != "cancelled" {
			out = append(out, clip(t.Text, MaxPendingLen))
		}
	}
	unanswered := r.LastUser != "" && !r.Replied
	limit := MaxPending
	if unanswered {
		limit--
	}
	if len(out) > limit {
		out = out[:limit]
	}
	if unanswered {
		out = append(out, r.LastUser)
	}
	return out
}

func isDenial(content string) bool {
	s := strings.ToLower(strings.TrimSpace(content))
	s = strings.TrimSpace(strings.TrimPrefix(s, "error:"))
	return strings.HasPrefix(s, "permission denied")
}

func removeString(list []string, v string) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		if item != v {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// todosFrom reads a todo_write input. A plan the tool would reject (empty
// text, unknown status) is ignored whole, because the call failed.
func todosFrom(input map[string]any) ([]Todo, bool) {
	var entries []map[string]any
	switch raw := input["todos"].(type) {
	case []any:
		for _, e := range raw {
			m, ok := e.(map[string]any)
			if !ok {
				return nil, false
			}
			entries = append(entries, m)
		}
	case []map[string]any:
		entries = raw
	default:
		return nil, false
	}
	var out []Todo
	for _, m := range entries {
		text, _ := m["text"].(string)
		status, _ := m["status"].(string)
		if strings.TrimSpace(text) == "" || !todoStatuses[status] {
			return nil, false
		}
		if len(out) < MaxTodos {
			out = append(out, Todo{Text: clip(text, MaxTodoLen), Status: status})
		}
	}
	return out, true
}

// clone copies every slice so Apply never writes through its input.
func clone(r Record) Record {
	r.Constraints = append([]string(nil), r.Constraints...)
	r.Files = append([]FileTouch(nil), r.Files...)
	r.Cmds = append([]CmdRun(nil), r.Cmds...)
	r.Todos = append([]Todo(nil), r.Todos...)
	r.Verified = append([]string(nil), r.Verified...)
	r.Pending = append([]string(nil), r.Pending...)
	r.Denied = append([]string(nil), r.Denied...)
	r.Open = append([]OpenCall(nil), r.Open...)
	r.Undo = append([]TodoUndo(nil), r.Undo...)
	return r
}
