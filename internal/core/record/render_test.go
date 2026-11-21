package record

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func bigRecord() Record {
	var msgs []types.Message
	long := strings.Repeat("q", 400)
	msgs = append(msgs, user("Build the thing "+long+". Never push to main. Always run vet."))
	for i := 0; i < 20; i++ {
		msgs = append(msgs, fileCall(fmt.Sprint("f", i), "edit", fmt.Sprintf("internal/pkg%d/%s.go", i, long))...)
		msgs = append(msgs, bashCall(fmt.Sprint("b", i), fmt.Sprintf("go test ./pkg%d %s", i, long), long, i%3 == 0)...)
	}
	msgs = append(msgs, todoCall("t", manyTodos(8)...), user("what next? "+long))
	return Fold(msgs)
}

func TestRenderStaysUnderMaxTokens(t *testing.T) {
	r := bigRecord()
	for _, max := range []int{300, 600, 1200, 50, 10, 4} {
		out := Render(r, max)
		if est := (len(out) + 3) / 4; est > max {
			t.Errorf("max %d: estimate %d", max, est)
		}
		if out == "" || !strings.HasPrefix(out, Header) && max >= 10 {
			t.Errorf("max %d: bad output %q", max, out)
		}
		msg := messages.EstimateTokens([]types.Message{{Content: []types.ContentBlock{types.TextBlock{Text: out}}}})
		if msg > max {
			t.Errorf("max %d: EstimateTokens %d", max, msg)
		}
	}
}

func TestRenderShedsOldestFirst(t *testing.T) {
	r := Record{Goal: "g", Cmds: []CmdRun{{Cmd: "old"}, {Cmd: "newest"}}, Files: []FileTouch{{Path: "keep.go", Op: "edit", N: 1}}}
	full := Render(r, 1000)
	// Leave room for everything but one command line.
	tight := Render(r, (len(full)-len("\n- ok old"))/4)
	if strings.Contains(tight, "ok old") || !strings.Contains(tight, "ok newest") || !strings.Contains(tight, "keep.go") {
		t.Fatalf("shed wrong item:\n%s", tight)
	}
}

func TestRenderFixedOrderAndContent(t *testing.T) {
	r := Fold([]types.Message{
		user("Ship it. Never push to main."),
		todoCall("t", [2]string{"write tests", "in_progress"}),
		use("1", "edit", map[string]any{"file_path": "a.go"}), result("1", "ok", false),
		use("2", "bash", map[string]any{"command": "go vet ./..."}), result("2", "fine", false),
	})
	out := Render(r, 600)
	order := []string{Header, "Goal:", "Constraints:", "Files:", "Commands:", "Todos:", "Verified:", "Pending:"}
	pos := -1
	for _, want := range order {
		i := strings.Index(out, want)
		if i < 0 || i <= pos {
			t.Fatalf("%q out of order in:\n%s", want, out)
		}
		pos = i
	}
}

func TestRenderEmptyAndZeroBudget(t *testing.T) {
	if Render(Record{}, 600) != "" || Render(bigRecord(), 0) != "" {
		t.Fatal("expected empty output")
	}
}

func TestRenderDeterministic(t *testing.T) {
	a, b := Render(bigRecord(), 600), Render(bigRecord(), 600)
	if a != b {
		t.Fatal("render differs between identical inputs")
	}
	ja, _ := json.Marshal(bigRecord())
	jb, _ := json.Marshal(bigRecord())
	if string(ja) != string(jb) {
		t.Fatal("fold JSON differs between identical inputs")
	}
}

func TestRecordJSONNeverContainsMessageTypeMarker(t *testing.T) {
	r := Fold([]types.Message{user(`Never emit "type":"message" lines`), todoCall("t", [2]string{`x "type":"message" y`, "pending"})})
	line, err := json.Marshal(map[string]any{"type": "record", "record": r})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(line), `"type":"message"`) {
		t.Fatalf("record line contains the message marker: %s", line)
	}
}
