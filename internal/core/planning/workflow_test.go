package planning

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowLoadsActivePlanVerifiesAndCreatesNextDomain(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/agent-harness/PLAN.md", "# Index\n\n- Date: `2026-06-13`\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/GOAL.md", "# Goal\n\n## Today\n\n- [ ] Goal fallback\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/PLAN.md", "# Plan\n\n## Today\n\n- [ ] Implement self-improvement command.\n")

	workflow := Workflow{
		Root: root,
		VerifyFunc: func(context.Context, string) VerificationResult {
			return VerificationResult{Command: "go test ./...", Passed: true}
		},
	}

	result, err := workflow.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ActiveDate != "2026-06-13" {
		t.Fatalf("ActiveDate = %q, want 2026-06-13", result.ActiveDate)
	}
	if result.NextAction != "Implement self-improvement command." {
		t.Fatalf("NextAction = %q", result.NextAction)
	}
	if result.NextDate != "2026-06-14" {
		t.Fatalf("NextDate = %q, want 2026-06-14", result.NextDate)
	}
	if !result.CreatedGoal || !result.CreatedPlan {
		t.Fatalf("created goal/plan = %v/%v, want true/true", result.CreatedGoal, result.CreatedPlan)
	}

	nextGoal := readFile(t, root, "plans/agent-harness/2026-06-14/GOAL.md")
	if !strings.Contains(nextGoal, "Continue: Implement self-improvement command.") {
		t.Fatalf("next goal did not carry action:\n%s", nextGoal)
	}
	nextPlan := readFile(t, root, "plans/agent-harness/2026-06-14/PLAN.md")
	if !strings.Contains(nextPlan, "AH_E2E_OPENROUTER=1") {
		t.Fatalf("next plan missing live e2e gate:\n%s", nextPlan)
	}
}

func TestWorkflowDoesNotOverwriteExistingNextDomain(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/agent-harness/PLAN.md", "# Index\n\n- Date: `2026-06-13`\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/GOAL.md", "# Goal\n\n## Today\n\n- [ ] Current action\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/PLAN.md", "# Plan\n\n## Today\n\n- [ ] Current action\n")
	writeFile(t, root, "plans/agent-harness/2026-06-14/GOAL.md", "keep goal\n")
	writeFile(t, root, "plans/agent-harness/2026-06-14/PLAN.md", "keep plan\n")

	result, err := (Workflow{
		Root: root,
		VerifyFunc: func(context.Context, string) VerificationResult {
			return VerificationResult{Command: "go test ./...", Passed: true}
		},
	}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.CreatedGoal || result.CreatedPlan {
		t.Fatalf("created goal/plan = %v/%v, want false/false", result.CreatedGoal, result.CreatedPlan)
	}
	if got := readFile(t, root, "plans/agent-harness/2026-06-14/GOAL.md"); got != "keep goal\n" {
		t.Fatalf("next goal overwritten: %q", got)
	}
	if got := readFile(t, root, "plans/agent-harness/2026-06-14/PLAN.md"); got != "keep plan\n" {
		t.Fatalf("next plan overwritten: %q", got)
	}
}

func TestWorkflowFallsBackToLatestDatedDomainAndTodayLine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/agent-harness/PLAN.md", "# Index\n")
	writeFile(t, root, "plans/agent-harness/2026-06-12/GOAL.md", "# Old Goal\n\n## Today\n\nOld\n")
	writeFile(t, root, "plans/agent-harness/2026-06-12/PLAN.md", "# Old Plan\n\n## Today\n\nOld\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/GOAL.md", "# Goal\n\n## Today\n\nGoal fallback\n")
	writeFile(t, root, "plans/agent-harness/2026-06-13/PLAN.md", "# Plan\n\nToday\n\nRun deterministic local verification.\n")

	result, err := (Workflow{
		Root: root,
		Now: func() time.Time {
			return time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
		},
		VerifyFunc: func(context.Context, string) VerificationResult {
			return VerificationResult{Command: "go test ./...", Passed: true}
		},
	}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ActiveDate != "2026-06-13" {
		t.Fatalf("ActiveDate = %q, want 2026-06-13", result.ActiveDate)
	}
	if result.NextAction != "Run deterministic local verification." {
		t.Fatalf("NextAction = %q", result.NextAction)
	}
}

func TestDeterministicEnvRemovesLiveProviderInputs(t *testing.T) {
	env := deterministicEnv([]string{
		"PATH=/bin",
		"AH_E2E_OPENROUTER=1",
		"AH_API_KEY=secret",
		"AGENT_HARNESS_API_KEY=secret",
		"OPENROUTER_API_KEY=secret",
		"KEEP=value",
	})

	joined := strings.Join(env, "\n")
	for _, blocked := range []string{
		"AH_E2E_OPENROUTER=1",
		"AH_API_KEY=secret",
		"AGENT_HARNESS_API_KEY=secret",
		"OPENROUTER_API_KEY=secret",
	} {
		if strings.Contains(joined, blocked) {
			t.Fatalf("env retained blocked value %q in:\n%s", blocked, joined)
		}
	}
	if !strings.Contains(joined, "AH_E2E_OPENROUTER=0") {
		t.Fatalf("env missing disabled e2e flag:\n%s", joined)
	}
	if !strings.Contains(joined, "KEEP=value") {
		t.Fatalf("env removed unrelated value:\n%s", joined)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
