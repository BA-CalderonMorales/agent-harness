package state

import (
	"encoding/json"
	"os"
	"reflect"
	"runtime"
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

// A file the process can append to but not read makes the physical message
// count unavailable. A snapshot with msgs: 0 would make the loader replay the
// whole transcript onto a record that already holds it, so the snapshot must
// be skipped instead; the loader's replay boundary covers the gap.
func TestRecordSnapshotSkippedWhenMessageCountUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a writable but unreadable file, which this environment cannot make")
	}
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	linesBefore := len(recordLines(t, path))

	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	if err := os.Chmod(path, 0o200); err != nil {
		t.Fatal(err)
	}
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("an unreadable count must not fail the save: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := len(recordLines(t, path)); got != linesBefore {
		t.Fatalf("a snapshot was written without a message count (%d record lines, was %d)", got, linesBefore)
	}
	if physical := messageLineCount(t, path); physical != len(s.Messages) {
		t.Fatalf("physical message lines = %d, want %d", physical, len(s.Messages))
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Record, s.Record) {
		t.Fatalf("loaded record differs from the live one:\n got %#v\nwant %#v", loaded.Record, s.Record)
	}

	// The next save, with the file readable again, writes the snapshot.
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if got := len(recordLines(t, path)); got != linesBefore+1 {
		t.Fatalf("the snapshot was not retried once the file was readable (%d record lines, want %d)", got, linesBefore+1)
	}
}
