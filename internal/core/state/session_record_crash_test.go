package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func crashMsg(id string, role types.MessageRole, blocks ...types.ContentBlock) types.Message {
	return types.Message{UUID: id, Role: role, Content: blocks, Timestamp: time.Unix(1700000000, 0).UTC()}
}

// laterEvidence is what a turn does after the last record snapshot: touch a
// file, replace the todo plan, and run a failing command.
func laterEvidence() []types.Message {
	return []types.Message{
		crashMsg("a2", types.RoleAssistant,
			types.ToolUseBlock{ID: "t2", Name: "edit", Input: map[string]any{"file_path": "later.go"}},
			types.ToolUseBlock{ID: "t3", Name: "todo_write", Input: map[string]any{"todos": []any{map[string]any{"text": "ship it", "status": "pending"}}}},
			types.ToolUseBlock{ID: "t4", Name: "bash", Input: map[string]any{"command": "go test ./..."}}),
		crashMsg("u2", types.RoleUser,
			types.ToolResultBlock{ToolUseID: "t2", Content: "ok"},
			types.ToolResultBlock{ToolUseID: "t4", Content: "FAIL", IsError: true}),
	}
}

func newCrashSession(t *testing.T) (*SessionManager, *Session) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())
	sm, err := NewSessionManagerForProject(filepath.Join(t.TempDir(), "p"))
	if err != nil {
		t.Fatal(err)
	}
	s := sm.CreateSession("m")
	for _, m := range []types.Message{
		crashMsg("u1", types.RoleUser, types.TextBlock{Text: "Build it. Never push."}),
		crashMsg("a1", types.RoleAssistant, types.ToolUseBlock{ID: "t1", Name: "read", Input: map[string]any{"file_path": "first.go"}}),
		crashMsg("u1b", types.RoleUser, types.ToolResultBlock{ToolUseID: "t1", Content: "ok"}),
	} {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	return sm, s
}

// A process that dies between writing messages and writing the record leaves
// complete messages after an older snapshot. The loader replays them.
func TestRecordIsReplayedOverMessagesWrittenAfterTheSnapshot(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(recordLines(t, path), "\n"), `"msgs":3`) {
		t.Fatalf("record line does not say how many messages it covers: %v", recordLines(t, path))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range laterEvidence() {
		raw, _ := json.Marshal(m)
		if err := writeEvent(f, storeEvent{Kind: "message", Message: rawPtr(raw)}); err != nil {
			t.Fatal(err)
		}
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	_ = f.Close()

	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := loaded.Record
	paths := []string{}
	for _, f := range r.Files {
		paths = append(paths, f.Path)
	}
	if !reflect.DeepEqual(paths, []string{"first.go", "later.go"}) {
		t.Fatalf("files = %v", paths)
	}
	if len(r.Todos) != 1 || r.Todos[0].Text != "ship it" {
		t.Fatalf("todos = %+v", r.Todos)
	}
	if len(r.Cmds) != 1 || !r.Cmds[0].Failed {
		t.Fatalf("cmds = %+v", r.Cmds)
	}
	if !reflect.DeepEqual(r, s.Record) {
		t.Fatalf("replayed record differs from the live one:\n%+v\n%+v", r, s.Record)
	}
}

func TestRecordNormalPathLoadsIdenticallyAndIsNotRewritten(t *testing.T) {
	sm, s := newCrashSession(t)
	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Record, s.Record) {
		t.Fatal("normal load changed the record")
	}
	before := len(recordLines(t, path))
	for i := 0; i < 3; i++ {
		if _, err := sm.SaveCurrent(); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(recordLines(t, path)); n != before {
		t.Fatalf("unchanged record rewritten: %d -> %d", before, n)
	}
}

func TestRecordRewriteStoresMessageCount(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	s.Messages = s.Messages[1:]
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	lines := recordLines(t, path)
	if len(lines) != 1 || !strings.Contains(lines[0], fmt.Sprintf(`"msgs":%d`, len(s.Messages))) {
		t.Fatalf("rewrite record lines = %v", lines)
	}
}
