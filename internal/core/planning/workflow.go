package planning

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultDomain = "agent-harness"
	dateLayout    = "2006-01-02"
)

// Workflow runs the deterministic self-improvement planning loop for a repo.
type Workflow struct {
	Root       string
	Domain     string
	Now        func() time.Time
	VerifyFunc func(context.Context, string) VerificationResult
}

type Result struct {
	Root         string
	ActiveDate   string
	GoalPath     string
	PlanPath     string
	NextAction   string
	Verification VerificationResult
	NextDate     string
	NextGoalPath string
	NextPlanPath string
	CreatedGoal  bool
	CreatedPlan  bool
}

func (w Workflow) Run(ctx context.Context) (Result, error) {
	root := w.Root
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return Result{}, err
		}
	}

	domain := w.Domain
	if domain == "" {
		domain = defaultDomain
	}

	active, err := loadActive(root, domain, w.now())
	if err != nil {
		return Result{}, err
	}

	verify := w.VerifyFunc
	if verify == nil {
		verify = RunDeterministicGoTests
	}
	verification := verify(ctx, root)

	nextDate, nextGoal, nextPlan, createdGoal, createdPlan, err := ensureNextDomain(root, domain, active)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Root:         root,
		ActiveDate:   active.Date,
		GoalPath:     active.GoalPath,
		PlanPath:     active.PlanPath,
		NextAction:   active.NextAction,
		Verification: verification,
		NextDate:     nextDate,
		NextGoalPath: nextGoal,
		NextPlanPath: nextPlan,
		CreatedGoal:  createdGoal,
		CreatedPlan:  createdPlan,
	}, nil
}

func (w Workflow) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func (r Result) Summary() string {
	status := "failed"
	if r.Verification.Passed {
		status = "passed"
	}

	created := make([]string, 0, 2)
	if r.CreatedGoal {
		created = append(created, rel(r.Root, r.NextGoalPath))
	}
	if r.CreatedPlan {
		created = append(created, rel(r.Root, r.NextPlanPath))
	}
	if len(created) == 0 {
		created = append(created, "next dated files already existed")
	}

	return strings.Join([]string{
		fmt.Sprintf("Self-improvement workflow: %s", status),
		fmt.Sprintf("Active domain: %s", r.ActiveDate),
		fmt.Sprintf("Goal: %s", rel(r.Root, r.GoalPath)),
		fmt.Sprintf("Plan: %s", rel(r.Root, r.PlanPath)),
		fmt.Sprintf("Next action: %s", r.NextAction),
		fmt.Sprintf("Verification: %s", r.Verification.Command),
		fmt.Sprintf("Next domain: %s", r.NextDate),
		fmt.Sprintf("Updated: %s", strings.Join(created, ", ")),
	}, "\n")
}

func rel(root, path string) string {
	if root == "" {
		return path
	}
	if relPath, err := filepath.Rel(root, path); err == nil {
		return relPath
	}
	return path
}
