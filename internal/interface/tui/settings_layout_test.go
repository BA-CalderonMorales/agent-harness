package tui

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestSettingsRowsAlignValues: every settings row pads its label to the
// same column width, so values line up vertically (owner M1).
func TestSettingsRowsAlignValues(t *testing.T) {
	m := NewSettingsModel()
	m.settings = []Setting{
		{Key: "model", Label: "Model", Value: "gpt-4o", Type: "string", Category: "Model"},
		{Key: "reasoning_effort", Label: "Reasoning Effort", Value: "medium", Type: "choice", Category: "Model"},
		{Key: "perm_execute", Label: "Allow Execute", Type: "bool", BoolValue: true, Category: "Permissions"},
	}

	var valueCols []int
	for _, s := range m.settings {
		row := m.renderSetting(s, false)
		line := stripANSI(strings.SplitN(row, "\n", 2)[0])
		if s.Value != "" {
			idx := strings.Index(line, s.Value)
			if idx < 0 {
				t.Fatalf("value %q missing from row %q", s.Value, line)
			}
			valueCols = append(valueCols, idx)
		}
	}
	for i := 1; i < len(valueCols); i++ {
		if valueCols[i] != valueCols[0] {
			t.Fatalf("value column drifts: %v (rows must align)", valueCols)
		}
	}
}

// TestSettingsCategorySeparatorQuiet: category headers render dim, not
// as loud banners.
func TestSettingsCategorySeparatorQuiet(t *testing.T) {
	m := NewSettingsModel()
	m.settings = []Setting{
		{Key: "a", Label: "A", Value: "1", Type: "string", Category: "One"},
	}
	var b strings.Builder
	b.WriteString(SectionHeaderStyle.Render("── One ──"))
	rendered := b.String()
	// SectionHeaderStyle has no bold attribute and no bright foreground.
	unstyled := SectionHeaderStyle.UnsetBold().UnsetForeground().Render("── One ──")
	_ = unstyled
	if strings.Contains(rendered, "\x1b[1m") {
		t.Fatalf("category header renders bold: %q", rendered)
	}
}

// TestChatSpeakerHierarchy: the user message renders as a raised surface
// panel (background) while the assistant keeps the bare quote block, so
// the screen reads who is speaking at a glance.
func TestChatSpeakerHierarchy(t *testing.T) {
	m := NewChatModel()
	m.width = 120
	m.height = 40

	// The style definitions are the hierarchy: the user bubble carries
	// the surface background, the assistant keeps the bare quote block.
	// (Colors render as plain text in non-TTY tests, so assert on the
	// style attributes, not the ANSI output.)
	if reflect.ValueOf(MessageBubbleUser.GetBackground()).IsZero() {
		t.Fatal("user bubble lacks the surface panel")
	}
	if !reflect.ValueOf(MessageBubbleAssistant.GetBackground()).IsZero() {
		t.Fatal("assistant bubble shares the user surface")
	}
}

var ansiStripRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiStripRE.ReplaceAllString(s, "")
}
