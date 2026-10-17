package tui

import (
	"strings"
	"testing"
)

func TestCompactToolPathsKeepWindowsAndUnixBasenames(t *testing.T) {
	for _, path := range []string{
		`C:\Users\someone\a-very-long-project-directory\README.md`,
		`/home/someone/a-very-long-project-directory/README.md`,
	} {
		if got := compactCommandForWidth(path, 24); !strings.Contains(got, "README.md") {
			t.Errorf("compactCommandForWidth(%q) = %q, missing basename", path, got)
		}
	}
	if got := pathBase(`C:\project\nested\`); got != "nested" {
		t.Errorf("pathBase() = %q, want nested", got)
	}
}
