package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/record"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// The read side of the JSONL session store: folding a file back into a
// Session, and listing and finding a project's session files.

// loadSessionFile folds a JSONL file back into a Session.
func loadSessionFile(path string) (*Session, error) {
	s, _, err := loadSessionFileWithDiskRecord(path)
	return s, err
}

// loadSessionFileWithDiskRecord folds a JSONL file into a Session and also
// reports the last record snapshot physically present on disk (before any
// tail replay), so SessionManager can hydrate its journal cache in one read.
func loadSessionFileWithDiskRecord(path string) (*Session, record.Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, record.Record{}, err
	}

	s := &Session{}
	var diskRecord record.Record
	haveRecord, recordCovers := false, 0
	lines := strings.Split(string(data), "\n")
	for lineNumber, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event storeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, record.Record{}, fmt.Errorf("session file %s is corrupt at line %d: %w", filepath.Base(path), lineNumber+1, err)
		}
		switch event.Kind {
		case "header":
			s.ID, s.CreatedAt = event.ID, event.CreatedAt
			s.Model, s.Persona, s.Version = event.Model, event.Persona, event.Version
			s.Turns, s.PlanMode, s.ToolLimit = event.Turns, event.PlanMode, event.ToolLimit
		case "meta":
			if event.Meta == nil {
				return nil, record.Record{}, fmt.Errorf("session file %s has an empty meta event at line %d", filepath.Base(path), lineNumber+1)
			}
			if event.Meta.Model != "" {
				s.Model = event.Meta.Model
			}
			if event.Meta.Persona != "" {
				s.Persona = event.Meta.Persona
			}
			s.Turns = event.Meta.Turns
			s.PlanMode = event.Meta.PlanMode
			s.ToolLimit = event.Meta.ToolLimit
		case "record":
			if event.Record == nil {
				return nil, record.Record{}, fmt.Errorf("session file %s has an empty record at line %d", filepath.Base(path), lineNumber+1)
			}
			s.Record = *event.Record
			diskRecord = *event.Record
			haveRecord = true
			// A line without msgs (written before the field existed) covers
			// every message read so far.
			recordCovers = len(s.Messages)
			if event.Msgs != nil && *event.Msgs >= 0 && *event.Msgs < recordCovers {
				recordCovers = *event.Msgs
			}
		case "message":
			if event.Message == nil {
				return nil, record.Record{}, fmt.Errorf("session file %s has an empty message at line %d", filepath.Base(path), lineNumber+1)
			}
			var msg types.Message
			if err := json.Unmarshal(*event.Message, &msg); err != nil {
				return nil, record.Record{}, fmt.Errorf("session file %s has a corrupt message at line %d: %w", filepath.Base(path), lineNumber+1, err)
			}
			s.Messages = append(s.Messages, msg)
		}
	}
	if haveRecord {
		// Messages written after the last snapshot (a save that died before
		// its record line) are folded onto it.
		for _, msg := range s.Messages[recordCovers:] {
			s.Record = record.Apply(s.Record, msg)
		}
	}
	if s.ID == "" {
		return nil, record.Record{}, fmt.Errorf("session file %s has no header", filepath.Base(path))
	}
	s.UpdatedAt = fileModTime(path)
	return s, diskRecord, nil
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
