// Package audit provides append-only logging of tool executions and approvals
// for security review and accountability.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry represents a single audited event.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	ToolName    string    `json:"tool_name"`
	InputHash   string    `json:"input_hash"`
	Approved    bool      `json:"approved"`
	Decision    string    `json:"decision"` // "approve", "reject", "approve-all", "auto"
	Persona     string    `json:"persona"`
	PermissionMode string `json:"permission_mode"`
}

// Logger handles audit log writes.
type Logger struct {
	dir string
}

// NewLogger creates a new audit logger.
func NewLogger() (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(home, ".agent-harness", "audit")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}
	return &Logger{dir: dir}, nil
}

// Log records a tool execution event.
func (l *Logger) Log(entry Entry) error {
	entry.Timestamp = time.Now().UTC()
	if entry.InputHash == "" && entry.ToolName != "" {
		entry.InputHash = hashInput(entry.ToolName)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	path := l.currentLogPath()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}
	return nil
}

// Recent returns the last n audit entries.
func (l *Logger) Recent(n int) ([]Entry, error) {
	entries, err := l.loadAll()
	if err != nil {
		return nil, err
	}

	// Sort by timestamp descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if n > len(entries) {
		n = len(entries)
	}
	return entries[:n], nil
}

// loadAll loads all entries from current and previous daily logs.
func (l *Logger) loadAll() ([]Entry, error) {
	files, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".log") {
			continue
		}
		path := filepath.Join(l.dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry Entry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// currentLogPath returns the path for today's log file.
func (l *Logger) currentLogPath() string {
	return filepath.Join(l.dir, time.Now().UTC().Format("2006-01-02")+".log")
}

// hashInput creates a deterministic hash of tool input for privacy.
func hashInput(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// FormatEntries returns a human-readable representation of audit entries.
func FormatEntries(entries []Entry) string {
	if len(entries) == 0 {
		return "No audit entries found."
	}

	var lines []string
	lines = append(lines, "Recent activity:")
	lines = append(lines, "")

	for _, e := range entries {
		status := "✓"
		if !e.Approved {
			status = "✗"
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %-12s %-10s %s",
			status,
			e.Timestamp.Format("15:04:05"),
			e.ToolName,
			e.Decision,
			e.InputHash[:8],
		))
	}

	return strings.Join(lines, "\n")
}
