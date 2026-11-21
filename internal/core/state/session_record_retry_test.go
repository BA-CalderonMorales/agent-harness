package state

import (
	"encoding/json"
	"errors"
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

// An append that fails after writing its messages must leave the file as it
// found it. Otherwise the retry appends the same messages again, the file
// holds duplicate message events, and a loader replays them over a snapshot
// that already counted them (a file touch counted twice).
func TestFailedAppendRollsBackSoRetryNeverDuplicates(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	sm.appendFault = func() error { return errors.New("disk full") }
	if _, err := sm.SaveCurrent(); err == nil {
		t.Fatal("the injected fault did not fail the save")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("a failed append changed the file (%d bytes before, %d after)", len(before), len(after))
	}

	sm.appendFault = nil
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("the retry failed: %v", err)
	}
	if physical := messageLineCount(t, path); physical != len(s.Messages) {
		t.Fatalf("physical message lines = %d, want %d: the retry duplicated messages", physical, len(s.Messages))
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Record, s.Record) {
		t.Fatalf("loaded record differs from the live one:\n got %#v\nwant %#v", loaded.Record, s.Record)
	}
}

// The combination CodeRabbit raised: a failed append, then a retry on a file
// that is writable but unreadable, then the file becomes readable again and
// is loaded before another save. No duplicates and no double counting.
func TestFailedAppendThenUnreadableRetryLoadsCleanly(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a writable but unreadable file, which this environment cannot make")
	}
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	sm.appendFault = func() error { return errors.New("disk full") }
	if _, err := sm.SaveCurrent(); err == nil {
		t.Fatal("the injected fault did not fail the save")
	}
	sm.appendFault = nil

	if err := os.Chmod(path, 0o200); err != nil {
		t.Fatal(err)
	}
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("the unreadable retry must not fail: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if physical := messageLineCount(t, path); physical != len(s.Messages) {
		t.Fatalf("physical message lines = %d, want %d", physical, len(s.Messages))
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Record, s.Record) {
		t.Fatalf("a load between saves double-counted evidence:\n got %#v\nwant %#v", loaded.Record, s.Record)
	}
}

// If the rollback itself fails, the file may end in the partial events of the
// failed append. The save must say so, and the next save must rewrite the file
// from memory instead of appending to a damaged tail: no duplicate message
// lines, and a loaded record equal to the live one.
func TestFailedRollbackForcesARewriteOnTheNextSave(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	sm.appendFault = func() error { return errors.New("disk full") }
	sm.truncateFault = func() error { return errors.New("rollback refused") }
	_, err = sm.SaveCurrent()
	if err == nil {
		t.Fatal("the injected fault did not fail the save")
	}
	if !strings.Contains(err.Error(), "disk full") || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("the error must report both the failed append and the failed rollback, got: %v", err)
	}
	if got := messageLineCount(t, path); got <= 2 {
		t.Fatalf("the fault left the file clean (%d message lines), so this test checks nothing", got)
	}

	sm.appendFault, sm.truncateFault = nil, nil
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatalf("the repairing save failed: %v", err)
	}
	if physical := messageLineCount(t, path); physical != len(s.Messages) {
		t.Fatalf("physical message lines = %d, want %d: the repair appended to a damaged tail", physical, len(s.Messages))
	}
	loaded, err := loadSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Record, s.Record) {
		t.Fatalf("loaded record differs from the live one:\n got %#v\nwant %#v", loaded.Record, s.Record)
	}
	if sm.needsRewrite {
		t.Fatal("the rewrite flag stayed set after a successful repair")
	}

	// And the save after that is an ordinary append again.
	extra := laterEvidence()[0]
	s.AddMessage(extra)
	s.Record = record.Apply(s.Record, extra)
	if _, err := sm.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if physical := messageLineCount(t, path); physical != len(s.Messages) {
		t.Fatalf("physical message lines = %d, want %d after the next append", physical, len(s.Messages))
	}
}

// Two app instances in one project resume the same session file. If this
// save fails after another writer has appended its own save, rolling back to
// the pre-save size would erase the other writer's data. The rollback must
// notice that the file grew by more than this save wrote and leave it alone.
func TestRollbackNeverErasesAnotherWritersAppend(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}
	const otherWriter = `{"type":"meta","meta":{"model":"saved-by-another-instance"}}` + "\n"
	sm.appendFault = func() error {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if _, err := f.WriteString(otherWriter); err != nil {
			return err
		}
		return errors.New("disk full")
	}
	_, err = sm.SaveCurrent()
	if err == nil {
		t.Fatal("the injected fault did not fail the save")
	}
	if !strings.Contains(err.Error(), "not rolled back") {
		t.Fatalf("the error must say the file was not rolled back, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "saved-by-another-instance") {
		t.Fatal("the rollback erased another writer's save")
	}
}

// A rewrite (compaction, or the repair after a failed rollback) replaces the
// only copy of the session. If it fails partway, the saved messages must
// still be there: the replacement is written to a temp file and renamed over
// the original only once it is complete.
func TestFailedRewriteLeavesTheSavedSessionIntact(t *testing.T) {
	sm, s := newCrashSession(t)
	path, err := sm.SaveCurrent()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range laterEvidence() {
		s.AddMessage(m)
		s.Record = record.Apply(s.Record, m)
	}

	sm.rewriteFault = func() error { return errors.New("disk full") }
	if err := sm.createSessionFile(path); err == nil {
		t.Fatal("the injected fault did not fail the rewrite")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the saved session is gone after a failed rewrite: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("a failed rewrite changed the saved session (%d bytes before, %d after)", len(before), len(after))
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("the failed rewrite left its temp file behind (stat: %v)", err)
	}

	sm.rewriteFault = nil
	if err := sm.createSessionFile(path); err != nil {
		t.Fatalf("the rewrite failed once the fault cleared: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("a successful rewrite left its temp file behind (stat: %v)", err)
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
}
