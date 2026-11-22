package agent

import (
	"slices"
	"testing"
)

// The live A/B scorer runs only against a real server, so its checks are
// pinned here, offline.

func TestLiveScoreRequiresTheNoPushToMainConstraint(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"The goal is settings persistence on one write path. Never push directly to main. Pending: the regression test and the changelog.", true},
		{"Goal: migrate settings persistence. Constraint: no direct pushes to the main branch. Pending: regression test, changelog.", true},
		{"Goal: settings persistence. We must not push to `main`. Still pending: concurrent save test, changelog entry.", true},
		// "main" appears, but no constraint is stated.
		{"The main goal is settings persistence. The regression test and changelog are pending.", false},
		{"Goal: settings persistence. Pushed the fix to main. Pending: regression test, changelog.", false},
	}
	for _, c := range cases {
		_, missed := liveScoreAnswer(c.answer)
		if got := !slices.Contains(missed, "constraint"); got != c.want {
			t.Errorf("constraint scored %v, want %v, for %q", got, c.want, c.answer)
		}
	}
}
