package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
)

func TestLoadSecureTightensLooseStorePermissions(t *testing.T) {
	requirePOSIXPerms(t)
	testenv.SetHome(t, t.TempDir())

	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-test", Model: "m"}); err != nil {
		t.Fatalf("SaveSecure() error = %v", err)
	}
	if err := os.Chmod(cm.configPath, 0o644); err != nil {
		t.Fatalf("loosen store: %v", err)
	}

	if _, err := NewCredentialManager().LoadSecure(); err != nil {
		t.Fatalf("LoadSecure() error = %v", err)
	}
	assertPerm(t, cm.configPath, 0o600)
}

func TestLoadSecureTightensLooseMachineKeyPermissions(t *testing.T) {
	requirePOSIXPerms(t)
	testenv.SetHome(t, t.TempDir())

	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-test"}); err != nil {
		t.Fatalf("SaveSecure() error = %v", err)
	}
	if err := os.Chmod(MachineKeyPath(), 0o644); err != nil {
		t.Fatalf("loosen machine key: %v", err)
	}

	if _, err := NewCredentialManager().LoadSecure(); err != nil {
		t.Fatalf("LoadSecure() error = %v", err)
	}
	assertPerm(t, MachineKeyPath(), 0o600)
}

func TestSaveSecureTightensLooseConfigDirectory(t *testing.T) {
	requirePOSIXPerms(t)
	testenv.SetHome(t, t.TempDir())

	cm := NewCredentialManager()
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("loosen dir: %v", err)
	}

	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-test"}); err != nil {
		t.Fatalf("SaveSecure() error = %v", err)
	}
	assertPerm(t, dir, 0o700)
}

func TestLoadSecureRefusesSymlinkedStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on Windows")
	}
	testenv.SetHome(t, t.TempDir())

	cm := NewCredentialManager()
	if err := cm.SaveSecure(&SecureConfig{Provider: "openrouter", APIKey: "sk-test"}); err != nil {
		t.Fatalf("SaveSecure() error = %v", err)
	}
	real := cm.configPath + ".real"
	if err := os.Rename(cm.configPath, real); err != nil {
		t.Fatalf("move store: %v", err)
	}
	if err := os.Symlink(real, cm.configPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := NewCredentialManager().LoadSecure()
	if err == nil {
		t.Fatal("LoadSecure() = nil error, want a symlink refusal")
	}
	if !strings.Contains(err.Error(), "symlinked credential file") {
		t.Fatalf("LoadSecure() error = %v, want a symlink refusal", err)
	}
}

func requirePOSIXPerms(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not enforced on Windows")
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
