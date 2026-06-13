package planning

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultDomain = "agent-harness"
	dateLayout    = "2006-01-02"
)

var dateLinePattern = regexp.MustCompile("(?m)^-\\s*Date:\\s*`([0-9]{4}-[0-9]{2}-[0-9]{2})`\\s*$")

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

type VerificationResult struct {
	Command string
	Passed  bool
	Output  string
	Error   error
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

type activeDomain struct {
	Date       string
	GoalPath   string
	PlanPath   string
	Goal       string
	Plan       string
	NextAction string
}

func loadActive(root, domain string, now time.Time) (activeDomain, error) {
	base := filepath.Join(root, "plans", domain)
	indexPath := filepath.Join(base, "PLAN.md")

	index, err := os.ReadFile(indexPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read plan index: %w", err)
	}

	date := activeDateFromIndex(string(index))
	if date == "" {
		date, err = latestDatedDomain(base)
		if err != nil {
			return activeDomain{}, err
		}
	}
	if date == "" {
		date = now.Format(dateLayout)
	}

	goalPath := filepath.Join(base, date, "GOAL.md")
	planPath := filepath.Join(base, date, "PLAN.md")

	goalBytes, err := os.ReadFile(goalPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read active goal: %w", err)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read active plan: %w", err)
	}

	next := nextAction(string(planBytes))
	if next == "" {
		next = nextAction(string(goalBytes))
	}
	if next == "" {
		return activeDomain{}, errors.New("active goal/plan has no actionable Today or unchecked item")
	}

	return activeDomain{
		Date:       date,
		GoalPath:   goalPath,
		PlanPath:   planPath,
		Goal:       string(goalBytes),
		Plan:       string(planBytes),
		NextAction: next,
	}, nil
}

func activeDateFromIndex(index string) string {
	match := dateLinePattern.FindStringSubmatch(index)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func latestDatedDomain(base string) (string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("read plan domain: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := time.Parse(dateLayout, name); err == nil {
			dates = append(dates, name)
		}
	}
	sort.Strings(dates)
	if len(dates) == 0 {
		return "", nil
	}
	return dates[len(dates)-1], nil
}

func nextAction(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ] "))
		}
	}

	today := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if isTodayHeading(trimmed) {
			today = true
			continue
		}
		if today && strings.HasPrefix(trimmed, "## ") {
			return ""
		}
		if !today || trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "[ ] ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[x] ") {
			return trimmed
		}
	}
	return ""
}

func isTodayHeading(line string) bool {
	return line == "Today" || line == "## Today"
}

func ensureNextDomain(root, domain string, active activeDomain) (string, string, string, bool, bool, error) {
	activeDate, err := time.Parse(dateLayout, active.Date)
	if err != nil {
		return "", "", "", false, false, fmt.Errorf("parse active date: %w", err)
	}

	nextDate := activeDate.AddDate(0, 0, 1).Format(dateLayout)
	nextDir := filepath.Join(root, "plans", domain, nextDate)
	if err := os.MkdirAll(nextDir, 0o755); err != nil {
		return "", "", "", false, false, fmt.Errorf("create next plan directory: %w", err)
	}

	goalPath := filepath.Join(nextDir, "GOAL.md")
	planPath := filepath.Join(nextDir, "PLAN.md")

	createdGoal, err := writeIfMissing(goalPath, renderGoal(nextDate, active))
	if err != nil {
		return "", "", "", false, false, err
	}
	createdPlan, err := writeIfMissing(planPath, renderPlan(nextDate, active))
	if err != nil {
		return "", "", "", false, false, err
	}

	return nextDate, goalPath, planPath, createdGoal, createdPlan, nil
}

func writeIfMissing(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func renderGoal(date string, active activeDomain) string {
	return fmt.Sprintf(`# %s Goal

## Domain

`+"`agent-harness` self-improvement UX."+`

## Outcome

Continue the carry-forward from %s: resume useful context, read the active dated goal and plan, execute the next actionable item, verify locally, and keep the next dated planning domain ready.

## Today

- [ ] Continue: %s
- [ ] Run deterministic local verification before committing.
- [ ] Update `+"`GOAL.md`"+` and `+"`PLAN.md`"+` together when direction changes.

## Done Means

- Active context can be resumed from the TUI without manual file hunting.
- The next actionable dated-plan item is visible before execution.
- Normal verification stays deterministic, local, and secret-free.
- Changes are committed and pushed to `+"`develop`"+` only after green checks.

## Carry Forward

- Preserve this heading shape.
- Convert Today items into checkable plan items.
- Keep live provider e2e behind explicit environment gates.
`, date, active.Date, active.NextAction)
}

func renderPlan(date string, active activeDomain) string {
	return fmt.Sprintf(`# %s Plan

## Domain

`+"`agent-harness` self-improvement UX."+`

## Outcome

Make the self-improvement workflow increasingly natural from inside the TUI while preserving deterministic local verification.

## Today

- [ ] Continue: %s
- [ ] Run `+"`go test ./...`"+` with live API e2e disabled.
- [ ] Update this dated `+"`GOAL.md`"+` and `+"`PLAN.md`"+` pair together if the direction changes.
- [ ] Refresh the top-level plan index if this date becomes active.

## Done Means

- The active goal and plan can be loaded from `+"`plans/agent-harness/PLAN.md`"+`.
- The next actionable item is selected deterministically.
- Local verification does not require secrets or live providers.
- The next dated `+"`GOAL.md`"+` and `+"`PLAN.md`"+` pair exists.

## Carry Forward

- Source date: `+"`%s`"+`
- Keep normal tests deterministic and secret-free.
- Use `+"`AH_E2E_OPENROUTER=1`"+` only for explicit live provider e2e.
`, date, active.NextAction, active.Date)
}

// RunDeterministicGoTests runs normal Go tests with live provider e2e and API
// key environment variables disabled for this subprocess.
func RunDeterministicGoTests(ctx context.Context, root string) VerificationResult {
	const command = "go test ./..."
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = root
	cmd.Env = deterministicEnv(os.Environ())

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return VerificationResult{
		Command: command,
		Passed:  err == nil,
		Output:  strings.TrimSpace(output.String()),
		Error:   err,
	}
}

func deterministicEnv(env []string) []string {
	blocked := map[string]bool{
		"AH_E2E_OPENROUTER":     true,
		"AH_API_KEY":            true,
		"AGENT_HARNESS_API_KEY": true,
		"OPENROUTER_API_KEY":    true,
	}
	cleaned := make([]string, 0, len(env)+1)
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && blocked[key] {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	return append(cleaned, "AH_E2E_OPENROUTER=0")
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
