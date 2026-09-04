package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	tea "github.com/charmbracelet/bubbletea"
)

// logsKeyApp builds a live App on the Logs tab with n entries, entering
// the tab the way a user does (the "4" key) through the real dispatch.
func logsKeyApp(t *testing.T, n int) *App {
	t.Helper()
	app := NewApp()
	app.width = 120
	app.height = 32
	// A real boot delivers a WindowSizeMsg; without it the Logs model
	// renders "Loading..." and the table never exists.
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	// NewApp seeds the Logs tab from the diag ring; tests own their
	// stream, so drop whatever earlier tests logged into the ring.
	app.logsModel.entries = nil
	app.logsModel.visible = app.logsModel.visible[:0]
	app.logsModel.cursor = 0
	for i := 0; i < n; i++ {
		app.logsModel.AppendEntry(diag.Entry{
			Level: "info", Site: fmt.Sprintf("site.%02d", i),
			Message: fmt.Sprintf("entry %d", i), Timestamp: time.Now(),
		})
	}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	return app
}

func logsPress(app *App, key string) {
	var msg tea.KeyMsg
	switch key {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	app.Update(msg)
}

// TestLogsTabCursorKeys is the property the user reported broken: on the
// Logs tab itself, j/k and the arrow keys must move the selection — for
// any entry count, any starting position, in every mode the tab can be
// reached in. Each cell drives App.Update (the real router), never the
// model directly: the original bug lived in the routing, and the model-
// level tests could not see it.
func TestLogsTabCursorKeys(t *testing.T) {
	keys := []struct {
		name string
		down string
		up   string
	}{
		{"vim-jk", "j", "k"},
		{"arrows", "down", "up"},
	}
	modes := []struct {
		name   string
		insert bool
	}{
		{"normal", false},
		{"insert", true},
	}

	for _, entryCount := range []int{5, 40} {
		for _, m := range modes {
			for _, k := range keys {
				name := fmt.Sprintf("%s/%s/%d-entries", m.name, k.name, entryCount)
				t.Run(name, func(t *testing.T) {
					app := logsKeyApp(t, entryCount)
					if m.insert {
						logsPress(app, "i")
						logsPress(app, "4")
					}

					if got := app.logsModel.cursor; got != 0 {
						t.Fatalf("start: cursor = %d, want 0", got)
					}

					// Down must walk forward, one row per key.
					for step := 1; step <= 3; step++ {
						logsPress(app, k.down)
						if got := app.logsModel.cursor; got != step {
							t.Fatalf("%s down #%d: cursor = %d, want %d", name, step, got, step)
						}
					}

					// Up must walk back.
					logsPress(app, k.up)
					if got := app.logsModel.cursor; got != 2 {
						t.Fatalf("%s up: cursor = %d, want 2", name, got)
					}
				})
			}
		}
	}
}

// TestLogsTabCursorSurvivesNewEntries pins the snap-back: the selection
// must stay where the user put it when the diagnostics stream appends a
// new entry. The original report was "stuck on the first log" — a
// cursor that jumps back to row 0 on every append reads as exactly that.
func TestLogsTabCursorSurvivesNewEntries(t *testing.T) {
	app := logsKeyApp(t, 10)
	logsPress(app, "down")
	logsPress(app, "down")
	logsPress(app, "down")
	if got := app.logsModel.cursor; got != 3 {
		t.Fatalf("setup: cursor = %d, want 3", got)
	}

	for i := 0; i < 5; i++ {
		app.logsModel.AppendEntry(diag.Entry{
			Level: "info", Site: "late.append", Message: "late", Timestamp: time.Now(),
		})
		if got := app.logsModel.cursor; got != 3 {
			t.Fatalf("after append %d: cursor = %d, want 3 (snap-back)", i+1, got)
		}
	}
}

// TestLogsTabCursorAtEdges pins clamping: j past the last entry and k
// before the first must hold the selection, not panic and not wrap
// silently into an out-of-range index.
func TestLogsTabCursorAtEdges(t *testing.T) {
	app := logsKeyApp(t, 3)
	for i := 0; i < 10; i++ {
		logsPress(app, "down")
	}
	if got := app.logsModel.cursor; got != 2 {
		t.Fatalf("bottom clamp: cursor = %d, want 2", got)
	}
	for i := 0; i < 10; i++ {
		logsPress(app, "up")
	}
	if got := app.logsModel.cursor; got != 0 {
		t.Fatalf("top clamp: cursor = %d, want 0", got)
	}
}

// TestLogsTabEnterOpensSelectedDetail ties the cursor to its purpose:
// Enter must open the detail of the row the cursor is on, not always
// the first row.
func TestLogsTabEnterOpensSelectedDetail(t *testing.T) {
	app := logsKeyApp(t, 6)
	logsPress(app, "down")
	logsPress(app, "down")
	logsPress(app, "enter")
	if got := app.logsModel.DetailOpen(); !got {
		t.Fatal("Enter did not open the detail modal")
	}
	if app.logsModel.detail.Site != "site.02" {
		t.Fatalf("detail site = %q, want site.02 (the row the cursor was on)", app.logsModel.detail.Site)
	}
}

// TestLogsTabCursorRandomWalk is the shrinking property: for a seeded
// random sequence of cursor keys — interleaved with stream appends and
// tab round-trips — the cursor must always stay inside the list and
// equal the clamped net displacement of the keys so far. Any state that
// freezes, resets, or misroutes the selection fails this invariant
// within a few steps, and the seed reproduces it.
func TestLogsTabCursorRandomWalk(t *testing.T) {
	type keyStep struct {
		key   string
		delta int
	}
	steps := []keyStep{
		{"j", 1}, {"down", 1}, {"k", -1}, {"up", -1},
	}
	rng := rand.New(rand.NewSource(20260904))

	for trial := 0; trial < 200; trial++ {
		n := 6 + rng.Intn(20) // stream size
		app := logsKeyApp(t, n)
		want := 0
		for stepIdx := 0; stepIdx < 30; stepIdx++ {
			switch rng.Intn(10) {
			case 0: // stream append mid-walk
				app.logsModel.AppendEntry(diag.Entry{
					Level: "info", Site: "walk.append", Message: "walk", Timestamp: time.Now(),
				})
				n++
			case 1: // tab round-trip: away and back
				logsPress(app, "1")
				logsPress(app, "4")
			default:
				s := steps[rng.Intn(len(steps))]
				logsPress(app, s.key)
				want += s.delta
			}
			if want < 0 {
				want = 0
			}
			if want > n-1 {
				want = n - 1
			}
			if got := app.logsModel.cursor; got != want {
				t.Fatalf("trial %d step %d: cursor = %d, want %d (seed reproduces)",
					trial, stepIdx, got, want)
			}
		}
	}
}

// TestLogsSelectionMarkerVisible pins the theme-proof cue: exactly one
// row carries the ▸ marker, and it is the row the cursor points at —
// on any terminal, any palette. A background-only highlight proved
// invisible on dark themes; this is the regression that must never
// come back.
func TestLogsSelectionMarkerVisible(t *testing.T) {
	app := logsKeyApp(t, 6)
	logsPress(app, "down")
	logsPress(app, "down")

	out := app.logsModel.View()
	rows := strings.Split(out, "\n")
	marked := 0
	for _, row := range rows {
		if strings.Contains(row, "▸") {
			marked++
			if !strings.Contains(row, "site.02") {
				t.Fatalf("marker on the wrong row: %q", row)
			}
		}
	}
	if marked != 1 {
		t.Fatalf("marker count = %d, want exactly 1", marked)
	}
}
