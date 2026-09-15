package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Pasting a large blob collapses to a placeholder token in the
// composer; submitting expands the token back to the full content for
// the model while the transcript keeps the collapsed display.
func TestLargePasteCollapsesAndExpandsOnSubmit(t *testing.T) {
	SubmitDebounceDuration = 0
	defer func() { SubmitDebounceDuration = 80 * time.Millisecond }()

	h := newSubmitDebounceHarness()
	blob := "LOG LINE\n" + strings.Repeat("x", 5000)

	paste := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(blob),
		Paste: true,
	}
	h.sendMsg(paste)

	composer := h.model.textarea.Value()
	if !strings.Contains(composer, "[paste #1") || !strings.Contains(composer, "K]") {
		t.Fatalf("composer = %q, want a collapsed paste token", composer)
	}
	if len(composer) > 200 {
		t.Fatalf("composer still carries the raw blob (%d chars)", len(composer))
	}

	h.enter()
	h.fireDebounce()

	if len(h.subs) != 1 {
		t.Fatalf("submits = %d, want 1", len(h.subs))
	}
	if h.subs[0] != blob {
		t.Fatalf("submitted text lost the paste content: %d chars, want %d", len(h.subs[0]), len(blob))
	}
}

// Small pastes insert verbatim — collapsing is for material, not for a
// sentence.
func TestSmallPasteInsertsVerbatim(t *testing.T) {
	h := newSubmitDebounceHarness()

	h.sendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a short paste"), Paste: true})

	if got := h.model.textarea.Value(); got != "a short paste" {
		t.Fatalf("composer = %q, want the verbatim paste", got)
	}
}

// A multibyte paste below the rune threshold inserts verbatim: 250 CJK
// runes are 750 bytes, which the old byte counting wrongly collapsed.
func TestMultibytePasteBelowThresholdStaysVerbatim(t *testing.T) {
	h := newSubmitDebounceHarness()
	pasted := strings.Repeat("あ", 250)

	h.sendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(pasted), Paste: true})

	if got := h.model.textarea.Value(); got != pasted {
		t.Fatalf("composer = %q, want the verbatim multibyte paste", got)
	}
}

// The transcript preview counts characters, not bytes: 250 CJK runes
// must report 250, not their 750 bytes.
func TestPastePreviewCountsRunes(t *testing.T) {
	got := pastePreview(strings.Repeat("あ", 250))
	if !strings.Contains(got, "250 characters total") {
		t.Fatalf("preview = %q, want a 250-character count", got)
	}
	if strings.Contains(got, "750") {
		t.Fatalf("preview = %q, must not report byte length as characters", got)
	}
}

// Below the display threshold, a byte-heavy multibyte paste shows no
// collapse marker at all: 150 CJK runes are 450 bytes, previously
// mislabeled as over-threshold.
func TestPastePreviewNoMarkerForMultibyteBelowThreshold(t *testing.T) {
	pasted := strings.Repeat("あ", 150)
	if got := pastePreview(pasted); got != pasted {
		t.Fatalf("preview = %q, want the verbatim paste with no marker", got)
	}
}

// The no-bracketed-paste heuristic judges the keystroke jump in runes:
// 10 CJK runes (30 bytes) are typing, not a paste.
func TestHeuristicPasteDetectionCountsRunes(t *testing.T) {
	h := newSubmitDebounceHarness()

	h.sendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("あ", 10))})

	if h.model.pasteDetected {
		t.Fatal("pasteDetected = true for a 10-rune jump, want false")
	}
}

// Abandoning the draft drops stashed pastes: a token typed from a stale
// draft must not resurrect old content.
func TestClearInputDropsPendingPastes(t *testing.T) {
	h := newSubmitDebounceHarness()

	h.sendMsg(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(strings.Repeat("y", 900)),
		Paste: true,
	})

	// Ctrl+C abandons the draft: composer emptied, stashes dropped.
	model, _ := h.model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	h.model = model.(ChatModel)
	h.typeRunes("rebuilt plan")
	h.enter()
	h.fireDebounce()

	if len(h.subs) != 1 || h.subs[0] != "rebuilt plan" {
		t.Fatalf("submitted = %q, want the typed text with no resurrected paste", h.subs)
	}
}
