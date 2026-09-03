# SHIP-REVIEW — release/0.3.8

## UX / correctness fixes landed this pass (source + tests + live TUI verified)

- `/settings` sentinel leak fixed: `handleUserCommand` (cmd/agent-harness/handlers.go) now
  swallows the `__SETTINGS__` token via `commands.IsSettings`; the literal no longer lands
  in the chat transcript. Verified live (tmux): `/settings` → "Switched to Settings tab."
  only.
- P3-6 submit debounce reworked (internal/interface/tui/chat_keys.go): a printable key
  after Enter no longer cancels the submit and eats a newline. Burst-speed keys (< 20 ms,
  `PasteBurstThreshold` in chat.go) still count as Termux-style paste continuation; human
  pacing flushes the submit and the keystroke starts the next message. Bracketed paste
  (`msg.Paste`) keeps the old mitigation. Both Termux paste contracts still pass.
- Property tests (red/green by construction):
  - `chat_debounce_property_test.go` — "Enter submits exactly once, no keys eaten" under
    randomized text/pacing; burst pastes assemble into one message.
  - `settings_cycle_property_test.go` — choice cycling is a rotation; found two real bugs:
    (1) the Enter (-1) vs arrows (0) fallback split, (2) duplicate options trapping the
    rotation. `cycleChoice` (settings_update.go) now dedupes options and snaps unknown
    values deterministically.
- Temperature validation: `validateSetting` rejects non-numbers and out-of-range values
  ("must be a number (0.0-2.0)", live-verified); the delegate no longer silently drops an
  unparsable value — it answers with an error status instead of leaving the row lying.
- `AH_MODEL` env pin survives session resume: `ModelPinned` (internal/core/config) follows
  the `EndpointPinned`/`TimeoutPinned` pattern; init.go adopts the session model only when
  no env pin exists. Live-verified: demo boot now runs `demo-1.0` instead of a stale
  `gemma4:2b` from the resumed session; settings.json persists the pinned model.
- Approval dialog honesty (internal/interface/tui/approval_dialog.go, cmd/agent-harness):
  - "Approve All" now remembers the exact command for the session
    (`App.approvedCommands`); a repeat auto-approves with a visible status line.
  - "Reject + Suggest" is real: `R`/`4` opens a one-line input; the note rides
    `ApprovalRequest.Note` and becomes the deny reason the agent sees
    ("Rejected by user: <note>").
  - Number keys (1-4) confirm on press; letter keys (a/A/r/R) unchanged; Esc rejects.
- Palette hint drift fixed: "Type to filter, Enter to select, Esc to cancel" (the old
  text advertised a `/` trigger that was not wired).
- Vim navigation: `l` is now vim-right (next tab) whenever the provider is ready; it opens
  the login wizard only in the setup dead-end state the badge advertises. `h` remains Home.

## Diagnostics logging (fast trace-back)

- New `internal/core/diag`: fail-silent JSONL at `~/.agent-harness/logs/YYYY-MM-DD.log`,
  site-tagged entries (same daily-file pattern as audit). Panics carry a stack.
- Wired sites: `tui.app_update` panic (also surfaces a visible transcript line),
  `tui.chat_submit` panic, `tui.textarea` panic, `tui.send.drop` (channel-full),
  `session.save.{submit,turn,clear,model,persona}` (persistence failures that were
  previously `_, _ =`).

## Evidence (durable)

- `evidence/release-tui-verify.gif` — real binary, real mock LLM: tab walk, settings
  cycling + editor rejection, palette filter, `/cost`. Recorded via
  `scripts/vhs/release-verify.tape` (run from repo root: `vhs scripts/vhs/release-verify.tape`).
- `go test ./...` green (18 packages, 0 failures), `go vet` clean, `make verify` clean.
- tmux live sessions verified: token-leak absence, ModelPinned footer, temperature
  validation + persisted `settings.json`, palette hint.

## Still open (documented, not glossed)

- NVIDIA reasoning gap unchanged: SSE `reasoning_content` delta has no path to
  `ThinkingBlock` (pkg/types has Thinking/Signature only). Payload + parser verified
  correct; the mapping is a feature decision, not a bug fix.
- `/git` still unregistered (`WorktreeHandler` at internal/interface/commands/slash_git.go
  remains dead code). `/plan` toggles a flag with no consumer. `/permissions` still means
  two things (PermissionMode report + ExecutionMode setter).
- Statusbar transient status still never auto-expires.
- Pre-existing unformatted files (not touched this pass): pkg/types/reasoning_test.go and
  the older unformatted test files were formatted where they intersected this diff.
