package state

import (
	"encoding/json"
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

// The record line must be invisible to builds that predate it and must never
// disturb the message accounting the append path depends on.

// oldStoreEvent is storeEvent as a build without the record kind declares it.
type oldStoreEvent struct {
	Kind    string           `json:"type"`
	Message *json.RawMessage `json:"message,omitempty"`
	Meta    *storeMeta       `json:"meta,omitempty"`
	ID      string           `json:"id,omitempty"`
}

// loadAsOldBuild folds a session file the way a build without the record
// kind does: a switch with no default, so unknown kinds fall through.
func loadAsOldBuild(t *testing.T, path string) []types.Message {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.Message
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event oldStoreEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("old build cannot parse a line: %v", err)
		}
		switch event.Kind {
		case "header", "meta":
		case "message":
			var msg types.Message
			if err := json.Unmarshal(*event.Message, &msg); err != nil {
				t.Fatal(err)
			}
			out = append(out, msg)
		}
	}
	return out
}

func recordLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, recordLinePrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

func rawLastRecord(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(compatFixturePath(name))
	if err != nil {
		t.Fatal(err)
	}
	var last map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["type"] == "record" {
			last, _ = event["record"].(map[string]any)
		}
	}
	return last
}

func TestRecordFixtureLoadsLastRecordMatchingRawFile(t *testing.T) {
	session, err := loadSessionFile(compatFixturePath("v3-record"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(session.Record)
	var gotMap map[string]any
	_ = json.Unmarshal(got, &gotMap)
	if want := rawLastRecord(t, "v3-record"); !reflect.DeepEqual(gotMap, want) {
		t.Fatalf("loaded record differs from the file's last record line:\n got  %v\n want %v", gotMap, want)
	}
	if session.Record.V != record.Version || session.Record.Goal == "" || len(session.Record.Files) == 0 || len(session.Record.Todos) != 2 {
		t.Fatalf("record not restored: %+v", session.Record)
	}
}

func TestOldFixturesCarryNoRecord(t *testing.T) {
	for _, name := range []string{"v1-header-meta", "v1-tools-errors", "v2-turns-orphan", "v1-text-latest"} {
		session, err := loadSessionFile(compatFixturePath(name))
		if err != nil {
			t.Fatal(err)
		}
		if !session.Record.IsZero() {
			t.Errorf("%s: record = %+v, want empty", name, session.Record)
		}
	}
}

func TestOldBuildIgnoresRecordAndUnknownKinds(t *testing.T) {
	path := compatFixturePath("v3-record")
	newBuild, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := messagesJSON(t, newBuild.Messages)

	if got := messagesJSON(t, loadAsOldBuild(t, path)); !reflect.DeepEqual(got, want) {
		t.Fatal("an old loader reads different messages from a file with record lines")
	}

	data, _ := os.ReadFile(path)
	var stripped, unknown []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, recordLinePrefix) {
			unknown = append(unknown, `{"type":"from-the-future","payload":{"x":1}}`)
			continue
		}
		stripped = append(stripped, line)
		unknown = append(unknown, line)
	}
	dir := t.TempDir()
	for name, lines := range map[string][]string{"stripped": stripped, "unknown": unknown} {
		p := filepath.Join(dir, name+".jsonl")
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// The current loader tolerates an unknown kind too, and keeps the messages.
		session, err := loadSessionFile(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := messagesJSON(t, session.Messages); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: messages differ", name)
		}
		if !session.Record.IsZero() {
			t.Fatalf("%s: record should be empty", name)
		}
		if got := messagesJSON(t, loadAsOldBuild(t, p)); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: old loader reads different messages", name)
		}
	}
}

func TestRecordResumeAppendWritesOnlyChangedRecords(t *testing.T) {
	sm, path := installCompatFixture(t, "v3-record")
	resumed, err := sm.ResumeLatestSessionWithError()
	if err != nil || resumed == nil {
		t.Fatalf("resume = (%v, %v)", resumed, err)
	}
	if resumed.Record.IsZero() {
		t.Fatal("resume lost the record")
	}
	before := len(recordLines(t, path))

	// An unchanged record is not written again.
	for i := 0; i < 2; i++ {
		if _, err := sm.SaveCurrent(); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(recordLines(t, path)); n != before {
		t.Fatalf("record lines %d -> %d with no record change", before, n)
	}

	// A changed record is written once, and the last one wins on reload.
	msg := types.Message{UUID: "late", Role: types.RoleAssistant, Timestamp: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC),
		Content: []types.ContentBlock{types.ToolUseBlock{ID: "late-1", Name: "bash", Input: map[string]any{"command": "go test ./..."}},
			types.ToolResultBlock{ToolUseID: "late-1", Content: "ok"}}}
	resumed.AddMessage(msg)
	resumed.Record = record.Apply(resumed.Record, msg)
	for i := 0; i < 2; i++ {
		if _, err := sm.SaveCurrent(); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(recordLines(t, path)); n != before+1 {
		t.Fatalf("record lines = %d, want %d", n, before+1)
	}
	reloaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Record, resumed.Record) {
		t.Fatalf("reloaded record differs:\n%+v\n%+v", reloaded.Record, resumed.Record)
	}
	raw, _ := rawFixture(t, "v3-record")
	assertMatchesRaw(t, reloaded.Messages[:len(raw)], raw)
}

func TestRecordSurvivesFullRewriteAsOneLine(t *testing.T) {
	sm, path := installCompatFixture(t, "v3-record")
	resumed, err := sm.ResumeLatestSessionWithError()
	if err != nil || resumed == nil {
		t.Fatal(err)
	}
	want := resumed.Record
	resumed.Messages = append([]types.Message(nil), resumed.Messages[6:]...)
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if n := len(recordLines(t, path)); n != 1 {
		t.Fatalf("rewritten file has %d record lines, want 1", n)
	}
	reloaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Record, want) {
		t.Fatal("the rewrite changed the record")
	}
}

func TestRecordLineNeverCountsAsAMessage(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("AGENT_HARNESS_SESSION_DIR", t.TempDir())
	sm, err := NewSessionManagerForProject(filepath.Join(t.TempDir(), "p"))
	if err != nil {
		t.Fatal(err)
	}
	s := sm.CreateSession("m")
	hostile := types.Message{UUID: "u", Role: types.RoleUser, Timestamp: time.Unix(0, 0).UTC(), Content: []types.ContentBlock{
		types.TextBlock{Text: `Never emit {"type":"message"} lines`}}}
	s.AddMessage(hostile)
	s.Record = record.Apply(s.Record, hostile)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Record.Constraints) != 1 || !strings.Contains(s.Record.Goal, `"type":"message"`) {
		t.Fatalf("record does not carry the hostile text: %+v", s.Record)
	}
	lines := recordLines(t, path)
	if len(lines) != 1 || strings.Contains(lines[0], `"type":"message"`) {
		t.Fatalf("record line contains the message marker: %v", lines)
	}
	if n := countMessageEvents(path); n != 1 {
		t.Fatalf("countMessageEvents = %d, want 1", n)
	}
}
