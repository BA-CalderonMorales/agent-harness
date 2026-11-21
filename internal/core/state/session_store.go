package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
)

// The session store is append-only JSONL, one file per session, grouped
// by project directory — the layout Pi uses. Messages append as they
// happen (a crash loses at most the last line), files never get
// rewritten (compaction excepts), and the filename carries the start
// time so a plain `ls` reads like a history.

// storeEvent is one JSONL line. Kind discriminates: "header" opens a
// session, "message" appends a message, "meta" updates rolling fields,
// "record" snapshots the structured session record (the last one wins).
// A loader that does not know a kind ignores the line.
type storeEvent struct {
	Kind    string           `json:"type"`
	Message *json.RawMessage `json:"message,omitempty"`
	Meta    *storeMeta       `json:"meta,omitempty"`
	Record  *record.Record   `json:"record,omitempty"`
	// Msgs, on a record line, is how many messages the file held when the
	// snapshot was written. A loader replays any messages after that point,
	// so a crash between a message and its snapshot loses no evidence. Not
	// part of Record: it describes the file, not the session.
	Msgs *int `json:"msgs,omitempty"`

	// Header fields.
	ID        string    `json:"id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Model     string    `json:"model,omitempty"`
	Persona   string    `json:"persona,omitempty"`
	Version   int       `json:"version,omitempty"`
	Turns     int       `json:"turns,omitempty"`
	PlanMode  bool      `json:"plan_mode,omitempty"`
	ToolLimit int       `json:"tool_limit,omitempty"`
}

// storeMeta carries rolling session fields that change mid-session.
type storeMeta struct {
	Model     string `json:"model,omitempty"`
	Persona   string `json:"persona,omitempty"`
	Turns     int    `json:"turns,omitempty"`
	PlanMode  bool   `json:"plan_mode,omitempty"`
	ToolLimit int    `json:"tool_limit,omitempty"`
}

// projectSlug turns a project directory into a filesystem-safe session
// group name. /home/me/proj → home-me-proj.
func projectSlug(cwd string) string {
	if runtime.GOOS == "windows" {
		cwd = strings.ReplaceAll(filepath.ToSlash(cwd), ":", "")
	}
	return strings.ReplaceAll(strings.Trim(cwd, "/"), "/", "-")
}

// sessionFileName is the self-describing store filename: the session's
// start time plus its ID, so `ls` reads like a history.
func sessionFileName(s *Session) string {
	return s.CreatedAt.UTC().Format("2006-01-02T15-04-05") + "_" + s.ID + ".jsonl"
}

// projectSessionsDir returns the per-project session directory.
func (sm *SessionManager) projectSessionsDir() string {
	return filepath.Join(sm.sessionsDir, projectSlug(sm.projectRoot))
}

// sessionPath locates the current session's JSONL file.
func (sm *SessionManager) sessionPath() string {
	if sm.current == nil {
		return ""
	}
	return filepath.Join(sm.projectSessionsDir(), sessionFileName(sm.current))
}

// createSessionFile writes the header line and appends any messages the
// in-memory session already carries.
func (sm *SessionManager) createSessionFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create project sessions directory: %w", err)
	}

	s := sm.current
	events := []storeEvent{{
		Kind:      "header",
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		Model:     s.Model,
		Persona:   s.Persona,
		Version:   s.Version,
		Turns:     s.Turns,
		PlanMode:  s.PlanMode,
		ToolLimit: s.ToolLimit,
	}}
	for i := range s.Messages {
		raw, err := json.Marshal(s.Messages[i])
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		events = append(events, storeEvent{Kind: "message", Message: rawPtr(raw)})
	}
	if !s.Record.IsZero() {
		rec, n := s.Record, len(s.Messages)
		events = append(events, storeEvent{Kind: "record", Record: &rec, Msgs: &n})
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer func() { _ = f.Close() }()
	for _, event := range events {
		if err := writeEvent(f, event); err != nil {
			return err
		}
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync session file: %w", err)
	}
	sm.recordOnDisk = s.Record
	return nil
}

// countMessageEvents counts message events already on disk — the
// boundary between "append the delta" and "rewrite after compaction".
func countMessageEvents(path string) int {
	n, _ := countMessageEventsChecked(path)
	return n
}

// countMessageEventsChecked is countMessageEvents that says when the file
// could not be read, for callers that must not treat that as "no events".
func countMessageEventsChecked(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"type":"message"`) {
			count++
		}
	}
	return count, nil
}

// appendSessionEventsFrom appends message and meta events for everything
// not yet on disk, starting at message index `from`.
func (sm *SessionManager) appendSessionEventsFrom(path string, from int) (retErr error) {
	s := sm.current

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to inspect session file: %w", err)
	}
	size := info.Size()
	out := &countingWriter{w: f}
	defer func() {
		// A failed append leaves the file as it found it. Otherwise the
		// caller's retry (it keeps the old offset) writes the same messages
		// again, and the loader would replay duplicates over a snapshot that
		// already counted them.
		if retErr != nil {
			// Only roll back if this save's own writes are all that was
			// added. If the file grew by anything else, another writer
			// appended to it, and truncating would erase their save; keep
			// it and report instead (a duplicate is recoverable, a lost
			// save is not).
			if now, serr := os.Stat(path); serr == nil && now.Size() != size+out.n {
				retErr = fmt.Errorf("%w (the file changed during the save, so it was not rolled back)", retErr)
				_ = f.Close()
				return
			}
			truncate := func(n int64) error { return truncateThroughWriteHandle(f, path, n) }
			if sm.truncateFault != nil {
				truncate = func(int64) error { return sm.truncateFault() }
			}
			if terr := truncate(size); terr != nil {
				// Partial events may remain, and a plain retry would
				// duplicate them. Say so, and make the next save rewrite
				// the file whole.
				sm.needsRewrite = true
				retErr = fmt.Errorf("%w (the rollback also failed, the next save rewrites the session file: %v)", retErr, terr)
			}
		}
		_ = f.Close()
	}()

	for i := from; i < len(s.Messages); i++ {
		raw, err := json.Marshal(s.Messages[i])
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if err := writeEvent(out, storeEvent{Kind: "message", Message: rawPtr(raw)}); err != nil {
			return err
		}
	}

	wroteRecord := false
	// The record is written only when it changed since the last write, so
	// an unchanged record never costs a second line.
	if !s.Record.IsZero() && !reflect.DeepEqual(s.Record, sm.recordOnDisk) {
		// msgs counts the message events physically in the file, which a
		// retried append can leave above len(s.Messages).
		// If the file cannot be read back, the count is unknown: a snapshot
		// with a guessed msgs would make the loader replay messages the
		// record already holds. Skip it; recordOnDisk stays behind so the
		// next save retries, and the loader's replay covers the gap.
		if n, err := countMessageEventsChecked(path); err == nil {
			rec := s.Record
			if err := writeEvent(out, storeEvent{Kind: "record", Record: &rec, Msgs: &n}); err != nil {
				return err
			}
			wroteRecord = true
		}
	}

	if sm.appendFault != nil {
		if err := sm.appendFault(); err != nil {
			return err
		}
	}

	// Rolling meta rides along on every save: one short line, and the
	// loader folds the latest over.
	meta := storeMeta{Model: s.Model, Persona: s.Persona, Turns: s.Turns, PlanMode: s.PlanMode, ToolLimit: s.ToolLimit}
	if err := writeEvent(out, storeEvent{Kind: "meta", Meta: &meta}); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync session file: %w", err)
	}
	// Remember the record as written only if it was, so a skipped snapshot
	// is retried on the next save.
	if wroteRecord || reflect.DeepEqual(s.Record, sm.recordOnDisk) || s.Record.IsZero() {
		sm.recordOnDisk = s.Record
	}
	return nil
}

func writeEvent(f io.Writer, event storeEvent) error {
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal store event: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("failed to append session event: %w", err)
	}
	return nil
}

func rawPtr(raw json.RawMessage) *json.RawMessage {
	return &raw
}

// migrateLegacySessionFiles converts legacy flat whole-JSON sessions
// into the JSONL layout under the current project. Conversion is a
// move: the .jsonl is verified loadable before the .json is removed.
func (sm *SessionManager) migrateLegacySessionFiles() int {
	legacyPaths, err := filepath.Glob(filepath.Join(sm.sessionsDir, "*.json"))
	if err != nil || len(legacyPaths) == 0 {
		return 0
	}

	dir := sm.projectSessionsDir()
	converted := 0
	for _, legacyPath := range legacyPaths {
		session, err := LoadSession(legacyPath)
		if err != nil {
			continue // not ours / unreadable: leave it alone
		}
		jsonlPath := filepath.Join(dir, sessionFileName(session))
		if _, err := os.Stat(jsonlPath); err == nil {
			continue // already converted
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			continue
		}
		sm.current = session
		if err := sm.createSessionFile(jsonlPath); err != nil {
			sm.current = nil
			continue
		}
		if verified, err := loadSessionFile(jsonlPath); err == nil && verified.ID == session.ID {
			_ = os.Remove(legacyPath)
			converted++
		}
	}
	sm.current = nil
	return converted
}

// truncateThroughWriteHandle cuts the file at path back to size using a fresh
// write handle. The append handle cannot do it everywhere: on Windows a file
// opened with O_APPEND has no right to set its end, so truncating through it
// fails and a rolled-back append would silently stay in the file. The append
// handle is closed first so the two never overlap.
func truncateThroughWriteHandle(appendHandle *os.File, path string, size int64) error {
	_ = appendHandle.Close()
	w, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()
	return w.Truncate(size)
}

// countingWriter counts the bytes that actually reached the file, so a
// rollback can tell whether anything but this save's own writes is there.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
