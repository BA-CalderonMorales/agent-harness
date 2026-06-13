# Agent Harness Plan

This plan tracks Codex-parity UX and reliable local integration behavior for `agent-harness`.

## Current Slice

- Keep the chat composer visible without requiring viewport scrolling.
- Make multi-line input height compact and predictable.
- Keep local session resume deterministic and isolated by `AGENT_HARNESS_SESSION_DIR`.
- Preserve readable Codex-like ordering: user message, tool activity, assistant output.
- Gate live OpenRouter e2e behind `AH_E2E_OPENROUTER=1`.

See [PLAN.md](PLAN.md) for execution status and [ux-quality/composer-and-e2e.md](ux-quality/composer-and-e2e.md) for test details.
