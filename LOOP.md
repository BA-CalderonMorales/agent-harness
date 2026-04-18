# Outcome Loop — Agent Harness

## Constraints
- One active outcome at a time
- One active experiment per outcome
- Commit after each experiment
- KISS: minimal changes, maximum DX impact

## Active Outcome
**014: `/memory` command reveals system prompt and context state**
> Agent harness lets users inspect the current system prompt, recent messages, and token budget via `/memory`, making context debugging transparent.

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
| 008 | slash-branch | If we add a /branch command for create/switch/list/delete, users manage branches without leaving the TUI. | validated |
| 009 | slash-plan | If we add a /plan command that puts the agent into planning mode, users get visibility into multi-step tasks before execution. | validated |
| 010 | sub-agent-execution | If the agent tool actually runs a sub-query with fresh context and returns results, users can delegate parallel tasks. | validated |
| 011 | slash-pr | If we add a /pr command that creates PRs via gh CLI, users complete the full git workflow inside the TUI. | validated |
| 012 | slash-init | If we add a /init command that scaffolds standard project files, users bootstrap projects without leaving the TUI. | validated |
| 013 | llm-summarize-compact | If we summarize old messages with the LLM before dropping them, context quality stays high in long sessions. | validated |
| 014 | slash-memory | If we add a /memory command that shows system prompt and recent context, users can debug what the LLM sees. | active |

## Picks
- **001 auto-context-injection**: Enriching git context + system prompt improves startup awareness. Commit: 13eb7e9.

## Failures
_None yet._

## Decisions
- 2026-04-17: Focus on context injection first (highest DX leverage) before touching TUI or tools.
