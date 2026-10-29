# Changelog

## [Unreleased]

### Fixed

- File writes now reject missing, null, or non-string `content` instead of
  implicitly truncating the destination. Edits reject missing or wrongly
  typed `new_string`. Both validation and execution enforce the check;
  explicit empty strings still support intentional empty files/deletions.
- Directory-listing failures now propagate as tool errors, with guidance
  about literal paths and shell syntax. Directory rows use List Files
  rather than Shell.
- An identical failed read-only call gets one correction opportunity
  without re-executing the duplicate. Persistent repetition still stops,
  with rejected tool results recorded so the conversation can continue.
  Agent work-limit feedback no longer suggests a provider/model switch.

### Changed

- From 0.4.0, native Termux builds are best-effort and no longer a release
  gate. Installations below the selected release's Go requirement are
  unsupported. The Android goal is a dedicated app, currently planned
  with no APK or committed date. Installation docs, contributor guidance,
  and historical portability notes now distinguish these paths.

## [0.3.42] - 2026-10-24

### Added
- Sub-agent definitions: a Markdown file with frontmatter under
  `.agent-harness/agents` (project) or `<config home>/agents` (user). The
  name, description, model, and tool allowlist come from the file, and the
  body becomes the child's instructions. The agent tool's `agent_type`
  used to be accepted and then ignored — it only changed the label on the
  transcript row — so every delegation was the same generic worker. The
  installed definitions now reach the model in the system prompt, `/agents`
  lists them, and an unknown name is an error that lists the available
  ones rather than a silent fallback. A delegated worker still never gets
  the agent tool, and the parent's permission callback still decides
  every call.
- `/doctor`, a first-run readiness checklist: one line per fact (workspace,
  provider, api key, endpoint, model, skills, sub-agents, mcp,
  permissions) and a `Next:` line naming the first fix worth acting on.
  The endpoint check reuses the boot prober, so the checklist and the
  status bar never disagree.
- `/keys`, a listing of the app's keyboard shortcuts.
- `docs/subagents.md`, the definition format, the precedence rules, and
  three roles worth copying.

### Fixed
- Skills resolve from one place. The system prompt loaded the project's
  `.agent-harness/skills` while `/skills` listed `.workspace/skills`, so
  the agent could know a skill the user could not see and `/skills` could
  list a skill the agent never received. One resolver now serves both,
  walking the user config home, the legacy workspace root, and the
  project skills, with later roots overriding earlier ones. The skill
  list is sorted by name, which keeps the system prompt stable for
  provider-side caching.

### Changed
- The key bindings are data. The global chords, the view switches, the
  mode bindings, and the navigate-mode bindings each carry their keys, a
  name, a description, and the guard that decides whether they apply,
  and `/keys` lists them. `handleKeys` drops from complexity 102 to 61
  and from 366 to 222 lines, and `app_keys.go` now fits the 400-line
  budget. The guards carry the old conditions exactly: a binding whose
  view does not apply is not claimed and the key falls through to the
  view behind it.

## [0.3.41] - 2026-10-24

### Fixed
- Terminal control sequences in model, tool, and log text are filtered
  before they reach the terminal. A response could emit OSC 52 (clipboard
  write), OSC 8 (hyperlink), or cursor and erase sequences and take over
  the pane; the filter drops OSC, DCS, PM, APC, SOS, non-SGR CSI, stray
  C0 controls, and the C1 range while keeping SGR, so colored `git` and
  test output still look colored. It covers streaming replies, restored
  sessions, tool rows, log entries, the approval dialog, and the question
  modal. The question and its options are filtered before the user sees
  them, so the answer the model receives is the answer that was shown.
  This closes the open S-4 item in the security review.
- The CI security job pins `govulncheck` to v1.9.0. Installing `@latest`
  made the scan depend on whatever upstream shipped that day, which failed
  a release pull request on a download error rather than on a finding.

### Added
- `internal/testharness/ttui`, a terminal harness that renders the real
  model and asserts on the characters a terminal would receive: frame
  width, content, and golden files (regenerate with `go test ./...
  -update`).
- Terminal tests for the home and chat panes at 40x15, 80x24, and
  120x30, for input ownership (no overlay leaks a keystroke or a chord
  into the composer), for every overlay closing on Esc, and for a 16 KB
  paste keeping the frame intact.
- `docs/tui-testing.md` describes the harness, the golden workflow, and
  what belongs in a terminal test.

## [0.3.40] - 2026-10-24

### Fixed
- `ask_user_question` no longer writes its prompt to stdout and reads
  `os.Stdin` while the TUI owns the terminal. Under a full-screen terminal
  (including Omarchy's) the question landed inside a rendered frame and the
  reader raced Bubble Tea for keystrokes, so the options were unusable. The
  tool now asks through a modal: digits 1-9 pick a numbered option, typing
  takes a free-form answer, Enter sends, Esc skips, and cancelling the turn
  closes the modal instead of leaving it owning the keyboard. The stdin path
  remains for headless runs, and the transcript row shows the question text.
- The ask row's display name and command preview accept the registered
  `ask_user_question` name instead of dumping the raw input map.

### Added
- An official container image: multi-stage build, Ubuntu 24.04 runtime as the
  unprivileged `sandbox` user, `/workspace` workdir, and `bash`, `git`,
  `curl`, and CA certificates. The `Container` workflow builds every PR
  (amd64, no push) and publishes `linux/amd64` and `linux/arm64` images on
  `main` and `v*` tags.
- `docs/openshell.md` and `examples/openshell/` document running the harness
  under NVIDIA OpenShell with a restrictive policy, a GitHub-enabled variant,
  and an OpenRouter provider profile that keeps the real key outside the
  sandbox. The security review now names this as the out-of-process
  mitigation for the accepted `bash` risk.
- `docs/design/openshell-integration-decision.md` records why the integration
  stays external, why the Go SDK waits for 1.0, and why NVIDIA Sentry is out
  of scope (BlueField-4 reference design, no download).

### Validation
- New tests cover the question modal (option picks, typed answers, editing
  keys, Esc, context cancellation, and app-level key ownership), the host
  callback contract, and the ask display path. The full suite and a race run
  over the touched packages pass.
- The container workflow is the build gate for the image; the release tag
  publishes the versioned and `latest` tags.

## [0.3.39] - 2026-10-20

### Fixed
- `web_fetch` no longer treats any `http`-prefixed string as a URL. It
  accepts only http and https with a host and no embedded credentials,
  refuses loopback, private, link-local, and carrier-grade NAT
  destinations after DNS resolution (which also covers redirects and
  rebinding), stops after five redirects, and reads at most 1 MiB before
  applying the existing 10k-character window. Set
  `AGENT_HARNESS_ALLOW_PRIVATE_FETCH=1` to allow local and private hosts
  for local servers and tests. Non-2xx responses still return their
  status and body.
- The `write` and `edit` tools stage through a unique temp file in the
  destination directory instead of a predictable `<path>.tmp`, so
  concurrent calls cannot interleave and a planted symlink is replaced
  rather than followed.
- Credential loads repair a loose `credentials.enc`, `master.key`, or
  config-directory mode and refuse a symlinked store; saving tightens a
  loose config directory to 0700.
- Session exports redact authorization and x-api-key headers, private
  key blocks, `secret://` references, and the xAI, Google, Hugging Face,
  GitLab, npm, PyPI, and `sk_live_` token formats, in addition to the
  existing key/value, OpenAI, Anthropic, GitHub, AWS, and local-path
  rules.

### Changed
- The toolchain moves to go1.26.9 in `go.mod`, CI, release, and Windows
  workflows, and the `golang.org/x/*` modules are current. This clears
  nine reachable standard-library advisories, five of which also
affected `x/net`.
- `make complexity` reports function and package complexity hot spots
  from `scripts/verify/complexity-report/`, optionally gated with
  `COMPLEXITY_ARGS="-strict"` or `-max-complexity=N`.

### Added
- `docs/security-review.md` records the threat model, the findings
  fixed here, and the risks accepted with a follow-up plan.
- `docs/complexity-analysis.md` records per-domain size, complexity,
  coverage, and a ranked set of maintainability recommendations.

### Validation
- New tests cover the fetch guard (schemes, private addresses, redirect
  limits, body bounds, status preservation), atomic writes, credential
  permission repair and symlink refusal, and the widened export
  redaction rules.
- `go build ./...`, `go vet ./...`, `go test ./...`, and
  `govulncheck ./...` pass; the vulnerability scan reports no reachable
  findings.

## [0.3.38] - 2026-10-17

### Fixed
- Home actions remain focused after Escape or Ctrl+N, so New Chat and Export
  respond to Enter, Space, and their action keys.
- The API-key login modal offers clickable provider links, Ctrl+O to open the
  page, and Ctrl+Y to copy its URL, with feedback inside the modal.
- MCP subprocesses now start and complete initialization through the official
  Go SDK. Paginated catalogs, concurrent calls, cancellation, and late replies
  preserve request identity. Failed startup cleans up; healthy servers remain
  available when another server fails; shutdown closes owned processes.
- MCP results retain structured evidence, provenance, resources, and actionable
  tool errors. Oversized evidence keeps its complete private retrieval file;
  error status survives budgeting, audit, model continuation, and session reload.
  Matching progress notifications appear as sanitized transient TUI status.
- Test fixtures clear inherited application and XDG storage overrides, preventing
  tests from writing to the developer's real credential, settings, and data roots.

### Validation
- Independent raw JSON-RPC fixtures exercise lifecycle and failure recovery.
  End-to-end tests cover both agent execution modes and approval checkpoints.
- An opt-in REA 6.5.0 compatibility check verifies executable evidence against
  SHA-256, Go build metadata, and exact source byte ranges. See
  [MCP evidence and lifecycle](mcp-evidence.md) for reproduction and limitations.

## [0.3.37] - 2026-09-15

### Added
- Every overlay renders through one shared modal frame: a single width
  policy, one padding, and one scroll window that marks "... n more
  above/below" instead of dropping content, so a short pane can reach
  the whole list. Done before this, each modal picked its own width and
  placement, and the ones without a scroll window simply ran off the
  bottom of the pane.
- The login wizard shows where to get an API key for each hosted
  provider. The local runtimes (local, ollama, flm) are left out, since
  they authenticate with nothing.
- A boot-time credential failure now carries its fix: where to get a key
  and which key opens the wizard, instead of a bare verdict.
- The agent agrees on the goal before acting: answer first, ask one
  specific question when the request is ambiguous, then write the plan
  with `todo_write` before the first change.
- Suggested next steps, opened with `s` in the chat tab: a short list
  derived from the turn that just ran, each entry carrying the reason it
  is suggested, run with Enter like typed input.

### Fixed
- The chat composer and the working indicator are no longer clipped on a
  short pane. A 100x16 pane lost both, so the driver could not see that
  work was happening or type; the chrome is now measured and yields the
  decoration before the composer.
- The command palette always rendered its tersest footer hint, because
  the candidate list was ordered shortest-first while the loop returned
  the first fit.
- Selected rows in the model picker and command palette sat one column
  right of their neighbours; both selection markers are the same width.
- An unrecognized `todo_write` status is rejected with the allowed values
  instead of being accepted and silently rendered as pending, and the
  plan is echoed back so the model can read what it wrote.
- The login wizard asked ollama and flm for an API key the moment after
  promising they needed none.
- A model with no recorded provider no longer renders an empty `[]` tag.

## [0.3.36] - 2026-09-14

### Added
- The model cycle names the alternatives it moves between and states what
  cycling does, instead of switching model with no announcement.
- Settings rationale moves into a dedicated detail panel, so the value
  being chosen and the reason for it are readable at the same time.
- Logging in greets with a fresh session and a pointer to `/help`.
- The working indicator is separated from the content around it by a
  blank row above and below.

### Fixed
- A message submitted while a turn is running is queued instead of racing
  the turn, along with dropping the unreachable submit fallthrough the
  queue early-return left behind.
- Provider errors surface the provider's own sentence rather than the
  JSON envelope it arrived in.
- User settings persist as a delta, so a stale key cannot blank the
  project, and settings tests no longer write to the real home directory.
- Config layer merging drops no-op values, so a stale settings file can no
  longer blank out the project's configuration.
- Draft row height mirrors the textarea wrap pipeline, so an unbroken
  draft counts as the rows it actually occupies.
- Paste thresholds and preview counts measure runes, not bytes.
- Truecolor is enabled only on interactive terminals and honours colour
  opt-outs, and the colour profile is pinned so themes stay
  distinguishable.
- `/reset` deletes sessions through the session manager rather than a
  home-directory path.
- The NVIDIA default points at a model the endpoint actually serves.

### Changed
- Local llama.cpp defaults halve the context window and lower the
  sampling temperature.

## [0.3.35] - 2026-09-14

### Fixed
- Sessions stay visible on narrow and mobile panes: the sessions list
  reserved a five-row content floor and a four-column label budget, so
  a phone-sized pane could allocate more rows and width than it had and
  push the list out of the frame. A row now budgets strictly to the pane
  it is rendered for: the label takes only the width left over, and a
  pane too narrow for the age or the status badge drops those fields
  rather than rendering past the edge for the frame clip to cut in half.

### Changed
- The toolchain moves to go1.26.8 everywhere it is declared (CI, the
  release pipeline, and the module's minimum toolchain), ending a skew
  where CI ran 1.25 while releases built on 1.26.1. That clears the five
  reachable standard-library advisories govulncheck reported against
  go1.26.5, including encoding/xml (GO-2026-6088), encoding/asn1
  (GO-2026-5972) and net/http (GO-2026-5026), all fixed from go1.26.6.
  The install guide and the Termux skill name the same line, so a source
  build cannot silently reintroduce them.

## [0.3.34] - 2026-09-09

### Fixed
- The Sessions tab no longer vanishes on narrow mobile panes (Termux):
  the session row renderer sliced its label with a width budget that
  went negative on phone panes, panicked, and the frame's panic
  recovery rendered an empty screen. Rows now budget style padding,
  prefix, age, and status badge before truncating (ANSI-aware), and
  mobile panes render a single-pane full-width list — the two-pane
  list+detail split stays desktop-only.

## [0.3.33] - 2026-09-09

### Fixed
- TUI frame borders stay put when switching tabs on mobile panes
  (Termux): a sub-view row wider than the frame's inner width wrapped
  at the terminal and shifted the bottom chrome. The frame now clips
  any overflowing row (ANSI-aware), and Home session lines truncate
  with an ellipsis instead of overflowing. Regression test pins the
  frame invariant across all tabs at phone-pane sizes.

## [0.3.32] - 2026-09-09

### Added
- User-settable Home tagline (Settings → Appearance → Home Tagline), saved
  to the user config layer; the Home header now titles the app with the
  tagline as its subtitle.
- Settings navigation wraps at both ends, and the footer hint names the
  action for the selected row type (Toggle / Next choice / Edit text).

### Changed
- The layered config is the source of truth for the preferred model: new
  chats from Home or Sessions adopt it, /model updates it, and historical
  sessions no longer overwrite the user's default on restart. Environment
  pins still win.
- Home recent sessions sort newest-first and keep the selection visible
  while navigating.

### Fixed
- The agent stream preserves narration text between tool calls in the
  persisted message content instead of dropping it.

## [0.3.31] - 2026-09-08

### Changed
- Version bump to v0.3.31

## [0.3.30] - 2026-09-08

### Changed
- Version bump to v0.3.30

## [0.3.29] - 2026-09-08

### Changed
- Give development turns room for 50 model responses and 100 tool calls by
  default. Rechecking a command after intervening work no longer triggers
  duplicate-loop protection; explicit session limits remain enforced.
- Render live tools inside the agent response with incremental transcript
  assembly, clearer working indicators, and shell-style input history.

### Fixed
- Harden cancellation, tool completion records, delegated-agent bounds,
  malformed provider responses, and shell process cleanup.
- Preserve session settings and tool results across reloads, and surface
  unreadable saved sessions without silently losing recovery information.
- Make manual permissions request approval for every tool call and improve
  approval cancellation handling.
- Add development-workflow, session-recovery, and memory-guard regression
  coverage.

## [0.3.28] - 2026-09-07

### Added
- Codex-style tool-call display: consecutive same-class tool calls group
  under one header (Shell/Read File/Search) with each call as an indented
  sub-list row; `ls` renders under Shell, `edit` renames to Update, todo
  calls render as a visible ✓/→/○ checklist, shell calls show
  `$ <command>` rows, and the duration column right-aligns consistently
  (PR #22).
- `/cleanup`: list saved sessions by size/age; delete named sessions or
  everything non-active with `--confirm`. Deletion goes through
  SessionManager.DeleteSession (active-session guard, audit trail).
- Release-flow gates: release branches must be named exactly
  `release/X.Y.Z` (CI job + pre-push hook + script unit tests), and tag
  pushes are verified against `main.go` Version before any binary is
  built — the 0.3.25/0.3.26 shipped-as-0.3.24 failure mode is now a
  fast CI failure (PR #22).

### Changed
- ToolInputJSON is now populated on both the live path and session
  reload, so expanded tool records show the real input and todo
  checklists survive a reload (PR #22).

### Fixed
- Scroll-back was impossible during a live turn: the 0.3.27 deferred
  refresh flushed with force-bottom, so timer ticks yanked scrolled-up
  users back to the bottom; wheel events also scrolled stale content.
  Follow semantics restored, wheel flushes pending rebuilds first
  (PR #22).
- ESC on a streaming turn left a frozen thinking spinner on the
  cancelled message; cancel now finalizes partial content and clears
  the streaming state (PR #22).
- /model slash switches left the composer mode line showing the old
  model; the slash path now notifies the TUI like the settings path
  (PR #22).
- Session reload rendered tool rows as bare carets (no tool name);
  the reload mapping now populates the same display fields as the
  live path (PR #22).

### Performance
- Marathon lag eliminated: the markdown render cache (4096) and group
  cache (8192) were both smaller than a 10k-event session's working
  set (~10,000 groups), thrashing the LRU into a full glamour
  re-render every frame. Capacities raised to 32768/65536: streaming
  frames at 10k events 2.60s/op → 71.6ms/op, allocations 16.6M →
  52.8K per frame; composer keystrokes stay flat (~0.4ms) (PR #22).

## [0.3.27] - 2026-09-06

### Added
- Group render cache and deferred refresh: each transcript group renders
  once and memoizes on a signature covering every field its render reads,
  and high-frequency mutators (per-chunk stream updates, timer ticks)
  coalesce into one rebuild per frame (PR #21).

### Changed
- PR descriptions must end with the `Generated by Agent-Harness` footer;
  CODEOWNERS added (PR #21).

### Fixed
- End-of-session slow-down in marathon transcripts: keystroke latency
  while streaming is now flat in transcript size (~0.4ms at 10k events,
  previously degrading to seconds); streaming frames ~0.3ms at 10k events.
  Review findings on the same PR fixed stale fingerprints on same-length
  content replacement, value-receiver flush loss, unbounded group-ref
  growth, and a benchmark that measured a no-op.

## [0.3.26] - 2026-09-06

### Added
- Whole-conversation copy via `Y` in chat navigate mode — content copied
  verbatim, trimmed only for emptiness (PR #17).

### Changed
- Symmetric mode-line hints: navigate and typing modes now advertise each
  other's entry key in the same slot (PR #17).

### Fixed
- Bounded markdown render cache with LRU eviction: ~3x faster repaints,
  36x fewer allocations on 500-message transcripts, and the OOM
  regression in unbounded block-fitting pinned by test (PR #17).

## [0.3.25] - 2026-09-06

### Added
- Omniroute provider support (PR #18).

### Fixed
- fitBlockCode wrapping pinned against the OOM spin regression by test
  (PR #18).

## [0.3.7] - 2026-08-08

### Added
- Spinning-diamond thinking indicator in agent headers with a one-second
  quiet beat before the response materializes; instant answers skip the
  thinking phase entirely.
- Command palette (Ctrl+P) and reasoning-effort cycling (Ctrl+R).
- Layered user settings persistence with delta writes, local-over-user
  precedence, and encrypted credential storage.
- Laptop-first local model flow: agent-harness.yml targets a llama.cpp
  server; /diagnose resolves config and endpoint reachability.
- VHS demo stack: tab-tour tape, mock OpenAI-compatible server, and
  demo boot wrapper under scripts/demo.

### Changed
- Chat lands in navigate mode with the composer blurred; digits, j/k and
  h/c own the keyboard until 'i' enables typing.
- Composer and footer span the full terminal width with the input block
  on a solid surface, quiet system messages, and streamed chunk counts
  in agent headers.
- Welcome message renders the git root as its basename, never the full
  path.
- README and docs reorganized in the terminal-jarvis shape with a
  docs-index hub and a demo guide.

### Fixed
- Slash command output no longer truncated by synchronous dispatch.
- Provider switching keeps the current model and follows the endpoint.
- Session model wins at boot instead of being clobbered by defaults.
- Repo no longer tracks a stale 20MB root-level binary.

## [0.3.6] - 2026-07-31

### Fixed
- Fixed TUI slash command execution by dispatching `UserCommandMsg` asynchronously via `tea.Cmd`, preventing system response truncation.
- Fixed command palette selection to immediately execute zero-argument commands and format prompt inputs for parameterized commands.
- Enforced single source of truth for command discovery across `/help`, tab autocomplete, descriptions, and command palette.
- Added Gopter property-based test suite for slash command execution invariants and discovery surface feature-flag isolation.

## [0.3.5] - 2026-07-30

### Fixed
- Feature-flagged slash commands hidden from `/help`, tab completion, and autocomplete descriptions.

## [0.3.4] - 2026-05-01

### Changed
- Version bump to v0.3.4

## [0.3.3] - 2026-05-01

### Changed
- Version bump to v0.3.3

## [0.3.2] - 2026-04-27

### Changed
- Version bump to v0.3.2

## [0.3.1] - 2026-04-27

### Fixed
- Home tab now focused on startup so 'n' (new chat) and 'e' (export) shortcuts work immediately
- Chat tab defaults to insert mode when switching views — users can type without pressing 'i' first

## [0.3.0] - 2026-04-26

### Changed
- Version bump to v0.3.0

## [0.2.8] - 2026-04-26

### Added
- `MaxToolCalls` limit (default 15) to agent loop to prevent runaway exploration
- Ginkgo BDD specs for loop tool-call limits and max-turns behavior
- Structured output with counts and truncation for `ls_recursive` and `find`
- Truncation at 200 entries for `ls_recursive` and `find` to prevent context bloat

### Fixed
- Reduced `DefaultMaxTurns` from 100 to 10
- Added explicit "stop exploring after 3-4 attempts" guidance to system prompt

## [0.2.7] - 2026-04-26

### Added
- `ls` tool: single-directory listing with file/dir markers and counts
- `find` tool: recursive filename search by glob pattern, skips ignored dirs
- Ginkgo BDD specs for filesystem tools (ls, find, ls_recursive, glob)

### Fixed
- Registered `ls_recursive` tool (was implemented but never exposed to LLM)
- `glob` tool description no longer falsely claims `**` recursive support
- System prompt now directs LLM to use dedicated filesystem tools instead of bash

## [0.2.6] - 2026-04-26

### Changed
- Version bump to v0.2.6

## [0.2.5] - 2026-04-26

### Changed
- Version bump to v0.2.5

## [0.2.4] - 2026-04-25

### Added
- Comprehensive BDD test coverage for all TUI tabs
- App-level Ginkgo specs for view switching, navigation, and message routing

### Fixed
- Viewport reserved space calculation (3 → 5) for accurate content height
- Command palette, model picker, and approval dialog bugs found via TDD

## [0.2.3] - 2026-04-25

### Changed
- Version bump to v0.2.3

## [0.2.2] - 2026-04-25

### Changed
- Version bump to v0.2.2

## [0.2.1] - 2026-04-22

### Changed
- Version bump to v0.2.1

## [0.2.0] - 2026-04-22

### Added
- Functional Persona system with 5 specialized roles (internal/core/persona)
- Home dashboard tab in TUI with project overview and stats
- Granular security permission toggles in Settings view
- Audit logging and sandbox preview for tool execution
- Red-team containment tooling and block-all-squatters utility

### Fixed
- Strengthened symlink-aware workspace containment for security
- Resolved all outstanding Copilot review comments on Persona dashboard
- Gofmt formatting and quality check alignment

### Changed
- Refactored workspace containment return logic for simplicity
- Simplified TUI/Security interaction for better responsiveness

## [0.1.15] - 2026-04-19

### Changed
- Version bump to v0.1.15

## [0.1.14] - 2026-04-18

### Changed
- Version bump to v0.1.14

## [0.1.13] - 2026-04-18

### Changed
- Version bump to v0.1.13

## [0.1.12] - 2026-04-18

### Changed
- Version bump to v0.1.12

## [0.1.11] - 2026-04-18

### Changed
- Version bump to v0.1.11

## [0.1.10] - 2026-04-18

### Changed
- Version bump to v0.1.10

## [0.1.9] - 2026-04-18

### Fixed
- Suggestion dropdown scrolls window as user navigates past visible items
- ModelChangedMsg now syncs model value to settings tab
- ClearChatMsg handler keeps async message listener alive
- Add /logout and /login commands for auth re-prompt flow

## [0.1.8] - 2026-04-18

### Fixed
- Model status bar now updates when switching models via /model or Settings
- Inline autocomplete replaces modal command palette (type / to filter commands)
- /clear race condition fixed: confirmation message preserved atomically
- HTTP client recreated on provider change, fixing 401 auth errors
- Settings tab model changes now sync to chat status bar

## [0.1.7] - 2026-04-18

### Fixed
- Chat loop: errors now visible instead of silent failures
- Model status bar: updates correctly when switching models
- CI: all staticcheck and gofmt issues resolved

### Added
- StreamError event type for error propagation
- ModelChangedMsg for Bubble Tea event loop integration
- Loop tests for error and empty stream edge cases

## [0.1.5] - 2026-04-13

### Changed
- Version bump to v0.1.5

## [0.1.4] - 2026-04-13

### Fixed
- Slash command help deduplication: removed duplicate /exit and /memory entries
- Race condition in command palette execution (removed goroutine mutation of TUI state)
- Deterministic /help output by using ordered category slices
- Missing user message logging for slash commands in chat history
- Missing /workspace command in help and command palette

## [0.1.3] - 2026-04-13

### Fixed
- Respect AGENT_HARNESS_PROVIDER and AGENT_HARNESS_MODEL environment variables
- Prevent secure credentials from overriding explicit env-based provider/model selection

## [0.0.54] - 2026-04-06

### Changed
- Version bump to v0.0.54

## [0.0.48] - 2026-04-04

### Fixed
- Credential decryption error handling with user-friendly recovery options
- Numeric model input (1, 2, 3) now maps to actual models instead of literal "1"
- ESC key now properly cancels running agent execution
- Model display validation to catch invalid numeric-only model names

### Changed
- Tool calling UX: now shows grey command preview like Kimi does
- Animated tool display in yolo mode: spinner + tool name + command preview
- Single-line tool animation instead of endless [bash] bash repetitions
- Better password input handling with whitespace trimming

### Security
- Added validation for corrupted credential files (salt/nonce/ciphertext)
- Clear master key on decryption failure to force fresh password prompt

## [0.0.47] - 2026-04-04

### Added
- Command approval system with two execution modes:
  - Interactive mode: prompts for approval before executing shell/write/edit commands
  - Yolo mode: auto-approves commands but shows what is happening in the UI
- Approval dialog with four options: Approve, Approve All, Reject, Reject + Suggest
- Pattern memory: remembers "Approve All" and "Reject All" choices per session
- ESC key integration to cancel agent execution at any time
- Command visibility: always see what commands are about to run

### Changed
- Tool display name changed from "bash" to "Shell" for clarity
- Removed emojis from error messages (replaced with text indicators)
- Slimmed down README with clearer documentation structure
- Added awesome-tuis credit to acknowledgments

### Documentation
- New docs/command-approval.md explaining the approval system
- Updated debugging-patterns.md with approval system patterns

## [0.0.46] - 2026-04-04

### Fixed
- TUI status bar now updates correctly when selecting model via picker
- Model selection via /model command or Settings view reflects immediately in status bar

### Added
- Release publish script for streamlined release workflow

All notable changes to agent-harness will be documented in this file.

## [0.0.45] - 2026-04-04

### Changed
- Status bar now shows actionable hints instead of "default" when no model is set
- Status bar shows [⚠ no model] warning when model is not configured
- Improved model display: shows shortened model name or "(use /model)" hint

### Added
- Visual feedback with actionable hints when models fail to respond
- Error messages now include specific guidance for common failure patterns:
  - Timeout errors: suggests switching models with /model command
  - Connection errors: suggests checking /config and Settings
  - Rate limit errors: suggests trying different models
  - Authentication errors: suggests updating API key in Settings
  - Model not found: suggests using /model to list available models
- Follows visual-ux skill patterns: uses ⚠, ?, → indicators consistently

## [0.0.43] - 2026-04-04

### Added
- Release workflow skill to prevent version mismatches
- check-version.sh script for version validation
- bump-version.sh script for semver calculations
- release.sh script for one-command releases

### Fixed
- Version alignment: bumped to 0.0.43 to match release process

## [0.0.41] - 2026-04-04

### Added
- TUI design patterns documentation based on awesome-tuis research
- Analyzed top TUI projects (lazygit, k9s, lazydocker, yazi, gh-dash)
- Documented universal patterns: viewport components, status bars, vim navigation

### Changed
- Enhanced TUI architecture with patterns from top-starred TUI projects
- Improved visual design consistency with semantic color system

## [0.0.40] - 2026-04-03

### Added
- Tab-based navigation (Chat, Sessions, Settings)
- Command palette for quick command access
- Model picker for interactive model selection
- Vim-style navigation modes (insert/normal)
- Real-time streaming response display
- Markdown rendering for assistant responses
- Response time tracking and display
- Status bar with contextual keybindings
- Activity indicators for tabs with unseen updates

### Changed
- Migrated to bubbletea-based TUI architecture
- Improved terminal handling for Termux environment

## [0.0.39] - 2026-04-01

### Added
- Initial TUI mode with basic viewport
- Session management UI
- Settings configuration view

### Fixed
- Terminal input handling on mobile devices
## [0.0.49] - 2026-04-05

### Fixed
- Tool UI now uses per-tool UserFacingName and GetActivityDescription methods
- Rich activity descriptions show what tools are doing (e.g., 'Reading file.go (lines 10-20)')
- Anti-pattern fixed: removed hardcoded switch statements for tool display names
