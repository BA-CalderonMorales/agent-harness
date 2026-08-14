// Session management with save/restore/compaction
// Inspired by claw-code's session handling

package state

import (
	"encoding/json"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/pkg/messages"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	Persona   string          `json:"persona"`
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
	// Compaction is skipped when this is nil or cannot produce a summary.
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
		Persona:   "developer",
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
	return messages.EstimateTokens(s.Messages)
}

// SaveToFile saves the session to a file
func (s *Session) SaveToFile(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}
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
	if config.Summarizer == nil {
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

	summarized, err := config.Summarizer(s.Messages[:startIdx])
	if err != nil || strings.TrimSpace(summarized) == "" {
		return &CompactionResult{
			RemovedCount:     0,
			KeptCount:        currentCount,
			Skipped:          true,
			CompactedSession: s,
		}
	}
	summaryText := fmt.Sprintf("[Earlier conversation summarized]: %s", summarized)
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

	newSession := *s
	newSession.UpdatedAt = time.Now()
	newSession.Messages = keptMessages
	newSession.Version++

	return &CompactionResult{
		RemovedCount:     currentCount - preserveCount,
		KeptCount:        len(keptMessages),
		Skipped:          false,
		CompactedSession: &newSession,
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
		Persona:   s.Persona,
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
