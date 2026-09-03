---
name: vibe-loop
description: "Use when the user gives TUI/UX feedback, reports something feeling off in agent-harness, or asks to vet/polish the terminal experience. Runs the fast loop: reproduce in a live TUI, diagnose to source, fix with one surgical commit, verify with before/after evidence, then check in for a vibe call. Examples: 'the spinner feels dead', 'that dialog lies', 'ship the next round of polish'."
---

# Vibe Loop — fast TUI feedback → surgical fix → vibe check

The contract: the user gives a feeling or a complaint. You turn it into a
reproduction, a root cause, one surgical commit, live evidence, and a
steering question. Never batch unrelated fixes; never fix without seeing
the real TUI first.

## The Loop

1. **Reproduce live** — never diagnose from code alone.
2. **Diagnose to source** — file:line, event flow, and the message path.
3. **Fix red/green** — failing test first for behavior bugs; property
   tests for invariants.
4. **One surgical commit** — conventional format, fix + its tests
   together, no unrelated hunks.
5. **Evidence** — live before/after from the tmux harness.
6. **Check in** — ranked findings menu + a specific vibe question. The
   user is the ultimate judge; adjust course on feel, not argument.

## Live harness (the only way to observe)

```bash
# terminal 1 — mock OpenAI-compatible server on 127.0.0.1:8080
(setsid python3 scripts/demo/mock-llm.py >/tmp/mock.log 2>&1 </dev/null &)

# terminal 2 — the real TUI with env pins (120x32 tmux pane)
make build
tmux new-session -d -s vet -x 120 -y 32 './scripts/demo/demo-boot.sh'
tmux capture-pane -t vet -p          # read the screen as text
tmux send-keys -t vet i              # insert mode, then type
tmux capture-pane -t vet -p > /tmp/frame.txt
```

- `burst20` as a chat message fires 15 real tool calls (collapse +
  ordering exercise). Any other text streams the welcome response.
- Key map: `1-4` tabs, `i`/`Esc` modes, `?` help, `Ctrl+P` palette,
  `j/k` scroll.

### Harness traps (each one cost an hour once)

- `pkill -f mock-llm` matches your own shell's command line and kills
  it — write the pattern with a bracket: `pkill -f "mock[-]llm"`.
- `send-keys i` then more keys can leak the first rune into the
  composer ("iburst20") — verify the composer content with a capture
  before pressing Enter.
- Pipes break the TUI (`cancelreader` epoll error). The pane's TTY is
  the display; redirect to a file only for crash forensics.
- Streaming turns finish fast against the mock — capture during the
  window or you will only see the finalized header.
- A frozen elapsed clock is a bug, not a paused video: the tick handler
  must repaint (see `timerTickMsg` in `chat_update.go`).

## Diagnosis patterns that paid off

- **Export vs render**: the session export (`/export`,
  `~/.agent-harness/sessions/*.json`) holds the true event order. If the
  TUI shows a different order, the bug is in the render path, not the
  agent loop.
- **Enshrined wrongness**: a test can pin a bug as spec (we found one
  titled "tools before the response" asserting the opposite). When a fix
  contradicts a test, read the test's *title intent* before trusting its
  assertions.
- **Silent swallows**: `_, _ =` and stderr-only panics lose the trail.
  New swallow paths get a `diag.Error/Panic` site tag
  (`internal/core/diag`, log at `~/.agent-harness/logs/`).
- **Dead view state**: nothing repaints without `refreshViewport` — any
  new animation or live counter must ride the tick repaint.
- **Stored-key trust**: `config.APIKey` may hold a local dummy or
  another provider's key. Auth decisions go through
  `storedKeyForProvider` (cmd/agent-harness/credentials.go), never
  raw-trust the field. Local/ollama keep their dummy by design.

## Fix discipline

- One concept per commit: `type(scope): description`. Fix and its tests
  in the same commit. Split files, never mix refactors with logic.
- Behavior bug → red test first, then green. Invariant → property test
  (gopter is wired; see `chat_debounce_property_test.go`,
  `settings_cycle_property_test.go` — both found real bugs).
- POLA gate: would this astonish a vim-fluent engineer? Env pins
  (`AH_*`) outrank persisted state; honest failure beats plausible
  lies ("no stored API key for X" > sending garbage auth).
- Version source of truth: `Version` constant in `cmd/agent-harness/main.go`
  (the Makefile reads it; bump it when cutting a release).

## Repo conventions to enforce on sight

Standing conventions — call them out and fix on touch, they keep the
codebase legible for future agentic sessions:

- **No hardcoded shared names.** Strings that name shared things
  (providers, data-tree dirs, status kinds, tool names) get constants or
  accessors in the owning package. Example: the data tree's subdir names
  live only as `config.DataSessions()/DataAudit()/DataLogs()/
  DataToolResults()` — no other file spells "audit" as a literal.
- **Early returns over nesting.** Guard clauses flatten ladders; if a
  function reads as an if/else-if pyramid, extract the policy into a
  helper that returns early. Max practical nesting ≈ 2.
- **Unbounded growth is a bug.** Anything that writes dated/accumulating
  files gets a retention sweep (see `diag.PruneDailyFiles`).

## Ship (release strategy)

```bash
# on release/X.Y.Z, quality gates green (go test ./..., make verify):
git checkout develop && git merge --no-ff release/X.Y.Z
git checkout main && git merge --no-ff develop
git tag -a vX.Y.Z && git push origin develop main vX.Y.Z   # tag fires release.yml
```

## Check-in protocol

Close every round with: what shipped (commit list), live before/after
captures, the ranked findings you did NOT fix, and one concrete vibe
question (e.g. "full-row highlight, or is the `>` cursor right?"). Then
shut up and let the judge steer.
