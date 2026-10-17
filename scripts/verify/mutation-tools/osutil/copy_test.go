package osutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFileReplacesAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	if err := os.WriteFile(source, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("long previous contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(source, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "short" {
		t.Fatalf("copy produced %q", contents)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("copy must preserve source permissions")
	}
}

func TestCopyFileReportsErrors(t *testing.T) {
	dir := t.TempDir()
	if err := CopyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "destination")); err == nil {
		t.Fatal("missing source must fail")
	}
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(source, filepath.Join(dir, "missing", "destination")); err == nil {
		t.Fatal("missing destination directory must fail")
	}
}
