# Complexity analysis

A snapshot of where the shipping code is hard to change, measured rather
than guessed. The numbers are reproducible with `make complexity`; see
[the report tool](../scripts/verify/complexity-report/README.md) for the
definitions and flags.

Snapshot taken at 0.3.39, before the release commit:

| Metric | Value |
|---|---|
| Source files | 298 |
| Source lines | 47,451 |
| Functions | 1,871 |
| Packages | 50 |
| Test lines | 34,250 (0.72x source) |
| Source files over the 400-line goal | 10 |
| Functions at complexity 21 or higher | 34 |

The distribution is steep and healthy at the low end: 1,487 functions at
complexity 1-5, 273 at 6-10, 57 at 11-15, 20 at 16-20, and 34 at 21 or
above. The tail is small enough to name.

Cyclomatic complexity here is `1 +` every branch (`if`, `for`, `range`,
`case`, `select`, `&&`, `||`). Tests are excluded from source metrics so
the numbers describe the code a maintainer has to hold in their head.

## Domains

| Domain | Files | Source lines | Functions | Max complexity | Over 400 | Tests |
|---|---:|---:|---:|---:|---:|---:|
| `internal/interface/tui` | 71 | 15,226 | 570 | 101 | 9 | 15,538 |
| `internal/runtime` | 65 | 6,851 | 209 | 62 | 0 | 3,439 |
| `internal/session` | 36 | 6,305 | 260 | 20 | 0 | 0 |
| `cmd/agent-harness` | 39 | 5,078 | 166 | 42 | 0 | 4,993 |
| `internal/core` | 32 | 4,759 | 214 | 43 | 1 | 2,450 |
| `internal/agent` | 17 | 3,119 | 116 | 46 | 0 | 3,549 |
| `internal/ui` | 13 | 2,312 | 123 | 27 | 0 | 156 |
| `pkg` | 8 | 1,045 | 71 | 12 | 0 | 276 |
| tests, e2e, scripts | 17 | 2,756 | 142 | 24 | 0 | 3,849 |

Three observations drive the recommendations:

1. **The TUI is a third of the source and holds every over-budget file.**
   It is also the best-tested domain by line count (15.5k test lines), so
   the pressure is comprehension, not correctness.
2. **`internal/session` is 13% of the source with no tests at all.** Per
   `AGENTS.md` it is the modular bucket rewrite and not the live path.
   Untested and unreachable is dead weight for maintainers and a trap for
   agents that find it first and assume it runs.
3. **`internal/runtime/tools/builtin` is where untrusted input meets the
   OS and the network, and it has the thinnest coverage** (39.1%, versus
   82.2% for the TUI). New security tests in 0.3.39 raised the fetch
   path; the shell classifier and filesystem tools remain lightly covered.

## Hot spots

The ten functions with the highest cyclomatic complexity:

| Complexity | Lines | Function | File |
|---:|---:|---|---|
| 101 | 366 | `handleKeys` | `internal/interface/tui/app_keys.go` |
| 89 | 500 | `Update` | `internal/interface/tui/chat_update.go` |
| 76 | 383 | `Update` | `internal/interface/tui/app_update.go` |
| 62 | 271 | `readSSE` | `internal/runtime/llm/sse.go` |
| 58 | 265 | `handleKeys` | `internal/interface/tui/chat_keys.go` |
| 51 | 227 | `queryLoop` | `internal/agent/loop.go` |
| 43 | 129 | `extractValues` | `internal/core/config/layered_merge.go` |
| 42 | 205 | `runDiagnose` | `cmd/agent-harness/diagnose.go` |
| 42 | 195 | `runSingleTool` | `internal/agent/executor_run.go` |
| 42 | 159 | `consumeStream` | `internal/agent/loop_stream.go` |

The shape is consistent: three of the top five are Bubble Tea handlers
whose complexity comes from a flat `switch` over message types, not from
deep nesting. That is cheap to read in small doses and expensive to
change because every new key or message widens the same function.
`readSSE`, `queryLoop`, `consumeStream`, and `runSingleTool` are
different: their branches are protocol and lifecycle state, and they are
the functions most likely to hide a regression.

Files over the 400-line goal, all but one in the TUI:

| Lines | File |
|---:|---|
| 696 | `internal/interface/tui/chat.go` |
| 674 | `internal/interface/tui/chat_view.go` |
| 561 | `internal/interface/tui/logs.go` |
| 559 | `internal/interface/tui/chat_update.go` |
| 496 | `internal/interface/tui/app_keys.go` |
| 483 | `internal/interface/tui/chat_turns.go` |
| 437 | `internal/core/config/secure.go` |
| 427 | `internal/interface/tui/components.go` |
| 405 | `internal/interface/tui/app_update.go` |
| 403 | `internal/interface/tui/app.go` |

## Test coverage by domain

Statement coverage from `go test -cover ./...`, grouped by the same
domains:

| Package | Coverage |
|---|---:|
| `internal/interface/tui` | 82.2% |
| `internal/interface/tui/themeinit` | 100% |
| `internal/interface/approval` | 89.2% |
| `internal/interface/commands` | 86.4% |
| `internal/runtime/services/mcp` | 81.2% |
| `internal/runtime/llm` | 72.3% |
| `internal/runtime/tools` | 72.7% |
| `internal/core/diag` | 81.1% |
| `internal/core/planning` | 66.4% |
| `internal/agent` | 66.1% |
| `internal/core/state` | 63.7% |
| `internal/core/config` | 54.9% |
| `cmd/agent-harness` | 49.6% |
| `internal/runtime/permissions` | 47.6% |
| `pkg/git` | 35.3% |
| `internal/runtime/tools/builtin` | 39.1% |
| `internal/ui` | 7.7% |
| `internal/session`, `internal/skills`, `internal/tasks`, `internal/core/audit`, `pkg/bash`, `pkg/sandbox` | no tests |

Packages with no tests are not automatically a problem: `internal/session`
is not on the live path, and `pkg/sandbox` may be a thin wrapper. The
combination that matters is reachability plus low coverage. Today that is
`builtin` (all process and network I/O), `internal/ui` (terminal input),
and `pkg/git` (subprocess construction).

## Recommendations, in order

1. **Keep the TUI out of refactor-and-release changes.** Its handlers are
   the highest-complexity code, its tests are behavior-level, and a split
   of `handleKeys` or `Update` is a pure move that still touches every
   keybinding. If it is split, do it as its own release with no logic
   changes, one file per key family, and the existing tests as the gate.
2. **Give `builtin` the coverage its risk demands.** Start with table
   tests for `isDangerousCommand`/`detectDestructive`, path handling in
   `read`/`write`/`edit`, and the truncation boundaries in `bash_exec`.
   These are cheap tests for behavior that is currently asserted only by
   comments.
3. **Resolve `internal/session` one way or the other.** Either delete it
   before the rewrite ships or add tests as it is wired in. A 6.3k-line
   untested package that looks like the agent loop is the single largest
   source of false confidence for a new contributor or agent.
4. **Extract export redaction into one package.** The rule list now
   exists twice (`internal/core/state/session_export.go` and
   `internal/runtime/tools/builtin/export_redact.go`) and must be edited
   in lockstep. One package with one table removes a whole class of
   drift.
5. **Split `internal/core/config/secure.go`.** It is the only
   non-TUI file over budget; store, migration, and status are separable
   and already covered by tests.
6. **Ratchet, do not freeze.** `make complexity COMPLEXITY_ARGS="-strict"`
   and `-max-complexity=N` can gate new growth in CI once the worst
   offenders are below the chosen line. Gating today would fail on ten
   existing files and teach everyone to ignore the gate.

## Guardrails for agents

- `make complexity` prints the ranked lists and package table. Read it
  before picking a file to change.
- `make verify` enforces the 400-line report and the full suite. It is
  advisory on file size by design; `scripts/verify-structure.sh --strict`
  fails when a hard gate is wanted.
- When a change must touch a top-five function, say so in the pull
  request and name the behavior the tests pin. Complexity is a proxy for
  blast radius, not a verdict.
