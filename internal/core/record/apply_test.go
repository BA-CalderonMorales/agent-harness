package record

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func user(text string) types.Message {
	return types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: text}}}
}

func assistantText(text string) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.TextBlock{Text: text}}}
}

func use(id, name string, input map[string]any) types.Message {
	return types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{types.ToolUseBlock{ID: id, Name: name, Input: input}}}
}

func result(id, content string, isErr bool) types.Message {
	return types.Message{Role: types.RoleUser, Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: id, Content: content, IsError: isErr}}}
}

func fileCall(id, tool, path string) []types.Message {
	return []types.Message{use(id, tool, map[string]any{"file_path": path}), result(id, "ok", false)}
}

func bashCall(id, cmd, out string, isErr bool) []types.Message {
	return []types.Message{use(id, "bash", map[string]any{"command": cmd}), result(id, out, isErr)}
}

func todoCall(id string, items ...[2]string) types.Message {
	var list []any
	for _, it := range items {
		list = append(list, map[string]any{"text": it[0], "status": it[1]})
	}
	return use(id, "todo_write", map[string]any{"todos": list})
}

func fold(groups ...[]types.Message) Record {
	var all []types.Message
	for _, g := range groups {
		all = append(all, g...)
	}
	return Fold(all)
}

func one(m types.Message) []types.Message { return []types.Message{m} }

func TestGoalIsFirstUserMessageAndNeverOverwritten(t *testing.T) {
	r := fold(one(user("fix the parser")), one(assistantText("on it")), one(user("also update docs")))
	if r.Goal != "fix the parser" {
		t.Fatalf("goal = %q", r.Goal)
	}
}

func TestGoalIgnoresToolResultCarriers(t *testing.T) {
	r := fold(bashCall("a", "ls", "x", false), one(user("the real goal")))
	if r.Goal != "the real goal" {
		t.Fatalf("goal = %q", r.Goal)
	}
}

func TestGoalCapped(t *testing.T) {
	r := Fold(one(user(strings.Repeat("g", 5000))))
	if n := len([]rune(r.Goal)); n != MaxGoal {
		t.Fatalf("goal len = %d, want %d", n, MaxGoal)
	}
}

func TestConstraintExtraction(t *testing.T) {
	positives := []string{
		"Never push to main.", "do not touch the lockfile", "Don't add emojis", "MUST keep files small",
		"Only edit internal/core", "always run vet first",
	}
	for _, p := range positives {
		r := Fold(one(user("Please help. " + p + ". Thanks")))
		if len(r.Constraints) != 1 || !strings.EqualFold(r.Constraints[0], strings.TrimSuffix(p, ".")) {
			t.Errorf("%q -> %v", p, r.Constraints)
		}
	}
	negatives := []string{
		"I would never do that", "Mustard is tasty", "Onlyfans", "Neverland is a place", "Alwaysomething",
		"we must keep going", "Please do not break it",
	}
	for _, n := range negatives {
		if r := Fold(one(user(n))); len(r.Constraints) != 0 {
			t.Errorf("%q should not yield constraints: %v", n, r.Constraints)
		}
	}
}

func TestConstraintsOnlyFromUserMessages(t *testing.T) {
	r := Fold(one(assistantText("Never mind. Always be careful.")))
	if len(r.Constraints) != 0 {
		t.Fatalf("assistant text produced constraints: %v", r.Constraints)
	}
}

func TestConstraintsCapDedupeAndLength(t *testing.T) {
	var msgs []types.Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, user(fmt.Sprintf("Never do thing %d %s", i, strings.Repeat("x", 300))))
	}
	msgs = append(msgs, user("Never do thing 19 "+strings.Repeat("x", 300)))
	r := Fold(msgs)
	if len(r.Constraints) != MaxConstraints {
		t.Fatalf("constraints = %d", len(r.Constraints))
	}
	for _, c := range r.Constraints {
		if len([]rune(c)) > MaxConstraintLen {
			t.Fatalf("constraint too long: %d", len([]rune(c)))
		}
	}
	r = Fold([]types.Message{user("Never push."), user("Never push.")})
	if len(r.Constraints) != 1 {
		t.Fatalf("duplicate kept: %v", r.Constraints)
	}
}

func TestFilesDedupedByPathWithCount(t *testing.T) {
	r := fold(fileCall("1", "read", "a.go"), fileCall("2", "edit", "a.go"), fileCall("3", "write", "b.go"), fileCall("4", "read", "a.go"))
	want := []FileTouch{{Path: "b.go", Op: "write", N: 1}, {Path: "a.go", Op: "read", N: 3}}
	if !reflect.DeepEqual(r.Files, want) {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestFilesCapKeepsMostRecent(t *testing.T) {
	var msgs []types.Message
	for i := 0; i < 50; i++ {
		msgs = append(msgs, fileCall(fmt.Sprint("id", i), "read", fmt.Sprintf("f%d.go", i))...)
	}
	r := Fold(msgs)
	if len(r.Files) != MaxFiles || r.Files[0].Path != "f38.go" || r.Files[MaxFiles-1].Path != "f49.go" {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestFailedFileOpIsNotRecorded(t *testing.T) {
	r := Fold([]types.Message{use("1", "write", map[string]any{"file_path": "x.go"}), result("1", "Permission denied: write", true)})
	if len(r.Files) != 0 {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestFailedBashRecordedWithTruncatedError(t *testing.T) {
	r := fold(bashCall("1", "go build ./...", strings.Repeat("boom ", 100), true))
	if len(r.Cmds) != 1 || !r.Cmds[0].Failed {
		t.Fatalf("cmds = %+v", r.Cmds)
	}
	if n := len([]rune(r.Cmds[0].Err)); n == 0 || n > MaxErrLen {
		t.Fatalf("err len = %d", n)
	}
	if len(r.Verified) != 0 {
		t.Fatalf("failed build counted as verified: %v", r.Verified)
	}
}

func TestCmdsCapNewestAndTruncated(t *testing.T) {
	var msgs []types.Message
	for i := 0; i < 30; i++ {
		msgs = append(msgs, bashCall(fmt.Sprint("c", i), fmt.Sprintf("echo %d %s", i, strings.Repeat("y", 200)), "ok", false)...)
	}
	r := Fold(msgs)
	if len(r.Cmds) != MaxCmds || !strings.HasPrefix(r.Cmds[MaxCmds-1].Cmd, "echo 29") {
		t.Fatalf("cmds = %+v", r.Cmds)
	}
	for _, c := range r.Cmds {
		if len([]rune(c.Cmd)) > MaxCmdLen {
			t.Fatalf("cmd too long")
		}
	}
}

func TestTodosCopiedFromLatestCall(t *testing.T) {
	r := Fold([]types.Message{
		todoCall("1", [2]string{"a", "pending"}, [2]string{"b", "pending"}),
		todoCall("2", [2]string{"a", "done"}, [2]string{"c", "in_progress"}),
	})
	want := []Todo{{"a", "done"}, {"c", "in_progress"}}
	if !reflect.DeepEqual(r.Todos, want) {
		t.Fatalf("todos = %+v", r.Todos)
	}
	if !reflect.DeepEqual(r.Pending, []string{"c"}) {
		t.Fatalf("pending = %v", r.Pending)
	}
}

func TestInvalidTodoPlanIgnored(t *testing.T) {
	r := Fold([]types.Message{todoCall("1", [2]string{"a", "pending"}), todoCall("2", [2]string{"b", "bogus"})})
	if len(r.Todos) != 1 || r.Todos[0].Text != "a" {
		t.Fatalf("todos = %+v", r.Todos)
	}
}

func TestVerifiedVersusFailedCommand(t *testing.T) {
	r := fold(bashCall("1", "go test ./...", "ok", false), bashCall("2", "ls -la", "x", false), bashCall("3", "make lint", "bad", true))
	if !reflect.DeepEqual(r.Verified, []string{"go test ./..."}) {
		t.Fatalf("verified = %v", r.Verified)
	}
	r = fold(bashCall("1", "go test ./...", "ok", false), bashCall("2", "go test ./...", "FAIL", true))
	if len(r.Verified) != 0 {
		t.Fatalf("stale verification kept: %v", r.Verified)
	}
}

func TestVerifiedCap(t *testing.T) {
	var msgs []types.Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, bashCall(fmt.Sprint("v", i), fmt.Sprintf("go test ./p%d", i), "ok", false)...)
	}
	if r := Fold(msgs); len(r.Verified) != MaxVerified {
		t.Fatalf("verified = %v", r.Verified)
	}
}

func TestDeniedRecorded(t *testing.T) {
	r := Fold([]types.Message{
		use("1", "bash", map[string]any{"command": "rm -rf x"}),
		result("1", "Permission denied: bash", true),
		result("2", "permission denied: user rejected", true),
		result("3", "some other error", true),
		result("4", "Permission denied: not an error flag", false),
	})
	if len(r.Denied) != 2 {
		t.Fatalf("denied = %v", r.Denied)
	}
}

func TestPendingIncludesUnansweredUserMessage(t *testing.T) {
	r := Fold([]types.Message{user("do it"), use("1", "read", map[string]any{"file_path": "a"})})
	if !reflect.DeepEqual(r.Pending, []string{"do it"}) {
		t.Fatalf("pending = %v", r.Pending)
	}
	r = Apply(r, assistantText("done"))
	if len(r.Pending) != 0 {
		t.Fatalf("pending = %v", r.Pending)
	}
}

func TestApplyNoOpForUnknownToolsAndSystem(t *testing.T) {
	base := Fold([]types.Message{user("goal"), assistantText("ok")})
	for _, m := range []types.Message{
		use("1", "web_search", map[string]any{"q": "x"}),
		use("2", "glob", map[string]any{"pattern": "*"}),
		result("9", "whatever", false),
		{Role: types.RoleSystem, Content: []types.ContentBlock{types.TextBlock{Text: "Never do this."}}},
		{Role: types.RoleAssistant, Content: []types.ContentBlock{types.ThinkingBlock{Thinking: "hm"}}},
	} {
		if got := Apply(base, m); !reflect.DeepEqual(got, base) {
			t.Errorf("not a no-op for %+v:\n%+v", m, got)
		}
	}
	if got := Apply(Record{}, use("1", "web_search", nil)); !got.IsZero() {
		t.Fatalf("zero record changed: %+v", got)
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	base := fold(fileCall("1", "read", "a.go"), bashCall("2", "go test", "ok", false))
	snapshot := clone(base)
	_ = Apply(base, use("3", "write", map[string]any{"file_path": "b.go"}))
	_ = Apply(Apply(base, use("3", "write", map[string]any{"file_path": "b.go"})), result("3", "ok", false))
	if !reflect.DeepEqual(base, snapshot) {
		t.Fatal("input record mutated")
	}
}

func TestUseAndResultInOneMessagePair(t *testing.T) {
	m := types.Message{Role: types.RoleAssistant, Content: []types.ContentBlock{
		types.ToolResultBlock{ToolUseID: "1", Content: "ok"},
		types.ToolUseBlock{ID: "1", Name: "read", Input: map[string]any{"file_path": "a.go"}},
	}}
	if r := Apply(Record{}, m); len(r.Files) != 1 {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestFlood(t *testing.T) {
	var msgs []types.Message
	for i := 0; i < 400; i++ {
		long := strings.Repeat("z", 1000)
		msgs = append(msgs,
			user(fmt.Sprintf("Never %d %s. Only %d %s.", i, long, i, long)),
			assistantText(long),
			todoCall(fmt.Sprint("t", i), manyTodos(30)...),
		)
		msgs = append(msgs, fileCall(fmt.Sprint("f", i), "edit", fmt.Sprintf("%s/%d", long, i))...)
		msgs = append(msgs, bashCall(fmt.Sprint("b", i), "go test "+long, long, i%2 == 0)...)
		msgs = append(msgs, result(fmt.Sprint("d", i), "Permission denied: "+long, true))
		msgs = append(msgs, use(fmt.Sprint("o", i), "bash", map[string]any{"command": "pending " + long}))
	}
	r := Fold(msgs)
	caps := map[string][2]int{
		"constraints": {len(r.Constraints), MaxConstraints}, "files": {len(r.Files), MaxFiles},
		"cmds": {len(r.Cmds), MaxCmds}, "todos": {len(r.Todos), MaxTodos}, "verified": {len(r.Verified), MaxVerified},
		"pending": {len(r.Pending), MaxPending}, "denied": {len(r.Denied), MaxDenied}, "open": {len(r.Open), maxOpenCalls},
	}
	for name, c := range caps {
		if c[0] > c[1] {
			t.Errorf("%s = %d > cap %d", name, c[0], c[1])
		}
	}
	over := func(label string, s string, n int) {
		if len([]rune(s)) > n {
			t.Errorf("%s len %d > %d", label, len([]rune(s)), n)
		}
	}
	over("goal", r.Goal, MaxGoal)
	for _, c := range r.Constraints {
		over("constraint", c, MaxConstraintLen)
	}
	for _, f := range r.Files {
		over("path", f.Path, MaxPathLen)
	}
	for _, c := range r.Cmds {
		over("cmd", c.Cmd, MaxCmdLen)
		over("err", c.Err, MaxErrLen)
	}
	for _, td := range r.Todos {
		over("todo", td.Text, MaxTodoLen)
	}
	for _, p := range r.Pending {
		over("pending", p, MaxPendingLen)
	}
	for _, d := range r.Denied {
		over("denied", d, MaxDeniedLen)
	}
	for _, o := range r.Open {
		over("open arg", o.Arg, maxOpenCallArgLen)
	}
}

func manyTodos(n int) [][2]string {
	var out [][2]string
	for i := 0; i < n; i++ {
		out = append(out, [2]string{fmt.Sprintf("todo %d %s", i, strings.Repeat("w", 200)), "pending"})
	}
	return out
}
