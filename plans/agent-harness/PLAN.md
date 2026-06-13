# Agent Harness Plan Index

## Current Domain

- Date: `2026-06-13`
- Goal: [2026-06-13/GOAL.md](2026-06-13/GOAL.md)
- Plan: [2026-06-13/PLAN.md](2026-06-13/PLAN.md)
- Next prepared: [2026-06-14/GOAL.md](2026-06-14/GOAL.md) and [2026-06-14/PLAN.md](2026-06-14/PLAN.md)

## Pattern

Each daily domain under `plans/agent-harness/{date}/` should include:

- `GOAL.md`: intent, outcome, done criteria, and carry-forward shape.
- `PLAN.md`: sections, with `Today` and `Done Means` converted into checkable work.

## Active Focus

Make `agent-harness` good enough to improve inside the TUI:

- [x] resume the latest useful session
- [x] read the active dated goal
- [x] identify the next actionable dated-plan item
- [x] verify deterministic tests locally from `/improve`
- [x] optionally run live provider e2e with `AH_E2E_OPENROUTER=1`
- [x] write the next dated domain before ending the day
- [ ] enrich `/improve` with resumed-session context and managed plan refreshes

## TUI Workflow

- `/improve` reads `plans/agent-harness/PLAN.md`.
- It loads the active dated `GOAL.md` and `PLAN.md`.
- It selects the first unchecked item, or the first non-empty `Today` item.
- It runs `go test ./...` with live OpenRouter/API env disabled.
- It creates the next dated `GOAL.md` and `PLAN.md` pair when missing.

## Verification

- `go test ./internal/core/planning ./cmd/agent-harness`
- `go test ./...`
- Optional live e2e: `AH_E2E_OPENROUTER=1 go test ./e2e/behaviors -run TestOpenRouterLiveStreamSmoke`
