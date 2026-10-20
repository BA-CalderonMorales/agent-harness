package builtin

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces a file without exposing a partially written
// version and without a predictable temp path.
//
// The old write/replace pair used "<path>.tmp", which two concurrent
// tool calls (or an unattended retry) could interleave, and which an
// attacker able to write in the directory could pre-create as a symlink
// to redirect the write. The temp name is now unique per call and the
// rename replaces any symlink at the destination instead of following
// it.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	// Match the historical file mode (0644) rather than CreateTemp's
	// 0600: callers write project sources, not credentials.
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}
