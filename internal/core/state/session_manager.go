package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// SessionManager handles session lifecycle
type SessionManager struct {
	sessionsDir string
	current     *Session
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	return NewSessionManagerWithDir("")
}

// NewSessionManagerWithDir creates a session manager with a custom directory.
// If dir is empty, falls back to AGENT_HARNESS_SESSION_DIR env var, then to
// the shared data home's sessions directory.
func NewSessionManagerWithDir(dir string) (*SessionManager, error) {
	sessionsDir := dir
	if sessionsDir == "" {
		sessionsDir = os.Getenv("AGENT_HARNESS_SESSION_DIR")
	}
	if sessionsDir == "" {
		sessionsDir = config.DataSessions()
	}

	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionManager{
		sessionsDir: sessionsDir,
	}, nil
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

// SaveCurrent saves the current session
func (sm *SessionManager) SaveCurrent() (string, error) {
	if sm.current == nil {
		return "", fmt.Errorf("no active session")
	}

	path := filepath.Join(sm.sessionsDir, sm.current.ID+".json")
	if err := sm.current.SaveToFile(path); err != nil {
		return "", err
	}

	return path, nil
}

// LoadSession loads a session by ID
func (sm *SessionManager) LoadSession(id string) (*Session, error) {
	path := filepath.Join(sm.sessionsDir, id+".json")
	session, err := LoadSession(path)
	if err != nil {
		return nil, err
	}

	sm.current = session
	return session, nil
}

// ReadSession loads a session by ID without changing the active session.
func (sm *SessionManager) ReadSession(id string) (*Session, error) {
	path := filepath.Join(sm.sessionsDir, id+".json")
	return LoadSession(path)
}

// ListSessions lists all available sessions
func (sm *SessionManager) ListSessions() ([]SessionMetadata, error) {
	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	sessions := make([]SessionMetadata, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(sm.sessionsDir, entry.Name())
		session, err := LoadSession(path)
		if err != nil {
			continue
		}

		sessions = append(sessions, session.GetMetadata())
	}

	return sessions, nil
}

// GetSessionsDir returns the sessions directory
func (sm *SessionManager) GetSessionsDir() string {
	return sm.sessionsDir
}

// DeleteSession deletes a session by ID
func (sm *SessionManager) DeleteSession(id string) error {
	// Don't allow deleting the current session
	if sm.current != nil && sm.current.ID == id {
		return fmt.Errorf("cannot delete the active session")
	}

	path := filepath.Join(sm.sessionsDir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// GetDefaultSessionPath returns the path for auto-save sessions
func (sm *SessionManager) GetDefaultSessionPath() string {
	if sm.current == nil {
		return ""
	}
	return filepath.Join(sm.sessionsDir, sm.current.ID+".json")
}

// ResumeLatestSession loads the most recently updated session if one exists.
// Returns the session and true if resumed, nil and false if no sessions found.
func (sm *SessionManager) ResumeLatestSession() (*Session, bool) {
	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, false
	}

	var latestPath string
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestPath = filepath.Join(sm.sessionsDir, entry.Name())
		}
	}

	if latestPath == "" {
		return nil, false
	}

	session, err := LoadSession(latestPath)
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
