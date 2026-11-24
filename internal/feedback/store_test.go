package feedback

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

func sampleRecord() Record {
	return Record{
		Time:           time.Date(2026, 11, 23, 9, 30, 0, 0, time.UTC).Format(time.RFC3339),
		SessionID:      "sess-123",
		Provider:       "ollama",
		Model:          "test-model",
		HarnessVersion: "v0.4.2",
		Category:       CategoryBug,
		Text:           "crash on save",
	}
}

func TestAppendWritesExactlyOneJSONLineWithAgreedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "feedback.jsonl")

	if err := Append(path, sampleRecord()); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("file has %d lines, want 1: %q", len(lines), data)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("line is not newline-terminated: %q", data)
	}

	var got map[string]any
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	wantKeys := []string{"category", "harness_version", "model", "provider", "session_id", "text", "time"}
	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("record keys = %v, want exactly %v", gotKeys, wantKeys)
	}
	if got["category"] != "bug" || got["text"] != "crash on save" || got["session_id"] != "sess-123" ||
		got["provider"] != "ollama" || got["model"] != "test-model" || got["harness_version"] != "v0.4.2" {
		t.Fatalf("record values = %v", got)
	}
	ts, ok := got["time"].(string)
	if !ok {
		t.Fatalf("time is %T, want string", got["time"])
	}
	if parsed, err := time.Parse(time.RFC3339, ts); err != nil || parsed.Location() != time.UTC {
		t.Fatalf("time %q is not RFC3339 UTC (err %v)", ts, err)
	}
}

func TestAppendIsAppendOnlyAndCreatesMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits are not meaningful on windows")
	}
	path := filepath.Join(t.TempDir(), "feedback.jsonl")

	for i := 0; i < 2; i++ {
		if err := Append(path, sampleRecord()); err != nil {
			t.Fatalf("Append() #%d error = %v", i+1, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if n := bytes.Count(data, []byte("\n")); n != 2 {
		t.Fatalf("after two appends file has %d lines, want 2", n)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode = %o, want 0600", mode)
	}
}
