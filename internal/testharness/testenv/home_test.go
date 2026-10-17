package testenv

import (
	"os"
	"testing"
)

func TestSetHomeRedirectsPlatformLookup(t *testing.T) {
	dir := t.TempDir()
	SetHome(t, dir)
	got, err := os.UserHomeDir()
	if err != nil || got != dir {
		t.Fatalf("UserHomeDir() = (%q, %v), want %q", got, err, dir)
	}
}
