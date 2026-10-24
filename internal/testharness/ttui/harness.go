// Package ttui drives a Bubble Tea model in a test and reports what it
// rendered.
//
// The TUI's own tests assert model state, which is the right level for
// behavior and the wrong level for a terminal: a frame can be clipped,
// wider than the pane, or missing chrome while the model looks perfect.
// This harness renders the real model, records every view, and lets a
// test assert on the text a terminal would have received.
//
// It deliberately does not import the TUI package. The harness is
// generic over tea.Model so the TUI's own tests can use it without an
// import cycle.
package ttui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Harness wraps a model and records the views it produces.
type Harness struct {
	t     *testing.T
	model *recorder
}

// recorder is the model wrapper: it forwards messages and keeps every
// view the model rendered.
type recorder struct {
	tea.Model
	views []string
}

func (r *recorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := r.Model.Update(msg)
	if updated != nil {
		r.Model = updated
	}
	r.views = append(r.views, r.Model.View())
	return r, cmd
}

func (r *recorder) View() string {
	return r.Model.View()
}

// New creates a harness around a model.
func New(t *testing.T, model tea.Model) *Harness {
	t.Helper()
	if model == nil {
		t.Fatal("ttui.New: model is nil")
	}
	return &Harness{t: t, model: &recorder{Model: model}}
}

// Send delivers a message to the model. The recorder keeps the updated
// model and stores the view it rendered.
func (h *Harness) Send(msg tea.Msg) {
	h.t.Helper()
	h.model.Update(msg)
}

// Key sends one key message.
func (h *Harness) Key(key tea.KeyMsg) {
	h.t.Helper()
	h.Send(key)
}

// Press sends a named key: "enter", "esc", "tab", "up", "down", "left",
// "right", "backspace", "space", "ctrl+c".
func (h *Harness) Press(name string) {
	h.t.Helper()
	key, ok := namedKey(name)
	if !ok {
		h.t.Fatalf("ttui.Press: unknown key %q", name)
	}
	h.Key(key)
}

// Type sends each rune of text as its own key press.
func (h *Harness) Type(text string) {
	h.t.Helper()
	for _, r := range text {
		h.Key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// Resize tells the model the pane changed size.
func (h *Harness) Resize(width, height int) {
	h.t.Helper()
	h.Send(tea.WindowSizeMsg{Width: width, Height: height})
}

// View returns the most recent rendered view, ANSI sequences included.
func (h *Harness) View() string {
	h.t.Helper()
	if len(h.model.views) == 0 {
		return ""
	}
	return h.model.views[len(h.model.views)-1]
}

// Text returns the most recent rendered view with ANSI sequences
// removed, which is what a terminal would show as characters.
func (h *Harness) Text() string {
	return ansi.Strip(h.View())
}

// Frames returns every rendered view, ANSI stripped.
func (h *Harness) Frames() []string {
	frames := make([]string, 0, len(h.model.views))
	for _, view := range h.model.views {
		frames = append(frames, ansi.Strip(view))
	}
	return frames
}

// AssertContains fails unless the rendered text contains every fragment.
func (h *Harness) AssertContains(fragments ...string) {
	h.t.Helper()
	text := h.Text()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			h.t.Fatalf("rendered text is missing %q:\n%s", fragment, text)
		}
	}
}

// AssertNotContains fails if the rendered text contains any fragment.
func (h *Harness) AssertNotContains(fragments ...string) {
	h.t.Helper()
	text := h.Text()
	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			h.t.Fatalf("rendered text unexpectedly contains %q:\n%s", fragment, text)
		}
	}
}

// AssertFitsWidth fails when any rendered line is wider than the pane.
// A line that overflows the frame is the clipping bug the repo's own
// frame invariant is meant to prevent.
func (h *Harness) AssertFitsWidth(width int) {
	h.t.Helper()
	text := h.Text()
	for i, line := range strings.Split(text, "\n") {
		if got := ansi.StringWidth(line); got > width {
			h.t.Fatalf("rendered line %d is %d wide, want <= %d:\n%s", i+1, got, width, line)
		}
	}
}

// namedKey maps a name to a key message.
func namedKey(name string) (tea.KeyMsg, bool) {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}, true
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}, true
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}, true
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}, true
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}, true
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}, true
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}, true
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}, true
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}, true
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}, true
	case "ctrl+w":
		return tea.KeyMsg{Type: tea.KeyCtrlW}, true
	default:
		return tea.KeyMsg{}, false
	}
}
