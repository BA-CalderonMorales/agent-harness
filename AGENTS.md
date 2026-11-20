# AGENTS.md - Agent Harness

## Current Shape

- `cmd/agent-harness/` is the app wiring: boot, command registry, settings
  state, and TUI delegates.
- `internal/` holds domain packages. Each domain keeps a facade file where
  the public surface matters (e.g. `api.go`, `index.go`, `client.go`), plus
  one file per concept: types in `types.go`, errors in `errors.go`, styles
  in `styles.go` (tui), and tests mirrored beside sources as `*_test.go`.
  Files target ≤ 400 lines - growing past 400 is a signal to split, never a
  reason to keep writing.
- `internal/agent/` is the core agent loop; `internal/interface/tui/` the
  terminal UI; `internal/runtime/{llm,tools}/` providers and tools;
  `internal/core/{config,state,persona,audit}/` cross-cutting state.
  The dormant modular loop and alternate buckets were retired in the 0.4.1
  cleanup; use Git history for those designs, not as a runtime entry point.
- `pkg/` shared types, messages, git, bash, sandbox helpers.
- `scripts/` local automation; `docs/` the reference reading for every
  section below.
- Pre-rewrite leftovers are pruned; use Git history for legacy reference.

## Key Sections

| To understand... | Read |
|---|---|
| The agent loop, buckets, and naming pattern | `docs/loop-architecture.md` |
| Composer / chat behavior and conversation flow | `internal/interface/tui/`, `docs/conversation_flow.md` |
| Desktop interaction quality, recovery, and reference workflows | `docs/desktop-interaction.md` |
| Small live-model capability checks and baseline evidence | `evals/README.md` |
| Command approval and permissions stacking | `docs/command-approval.md` |
| Branch protection and release flow | `docs/branch-protection.md`, Makefile release targets |
| Edge cases and parity | `docs/edgecases.md`, `docs/parity.md` |
| Security posture and accepted risks | `docs/security-review.md` |
| Optional runtime isolation (OpenShell) | `docs/openshell.md`, `examples/openshell/` |
| Sub-agent definitions and delegation | `docs/subagents.md`, `.agent-harness/agents/` |
| Complexity hot spots and coverage gaps | `docs/complexity-analysis.md` |
| Environment variables | `docs/environment-variables.md` |
| Recent DX improvements | `docs/dx-improvements.md` |
| Everything else | `README.md`, then this file again |

Lost in the woods? Start with `docs/architecture.md` for *why*, then
`docs/conversation_flow.md` for *how* a turn moves.

## Run

- Build + run: `make build && make run`
- Verify everything: `go test ./...` (unit), `go test -race ./...`
- Local LLM: `agent-harness.yml` targets Ornith-1.0 GGUF via a llama.cpp
  OpenAI-compatible endpoint; `./scripts/ah-fast.sh` for small Ollama smoke
  tests; `./scripts/ah-local.sh` for the local server flow.
- Release: `gh workflow run cut-release.yml -f dry_run=true`, then
  `gh workflow run cut-release.yml` (see Branch Strategy).
- Structure check: `make verify` reports files over the line budget.
- Complexity snapshot: `make complexity` ranks functions and packages; see
  `docs/complexity-analysis.md` before choosing a file to refactor.
- Terminal tests: `internal/testharness/ttui` renders the real model and
  asserts on the frame a terminal receives (see `docs/tui-testing.md`).
  Golden files regenerate with `go test ./... -update`.
- `/persona <name>` switches behavior mode; `/audit` shows tool activity;
  `/commit <message>` stages and commits; `h`/`c` jump between Home and Chat.

## Android and Termux

- The Android goal is a dedicated app; see `docs/android.md`. No app or APK
  ships yet. Do not describe narrow-terminal tests as Android validation.
- From 0.4.0, native Termux builds are best-effort, not a release gate.
  Installations below the selected release's `go.mod` requirement are
  unsupported. Do not lower the Go baseline for Termux package availability.
- Keep useful portability fixes and narrow-pane tests. Historical Termux
  notes and helper scripts do not certify current device compatibility.
- The authoritative policy is `docs/install.md#termux-android`.

## CI

- `.github/workflows/ci.yml` runs on PRs to `main` and on pushes to `main`:
  `build`, `quality`, `security`, `secrets`, `sbom`, `scripts` (unit tests
  for the repository's own shell and Python tooling), and a sharded `test`
  matrix (`tui`, `core`, `rest`). The shard coverage check is a drift guard —
  a package in no shard fails the gates job rather than silently going
  untested.
- `release.yml` runs on a tag push and builds and publishes the release
  artifacts, stamping the tag into the binary through `-ldflags`.
- The git tag is the only place a version lives. No source file holds a
  version and there is no bump commit. `make build` and `make install`
  stamp `git describe`; `go install ...@vX.Y.Z` reports the module version;
  anything else reports `dev` (see `cmd/agent-harness/version.go`).
- Docs-only changes skip CI automatically where configured; trigger
  `workflow_dispatch` when needed.

## Branch Strategy

`main` is the only long-lived branch, and it is always releasable. The
runbook is `.agent-harness/skills/release-workflow/SKILL.md`; this section
records the invariants it rests on.

- Work happens on short-lived branches named for the change
  (`feat/...`, `fix/...`, `chore/...`) and lands on `main` through a PR.
  CI on the PR is the gate. Never push straight to `main`.
- Work that is not ready for users merges behind a feature flag, so `main`
  stays shippable without long-lived branches.
- A release is a tag on `origin/main` and nothing else, cut in GitHub
  Actions: `gh workflow run cut-release.yml` (or Actions → Cut release)
  computes the next version from the conventional commits since the last
  tag, refuses unless CI passed on that commit, pushes the tag, and starts
  `release.yml` and `container.yml` on it. `-f dry_run=true` shows what it
  would do. `make release` runs the same script locally as a fallback only.
- Remote first: remote CI is the gate and remote CD builds and publishes. A
  locally built release that bypasses CD is a last resort, not a route —
  say so out loud when taking it.
- CodeRabbit reviews PRs; Copilot review is intentionally off. When a
  CodeRabbit comment calls for a change, it gets its own commit, then the
  reply, then the resolve — in that order.
- Never merge dev-local working spaces (`scratch/`, `.workspace/`,
  working ledgers) into `main`.

## Rules

- Files target ≤ 400 lines (soft cap; spec files are exempt). Splits are
  pure moves: `gofmt`, adjust imports, `go test ./...`, then commit.
  Never mix a refactor-move with logic changes in one commit.
- One file = one concept inside the package; facades for public surfaces;
  tests mirror sources. Stay inside the package boundary; split files
  before splitting packages.
- No root-level helper scripts; scripts are bucketed under `scripts/`.
- Conventional commits: `type(scope): description`, enforced by the
  `commit-msg` hook, CI's `commits` job, and the `pr-title` workflow
  because `make release` picks the version from them. One change per commit; stop and explain before major
  architectural changes.
- Zero emojis in root-level .md files; lowercase filenames (except
  README.md, AGENTS.md); no horizontal rules as section separators.
- Tool calling must work flawlessly - no regressions.
- Follow the Bucket Suffix naming pattern for new buckets.
- Use `rg` for content search when available; `fzf` for interactive
  selection when available.
- Every PR description ends with the line `Generated by Agent-Harness`
  as its footer.
- PR descriptions describe the changes and how to verify them — never
  reference goal files, scratch notes, task numbers, or internal
  planning artifacts (reviewers can't see them; the summary must stand
  alone).
## Design Principles

- **SRP** - one concept per file; a domain's facade is its public face.
- **OCP** - extend by adding a file or bucket, not by widening an existing
  one; defaults live in `defaults` files, never inline.
- **ISP/DIP** - depend on the existing tool descriptors and domain interfaces;
  extend the live path with contract tests (see `docs/loop-architecture.md`).
- **DRY** - one authoritative home per piece of knowledge: one defaults
  store, one model per concept, tests mirror sources.
- **KISS** - boring beats novel; delete before adding.
- **CQS** - pure queries separated from state-changing commands; read-only
  vs destructive tool classification is this principle.
- **Self-documentation** - durable names; `make verify` measures the
  structure so shape is observable, not aspirational.
- **POLA** - behavior must not astonish: user settings persist silently,
  provider switches keep the current model, transient status never
  clobbers session state.
