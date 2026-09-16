package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestEffortChipNamesItsAxis pins why effort is not a bare value on the
// mode line: "medium" alone names no axis, and the old label glued to
// the value ("effort medium") read as a two-word fragment in a rail of
// single values. The chip states the axis and the level together.
func TestEffortChipNamesItsAxis(t *testing.T) {
	for _, level := range []string{"low", "medium", "high"} {
		chip := effortChip(level)
		if !strings.HasPrefix(chip, "[effort: ") || !strings.HasSuffix(chip, "]") {
			t.Errorf("effortChip(%q) = %q, want an [effort: level] chip", level, chip)
		}
		if !strings.Contains(chip, level) {
			t.Errorf("effortChip(%q) = %q, want the level named", level, chip)
		}
	}
}

// TestEffortLevelsAreVisuallyDistinct pins the showcase: the level must
// read without parsing the word, so two levels rendering identically is
// the regression.
func TestEffortLevelsAreVisuallyDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, level := range []string{"low", "medium", "high"} {
		rendered := effortChip(level)
		if prev, ok := seen[rendered]; ok {
			t.Errorf("levels %q and %q render identically: %q", prev, level, rendered)
		}
		seen[rendered] = level
	}
}

// TestModeLineCarriesEffortChip exercises the whole rail: the chip is on
// the line the user reads, and the line still fits the pane at every
// supported width (the chip is wider than the form it replaced, so the
// budget math has to absorb it).
func TestModeLineCarriesEffortChip(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120} {
		m := NewChatModel()
		m.modeLabel = "navigate"
		m.effort = "high"
		m.width = width

		line := m.renderModeLine()
		if !strings.Contains(line, "effort: high") {
			t.Errorf("width %d: effort chip missing from the mode line: %q", width, line)
		}
		if got := lipgloss.Width(line); got > width {
			t.Errorf("width %d: mode line width = %d, want <= %d: %q", width, got, width, line)
		}
	}
}

// TestModeLineEffortSurvivesCrowding: effort describes the active
// behavior, so it is the last metadata segment to yield. A crowded pane
// drops the selected implementation around it, never the chip.
func TestModeLineEffortSurvivesCrowding(t *testing.T) {
	m := NewChatModel()
	m.modeLabel = "navigate"
	m.persona = "developer"
	m.model = "provider/very-long-model-identifier-name"
	m.provider = "a-very-long-provider-name"
	m.agentMode = "manual"
	m.effort = "medium"
	m.width = 48

	line := m.renderModeLine()
	if !strings.Contains(line, "effort: medium") {
		t.Fatalf("effort chip yielded to optional context: %q", line)
	}
	if got := lipgloss.Width(line); got > m.width {
		t.Fatalf("width %d: mode line width = %d: %q", m.width, got, line)
	}
}
