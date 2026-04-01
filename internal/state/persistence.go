package state

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// SessionStore handles append-only JSONL persistence.
type SessionStore struct {
	Path string
}

// NewSessionStore creates a store for the given session file path.
func NewSessionStore(path string) *SessionStore {
	return &SessionStore{Path: path}
}

// EnsureDir creates the session directory if needed.
func (s *SessionStore) EnsureDir() error {
	return os.MkdirAll(filepath.Dir(s.Path), 0755)
}

// WriteMessage appends a message to the session log.
// User messages are written blocking for crash recovery.
func (s *SessionStore) WriteMessage(msg types.Message) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, string(line)); err != nil {
		return err
	}
	return f.Sync()
}

// WriteMessagesBlocking writes multiple messages and fsyncs.
func (s *SessionStore) WriteMessagesBlocking(msgs []types.Message) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, msg := range msgs {
		line, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, string(line)); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// ReadMessages loads all messages from the session log.
func (s *SessionStore) ReadMessages() ([]types.Message, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var msgs []types.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var msg types.Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue // Skip corrupted lines
		}
		msgs = append(msgs, msg)
	}
	return msgs, scanner.Err()
}
