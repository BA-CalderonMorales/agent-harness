package tui

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// A render panic must not turn the terminal into a blank screen. The
// session is still alive and the failure is recoverable; say so.
//
// The test runs in a subprocess: the recovered panic is written to the
// global diagnostics ring, and a later test that boots an App would see
// that entry instead of an empty Logs tab.
func TestViewPanicRendersVisibleFailure(t *testing.T) {
	if os.Getenv("AH_TUI_PANIC_CHILD") == "1" {
		assertViewPanicFrame(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestViewPanicRendersVisibleFailure$")
	cmd.Env = append(os.Environ(), "AH_TUI_PANIC_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render-panic child failed: %v\n%s", err, out)
	}
}

func assertViewPanicFrame(t *testing.T) {
	t.Helper()

	oldDir := diag.Dir()
	diag.SetDir(t.TempDir())
	defer diag.SetDir(oldDir)

	app := NewApp()
	app.width, app.height = 80, 24
	app.homeModel = nil // the next render of Home panics

	view := app.View()
	text := SanitizeANSI(view)
	if strings.TrimSpace(text) == "" {
		t.Fatal("render panic returned an empty frame")
	}
	for _, want := range []string{"Render error", "session is safe", "Any key redraws"} {
		if !strings.Contains(text, want) {
			t.Fatalf("failure frame missing %q:\n%s", want, text)
		}
	}
	// Long Windows temp paths wrap at the frame width. Compare visible
	// characters across wrapping, rather than requiring a single line.
	compact := strings.Join(strings.Fields(text), "")
	if !strings.Contains(compact, strings.Join(strings.Fields(diag.Dir()), "")) {
		t.Fatalf("failure frame does not point at the log directory %q:\n%s", diag.Dir(), text)
	}
}
