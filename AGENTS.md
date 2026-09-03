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

This project is indexed by GitNexus as **agent-harness** (6639 symbols, 24060 relationships, 444 execution flows).

> Index stale? Run `node .gitnexus/run.cjs analyze --index-only` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? Bootstrap with `npx`, `bunx`, or `pnpm dlx` — e.g. `bunx gitnexus@latest analyze` (npm 11 npx crash; #1939).

## Always Do

- **MUST run impact analysis before editing.** Use `impact({target: "symbolName", direction: "upstream"})` (MCP) or `node .gitnexus/run.cjs impact "symbolName" --direction upstream --repo .` (CLI fallback); report callers, processes, and risk. Never substitute grep for graph analysis.
- **MUST analyze graph changes before committing.** Use `detect_changes({scope: "all"})` (MCP) or `node .gitnexus/run.cjs detect-changes --scope all --repo .` (CLI fallback). `partial: true` or `truncated: true` is not a clean check — a zero means unseen, not unaffected; re-run it. For regression review: `detect_changes({scope: "compare", base_ref: "main"})` or `node .gitnexus/run.cjs detect-changes --scope compare --base-ref "main" --repo .`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- **MUST treat `risk: UNKNOWN` as unresolved, not as low.** An empty caller set is not evidence the symbol is unused — it can also mean the callers are not resolvable by the index (plain-object property access, dynamic dispatch, cross-language calls). `impact` pairs `UNKNOWN` with a `riskNote` saying so. Confirm with a text search before treating the symbol as safe to change or delete; do not proceed on the strength of a zero.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method before MCP/CLI impact analysis.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis, and never read `UNKNOWN` as an all-clear — it means the walk could not answer, which is the one verdict that requires confirming by other means.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit before MCP/CLI graph change analysis.

## Resources

| Resource | Use for |
| --- | --- |
| `gitnexus://repo/agent-harness/context` | Codebase overview, check index freshness |
| `gitnexus://repo/agent-harness/clusters` | All functional areas |
| `gitnexus://repo/agent-harness/processes` | All execution flows |
| `gitnexus://repo/agent-harness/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
| --- | --- |
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
