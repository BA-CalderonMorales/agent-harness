# 2026-06-13 Goal

## Domain

`agent-harness` self-improvement UX.

## Outcome

Make `agent-harness` feel natural enough that Brandon can resume context, read the active goal, execute the next plan step, verify locally, and update the next dated planning domain without manual ceremony.

## Today

- [x] Keep the composer visible and compact so TUI can be used for real work without scrolling friction.
- [x] Keep session resume/local harness behavior deterministic.
- [x] Keep tool activity and output ordering readable and Codex-like.
- [x] Keep API-backed e2e behavior available only with `AH_E2E_OPENROUTER=1`.
- [x] Establish a dated planning domain that pairs `GOAL.md` and `PLAN.md` for tomorrow's automatic carry-forward.
- [x] Add `/improve` so the TUI can read the active dated goal/plan, choose the next action, run deterministic local verification, and create the next dated pair.

## Done Means

- `plans/agent-harness/2026-06-13/GOAL.md` and `PLAN.md` exist and share the same sections.
- Top-level `plans/agent-harness/PLAN.md` points to the active dated domain.
- Deterministic local tests cover the first UX and reliability slice.
- Optional OpenRouter e2e remains env-gated and secret-free in normal runs.
- `/improve` is available from the TUI and uses normal `go test ./...` with live provider env disabled.
- `plans/agent-harness/2026-06-14/GOAL.md` and `PLAN.md` exist as the next carry-forward pair.
- Repo green, committed, pushed on `develop`.

## Carry Forward

Tomorrow's generated domain keeps:

- `Domain`
- `Outcome`
- `Today`
- `Done Means`
- `Carry Forward`
