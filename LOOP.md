# Outcome Loop — Agent Harness

## Constraints
- One active outcome at a time
- One active experiment per outcome
- Commit after each experiment
- KISS: minimal changes, maximum DX impact

## Active Outcome
**003: Session continuity — auto-resume the most recent session on startup**
> Agent harness resumes the last conversation automatically, matching Claude Code's continuity. Users never lose context between restarts.

## Experiments
| ID | Name | Hypothesis | Status |
|----|------|------------|--------|
| 001 | auto-context-injection | If we enrich git context and system prompt with status, commits, and file tree, the LLM will provide more relevant first responses without extra tool calls. | validated |
| 002 | output-truncation | If we cap bash and read outputs at ~12k chars / ~300 lines, the agent loop will not crash from context overflow on large files or verbose commands. | validated |
| 003 | auto-resume-session | If we load the most recent session on startup instead of always creating a new one, users experience seamless continuity across restarts. | active |

## Picks
- **001 auto-context-injection**: Enriching git context + system prompt improves startup awareness. Commit: 13eb7e9.

## Failures
_None yet._

## Decisions
- 2026-04-17: Focus on context injection first (highest DX leverage) before touching TUI or tools.
