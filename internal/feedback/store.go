package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Append adds rec to the file at path as one JSON line. The file and its
// directory are created when missing; the file is created 0600 because a
// report can name the project's problems. Existing lines are never touched.
func Append(path string, rec Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode feedback: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create feedback directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open feedback file: %w", err)
	}
	// OpenFile applies the mode only on create; tighten a file that
	// already exists so the 0600 promise holds for every append.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("restrict feedback file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write feedback: %w", err)
	}
	return f.Close()
}
