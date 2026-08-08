# Recent DX Improvements

- **Persona system**: 5 functional personas (developer, designer, pm, scientist, explorer) adapt prompts, tools, and UI
- **Home dashboard**: project overview, quick actions, recent sessions, contextual hints
- **Audit logging**: append-only log of tool executions and approvals at `~/.agent-harness/audit/`
- **Sandbox preview**: approval dialog shows diff preview for write/edit and risk level for bash
- **Granular permissions**: individual toggles for read/write/delete/execute with mode presets
- **Rich startup context**: system prompt auto-includes git status, recent commits, project file tree
- **Session auto-resume**: loads the most recent session on startup for continuity
- **Tool output truncation**: bash and read outputs are silently capped to protect context budget
- **Loop auto-compaction**: old messages are trimmed automatically when approaching token limits
- **Edit tool**: supports `replace_all` and gives actionable errors on mismatch
- **Persisted user settings**: provider/model/effort choices survive restarts via `~/.agent-harness/settings.json`
- **Composer UX**: centered 84-column input with mode line (mode · model · provider · effort) and a telemetry bottom bar (ctx % · cost · `ctrl+p commands`)
- **Reasoning effort**: `/effort` command and `ctrl+r` cycle (low/medium/high), persisted and sent as `reasoning_effort`
