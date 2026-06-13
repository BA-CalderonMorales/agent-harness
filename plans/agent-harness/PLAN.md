# Agent Harness Codex-Parity Plan

## Goal

Advance the local harness toward Codex-parity UX while keeping ordinary tests deterministic, stable, and secret-free.

## First Bounded Test Slice

- [x] Composer remains visible near the bottom of the chat view after long history.
- [x] Multi-line input uses compact dynamic height from one to four rows.
- [x] Session resume behavior is deterministic in an isolated local session directory.
- [x] Tool activity remains ordered before following assistant output.
- [x] OpenRouter-backed e2e is skipped by default and only runs with `AH_E2E_OPENROUTER=1`.

## Verification Plan

- Targeted deterministic tests:
  - `go test ./internal/interface/tui -run 'TestComposer|TestInputArea|TestCompletedTool'`
  - `go test ./internal/core/state -run 'TestResumeLatestSession'`
- Optional live provider smoke:
  - `AH_E2E_OPENROUTER=1 go test ./e2e/behaviors -run TestOpenRouterLiveStreamSmoke`
- Broader local verification:
  - `go test ./internal/interface/tui ./internal/core/state ./e2e/behaviors`
  - `go test ./...`

## Guardrails

- Prefer deterministic TUI and state tests for layout and resume invariants.
- Keep API-backed tests explicit, env-gated, and free from committed secrets.
- Split tests by behavior so no single test file becomes a catch-all.
