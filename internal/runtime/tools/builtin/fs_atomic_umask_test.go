//go:build !windows

package builtin

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteFileAtomicHonorsUmask pins the difference between opening the
// staging file with the requested mode and chmod-ing it afterwards: only
// the former lets the kernel apply the process umask.
func TestWriteFileAtomicHonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	if err := writeFileAtomic(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600 under umask 077", got)
	}
}
