# 2026-06-14 Plan

## Domain

`agent-harness` self-improvement UX.

## Outcome

Make the self-improvement workflow increasingly natural from inside the TUI while preserving deterministic local verification.

## Today

- [ ] Add resumed-session context to the `/improve` summary without leaking secrets or dumping full transcripts.
- [ ] Add managed-section refresh support for existing next dated `GOAL.md` and `PLAN.md` files.
- [ ] Split `/improve` output into stable sections: Context, Active Plan, Next Action, Verification, Plan Updates.
- [ ] Extend local tests for missing index date, existing next domain, malformed active files, and verifier failure output.

## Done Means

- The active goal and plan can be loaded from `plans/agent-harness/PLAN.md`.
- The next actionable item is selected deterministically.
- Local verification does not require secrets or live providers.
- The next dated `GOAL.md` and `PLAN.md` pair exists and can be refreshed without overwriting unrelated user edits.

## Carry Forward

- Source date: `2026-06-13`
- Keep normal tests deterministic and secret-free.
- Use `AH_E2E_OPENROUTER=1` only for explicit live provider e2e.
