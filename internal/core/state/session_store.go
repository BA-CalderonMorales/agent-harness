package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// The session store is append-only JSONL, one file per session, grouped
// by project directory — the layout Pi uses. Messages append as they
// happen (a crash loses at most the last line), files never get
// rewritten (compaction excepts), and the filename carries the start
// time so a plain `ls` reads like a history.

// storeEvent is one JSONL line. Kind discriminates: "header" opens a
// session, "message" appends a message, "meta" updates rolling fields.
type storeEvent struct {
	Kind    string           `json:"type"`
	Message *json.RawMessage `json:"message,omitempty"`
	Meta    *storeMeta       `json:"meta,omitempty"`

	// Header fields.
	ID        string    `json:"id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Model     string    `json:"model,omitempty"`
	Persona   string    `json:"persona,omitempty"`
	Version   int       `json:"version,omitempty"`
}

// storeMeta carries rolling session fields that change mid-session.
type storeMeta struct {
	Model    string `json:"model,omitempty"`
	Persona  string `json:"persona,omitempty"`
	Turns    int    `json:"turns,omitempty"`
	PlanMode bool   `json:"plan_mode,omitempty"`
}

// projectSlug turns a project directory into a filesystem-safe session
// group name. /home/me/proj → home-me-proj.
func projectSlug(cwd string) string {
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
	}}
	for i := range s.Messages {
		raw, err := json.Marshal(s.Messages[i])
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		events = append(events, storeEvent{Kind: "message", Message: rawPtr(raw)})
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()
	for _, event := range events {
		if err := writeEvent(f, event); err != nil {
			return err
		}
	}
	return nil
}

// countMessageEvents counts message events already on disk — the
// boundary between "append the delta" and "rewrite after compaction".
func countMessageEvents(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"type":"message"`) {
			count++
		}
	}
	return count
}

// appendSessionEventsFrom appends message and meta events for everything
// not yet on disk, starting at message index `from`.
func (sm *SessionManager) appendSessionEventsFrom(path string, from int) error {
	s := sm.current

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	for i := from; i < len(s.Messages); i++ {
		raw, err := json.Marshal(s.Messages[i])
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if err := writeEvent(f, storeEvent{Kind: "message", Message: rawPtr(raw)}); err != nil {
			return err
		}
	}

	// Rolling meta rides along on every save: one short line, and the
	// loader folds the latest over.
	meta := storeMeta{Model: s.Model, Persona: s.Persona, Turns: s.Turns, PlanMode: s.PlanMode}
	if err := writeEvent(f, storeEvent{Kind: "meta", Meta: &meta}); err != nil {
		return err
	}
	return nil
}

func writeEvent(f *os.File, event storeEvent) error {
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

// loadSessionFile folds a JSONL file back into a Session.
func loadSessionFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	s := &Session{}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event storeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // tolerate a torn final line: everything before survives
		}
		switch event.Kind {
		case "header":
			s.ID, s.CreatedAt = event.ID, event.CreatedAt
			s.Model, s.Persona, s.Version = event.Model, event.Persona, event.Version
		case "meta":
			if event.Meta == nil {
				continue
			}
			if event.Meta.Model != "" {
				s.Model = event.Meta.Model
			}
			if event.Meta.Persona != "" {
				s.Persona = event.Meta.Persona
			}
			s.Turns = event.Meta.Turns
			s.PlanMode = event.Meta.PlanMode
		case "message":
			if event.Message == nil {
				continue
			}
			var msg types.Message
			if err := json.Unmarshal(*event.Message, &msg); err == nil {
				s.Messages = append(s.Messages, msg)
			}
		}
	}
	if s.ID == "" {
		return nil, fmt.Errorf("session file %s has no header", filepath.Base(path))
	}
	s.UpdatedAt = fileModTime(path)
	return s, nil
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// listProjectSessionFiles returns the project's JSONL files, oldest
// first.
func (sm *SessionManager) listProjectSessionFiles() ([]string, error) {
	dir := sm.projectSessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// findSessionFile locates a session file by full or prefix ID within
// the project's sessions.
func (sm *SessionManager) findSessionFile(id string) (string, error) {
	paths, err := sm.listProjectSessionFiles()
	if err != nil {
		return "", err
	}
	var matches []string
	for _, path := range paths {
		base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if suffix := strings.SplitN(base, "_", 2); len(suffix) == 2 {
			base = suffix[1]
		}
		if base == id || strings.HasPrefix(base, id) {
			matches = append(matches, path)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("session not found")
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous session id %q matches %d sessions", id, len(matches))
	}
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
