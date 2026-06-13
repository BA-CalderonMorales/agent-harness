# 2026-06-13 Goal

## Domain

`agent-harness` self-improvement UX.

## Outcome

Make `agent-harness` feel natural enough that Brandon can use it to improve itself: resume context, read the active goal, execute the next plan step, verify locally, and update the next dated planning domain without manual ceremony.

## Today

- Keep the composer visible and compact so the TUI can be used for real work without scrolling friction.
- Keep session resume/local harness behavior deterministic so work can restart from the latest useful state.
- Keep tool activity and output ordering readable and Codex-like.
- Keep API-backed e2e behavior available but explicit with `AH_E2E_OPENROUTER=1`.
- Establish a dated planning domain that pairs `GOAL.md` and `PLAN.md` for tomorrow's automatic carry-forward.

## Done Means

- `plans/agent-harness/2026-06-13/GOAL.md` and `PLAN.md` exist and share the same sections.
- Top-level `plans/agent-harness/PLAN.md` points to the active dated domain.
- Deterministic local tests cover the first UX and reliability slice.
- Optional OpenRouter e2e remains env-gated and secret-free in normal runs.
- The repo is green, committed, and pushed on `develop`.

## Carry Forward

Tomorrow's generated domain should start from this shape:

- `Domain`
- `Outcome`
- `Today`
- `Done Means`
- `Carry Forward`
