package planning

import (
	"strings"
	"testing"
)

func TestRenderGoalTellsChangesToLandOnMainThroughPullRequest(t *testing.T) {
	goal := renderGoal("2026-06-14", activeDomain{Date: "2026-06-13", NextAction: "Ship it."})

	if strings.Contains(goal, "develop") {
		t.Fatalf("goal template mentions develop, a branch the trunk flow does not have:\n%s", goal)
	}
	want := "- Changes land on `main` through a pull request only after green checks."
	if !strings.Contains(goal, want) {
		t.Fatalf("goal template missing %q:\n%s", want, goal)
	}
}
