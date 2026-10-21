package builtin

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// writeFileAtomic replaces a file without exposing a partially written
// version and without a predictable temp path.
//
// The old write/replace pair used "<path>.tmp", which two concurrent
// tool calls (or an unattended retry) could interleave, and which an
// attacker able to write in the directory could pre-create as a symlink
// to redirect the write. The staging name is now unique per call and the
// rename replaces any symlink at the destination instead of following
// it.
//
// The staging file is opened with the caller's mode so the kernel applies
// the process umask exactly as the previous os.WriteFile did. Widening
// the mode after creation would hand back 0644 under a 077 umask.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpPath, tmp, err := createStagingFile(dir, filepath.Base(path), perm)
	if err != nil {
		return err
	}
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := renameReplacing(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}

// createStagingFile creates a uniquely named, exclusively owned file in
// dir. O_EXCL keeps a planted symlink or existing file from being
// followed or truncated.
func createStagingFile(dir, base string, perm os.FileMode) (string, *os.File, error) {
	for attempt := 0; attempt < 10; attempt++ {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", nil, err
		}
		name := filepath.Join(dir, fmt.Sprintf(".%s.tmp-%x", base, suffix))
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			return name, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("could not create a unique staging file for %s", base)
}

// renameReplacing renames temp onto path. Windows refuses a rename while
// another handle has the destination open (a concurrent writer, an
// indexer, or antivirus), so it retries briefly before giving up. Other
// platforms return their error immediately: POSIX rename is atomic and
// has no equivalent transient sharing violation.
func renameReplacing(tempPath, path string) error {
	const attempts = 10
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if err = os.Rename(tempPath, path); err == nil {
			return nil
		}
		if runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}
