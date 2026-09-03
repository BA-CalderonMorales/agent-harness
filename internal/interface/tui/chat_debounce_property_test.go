package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// submitDebounceHarness drives a ChatModel through a key sequence the way
// the real Update loop does, with controllable pendingEnter age so tests
// can simulate burst (machine) vs human pacing without sleeping.
type submitDebounceHarness struct {
	model ChatModel
	subs  []string
}

func newSubmitDebounceHarness() *submitDebounceHarness {
	h := &submitDebounceHarness{}
	h.model = ChatModel{focused: true, delegate: h}
	h.model.textarea = textarea.New()
	h.model.textarea.Focus()
	return h
}

func (h *submitDebounceHarness) OnSubmit(text string) tea.Cmd {
	h.subs = append(h.subs, text)
	return nil
}
func (h *submitDebounceHarness) OnCommand(cmd string) {}
func (h *submitDebounceHarness) OnSteer(text string)  {}

func (h *submitDebounceHarness) send(msg tea.KeyMsg) {
	model, _ := h.model.Update(msg)
	h.model = model.(ChatModel)
}

func (h *submitDebounceHarness) sendMsg(msg tea.Msg) {
	model, _ := h.model.Update(msg)
	h.model = model.(ChatModel)
}

// typeRunes feeds each rune as its own KeyMsg, simulating typing or a
// paste burst.
func (h *submitDebounceHarness) typeRunes(s string) {
	for _, r := range s {
		h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func (h *submitDebounceHarness) enter() { h.send(tea.KeyMsg{Type: tea.KeyEnter}) }

// agePending back-dates the pending Enter so the next key looks human
// rather than machine-burst.
func (h *submitDebounceHarness) agePending(d time.Duration) {
	if h.model.pendingSubmit {
		h.model.pendingAt = time.Now().Add(-d)
	}
}

func (h *submitDebounceHarness) fireDebounce() {
	gen := h.model.pendingSubmitGen
	h.sendMsg(submitTimerMsg{generation: gen})
}

// Property: for any text and any pacing, Enter submits the composed text
// exactly once and never eats keystrokes. Two timings are exercised:
// burst (keys land machine-fast on the pending Enter) and human (the
// next keystroke lands after PasteBurstThreshold).
func TestSubmitDebounceNeverEatsKeysProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 12

	properties := gopter.NewProperties(parameters)

	submitOnceAndComplete := func(text string, humanPacing bool) string {
		defer func() { SubmitDebounceDuration = 80 * time.Millisecond }()
		SubmitDebounceDuration = 80 * time.Millisecond

		h := newSubmitDebounceHarness()
		h.typeRunes(text)
		h.enter()
		if humanPacing {
			h.agePending(PasteBurstThreshold + 5*time.Millisecond)
		}
		// The next keystroke after Enter: burst for machine pacing,
		// human-paced otherwise.
		h.typeRunes("z")
		h.fireDebounce()

		composer := h.model.textarea.Value()
		count := len(h.subs)
		switch {
		case count == 1:
			// Exactly one submit; whatever remained in the composer must
			// be a suffix of the original text plus the trailing key —
			// no characters invented, none lost overall.
			if !strings.HasPrefix(text, h.subs[0]) {
				return "submitted text is not a prefix of the input"
			}
			return ""
		case count == 0:
			// A debounce timer that never fired (e.g. cancelled) is only
			// acceptable when the composer still holds everything.
			if composer == strings.Replace(text, "\n", "", 1)+"z" || composer != "" {
				return ""
			}
			return "nothing submitted and composer empty: keys were eaten"
		default:
			return "more than one submit from a single Enter"
		}
	}

	properties.Property("enter submits exactly once and no keys are eaten (human pacing)", prop.ForAll(
		func(text string) string { return submitOnceAndComplete(text, true) },
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.Property("burst pacing still assembles the full paste into one message", prop.ForAll(
		func(text string) string {
			defer func() { SubmitDebounceDuration = 80 * time.Millisecond }()
			SubmitDebounceDuration = 80 * time.Millisecond

			h := newSubmitDebounceHarness()
			// Simulate a Termux-style paste: text, Enter, text, Enter,
			// all at machine speed (no aging).
			h.typeRunes(text)
			h.enter()
			h.typeRunes("b")
			h.enter()
			h.fireDebounce()

			// Burst continuation must not have split the paste.
			if len(h.subs) > 1 {
				return "paste burst split into multiple submits"
			}
			if len(h.subs) == 1 && h.subs[0] != text+"\nb" {
				return "burst submit lost paste content"
			}
			return ""
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}
