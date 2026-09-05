package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	tea "github.com/charmbracelet/bubbletea"
)

// TestLogsDeleteSelected pins the x flow: the entry under the cursor
// leaves the stream, the selection lands on the row that took its
// place, and the underlying entries shrink by exactly one.
func TestLogsDeleteSelected(t *testing.T) {
	app := logsKeyApp(t, 5)
	logsPress(app, "down")
	logsPress(app, "down") // cursor on site.02

	app.logsModel.DeleteSelected()

	if got := app.logsModel.Count(); got != 4 {
		t.Fatalf("count after delete = %d, want 4", got)
	}
	if got := app.logsModel.visible[app.logsModel.cursor].Site; got != "site.03" {
		t.Fatalf("selection after delete = %q, want site.03 (the row that took its place)", got)
	}
	if got := strings.Count(app.logsModel.View(), "▸"); got != 1 {
		t.Fatalf("marker count after delete = %d, want exactly 1", got)
	}
}

// TestLogsDeleteLastRow pins the bottom clamp: deleting the last row
// walks the cursor back to the new last row instead of escaping.
func TestLogsDeleteLastRow(t *testing.T) {
	app := logsKeyApp(t, 3)
	app.logsModel.CursorBottom()
	app.logsModel.DeleteSelected()

	if got := app.logsModel.Count(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if got := app.logsModel.cursor; got != 1 {
		t.Fatalf("cursor = %d, want 1 (the new last row)", got)
	}
}

// TestLogsClearAll pins X: the stream empties, and a fresh entry
// renders again afterwards.
func TestLogsClearAll(t *testing.T) {
	app := logsKeyApp(t, 4)
	if got := app.logsModel.ClearAll(); got != 4 {
		t.Fatalf("ClearAll returned %d, want 4", got)
	}
	if got := app.logsModel.Count(); got != 0 {
		t.Fatalf("count after clear = %d, want 0", got)
	}
	app.logsModel.AppendEntry(diag.Entry{
		Level: "info", Site: "fresh", Message: "fresh", Timestamp: time.Now(),
	})
	if got := app.logsModel.cursor; got != 0 {
		t.Fatalf("cursor after refill = %d, want 0", got)
	}
}

// TestLogsDayFilter pins the d cycle: distinct days newest first, the
// visible slice shrinks to that day, day and level filters combine,
// and deleting a day's last entry retires the day.
func TestLogsDayFilter(t *testing.T) {
	app := NewApp()
	app.width = 120
	app.height = 32
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	app.logsModel.entries = nil
	app.logsModel.visible = app.logsModel.visible[:0]

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	older := now.AddDate(0, 0, -3)
	for _, at := range []time.Time{now, now, yesterday, older} {
		app.logsModel.AppendEntry(diag.Entry{
			Level: "info", Site: "day.test", Message: "m", Timestamp: at,
		})
	}

	if got := app.logsModel.DayLabel(); got != "all days" {
		t.Fatalf("initial label = %q, want all days", got)
	}

	// The cycle: today (2), yesterday (1), the older day dated, all (4).
	olderLabel := older.Local().Format("Jan 02")
	for step, want := range []struct {
		label string
		n     int
	}{
		{"today", 2},
		{"yesterday", 1},
		{olderLabel, 1},
		{"all days", 4},
	} {
		app.logsModel.CycleDay()
		if got := app.logsModel.DayLabel(); got != want.label {
			t.Fatalf("cycle step %d: label = %q, want %q", step+1, got, want.label)
		}
		if got := len(app.logsModel.visible); got != want.n {
			t.Fatalf("cycle step %d: visible = %d, want %d", step+1, got, want.n)
		}
	}

	// Day + level combine: warnings only, still scoped to the day.
	app.logsModel.AppendEntry(diag.Entry{
		Level: "warning", Site: "day.test", Message: "warn", Timestamp: now,
	})
	app.logsModel.AppendEntry(diag.Entry{
		Level: "warning", Site: "day.test", Message: "old-warn", Timestamp: older,
	})
	app.logsModel.CycleDay() // all days -> today
	app.logsModel.CycleLevel()
	if got := len(app.logsModel.visible); got != 1 {
		t.Fatalf("today+warning visible = %d, want 1 (only today's warning)", got)
	}

	// Deleting the last entry of a day retires the day from the cycle.
	app2 := logsKeyApp(t, 2)
	app2.logsModel.AppendEntry(diag.Entry{
		Level: "info", Site: "solo", Message: "m", Timestamp: yesterday,
	})
	// Cycle to yesterday (the solo entry's day) and delete it.
	for i := 0; i < 6; i++ {
		app2.logsModel.CycleDay()
		if app2.logsModel.DayLabel() == "yesterday" {
			break
		}
	}
	app2.logsModel.DeleteSelected()
	for i := 0; i < 6; i++ {
		app2.logsModel.CycleDay()
		if app2.logsModel.DayLabel() == "yesterday" {
			t.Fatal("yesterday should have retired after its last entry was deleted")
		}
	}
}

// TestLogsDeleteAtAppLevel ties x/X through the real router: the key
// deletes, and other tabs pass x through untouched.
func TestLogsDeleteAtAppLevel(t *testing.T) {
	app := logsKeyApp(t, 3)
	logsPress(app, "x")
	if got := app.logsModel.Count(); got != 2 {
		t.Fatalf("count after app-level x = %d, want 2", got)
	}
	logsPress(app, "X")
	if got := app.logsModel.Count(); got != 0 {
		t.Fatalf("count after app-level X = %d, want 0", got)
	}
	if _, ok := app.logsModel.DeleteSelected(); ok {
		t.Fatal("DeleteSelected on an empty stream must be a no-op")
	}
}

// TestLogsDayLabelDatesOlderDays pins the label convention: today and
// yesterday get names, older days get dates.
func TestLogsDayLabelDatesOlderDays(t *testing.T) {
	app := logsKeyApp(t, 1)
	app.logsModel.AppendEntry(diag.Entry{
		Level: "info", Site: "old", Message: "m",
		Timestamp: time.Now().AddDate(0, 0, -3),
	})
	app.logsModel.CycleDay() // today
	app.logsModel.CycleDay() // the older day
	label := app.logsModel.DayLabel()
	expected := time.Now().AddDate(0, 0, -3).Local().Format("Jan 02")
	if label != expected {
		t.Fatalf("older day label = %q, want %q", label, expected)
	}
}
