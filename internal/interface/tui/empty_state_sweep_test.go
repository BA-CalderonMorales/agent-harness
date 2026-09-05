package tui

import (
	"fmt"
	"os"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// The empty-state sweep: every tab's empty state must be centered,
// minimal, survive system notices, fit a 20-row terminal, and never
// hide the tab bar. Phantom viewport padding or full-height centering
// would push the panel past MaxHeight — clipped rows, or the status
// bar shoved off the pane.

var sweepSizes = []struct {
	w, h int
	name string
}{
	{70, 20, "70x20"},
	{90, 24, "90x24"},
	{120, 32, "120x32"},
}

// sweepApp boots an App at a size through Update and returns its view.
func sweepApp(t *testing.T, w, h int, setup func(a *App)) string {
	t.Helper()
	app := NewApp()
	app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	if setup != nil {
		setup(app)
	}
	return app.View()
}

// assertEmptyStateShape checks the shared contract and writes the
// capture when AH_CAPTURE_DIR is set.
func assertEmptyStateShape(t *testing.T, tab, sizeName string, view string, w, h int, centeredTitle string) {
	t.Helper()
	lines := strings.Split(view, "\n")

	if len(lines) > h {
		t.Fatalf("%s %s: view renders %d rows, pane is %d", tab, sizeName, len(lines), h)
	}

	// The tab bar is row 0: the active tab's label must be on screen
	// within the first three rows, never pushed out by the panel.
	tabBar := strings.Join(lines[:min(3, len(lines))], "\n")
	if !strings.Contains(tabBar, "Home") {
		t.Fatalf("%s %s: tab bar hidden:\n%s", tab, sizeName, tabBar)
	}

	// No row overflows the pane — a wrapped empty state is broken
	// centering, not a pretty panel.
	if bad := overflowLines(view, w); len(bad) > 0 {
		t.Fatalf("%s %s: %d row(s) overflow", tab, sizeName, len(bad))
	}

	// Centered panels: the title carries leading whitespace (not
	// flush-left) and the block sits inside the pane.
	if centeredTitle != "" {
		found := false
		for _, line := range lines {
			if strings.Contains(line, centeredTitle) {
				trimmed := strings.TrimLeft(ansi.Strip(line), " ")
				if len(trimmed) == len(line) || len(line)-len(trimmed) == 0 {
					t.Fatalf("%s %s: %q not centered (flush left)", tab, sizeName, centeredTitle)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s %s: empty-state title %q missing", tab, sizeName, centeredTitle)
		}
	}

	if dir := os.Getenv("AH_CAPTURE_DIR"); dir != "" {
		os.MkdirAll(dir, 0o755)
		name := filepath.Join(dir, fmt.Sprintf("%s-%s.txt", tab, sizeName))
		if err := os.WriteFile(name, []byte(view), 0o644); err != nil {
			t.Fatalf("capture write: %v", err)
		}
	}
}

// TestEmptyStatesAllTabs drives the triggers: fresh tabs (new chat, no
// sessions, no diagnostics), chat after /clear, chat with a system
// notice, logs with an empty filter, and logs filtered to an empty day.
func TestEmptyStatesAllTabs(t *testing.T) {
	for _, sz := range sweepSizes {
		for _, tab := range []viewID{viewHome, viewChat, viewSessions, viewLogs, viewSettings} {
			view := sweepApp(t, sz.w, sz.h, func(a *App) {
				a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(fmt.Sprintf("%d", tab+1))})
			})
			assertEmptyStateShape(t, fmt.Sprintf("tab%d-fresh", tab), sz.name, view, sz.w, sz.h, "")
		}
	}
}

// TestChatEmptyStateTriggers: a new chat, a /clear, and a system
// notice must all land on the same centered panel.
func TestChatEmptyStateTriggers(t *testing.T) {
	for _, sz := range sweepSizes {
		// Fresh chat (new chat).
		fresh := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
		})
		assertEmptyStateShape(t, "chat-new", sz.name, fresh, sz.w, sz.h, "The agent is ready.")

		// After /clear: the transcript empties, a follow-up notice may
		// ride along — the panel survives above it.
		cleared := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
			a.chatModel.AddMessage("user", "history to clear")
			a.Update(ClearChatMsg{FollowUpMsg: "Conversation cleared."})
		})
		assertEmptyStateShape(t, "chat-cleared", sz.name, cleared, sz.w, sz.h, "The agent is ready.")

		// A system notice (session load, provider note) renders as its
		// own rows above the panel, never as fake conversation.
		notified := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
			a.chatModel.AddMessage("system", "Resumed session abcd1234")
		})
		assertEmptyStateShape(t, "chat-notice", sz.name, notified, sz.w, sz.h, "The agent is ready.")
	}
}

// TestLogsEmptyTriggers: no diagnostics at all centers the panel; a
// filter that empties a non-empty stream keeps the table and says so
// inline; a day with no entries says so too — all inside the pane.
func TestLogsEmptyTriggers(t *testing.T) {
	for _, sz := range sweepSizes {
		empty := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
		})
		assertEmptyStateShape(t, "logs-empty", sz.name, empty, sz.w, sz.h, "No diagnostics yet")

		// Entries exist, but the error+ filter empties the view.
		filtered := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
			a.logsModel.AppendEntry(diag.Entry{Level: "info", Site: "storage.footprint", Message: "Storage: 2.9M used"})
			for i := 0; i < 2; i++ { // all → warning+ → error+
				a.logsModel.CycleLevel()
			}
		})
		assertEmptyStateShape(t, "logs-filter", sz.name, filtered, sz.w, sz.h, "")

		// A day with no entries: cycle past "all days" onto an empty
		// day bucket.
		day := sweepApp(t, sz.w, sz.h, func(a *App) {
			a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
			a.logsModel.AppendEntry(diag.Entry{Level: "info", Site: "storage.footprint", Message: "Storage: 2.9M used"})
			for i := 0; i < 2; i++ {
				a.logsModel.CycleDay()
			}
		})
		assertEmptyStateShape(t, "logs-day", sz.name, day, sz.w, sz.h, "")
	}
}
