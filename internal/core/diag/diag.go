// Package diag provides fail-silent diagnostics logging for errors that
// would otherwise vanish: recovered panics, swallowed persistence
// failures, dropped TUI messages. Entries are JSONL, one file per UTC
// day, so an error seen on screen can be traced back to its source site
// in seconds. The logger never reports its own failures to callers — a
// diagnostics sink must not become a new failure surface.
//
// Entries are leveled (INFO/WARNING/ERROR/PANIC) and carry the exact
// file:line of the diag call, so the Logs tab reads like a Splunk
// stream: what happened, how bad, where from, when.
package diag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
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

// Level labels, Splunk-style: the Logs tab filters and colors on these.
const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"
	LevelPanic   = "panic"
)

// Entry is one diagnostics record.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`             // "info" | "warning" | "error" | "panic"
	Site      string    `json:"site"`              // dot-tagged source, e.g. "tui.app_update.panic"
	Caller    string    `json:"caller,omitempty"`  // exact file:line of the diag call
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

// The ring keeps the most recent entries in memory so the Logs tab can
// render the stream without re-reading files from disk. The sink, when
// set, receives every entry as it is logged (the TUI routes it onto the
// event loop through its drop-safe channel).
var (
	streamMu sync.Mutex
	ring     []Entry
	ringCap  = 500
	sink     func(Entry)
)

// SetSink registers a callback fired for every entry. One sink per
// process; the callback must not block (the TUI's Send drops when full).
func SetSink(fn func(Entry)) {
	streamMu.Lock()
	sink = fn
	streamMu.Unlock()
}

// Recent returns the in-memory entries, oldest first.
func Recent() []Entry {
	streamMu.Lock()
	defer streamMu.Unlock()
	out := make([]Entry, len(ring))
	copy(out, ring)
	return out
}

// Info records a lifecycle event worth seeing in the stream: provider
// readiness, turn completion, exports.
func Info(site, message string) {
	write(Entry{Level: LevelInfo, Site: site, Message: message})
}

// Infof records a formatted info event.
func Infof(site, format string, args ...any) {
	write(Entry{Level: LevelInfo, Site: site, Message: fmt.Sprintf(format, args...)})
}

// Warn records a degraded-but-alive condition: dropped messages, retried
// operations. Warnings are the "look here soon" band.
func Warn(site string, err error) {
	write(Entry{Level: LevelWarning, Site: site, Message: errText(err)})
}

// Warnf records a formatted warning.
func Warnf(site, format string, args ...any) {
	write(Entry{Level: LevelWarning, Site: site, Message: fmt.Sprintf(format, args...)})
}

// Error records a swallowed error with the site that dropped it.
func Error(site string, err error) {
	write(Entry{Level: LevelError, Site: site, Message: errText(err)})
}

// Errorf records a swallowed error with extra site context.
func Errorf(site, format string, args ...any) {
	write(Entry{Level: LevelError, Site: site, Message: fmt.Sprintf(format, args...)})
}

// Panic records a recovered panic with its stack. The recovered value is
// formatted with the same %v rules the recover sites use.
func Panic(site string, recovered any) {
	write(Entry{
		Level:   LevelPanic,
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

// callerOf resolves the diag call site as file:line, relative to the
// module root when the build tree sits under it.
func callerOf() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return ""
	}
	if idx := strings.Index(file, "agent-harness/"); idx >= 0 {
		file = file[idx+len("agent-harness/"):]
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func write(entry Entry) {
	if logDir == "" {
		return
	}
	entry.Timestamp = time.Now().UTC()
	entry.Caller = callerOf()

	// Stream first: the ring and sink must see entries even when the
	// file write below fails (read-only disk, full partition).
	streamMu.Lock()
	ring = append(ring, entry)
	if len(ring) > ringCap {
		ring = ring[len(ring)-ringCap:]
	}
	fn := sink
	streamMu.Unlock()
	if fn != nil {
		fn(entry)
	}

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
