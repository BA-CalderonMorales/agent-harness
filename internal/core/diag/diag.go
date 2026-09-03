// Package diag provides fail-silent diagnostics logging for errors that
// would otherwise vanish: recovered panics, swallowed persistence
// failures, dropped TUI messages. Entries are JSONL, one file per UTC
// day, so an error seen on screen can be traced back to its source site
// in seconds. The logger never reports its own failures to callers — a
// diagnostics sink must not become a new failure surface.
package diag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// RetentionDays is how many daily log files survive: older dated files
// are pruned when the logger initializes. Log accumulation is the only
// unbounded growth a harness like this has — cap it.
const RetentionDays = 14

// PruneDailyFiles removes dated .log files older than keep days from
// dir. Best effort: removal failures are ignored (a diagnostics helper
// must not create new failure surfaces).
func PruneDailyFiles(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keep)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		stamp, err := time.Parse("2006-01-02", strings.TrimSuffix(name, ".log"))
		if err != nil {
			continue
		}
		if stamp.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// Entry is one diagnostics record.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`             // "panic" | "error"
	Site      string    `json:"site"`              // dot-tagged source, e.g. "tui.app_update.panic"
	Message   string    `json:"message,omitempty"` // error text or panic value
	Detail    string    `json:"detail,omitempty"`  // extra context the site chooses to add
	Stack     string    `json:"stack,omitempty"`   // goroutine stack, panics only
}

// logDir resolves once per process; tests can point it elsewhere.
var logDir string

func init() {
	logDir = config.DataLogs()
	PruneDailyFiles(logDir, RetentionDays)
}

// SetDir overrides the log directory (used by tests).
func SetDir(dir string) {
	logDir = dir
}

// Dir returns the active diagnostics directory.
func Dir() string {
	return logDir
}

// Error records a swallowed error with the site that dropped it.
func Error(site string, err error) {
	write(Entry{Level: "error", Site: site, Message: errText(err)})
}

// Errorf records a swallowed error with extra site context.
func Errorf(site, format string, args ...any) {
	write(Entry{Level: "error", Site: site, Message: fmt.Sprintf(format, args...)})
}

// Panic records a recovered panic with its stack. The recovered value is
// formatted with the same %v rules the recover sites use.
func Panic(site string, recovered any) {
	write(Entry{
		Level:   "panic",
		Site:    site,
		Message: fmt.Sprintf("%v", recovered),
		Stack:   string(debug.Stack()),
	})
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func write(entry Entry) {
	if logDir == "" {
		return
	}
	entry.Timestamp = time.Now().UTC()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return
	}
	path := filepath.Join(logDir, time.Now().UTC().Format("2006-01-02")+".log")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	// Sync is intentionally skipped: diagnostics are best-effort and
	// must stay cheap on the hot paths that call them.
	_, _ = f.WriteString(string(data) + "\n")
}
