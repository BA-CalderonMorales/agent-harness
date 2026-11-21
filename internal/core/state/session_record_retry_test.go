package state

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
)

func messageLineCount(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, `{"type":"message"`) {
			n++
		}
	}
	return n
}

// A save whose later write failed is retried from the old append offset, so
// the file can hold more message events than the transcript has messages. The
// record line must count the physical events, or the loader replays the wrong
// tail and double-counts evidence.
func TestRecordMsgsCountsPhysicalMessageEvents(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	oldOffset, oldRecord := sm.appendOffset, sm.recordOnDisk

	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	// The earlier save "failed" after writing its messages: the retry starts
	// from where the failed attempt started.
	sm.appendOffset, sm.recordOnDisk = oldOffset, oldRecord
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}

	physical := messageLineCount(t, path)
	if physical <= len(s.Messages) {
		t.Fatalf("the retry did not duplicate message lines (%d physical, %d in memory)", physical, len(s.Messages))
	}
	lines := recordLines(t, path)
	var last struct {
		Msgs *int `json:"msgs"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil || last.Msgs == nil {
		t.Fatalf("last record line has no msgs: %v", lines[len(lines)-1])
	}
	if *last.Msgs != physical {
		t.Fatalf("msgs = %d, file holds %d message events", *last.Msgs, physical)
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range loaded.Record.Files {
		if f.N != 1 {
			t.Fatalf("file touch double counted: %+v", loaded.Record.Files)
		}
	}
}
