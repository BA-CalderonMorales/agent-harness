package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

// populate fills each tab with content resembling real use: long
// session titles on Home, a busy transcript in Chat, log entries in
// Logs — the shapes that produced overflowing rows on phone panes.
func populateForFrameTest(t *testing.T, app *App, tab int) {
	t.Helper()
	switch tab {
	case 0: // Home
		app.homeModel.SetSessions([]SessionInfo{
			{ID: "s1", Title: "Fix the border flicker on mobile panes"},
			{ID: "s2", Title: "Investigate provider probe timeouts"},
		})
	case 1: // Chat
		app.chatModel.AddMessage("user", strings.Repeat("please summarize the transcript ", 20))
		app.chatModel.AddMessage("assistant", strings.Repeat("here is the summary of events ", 25))
		app.chatModel.AddMessage("tool", strings.Repeat("tool output detail ", 15))
		app.mode = ModeInsert
		app.chatModel.Focus()
	case 3: // Logs
		app.logsModel.AppendEntry(diagEntryForFrameTest("probe", "log line one"))
		app.logsModel.AppendEntry(diagEntryForFrameTest("probe", "log line two"))
	}
}

func diagEntryForFrameTest(site, msg string) diag.Entry {
	return diag.Entry{Level: "info", Site: site, Message: msg}
}

// TestFrameStableAcrossTabSwitches pins the frame invariant: at any
// pane size, every tab renders exactly the same number of rows and the
// frame borders sit on the same rows. A sub-view that renders one row
// too wide wraps at the terminal — the continuation lands outside the
// border and the frame grows, shifting the bottom chrome. That is the
// mobile border-flicker bug; this test is its regression net.
func TestFrameStableAcrossTabSwitches(t *testing.T) {
	for _, size := range [][2]int{{50, 24}, {60, 30}, {70, 40}} {
		w, h := size[0], size[1]
		var refRows, refBorder int = -1, -1
		for tab := viewID(0); tab < viewCount; tab++ {
			app := NewApp()
			app.Update(tea.WindowSizeMsg{Width: w, Height: h})
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(fmt.Sprintf("%d", int(tab)+1))})
			populateForFrameTest(t, app, int(tab))

			view := app.View()
			lines := strings.Split(view, "\n")

			// Frame borders: first and last rows must be the box edges.
			first := ansi.Strip(lines[0])
			last := ansi.Strip(lines[len(lines)-1])
			if !strings.HasPrefix(first, "┌") || !strings.HasPrefix(last, "└") {
				t.Errorf("w%d h%d tab%d: frame borders missing: first=%q last=%q",
					w, h, tab, first, last)
			}

			// No row may exceed the pane: a wider row wraps and shifts
			// the chrome.
			if bad := overflowLines(view, w); len(bad) > 0 {
				t.Errorf("w%d h%d tab%d: %d row(s) overflow the pane: %v",
					w, h, tab, len(bad), bad)
			}

			borderRow := -1
			for i, l := range lines {
				if strings.Contains(ansi.Strip(l), "─") && i < 6 {
					borderRow = i
					break
				}
			}
			if refRows < 0 {
				refRows, refBorder = len(lines), borderRow
				continue
			}
			if len(lines) != refRows || borderRow != refBorder {
				t.Errorf("w%d h%d tab%d: frame shifted — rows=%d borderRow=%d, want rows=%d borderRow=%d",
					w, h, tab, len(lines), borderRow, refRows, refBorder)
			}
		}
	}
}

// TestStyledSessionRowsFitModelWidth pins Copilot's review finding: the
// list styles pad 2 cells per side (4 total), so the unstyled session
// text must be budgeted for those cells — truncating to m.width-1 left
// the styled row at m.width+3 and the ellipsis was lost to the frame
// clip. Every Home row, styled, must fit the model's width budget.
func TestStyledSessionRowsFitModelWidth(t *testing.T) {
	for _, w := range []int{50, 60} {
		app := NewApp()
		app.Update(tea.WindowSizeMsg{Width: w, Height: 30})
		app.homeModel.SetSessions([]SessionInfo{
			{ID: "s1", Title: "Fix the border flicker on mobile panes for good"},
			{ID: "s2", Title: "Investigate provider probe timeouts", IsActive: true},
		})
		home := app.homeModel.View()
		for i, l := range strings.Split(home, "\n") {
			if lw := ansi.StringWidth(l); lw > app.homeModel.width {
				t.Errorf("w%d row %d width %d > %d: %q", w, i, lw, app.homeModel.width, ansi.Strip(l))
			}
		}
	}
}
