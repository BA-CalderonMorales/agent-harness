# Agent Harness Plan Index

## Current Domain

- Date: `2026-06-13`
- Goal: [2026-06-13/GOAL.md](2026-06-13/GOAL.md)
- Plan: [2026-06-13/PLAN.md](2026-06-13/PLAN.md)
- Quality slice: [ux-quality/composer-and-e2e.md](ux-quality/composer-and-e2e.md)

## Pattern

Each daily domain under `plans/agent-harness/{date}/` should include:

- `GOAL.md`: intent, outcome, done criteria, and carry-forward shape.
- `PLAN.md`: the same sections, with `Today` and `Done Means` converted into checkable work.

## Active Focus

Make `agent-harness` good enough to improve itself from inside the TUI:

- resume the latest useful session
- read the active dated goal
- execute the next plan item
- verify deterministic tests locally
- optionally run live provider e2e with `AH_E2E_OPENROUTER=1`
- write the next dated domain before ending the day

## Verification Commands

- `go test ./internal/interface/tui -run TestComposer`
- `go test ./internal/interface/tui -run TestInputAreaHeightTracksVisibleRows`
- `go test ./internal/interface/tui -run TestCompletedToolActivityStaysBeforeFollowingOutput`
- `go test ./internal/core/state -run TestResumeLatestSession`
- `go test ./e2e/behaviors -run TestOpenRouterLiveStreamSmoke`
- `go test ./...`
