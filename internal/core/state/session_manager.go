package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// SessionManager handles session lifecycle. Sessions are stored
// append-only per project (see session_store.go).
type SessionManager struct {
	sessionsDir string
	projectRoot string
	current     *Session

	// appendOffset is how many messages of the current session are
	// already on disk — tracked in memory so a save appends the delta
	// instead of re-reading the whole file. sessionMeta caches list
	// metadata keyed by file stat, so the Sessions tab never re-parses
	// unchanged files.
	appendOffset  int
	journalID     string
	sessionMetas  map[string]metaStamp
}

// metaStamp caches a session file's list metadata against the stat that
// produced it.
type metaStamp struct {
	modTime time.Time
	size    int64
	meta    SessionMetadata
}

// NewSessionManager creates a session manager scoped to the current
// working directory's project.
func NewSessionManager() (*SessionManager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}
	return NewSessionManagerForProject(cwd)
}

// NewSessionManagerForProject creates a session manager scoped to a
// project root: only that project's sessions list, resume, and load.
func NewSessionManagerForProject(root string) (*SessionManager, error) {
	sm := &SessionManager{projectRoot: root}
	sm.sessionsDir = os.Getenv("AGENT_HARNESS_SESSION_DIR")
	if sm.sessionsDir == "" {
		sm.sessionsDir = config.DataSessions()
	}

	if err := os.MkdirAll(sm.sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	sm.migrateLegacySessionFiles()
	return sm, nil
}

// NewSessionManagerWithDir creates a session manager pinned to an
// explicit directory (the SessionDir settings override).
func NewSessionManagerWithDir(dir string) (*SessionManager, error) {
	if dir == "" {
		return NewSessionManager()
	}
	sm := &SessionManager{sessionsDir: dir, projectRoot: dir}
	if err := os.MkdirAll(sm.sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	return sm, nil
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(model string) *Session {
	sm.current = NewSession(model)
	return sm.current
}

// GetCurrent returns the current session
func (sm *SessionManager) GetCurrent() *Session {
	return sm.current
}

// SetCurrent sets the current session
func (sm *SessionManager) SetCurrent(session *Session) {
	sm.current = session
}

// SaveCurrent persists the current session by appending whatever is new
// to its JSONL file. Compaction shrinks the transcript below what is on
// disk, which triggers a full rewrite of the (rare) kind.
func (sm *SessionManager) SaveCurrent() (string, error) {
	if sm.current == nil {
		return "", fmt.Errorf("no active session")
	}

	path := sm.sessionPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}

	// A file that does not exist yet starts with its header + all
	// in-memory messages.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := sm.createSessionFile(path); err != nil {
			return "", err
		}
		sm.appendOffset = len(sm.current.Messages)
		sm.journalID = sm.current.ID
		return path, nil
	}

	// A different session than the one journalled: adopt its on-disk
	// state (one read), then append deltas.
	if sm.journalID != sm.current.ID {
		sm.appendOffset = countMessageEvents(path)
		sm.journalID = sm.current.ID
	}

	// Compaction (or Clear) shrinks the transcript below what is on
	// disk: rewrite. Otherwise append only the delta.
	onDisk := sm.appendOffset
	if onDisk > len(sm.current.Messages) {
		if err := sm.createSessionFile(path); err != nil {
			return "", err
		}
		sm.appendOffset = len(sm.current.Messages)
		return path, nil
	}
	if err := sm.appendSessionEventsFrom(path, onDisk); err != nil {
		return "", err
	}
	sm.appendOffset = len(sm.current.Messages)
	return path, nil
}

// LoadSession loads a session by ID
func (sm *SessionManager) LoadSession(id string) (*Session, error) {
	path, err := sm.findSessionFile(id)
	if err != nil {
		return nil, err
	}
	session, err := loadSessionFile(path)
	if err != nil {
		return nil, err
	}

	sm.current = session
	return session, nil
}

// ReadSession loads a session by ID without changing the active session.
func (sm *SessionManager) ReadSession(id string) (*Session, error) {
	path, err := sm.findSessionFile(id)
	if err != nil {
		return nil, err
	}
	return loadSessionFile(path)
}

// ListSessions lists the current project's sessions. Metadata is cached
// per file stat: unchanged files never get re-parsed, so the Sessions
// tab can refresh as often as it likes.
func (sm *SessionManager) ListSessions() ([]SessionMetadata, error) {
	paths, err := sm.listProjectSessionFiles()
	if err != nil {
		return nil, err
	}
	if sm.sessionMetas == nil {
		sm.sessionMetas = make(map[string]metaStamp)
	}

	sessions := make([]SessionMetadata, 0)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if cached, ok := sm.sessionMetas[path]; ok && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
			sessions = append(sessions, cached.meta)
			continue
		}
		session, err := loadSessionFile(path)
		if err != nil {
			continue // torn or foreign file: skip, never fail the list
		}
		meta := session.GetMetadata()
		sm.sessionMetas[path] = metaStamp{modTime: info.ModTime(), size: info.Size(), meta: meta}
		sessions = append(sessions, meta)
	}

	return sessions, nil
}

// GetSessionsDir returns the sessions directory
func (sm *SessionManager) GetSessionsDir() string {
	return sm.projectSessionsDir()
}

// DeleteSession deletes a session by ID
func (sm *SessionManager) DeleteSession(id string) error {
	// Don't allow deleting the current session
	if sm.current != nil && sm.current.ID == id {
		return fmt.Errorf("cannot delete the active session")
	}

	path, err := sm.findSessionFile(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// GetDefaultSessionPath returns the path for auto-save sessions
func (sm *SessionManager) GetDefaultSessionPath() string {
	return sm.sessionPath()
}

// ResumeLatestSession loads the most recently updated session if one exists.
// Returns the session and true if resumed, nil and false if no sessions found.
func (sm *SessionManager) ResumeLatestSession() (*Session, bool) {
	paths, err := sm.listProjectSessionFiles()
	if err != nil || len(paths) == 0 {
		return nil, false
	}

	// Filenames sort by start time; the newest is last.
	session, err := loadSessionFile(paths[len(paths)-1])
	if err != nil {
		return nil, false
	}

	sm.current = session
	return session, true
}

// FormatSessionReport returns a formatted session report
func (sm *SessionManager) FormatSessionReport() string {
	if sm.current == nil {
		return "No active session"
	}

	meta := sm.current.GetMetadata()

	result := "Session\n"
	result += fmt.Sprintf("  ID               %s\n", meta.ID[:8])
	result += fmt.Sprintf("  Created          %s\n", meta.CreatedAt.Format("2006-01-02 15:04"))
	result += fmt.Sprintf("  Updated          %s\n", meta.UpdatedAt.Format("2006-01-02 15:04"))
	result += fmt.Sprintf("  Messages         %d\n", meta.MessageCount)
	result += fmt.Sprintf("  Turns            %d\n", meta.Turns)
	result += fmt.Sprintf("  Est. tokens      %d\n", meta.EstimatedTokens)
	result += fmt.Sprintf("  Model            %s\n", sm.current.Model)

	return result
}

// FormatCompactReport returns a formatted compaction report
func FormatCompactReport(result *CompactionResult) string {
	if result.Skipped {
		return fmt.Sprintf(`Compact
  Result           skipped
  Reason           Session is already below the compaction threshold
  Messages kept    %d`, result.KeptCount)
	}

	return fmt.Sprintf(`Compact
  Result           compacted
  Messages removed %d
  Messages kept    %d
  Tip              Use /status to review the trimmed session`, result.RemovedCount, result.KeptCount)
}
