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
  `internal/core/{config,state,persona,audit}/` cross-cutting state;
  `internal/loop/` the modular bucket rewrite (not yet the live path).
- `pkg/` shared types, messages, git, bash, sandbox helpers.
- `scripts/` local automation; `docs/` the reference reading for every
  section below.
- Pre-rewrite leftovers are pruned; use Git history for legacy reference.

## Key Sections

| To understand... | Read |
|---|---|
| The agent loop, buckets, and naming pattern | `docs/loop-architecture.md` |
| Composer / chat behavior and conversation flow | `internal/interface/tui/`, `docs/conversation_flow.md` |
| Command approval and permissions stacking | `docs/command-approval.md` |
| Branch protection and release flow | `docs/branch-protection.md`, Makefile release targets |
| Edge cases and parity | `docs/edgecases.md`, `docs/parity.md` |
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
- Structure check: `make verify` reports files over the line budget.
- `/persona <name>` switches behavior mode; `/audit` shows tool activity;
  `/commit <message>` stages and commits; `h`/`c` jump between Home and Chat.

## CI

- `.github/workflows/ci.yml` runs on PRs: build, vet, full test suite.
- `release.yml` tags and publishes releases; `bump-version.yml` bumps
  versions before a release cut.
- Docs-only changes skip CI automatically where configured; trigger
  `workflow_dispatch` when needed.

## Branch Strategy

- `main` and `develop` are protected (restrict deletions; local `git del`
  honors protection, `./scripts/prune-branches.sh` prunes merged branches).
- Feature and fix work lives on `release/<next-patch>` branches cut from
  `develop`; the branch accumulates the release, then merges into `develop`
  and is tagged from `main`.
- Never merge dev-local working spaces (`scratch/`, `.workspace/`,
  working ledgers) into `develop` or `main`.

## Rules

- Files target ≤ 400 lines (soft cap; spec files are exempt). Splits are
  pure moves: `gofmt`, adjust imports, `go test ./...`, then commit.
  Never mix a refactor-move with logic changes in one commit.
- One file = one concept inside the package; facades for public surfaces;
  tests mirror sources. Stay inside the package boundary; split files
  before splitting packages.
- No root-level helper scripts; scripts are bucketed under `scripts/`.
- Conventional commits: `type(scope): description`. One change per commit;
  stop and explain before major architectural changes.
- Zero emojis in root-level .md files; lowercase filenames (except
  README.md, AGENTS.md); no horizontal rules as section separators.
- Tool calling must work flawlessly - no regressions.
- Follow the Bucket Suffix naming pattern for new buckets.
- Use `rg` for content search when available; `fzf` for interactive
  selection when available.

## Design Principles

- **SRP** - one concept per file; a domain's facade is its public face.
- **OCP** - extend by adding a file or bucket, not by widening an existing
  one; defaults live in `defaults` files, never inline.
- **ISP/DIP** - buckets implement `LoopBase`; orchestrators depend on
  bucket faces, never internals (see `docs/loop-architecture.md`).
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

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **agent-harness** (5232 symbols, 15973 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/agent-harness/context` | Codebase overview, check index freshness |
| `gitnexus://repo/agent-harness/clusters` | All functional areas |
| `gitnexus://repo/agent-harness/processes` | All execution flows |
| `gitnexus://repo/agent-harness/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.agents/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.agents/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.agents/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.agents/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.agents/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.agents/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
