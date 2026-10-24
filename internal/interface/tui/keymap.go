package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The keymap.
//
// Key handling used to be a pile of switch statements: one for the
// global chords, one for the view switches, one inside the mode
// block. Every new shortcut meant editing a switch, and nothing could
// list what the app actually binds.
//
// The bindings are data now. Each one names its keys, says what it
// does, and carries the guard that decides whether it applies. The
// dispatcher looks them up in order, and /keys prints them, so the
// bindings are discoverable without reading the source.

// keyBinding is one keyboard shortcut.
type keyBinding struct {
	// Keys are the key names as Bubble Tea reports them ("ctrl+p",
	// "1", "esc").
	Keys []string
	// Action is the short name used in help and in /keys.
	Action string
	// Description explains the binding in a sentence.
	Description string
	// When decides whether the binding applies. A nil guard always
	// applies.
	When func(*App) bool
	// Run performs the binding. It reports whether it handled the key.
	Run func(*App) (tea.Cmd, bool)
}

// globalKeyBindings work from anywhere in the app.
var globalKeyBindings = []keyBinding{
	{
		Keys:        []string{"ctrl+p"},
		Action:      "command palette",
		Description: "Open the command palette",
		Run: func(a *App) (tea.Cmd, bool) {
			return func() tea.Msg { return openCommandPaletteMsg{} }, true
		},
	},
	{
		Keys:        []string{":"},
		Action:      "command palette",
		Description: "Open the command palette (navigate mode only)",
		When: func(a *App) bool {
			return a.mode == ModeNormal
		},
		Run: func(a *App) (tea.Cmd, bool) {
			return func() tea.Msg { return openCommandPaletteMsg{} }, true
		},
	},
	{
		Keys:        []string{"ctrl+r"},
		Action:      "reasoning effort",
		Description: "Cycle the reasoning effort",
		Run: func(a *App) (tea.Cmd, bool) {
			if a.onUserCommand != nil {
				a.onUserCommand("/effort", a)
			}
			return nil, true
		},
	},
}

// viewKeyBindings switch between the top-level views.
var viewKeyBindings = []keyBinding{
	{Keys: []string{"ctrl+1", "1"}, Action: "home", Description: "Go to Home", Run: func(a *App) (tea.Cmd, bool) {
		return a.switchView(viewHome), true
	}},
	{Keys: []string{"ctrl+2", "2"}, Action: "chat", Description: "Go to Chat", Run: func(a *App) (tea.Cmd, bool) {
		return a.switchView(viewChat), true
	}},
	{Keys: []string{"ctrl+3", "3"}, Action: "sessions", Description: "Go to Sessions", Run: func(a *App) (tea.Cmd, bool) {
		return a.switchView(viewSessions), true
	}},
	{Keys: []string{"ctrl+4", "4"}, Action: "logs", Description: "Go to Logs", Run: func(a *App) (tea.Cmd, bool) {
		return a.switchView(viewLogs), true
	}},
	{Keys: []string{"ctrl+5", "5"}, Action: "settings", Description: "Go to Settings", Run: func(a *App) (tea.Cmd, bool) {
		return a.switchView(viewSettings), true
	}},
}

// matchBinding finds the first binding that claims the key. A binding
// whose guard fails does not claim it, so the caller keeps looking.
func matchBinding(bindings []keyBinding, msg tea.KeyMsg, a *App) (keyBinding, bool) {
	key := msg.String()
	for _, binding := range bindings {
		if binding.When != nil && !binding.When(a) {
			continue
		}
		for _, candidate := range binding.Keys {
			if candidate == key {
				return binding, true
			}
		}
	}
	return keyBinding{}, false
}

// runBinding runs the first binding that claims the key. It reports
// whether a binding claimed it and whether it handled it: a binding can
// claim a key and decline to act on it, which still consumes the key.
//
// The receiver is a pointer on purpose: a binding mutates the app, and
// handleKeys works on a copy that it returns.
func (a *App) runBinding(bindings []keyBinding, msg tea.KeyMsg) (tea.Cmd, bool, bool) {
	binding, claimed := matchBinding(bindings, msg, a)
	if !claimed {
		return nil, false, false
	}
	cmd, handled := binding.Run(a)
	return cmd, handled, true
}

// handleGlobalBinding runs a global binding if one claims the key.
func (a *App) handleGlobalBinding(msg tea.KeyMsg) (tea.Cmd, bool, bool) {
	return a.runBinding(globalKeyBindings, msg)
}

// handleViewBinding runs a view-switch binding if one claims the key.
func (a *App) handleViewBinding(msg tea.KeyMsg) (tea.Cmd, bool, bool) {
	return a.runBinding(viewKeyBindings, msg)
}

// bindingListing is one row of the /keys report.
type bindingListing struct {
	Keys        string
	Action      string
	Description string
}

// bindingListings describes every binding, sorted by action.
func bindingListings() []bindingListing {
	listings := make([]bindingListing, 0, len(globalKeyBindings)+len(viewKeyBindings)+len(modeKeyBindings)+len(navigateKeyBindings))
	for _, group := range [][]keyBinding{globalKeyBindings, viewKeyBindings, modeKeyBindings, navigateKeyBindings} {
		for _, binding := range group {
			listings = append(listings, bindingListing{
				Keys:        strings.Join(binding.Keys, ", "),
				Action:      binding.Action,
				Description: binding.Description,
			})
		}
	}
	sort.Slice(listings, func(i, j int) bool {
		if listings[i].Action == listings[j].Action {
			return listings[i].Keys < listings[j].Keys
		}
		return listings[i].Action < listings[j].Action
	})
	return listings
}

// BindingRows describes the app's key bindings for the /keys report.
func BindingRows() []string {
	listings := bindingListings()
	rows := make([]string, 0, len(listings))
	for _, listing := range listings {
		rows = append(rows, fmt.Sprintf("%-14s %-18s %s", listing.Keys, listing.Action, listing.Description))
	}
	return rows
}
