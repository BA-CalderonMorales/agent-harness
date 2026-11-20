package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

const journeyRecordAnswer = "Summary written."

// recordTurnClient drives three requests: read a file, write a todo plan,
// then answer. It is the shape of a real multi-step turn.
type recordTurnClient struct {
	llm.Client
	mu       sync.Mutex
	calls    int
	readPath string
}

func (c *recordTurnClient) Stream(_ context.Context, _ llm.Request) (<-chan types.LLMEvent, error) {
	c.mu.Lock()
	c.calls++
	call := c.calls
	c.mu.Unlock()

	out := make(chan types.LLMEvent, 8)
	go func() {
		defer close(out)
		out <- types.LLMMessageStart{ID: "record-m"}
		switch call {
		case 1:
			input, _ := json.Marshal(map[string]any{"file_path": c.readPath})
			out <- types.LLMToolUseDelta{ID: "record-t1", Name: "read", Delta: string(input)}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
		case 2:
			out <- types.LLMToolUseDelta{ID: "record-t2", Name: "todo_write", Delta: `{"todos":[` +
				`{"id":"1","text":"read the notes","status":"done"},` +
				`{"id":"2","text":"write the summary","status":"in_progress"}]}`}
			out <- types.LLMMessageStop{StopReason: "tool_calls"}
		default:
			out <- types.LLMTextDelta{Delta: journeyRecordAnswer}
			out <- types.LLMMessageStop{StopReason: "stop"}
		}
	}()
	return out, nil
}

func sessionFilesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func recordLinesIn(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, `{"type":"record"`) {
			lines = append(lines, line)
		}
	}
	return lines
}

func lastRecordIn(t *testing.T, path string) record.Record {
	t.Helper()
	lines := recordLinesIn(t, path)
	if len(lines) == 0 {
		t.Fatal("session file has no record line")
	}
	var event struct {
		Record record.Record `json:"record"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	return event.Record
}

// The journey: the user states a goal and a constraint, the agent reads a
// file and writes a plan, and the session file carries a record of it. A
// restart keeps the record; a file from a build that predates the record
// gets one rebuilt from its transcript.
func TestJourneySessionRecordIsSavedReloadedAndRebuilt(t *testing.T) {
	workspace := journeyHome(t)
	journeyLocal(t)
	t.Setenv("AH_EXECUTION_MODE", "yolo")
	t.Setenv("AH_PERM_EXECUTE", "true")
	t.Setenv("AH_PERM_READ", "true")
	notes := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(notes, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := bootJourneyApp(t)
	client := &recordTurnClient{readPath: notes}
	app.client = client
	app.loop = agent.NewLoop(client)
	tuiApp := wireJourneyTUI(t, app)
	screen := newJourneyScreen(t, tuiApp, 80, 24)
	screen.Keys("2")
	screen.Keys("i")
	screen.Submit(t, app, "Summarize notes.txt for me. Never edit any file.")
	screen.waitFor(t, journeyRecordAnswer)
	screen.waitIdle(t)
	if client.calls != 3 {
		t.Fatalf("model requests = %d, want 3", client.calls)
	}

	files := sessionFilesUnder(t, filepath.Dir(workspace))
	if len(files) != 1 {
		t.Fatalf("session files = %v", files)
	}
	saved := lastRecordIn(t, files[0])
	if !strings.HasPrefix(saved.Goal, "Summarize notes.txt") {
		t.Errorf("goal = %q", saved.Goal)
	}
	if len(saved.Constraints) != 1 || !strings.HasPrefix(saved.Constraints[0], "Never edit") {
		t.Errorf("constraints = %v", saved.Constraints)
	}
	if len(saved.Files) != 1 || saved.Files[0].Op != "read" || !strings.HasSuffix(notes, strings.TrimPrefix(saved.Files[0].Path, "…")) || !strings.HasSuffix(saved.Files[0].Path, "notes.txt") {
		t.Errorf("files = %+v", saved.Files)
	}
	wantTodos := []record.Todo{{Text: "read the notes", Status: "done"}, {Text: "write the summary", Status: "in_progress"}}
	if !reflect.DeepEqual(saved.Todos, wantTodos) {
		t.Errorf("todos = %+v", saved.Todos)
	}
	if !reflect.DeepEqual(saved.Pending, []string{"write the summary"}) {
		t.Errorf("pending = %v", saved.Pending)
	}
	if !reflect.DeepEqual(saved, app.session.Record) {
		t.Errorf("the file's last record differs from the live one:\n%+v\n%+v", saved, app.session.Record)
	}
	// The record rides alongside the messages: it never became a message.
	for _, m := range app.session.Messages {
		for _, b := range m.Content {
			if tb, ok := b.(types.TextBlock); ok && strings.Contains(tb.Text, "[Session record]") {
				t.Fatal("the record leaked into the transcript")
			}
		}
	}
	sessionID := app.session.ID

	// Restart: the record comes back with the session.
	restarted := bootJourneyApp(t)
	if restarted.session.ID != sessionID || !reflect.DeepEqual(restarted.session.Record, saved) {
		t.Fatalf("resumed record differs:\n%+v\n%+v", restarted.session.Record, saved)
	}

	// A file from a build without records: strip the lines, resume, rebuild.
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if !strings.HasPrefix(line, `{"type":"record"`) {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(files[0], []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := bootJourneyApp(t)
	if old.session.ID != sessionID {
		t.Fatalf("resumed %s, want %s", old.session.ID, sessionID)
	}
	if !reflect.DeepEqual(old.session.Record, saved) {
		t.Fatalf("rebuilt record differs from the live one:\n%+v\n%+v", old.session.Record, saved)
	}
	// The rebuilt record is persisted on the next save, once.
	if _, err := old.sessionManager.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if _, err := old.sessionManager.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if n := len(recordLinesIn(t, files[0])); n != 1 {
		t.Fatalf("record lines after rebuild = %d, want 1", n)
	}
}
