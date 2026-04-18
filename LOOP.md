# Outcome Loop — Agent Harness

## Constraints
- One active outcome at a time
- One active experiment per outcome
- Commit after each experiment
- KISS: minimal changes, maximum DX impact

## Active Outcome
**007: Project type detection in welcome + AGENTS.md freshness**
> Agent harness welcome message shows the detected project language, and AGENTS.md stays current with recent capabilities.

## Experiments
| ID | Name | Hypothesis | Status |
|----|------|------------|--------|
| 001 | auto-context-injection | If we enrich git context and system prompt with status, commits, and file tree, the LLM will provide more relevant first responses without extra tool calls. | validated |
| 002 | output-truncation | If we cap bash and read outputs at ~12k chars / ~300 lines, the agent loop will not crash from context overflow on large files or verbose commands. | validated |
| 003 | auto-resume-session | If we load the most recent session on startup instead of always creating a new one, users experience seamless continuity across restarts. | validated |
| 004 | edit-tool-reliability | If we add replace_all and better error messages to the edit tool, failed edits drop and user trust rises. | validated |
| 005 | loop-auto-compact | If the loop auto-compacts old messages when token count exceeds 80% of limit, long sessions won't hit hard blocking errors. | validated |
| 006 | slash-commit | If we add a /commit command that stages and commits, users stay in flow without leaving the TUI. | validated |
| 007 | project-type-welcome | If the welcome message detects go.mod/package.json/etc., users immediately know the agent understands their stack. | validated |

## Picks
- **001 auto-context-injection**: Enriching git context + system prompt improves startup awareness. Commit: 13eb7e9.

## Failures
_None yet._

## Decisions
- 2026-04-17: Focus on context injection first (highest DX leverage) before touching TUI or tools.
