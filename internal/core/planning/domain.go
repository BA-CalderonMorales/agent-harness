package planning

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
