# 2026-06-13 Plan

## Domain

`agent-harness` self-improvement UX.

## Outcome

Turn the first Codex-parity slice into a repeatable daily workflow: a dated goal, a matching plan, deterministic local verification, and explicit API-backed e2e gate.

## Today

- [x] Create dated domain at `plans/agent-harness/2026-06-13/`.
- [x] Add `GOAL.md` with daily outcome and done criteria.
- [x] Align this `PLAN.md` to the same daily sections as `GOAL.md`.
- [x] Keep existing UX slice documented from the daily domain.
- [x] Keep top-level `PLAN.md` as the current-domain index.
- [x] Add a deterministic self-improvement workflow package.
- [x] Register `/improve` so the TUI can run the workflow.
- [x] Cover `/improve` with a local temp-module command test.

## Done Means

- `GOAL.md` and `PLAN.md` use matching headings.
- The bounded UX/reliability slice remains documented:
  - composer visibility
  - compact multi-line input
  - deterministic session resume
  - readable tool ordering
  - `AH_E2E_OPENROUTER=1` live e2e gate
- Normal tests stay deterministic and secret-free.
- `/improve` reads `plans/agent-harness/PLAN.md`, loads the active dated files, picks the next actionable item, runs `go test ./...` with live provider env disabled, and creates the next dated pair if missing.
- Targeted and full deterministic verification run.
- Update committed and pushed to `develop`.

## Carry Forward

Use this domain shape:

- Copy headings from `GOAL.md`.
- Convert each `Today` goal into checkable plan items.
- Keep verification commands in the dated subplan.
- Leave the top-level plan as a pointer to the current date until the active date changes.
