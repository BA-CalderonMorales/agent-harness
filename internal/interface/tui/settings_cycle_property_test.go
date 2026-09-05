package tui

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property: choice cycling is a rotation, not a lookup with side
// conditions. For any options list and any starting position, cycling
// forward len(options) times returns to the start, and an unknown
// stored value snaps deterministically (forward to the first option,
// backward to the last) instead of disagreeing between Enter and the
// arrows — the P3-4 fallback split.
func TestSettingsChoiceCycleProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	properties := gopter.NewProperties(parameters)

	// rotation states the identity over distinct option lists: cycling
	// through every option returns to the start, both directions.
	rotation := func(options []string, start int) string {
		s := &Setting{Type: "choice", Options: options, Value: options[start]}
		for i := 0; i < len(options); i++ {
			cycleChoice(s, 1)
		}
		if s.Value != options[start] {
			return "forward rotation did not return to start"
		}
		for i := 0; i < len(options); i++ {
			cycleChoice(s, -1)
		}
		if s.Value != options[start] {
			return "backward rotation did not return to start"
		}
		return ""
	}

	runeStr := func(r rune) string { return string(r) }
	distinct := func(n int) gopter.Gen {
		return gen.SliceOfN(n, gen.AlphaChar().Map(runeStr)).
			SuchThat(func(ss []string) bool {
				seen := map[string]bool{}
				for _, s := range ss {
					seen[s] = true
				}
				return len(seen) == n
			})
	}

	properties.Property("cycling through all distinct options is the identity", prop.ForAll(
		rotation,
		distinct(3),
		gen.IntRange(0, 2),
	))

	// duplicates: the cycle walks distinct values, so exactly
	// len(distinct values) steps return to start — a duplicate must
	// never trap the rotation.
	duplicateCycle := func(options []string, start int) string {
		uniq := map[string]bool{}
		for _, o := range options {
			uniq[o] = true
		}
		s := &Setting{Type: "choice", Options: options, Value: options[start]}
		for i := 0; i < len(uniq); i++ {
			cycleChoice(s, 1)
		}
		if s.Value != options[start] {
			return "cycling through the distinct values did not return to start"
		}
		return ""
	}

	properties.Property("duplicate options never trap the cycle", prop.ForAll(
		duplicateCycle,
		gen.SliceOfN(4, gen.AlphaChar().Map(runeStr)),
		gen.IntRange(0, 3),
	))

	// single option: the cycle is a no-op — the value cannot move, and
	// a selector that cannot move must never error or wrap around.
	singleOption := func() string {
		s := &Setting{Type: "choice", Options: []string{"solo"}, Value: "solo"}
		cycleChoice(s, 1)
		if s.Value != "solo" {
			return "single-option forward cycle moved the value"
		}
		cycleChoice(s, -1)
		if s.Value != "solo" {
			return "single-option backward cycle moved the value"
		}
		return ""
	}
	properties.Property("a single-option cycle is a no-op", prop.ForAll(
		singleOption,
	))

	unknownSnap := func(options []string) string {
		s := &Setting{Type: "choice", Options: options, Value: "no-such-value"}
		cycleChoice(s, 1)
		if s.Value != options[0] {
			return "unknown value with forward cycle must land on the first option"
		}
		s.Value = "no-such-value"
		cycleChoice(s, -1)
		if s.Value != options[len(options)-1] {
			return "unknown value with backward cycle must land on the last option"
		}
		return ""
	}

	properties.Property("unknown stored values snap deterministically", prop.ForAll(
		unknownSnap,
		distinct(4),
	))

	properties.TestingRun(t)
}
