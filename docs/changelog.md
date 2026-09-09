# Changelog

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
