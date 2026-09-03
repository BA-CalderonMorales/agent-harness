package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSecretPassThrough(t *testing.T) {
	if got, err := ResolveSecret("sk-plain"); err != nil || got != "sk-plain" {
		t.Fatalf("plain value mangled: %q, %v", got, err)
	}
	if got, err := ResolveSecret(""); err != nil || got != "" {
		t.Fatalf("empty value mangled: %q, %v", got, err)
	}
}

func TestResolveSecretEnvBackend(t *testing.T) {
	t.Setenv("AH_TEST_SECRET", "from-env")
	got, err := ResolveSecret("secret://env:AH_TEST_SECRET")
	if err != nil {
		t.Fatalf("env resolve: %v", err)
	}
	if got != "from-env" {
		t.Fatalf("got %q, want from-env", got)
	}
}

func TestResolveSecretFileBackend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(path, []byte("from-file\n"), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := ResolveSecret("secret://file:" + path)
	if err != nil {
		t.Fatalf("file resolve: %v", err)
	}
	if got != "from-file" {
		t.Fatalf("got %q, want from-file", got)
	}
}

func TestResolveSecretCmdBackend(t *testing.T) {
	got, err := ResolveSecret("secret://cmd:printf 'from-cmd'")
	if err != nil {
		t.Fatalf("cmd resolve: %v", err)
	}
	if got != "from-cmd" {
		t.Fatalf("got %q, want from-cmd", got)
	}
}

func TestResolveSecretUnknownSchemeErrors(t *testing.T) {
	if _, err := ResolveSecret("secret://k8s:my-secret"); err == nil {
		t.Fatal("unknown backend must error, not leak the reference")
	}
	if _, err := ResolveSecret("secret://env:"); err == nil {
		t.Fatal("empty reference must error")
	}
}

func TestIsSecretRef(t *testing.T) {
	if !IsSecretRef("secret://env:X") {
		t.Fatal("reference not detected")
	}
	if IsSecretRef("sk-plain") {
		t.Fatal("false positive on non-references")
	}
}
