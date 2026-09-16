package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// assertFitsPane is the family's core promise: an overlay never exceeds
// the pane it is drawn in. Before the shared frame, the provider list was
// 26 rows tall and a 14-row pane cut the last five providers with no way
// to reach them, and a modal taller than the pane pushed the app frame
// off screen entirely.
func assertFitsPane(t *testing.T, label string, view string, w, h int) {
	t.Helper()
	lines := strings.Split(SanitizeANSI(view), "\n")
	if len(lines) > h {
		t.Fatalf("%s: modal is %d rows in a %d-row pane\n%s", label, len(lines), h, view)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > w {
			t.Fatalf("%s: row %d is %d cells in a %d-cell pane\n%s", label, i, got, w, line)
		}
	}
}

func TestModalFamilyFitsEveryPane(t *testing.T) {
	panes := [][2]int{{100, 30}, {100, 14}, {60, 10}, {40, 8}}

	for _, pane := range panes {
		w, h := pane[0], pane[1]
		label := func(name string) string { return name + " @ " + strconv.Itoa(w) + "x" + strconv.Itoa(h) }

		login := NewLoginDialog()
		login.Open(w, h, NewStoredCredentials(nil, ""))
		assertFitsPane(t, label("login/provider"), login.View(), w, h)
		login.step = LoginStepAPIKey
		login.providerIdx = 2 // anthropic: the longest key journey
		assertFitsPane(t, label("login/apikey"), login.View(), w, h)
		login.step = LoginStepModel
		assertFitsPane(t, label("login/model"), login.View(), w, h)

		picker := NewProviderPicker()
		picker.Open(w, h)
		assertFitsPane(t, label("providerPicker"), picker.View(), w, h)

		models := NewModelPicker()
		models.SetModels(fakeModels(20))
		models.Open(w, h)
		assertFitsPane(t, label("modelPicker"), models.View(w, h), w, h)

		palette := NewCommandPalette()
		palette.SetCommands(fakeCommands(20))
		palette.Open(w, h)
		assertFitsPane(t, label("commandPalette"), palette.View(w, h), w, h)

		export := NewExportPicker()
		export.Open(w, h, fakeSessions(20))
		assertFitsPane(t, label("exportPicker"), export.View(w, h), w, h)
	}
}

// TestModalFamilySharesOneWidth pins the consistency requirement: every
// overlay aims at the same panel width in the same pane, so opening one
// modal after another does not shuffle the frame around.
func TestModalFamilySharesOneWidth(t *testing.T) {
	const w, h = 100, 30

	widths := map[string]int{}
	for _, m := range familyModals(w, h) {
		widths[m.name] = widestRow(m.view)
	}

	want := 0
	for name, got := range widths {
		if want == 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("modal widths disagree: %v (want every panel %d wide)", widths, want)
		}
		_ = name
	}
}

// TestWindowModalItemsKeepsCursorVisible pins the scroll window: the
// row the cursor is on is always inside the window, and content that
// does not fit is announced rather than dropped.
func TestWindowModalItemsKeepsCursorVisible(t *testing.T) {
	items := make([]modalItem, 10)
	for i := range items {
		items[i] = modalItem{lines: []string{"row" + strconv.Itoa(i)}}
	}

	for cursor := 0; cursor < len(items); cursor++ {
		got := windowModalItems(items, cursor, 5)
		joined := strings.Join(got, "\n")
		if !strings.Contains(joined, "row"+strconv.Itoa(cursor)) {
			t.Fatalf("cursor row %d not in window:\n%s", cursor, joined)
		}
		if len(got) > 5 {
			t.Fatalf("window returned %d rows for a 5-row budget:\n%s", len(got), joined)
		}
	}

	// At the top there is nothing above; the window says how much is below.
	top := strings.Join(windowModalItems(items, 0, 5), "\n")
	if strings.Contains(top, "more above") {
		t.Fatalf("top of list claims content above it:\n%s", top)
	}
	if !strings.Contains(top, "more below") {
		t.Fatalf("top of list does not announce the rows below:\n%s", top)
	}
}

// TestWindowModalItemsSpansMultiRowItems pins that an item's lines never
// split across the window boundary: a provider's name and its blurb
// scroll as one unit.
func TestWindowModalItemsSpansMultiRowItems(t *testing.T) {
	items := make([]modalItem, 6)
	for i := range items {
		items[i] = modalItem{lines: []string{"name" + strconv.Itoa(i), "blurb" + strconv.Itoa(i)}}
	}

	got := windowModalItems(items, 3, 6)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "blurb3") {
		t.Fatalf("cursor item's second line was dropped:\n%s", joined)
	}
	for _, row := range got {
		if strings.HasPrefix(row, "blurb") && !strings.Contains(joined, "name"+strings.TrimPrefix(row, "blurb")) {
			t.Fatalf("blurb %q rendered without its name:\n%s", row, joined)
		}
	}
}

// TestLoginSkipsKeyStepForLocalRuntimes pins the fix for a promise the
// wizard broke: the provider list said ollama and flm "need no API key",
// then asked for one.
func TestLoginSkipsKeyStepForLocalRuntimes(t *testing.T) {
	for _, local := range []string{"local", "ollama", "flm"} {
		d := NewLoginDialog()
		d.Open(80, 24, NewStoredCredentials(nil, ""))
		idx := -1
		for i, p := range loginProviders {
			if p == local {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("%s missing from the provider list", local)
		}
		d.providerIdx = idx
		d.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if d.step != LoginStepModel {
			t.Fatalf("%s stopped at step %v, want the model step (no key needed)", local, d.step)
		}
	}
}

// TestLoginOffersKeyAcquisitionSteps pins requirement: every provider
// that needs a key tells the user where to get one.
func TestLoginOffersKeyAcquisitionSteps(t *testing.T) {
	for _, p := range loginProviders {
		steps, ok := keyStepsFor(p)
		if !providerNeedsKey(p) {
			if ok {
				t.Fatalf("%s is local but advertises a key journey", p)
			}
			continue
		}
		if !ok || len(steps.Steps) == 0 || steps.URL == "" {
			t.Fatalf("%s needs a key but has no acquisition steps", p)
		}
	}
}

// ---------------------------------------------------------------------------
// harness helpers
// ---------------------------------------------------------------------------

func widestRow(block string) int {
	widest := 0
	for _, line := range strings.Split(SanitizeANSI(block), "\n") {
		if w := ansi.StringWidth(line); w > widest {
			widest = w
		}
	}
	return widest
}

type familyModal struct {
	name string
	view string
}

func familyModals(w, h int) []familyModal {
	login := NewLoginDialog()
	login.Open(w, h, NewStoredCredentials(nil, ""))
	login.providerIdx = 1 // openai: a provider with a key journey drives the panel width

	picker := NewProviderPicker()
	picker.Open(w, h)

	export := NewExportPicker()
	export.Open(w, h, fakeSessions(3))

	return []familyModal{
		{"login", login.View()},
		{"providerPicker", picker.View()},
		{"exportPicker", export.View(w, h)},
	}
}

func fakeModels(n int) []ModelItem {
	models := make([]ModelItem, n)
	for i := range models {
		models[i] = ModelItem{ID: "model-" + strconv.Itoa(i), Name: "model-" + strconv.Itoa(i)}
	}
	return models
}

func fakeCommands(n int) []CommandInfo {
	cmds := make([]CommandInfo, n)
	for i := range cmds {
		cmds[i] = CommandInfo{Command: "/cmd" + strconv.Itoa(i), Description: "command " + strconv.Itoa(i)}
	}
	return cmds
}

func fakeSessions(n int) []SessionInfo {
	sessions := make([]SessionInfo, n)
	for i := range sessions {
		sessions[i] = SessionInfo{ID: "s" + strconv.Itoa(i), Title: "session " + strconv.Itoa(i)}
	}
	return sessions
}
