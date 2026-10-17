package tui

import (
	"strings"
	"testing"
)

// sizedChat builds a chat pane with a little transcript at a given pane
// height, mirroring what resize hands the model.
func sizedChat(width, height int, thinking bool) ChatModel {
	m := newEmptyChatTestModel()
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.AddMessage("user", "hello there")
	m.AddMessage("assistant", "a short reply")
	m.refreshViewport()
	m.thinking = thinking
	return m
}

// TestChatViewFitsItsPaneHeight pins the budget: the chat pane must
// render inside the rows it was given. It used to floor the transcript at
// five rows by subtraction, which pushed the whole view over budget on a
// short pane and let the app frame's clip eat the working indicator and
// the entire composer.
func TestChatViewFitsItsPaneHeight(t *testing.T) {
	for _, height := range []int{40, 30, 24, 20, 18, 16, 14, 12, 10} {
		for _, thinking := range []bool{false, true} {
			m := sizedChat(100, height, thinking)
			rows := strings.Count(SanitizeANSI(m.View()), "\n") + 1
			if rows > height {
				t.Fatalf("thinking=%v: chat rendered %d rows into a %d-row pane",
					thinking, rows, height)
			}
		}
	}
}

// TestChatComposerSurvivesShortPane pins the priority order: the composer
// is the one thing that must never be clipped, because losing it strands
// the driver with no way to type.
func TestChatComposerSurvivesShortPane(t *testing.T) {
	for _, height := range []int{24, 20, 16, 12, 8} {
		m := sizedChat(100, height, true)
		view := SanitizeANSI(m.View())
		if !strings.Contains(view, "navigate") {
			t.Fatalf("pane height %d: composer mode line was clipped:\n%s", height, view)
		}
	}
}

// TestChatWorkingBandHasPaddingAboveAndBelow pins the requested look:
// the live status line reads as its own band, with a blank row between it
// and the transcript above and the composer rule below.
func TestChatWorkingBandHasPaddingAboveAndBelow(t *testing.T) {
	m := sizedChat(100, 30, true)
	lines := strings.Split(SanitizeANSI(m.View()), "\n")

	idx := -1
	for i, line := range lines {
		if strings.Contains(line, "Working") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no working line in a live turn:\n%s", strings.Join(lines, "\n"))
	}
	if idx == 0 || strings.TrimSpace(lines[idx-1]) != "" {
		t.Fatalf("no blank row above the working band:\n%s", strings.Join(lines, "\n"))
	}
	if idx+1 >= len(lines) || strings.TrimSpace(lines[idx+1]) != "" {
		t.Fatalf("no blank row below the working band:\n%s", strings.Join(lines, "\n"))
	}
}

// TestChatWorkingBandYieldsBeforeTheComposer pins the shedding order: a
// pane too short for both keeps the composer and drops the header and the
// band, rather than overflowing and losing everything below the fold.
func TestChatWorkingBandYieldsBeforeTheComposer(t *testing.T) {
	tall := SanitizeANSI(sizedChat(100, 30, true).View())
	if !strings.Contains(tall, "Agent conversation") {
		t.Fatalf("tall pane dropped the header:\n%s", tall)
	}

	short := SanitizeANSI(sizedChat(100, 8, true).View())
	if strings.Contains(short, "Agent conversation") {
		t.Fatalf("short pane kept the decorative header instead of the composer:\n%s", short)
	}
	if !strings.Contains(short, "navigate") {
		t.Fatalf("short pane dropped the composer:\n%s", short)
	}
}
