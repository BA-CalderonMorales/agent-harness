# Agent Harness Plan

This directory tracks Codex-parity UX and reliable local integration behavior for `agent-harness`.

## Current

- Daily domain: [2026-06-13](2026-06-13/)
- Goal: [2026-06-13/GOAL.md](2026-06-13/GOAL.md)
- Plan: [2026-06-13/PLAN.md](2026-06-13/PLAN.md)
- Index: [PLAN.md](PLAN.md)

## Daily Domain Pattern

Each `{date}/` directory should pair a `GOAL.md` with a `PLAN.md`.
The plan should reuse the goal's headings so the next day can be generated naturally from the previous day.

## Current Slice

- Keep the chat composer visible without requiring viewport scrolling.
- Make multi-line input height compact and predictable.
- Keep local session resume deterministic and isolated by `AGENT_HARNESS_SESSION_DIR`.
- Preserve readable Codex-like ordering: user message, tool activity, assistant output.
- Gate live OpenRouter e2e behind `AH_E2E_OPENROUTER=1`.

See [ux-quality/composer-and-e2e.md](ux-quality/composer-and-e2e.md) for test details.
