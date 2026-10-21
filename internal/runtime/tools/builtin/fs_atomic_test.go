package builtin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")

	if err := writeFileAtomic(path, []byte("first"), 0644); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}
	if err := writeFileAtomic(path, []byte("second"), 0644); err != nil {
		t.Fatalf("writeFileAtomic() second error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want the replacement text", data)
	}
	if runtime.GOOS != "windows" {
		// Compare with the API the helper replaced: os.WriteFile applies
		// the process umask, and so must the staging file.
		control := filepath.Join(dir, "control.txt")
		if err := os.WriteFile(control, []byte("control"), 0644); err != nil {
			t.Fatalf("seed control: %v", err)
		}
		gotInfo, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", path, err)
		}
		wantInfo, err := os.Stat(control)
		if err != nil {
			t.Fatalf("Stat(control) error = %v", err)
		}
		if gotInfo.Mode().Perm() != wantInfo.Mode().Perm() {
			t.Fatalf("mode = %o, want %o (os.WriteFile under the same umask)", gotInfo.Mode().Perm(), wantInfo.Mode().Perm())
		}
	}
	assertNoTempFiles(t, dir)
}

func TestWriteFileAtomicDoesNotFollowDestinationSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("do not touch"), 0644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := writeFileAtomic(link, []byte("replacement"), 0644); err != nil {
		t.Fatalf("writeFileAtomic(symlink) error = %v", err)
	}

	if data, err := os.ReadFile(target); err != nil || string(data) != "do not touch" {
		t.Fatalf("symlink target = %q (err %v), want it untouched", data, err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(link) error = %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("destination is still a symlink, want a regular file")
	}
	if data, err := os.ReadFile(link); err != nil || string(data) != "replacement" {
		t.Fatalf("link content = %q (err %v), want the replacement", data, err)
	}
	assertNoTempFiles(t, dir)
}

func TestWriteFileAtomicConcurrentWritesLeaveNoDebris(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.txt")

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if err := writeFileAtomic(path, []byte(strings.Repeat("x", 1024)), 0644); err != nil {
				t.Errorf("concurrent writeFileAtomic() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) != 1024 {
		t.Fatalf("content length = %d, want a complete 1024-byte write", len(data))
	}
	assertNoTempFiles(t, dir)
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", entry.Name())
		}
	}
}
