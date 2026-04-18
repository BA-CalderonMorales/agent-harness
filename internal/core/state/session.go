// Session management with save/restore/compaction
// Inspired by claw-code's session handling

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
)

// Session represents a persistent conversation session
type Session struct {
	ID        string          `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []types.Message `json:"messages"`
	Model     string          `json:"model"`
	Turns     int             `json:"turns"`
	Version   int             `json:"version"`
	PlanMode  bool            `json:"plan_mode"`
}

// SessionMetadata contains lightweight session info
type SessionMetadata struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	MessageCount    int       `json:"message_count"`
	Model           string    `json:"model"`
	Turns           int       `json:"turns"`
	EstimatedTokens int       `json:"estimated_tokens"`
}

// CompactionResult contains the result of a compaction operation
type CompactionResult struct {
	RemovedCount     int
	KeptCount        int
	Skipped          bool
	CompactedSession *Session
}

// CompactionConfig controls how compaction works
type CompactionConfig struct {
	MaxMessages        int
	MaxEstimatedTokens int
	PreserveRecent     int // Always preserve this many recent messages
	// Summarizer optionally summarizes removed messages before dropping them.
	// If nil, a generic compaction notice is inserted instead.
	Summarizer func(messages []types.Message) (string, error)
}

// DefaultCompactionConfig returns a sensible default config
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		MaxMessages:        100,
		MaxEstimatedTokens: 32000,
		PreserveRecent:     10,
	}
}

// NewSession creates a new session
func NewSession(model string) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New().String(),
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  make([]types.Message, 0),
		Model:     model,
		Turns:     0,
		Version:   1,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg types.Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
	if msg.Role == types.RoleUser {
		s.Turns++
	}
}

// GetMetadata returns session metadata
func (s *Session) GetMetadata() SessionMetadata {
	return SessionMetadata{
		ID:              s.ID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		MessageCount:    len(s.Messages),
		Model:           s.Model,
		Turns:           s.Turns,
		EstimatedTokens: s.EstimateTokens(),
	}
}

// EstimateTokens provides a rough token estimate
func (s *Session) EstimateTokens() int {
	total := 0
	for _, msg := range s.Messages {
		for _, block := range msg.Content {
			switch b := block.(type) {
			case types.TextBlock:
				// Rough estimate: 4 chars per token
				total += len(b.Text) / 4
			}
		}
	}
	return total
}

// SaveToFile saves the session to a file
func (s *Session) SaveToFile(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// ExportToMarkdown exports the session as a human-readable Markdown transcript.
func (s *Session) ExportToMarkdown() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Session %s\n\n", s.ID[:8]))
	b.WriteString(fmt.Sprintf("- **Model**: %s\n", s.Model))
	b.WriteString(fmt.Sprintf("- **Created**: %s\n", s.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Turns**: %d\n", s.Turns))
	b.WriteString(fmt.Sprintf("- **Messages**: %d\n\n", len(s.Messages)))

	for _, msg := range s.Messages {
		if msg.Role == "system" {
			continue
		}
		roleTitle := strings.Title(string(msg.Role))
		b.WriteString(fmt.Sprintf("## %s\n\n", roleTitle))
		for _, block := range msg.Content {
			switch v := block.(type) {
			case types.TextBlock:
				b.WriteString(v.Text)
				b.WriteString("\n\n")
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("**Tool**: `%s`\n\n", v.Name))
				inputJSON, _ := json.MarshalIndent(v.Input, "", "  ")
				b.WriteString("```json\n")
				b.WriteString(string(inputJSON))
				b.WriteString("\n```\n\n")
			case types.ToolResultBlock:
				b.WriteString("**Result**:\n\n```\n")
				b.WriteString(v.Content)
				b.WriteString("\n```\n\n")
			}
		}
	}
	return b.String()
}

// LoadSession loads a session from a file
func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// Compact compacts the session based on the provided config
func (s *Session) Compact(config CompactionConfig) *CompactionResult {
	currentTokens := s.EstimateTokens()
	currentCount := len(s.Messages)

	// Check if we need to compact
	if currentCount <= config.MaxMessages && currentTokens <= config.MaxEstimatedTokens {
		return &CompactionResult{
			RemovedCount:     0,
			KeptCount:        currentCount,
			Skipped:          true,
			CompactedSession: s,
		}
	}

	// Calculate how many messages to keep
	preserveCount := config.PreserveRecent
	if preserveCount >= currentCount {
		preserveCount = currentCount / 2
	}

	// Calculate start index for preserved messages
	startIdx := currentCount - preserveCount
	if startIdx < 0 {
		startIdx = 0
	}

	// Keep recent messages and compact older ones
	keptMessages := make([]types.Message, 0, preserveCount+1)

	// Add a compaction summary message
	summaryText := fmt.Sprintf("[Earlier conversation history was compacted. %d messages removed, %d kept]",
		currentCount-preserveCount, preserveCount)
	if config.Summarizer != nil {
		if summarized, err := config.Summarizer(s.Messages[:startIdx]); err == nil && summarized != "" {
			summaryText = fmt.Sprintf("[Earlier conversation summarized]: %s", summarized)
		}
	}
	summaryMsg := types.Message{
		UUID:      uuid.New().String(),
		Role:      types.RoleSystem,
		Timestamp: time.Now(),
		Content: []types.ContentBlock{
			types.TextBlock{Text: summaryText},
		},
	}
	keptMessages = append(keptMessages, summaryMsg)

	// Add the preserved recent messages
	keptMessages = append(keptMessages, s.Messages[startIdx:]...)

	newSession := &Session{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: time.Now(),
		Messages:  keptMessages,
		Model:     s.Model,
		Turns:     s.Turns,
		Version:   s.Version + 1,
	}

	return &CompactionResult{
		RemovedCount:     currentCount - preserveCount,
		KeptCount:        len(keptMessages),
		Skipped:          false,
		CompactedSession: newSession,
	}
}

// Clear creates a new empty session with the same ID
func (s *Session) Clear() *Session {
	return &Session{
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: time.Now(),
		Messages:  make([]types.Message, 0),
		Model:     s.Model,
		Turns:     0,
		Version:   s.Version + 1,
		PlanMode:  s.PlanMode,
	}
}

// GetLastNMessages returns the last n messages
func (s *Session) GetLastNMessages(n int) []types.Message {
	if n >= len(s.Messages) {
		result := make([]types.Message, len(s.Messages))
		copy(result, s.Messages)
		return result
	}
	return s.Messages[len(s.Messages)-n:]
}

// SessionManager handles session lifecycle
type SessionManager struct {
	sessionsDir string
	current     *Session
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	// Check for env var override first
	sessionsDir := os.Getenv("AGENT_HARNESS_SESSION_DIR")
	if sessionsDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		sessionsDir = filepath.Join(home, ".agent-harness", "sessions")
	}

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
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
