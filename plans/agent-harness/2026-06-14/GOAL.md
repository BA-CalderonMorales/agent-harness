# 2026-06-14 Goal

## Domain

`agent-harness` self-improvement UX.

## Outcome

Continue making `agent-harness` natural to improve from inside the TUI by turning `/improve` from a deterministic local workflow into a richer operator loop with clearer context, safer plan updates, and visible verification results.

## Today

- [ ] Make `/improve` surface recent resumed-session context alongside the active dated goal/plan.
- [ ] Let `/improve` update an existing next dated pair through a managed section instead of only creating missing files.
- [ ] Add command output that clearly separates context, next action, verification, and plan-file updates.
- [ ] Keep normal verification deterministic and secret-free.

## Done Means

- The TUI can show enough resumed context to continue work without manual session or plan file hunting.
- Existing next dated `GOAL.md` and `PLAN.md` files are preserved while managed carry-forward content can be refreshed.
- Local tests prove missing, existing, and malformed plan-domain edge cases.
- Live OpenRouter e2e remains skipped unless `AH_E2E_OPENROUTER=1`.
- Changes are committed and pushed to `develop` only after green checks.

## Carry Forward

- Preserve this heading shape.
- Convert Today items into checkable plan items.
- Keep live provider e2e behind explicit environment gates.
