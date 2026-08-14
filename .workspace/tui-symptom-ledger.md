# TUI Symptom Ledger — release/v0.3.8

Source of truth for the TUI quality stack. Every entry: screen → symptom
(what actually breaks / costs effort) → expected fix → the line/function
that should change. Ranked by user-visible pain. Verified by real-driver
walks (pty driver `/tmp/opencode/drive_tui.py` + tmux, TERM=tmux-256color,
binary built at HEAD d7c84b8, provider=local, no LLM).

## CRITICAL

### C1. App crashes on ESC followed by any key — osc reader overflow
- Screen: all (reproduced: Settings edit → ESC → Down; Chat → ESC → `:`/q/Tab; help → ESC → any key).
- Symptom: `panic: runtime error: index out of range [3] with length 3` in
  `oscStrippingReader.Read`, goroutine in tea's readLoop → **the whole app
  dies** on ordinary key sequences. The reader's `escPending` path emits two
  output bytes (ESC + the byte) for one input byte and never bounds-checks
  `w` against `n`; a read chunk of `ESC` + `ESC[B` (3 bytes → 4 output
  bytes) or `ESC` + `:` (1-byte read) writes past the buffer.
- Expected fix: bounded writes with a spill buffer so filtered output that
  exceeds the read is returned on the next Read; strip only complete OSC
  spans; never panic.
- One line: `internal/interface/tui/osc_input.go:121-127` (`buf[w] = ...`
  in the `escPending` and `default` cases).

### C2. Lone ESC keypresses are swallowed — ESC never reaches bubbletea
- Screen: all (reproduced: Settings edit mode — ESC does not cancel the
  edit; `?` help — ESC does not close it; Chat insert mode — ESC does not
  return to navigate).
- Symptom: a read ending in a lone `ESC` sets `escPending` and returns 0
  bytes; the ESC is held forever (if no further key arrives) or materializes
  as a phantom `ESC+key` (alt+key) on the next keystroke. Every ESC the user
  presses either does nothing or corrupts the following key.
- Expected fix: same rewrite as C1 — when a read ends with a pending ESC
  and the next byte is not `]`, emit the ESC immediately; only hold it
  within the read loop for OSC-span detection.
- One line: `internal/interface/tui/osc_input.go:109-129` (`escPending`
  handling).

## HIGH

### H1. ~5s boot stall: git context collected synchronously before the TUI
- Screen: boot → Home/Chat (everything waits).
- Symptom: `newApp()` runs `git.GetContext()` before the TUI starts
  (`cmd/agent-harness/app.go:69`); it shells out 8 sequential git
  subprocesses (`pkg/git/context.go:32-79`), including `git status`
  TWICE (≈2.1s each on this OneDrive/9p filesystem) → ~4.7s black screen.
  Verified in tmux: pane empty for 4-6s. Keys typed during the stall are
  echoed and land as garbage in the UI once it boots.
- Expected fix: collect git context in parallel (one `git status` used for
  both HasChanges and StatusFiles), and/or async: fire the collection in a
  goroutine and `Send()` a `GitContextMsg` when ready, matching the
  provider-probe pattern (`app.StartProviderProbe`).
- One line: `cmd/agent-harness/app.go:69` + `pkg/git/context.go:32`.

### H2. 0-5s flaky boot stall: terminal background-color query race
- Screen: boot (intermittent black screen 5-10s).
- Symptom: bubbletea's package `init()` calls `lipgloss.HasDarkBackground()`
  (`tea_init.go:21`) and glamour's `WithAutoStyle()` calls
  `termenv.HasDarkBackground()` on the first markdown render
  (`chat_markdown.go:28-31`); termenv waits `OSCTimeout` (5s) per query when
  the terminal's replies race/are eaten (observed deterministically in a raw
  pty, flakily in tmux). The app never uses the queried value — all colors
  are a hardcoded dark palette (no AdaptiveColor anywhere).
- Expected fix: pin the dark background before any query fires
  (`lipgloss.SetHasDarkBackground(true)` from an early-initialized package
  imported first by main; `glamour.WithStandardStyle("dark")` instead of
  `WithAutoStyle`).
- One line: `internal/interface/tui/chat_markdown.go:29` +
  new early-init package + `cmd/agent-harness/main.go` import order.

## MEDIUM — Settings tab

### M1. Settings layout: noisy category headers, misaligned values, inconsistent rows
- Screen: Settings.
- Symptom: `── Provider & Connection ──` ASCII headers; values start at
  different columns per row; descriptions only under the selected row;
  bool rows render a checkbox but other rows render `label  value [opts]`;
  `(empty)` for missing values; System Messages squeezed under the list;
  footer lists every key at once; selected row's description pushes the list
  height around while navigating.
- Expected fix: aligned label/value columns (fixed label width), consistent
  row rendering, dimmed descriptions, quieter category separators, a
  contextual footer, and a dedicated (scrollable) System Messages region.
- One line: `internal/interface/tui/settings_view.go:24-93` + `settings.go`
  render loop; styles in `styles_lists.go`/`styles_panels.go`.

## MEDIUM — slash commands / palette

### M2. Command palette: duplicate commands and duplicate category headers
- Screen: CommandPalette (Ctrl+P).
- Symptom: searching "persona" lists `/persona` 9 times; category headers
  repeat ("System" ×2, "Session" ×2). Noisy, hard to read, feels broken.
- Expected fix: dedupe palette entries by command text; unique category
  headers (merge repeated ones).
- One line: `internal/interface/tui/command_palette.go` list assembly +
  `cmd/agent-harness/commands_core.go` command registration.

### M3. `:` does not open the command palette
- Screen: all normal-mode views.
- Symptom: only `Ctrl+P` opens the palette; the k9s-style `:` is unbound
  (pressing `:` in navigate mode does nothing). Users trying `:` get no
  slash-command surface.
- Expected fix: bind `:` in normal mode → `openCommandPaletteMsg` (palette
  hint already says "Type / then search").
- One line: `internal/interface/tui/app_keys.go:63-71` (global chords
  switch).

### M4. Palette footer text clipped at narrow widths
- Screen: CommandPalette.
- Symptom: footer `j/k: navigate  Enter: select  Tab: auto-complete  scroll`
  is cut off ("scroll" truncated) at the standard panel width.
- Expected fix: shorten the hint or measure panel width.
- One line: `internal/interface/tui/command_palette.go:372-380`.

## MEDIUM — Chat tab

### M5. Chat layout: no visual hierarchy in the message pane
- Screen: Chat.
- Symptom: user messages render as `│`-column blocks with the text indented;
  the welcome block, system notices and agent turns all share one flat
  stream; the composer divider is a full-width `──` line; the placeholder
  `◆ Press i to type a message` changes to `◆ Type a message...` with no
  visible input until typing starts; mode line repeats context already in
  the status bar (`typing · developer · nvidia...120b(free) · local ·
  effort medium`).
- Expected fix: light redesign — clear message separation (bubble/quote
  block per speaker), tighter composer column, dimmer mode line, and an
  always-visible input affordance when focused.
- One line: `internal/interface/tui/chat_view.go:40-90` + `chat.go`
  textarea/composer styles; `styles_chat.go`.

### M6. Chat: keys pressed during the boot stall are lost/misinterpreted
- Screen: Chat (and everywhere).
- Symptom: with the H1 stall, typing "hello agent" before the TUI is up
  lands as global bindings (the `n` in "agent" triggered a new chat), keys
  get echoed, and the composer never sees the text.
- Expected fix: H1 (boot fast) — no extra code; the ledger entry documents
  the coupling.

## LOW / verify

### L1. Help overlay polish
- Screen: HelpView. ESC-close broken = C2. After C2, re-verify open/close
  and scrolling; help content is current and readable (width fits).

### L2. Model picker flow
- Screen: ModelPicker. Blocked by C1 during the audit; verify after C1:
  `/model` in chat insert mode → picker opens, j/k navigate, Enter selects,
  Esc cancels.

### L3. Approval dialog
- Screen: ApprovalDialog. Not reachable without an agent turn; validate
  with `scripts/demo/mock-llm.py` + a tool-permission request, or via the
  ginkgo suite. Verify focus, y/n keys, cancel path after C1.

### L4. Sessions tab
- Screen: Sessions. Layout already decent (list + details panel); verify
  Delete/Export/Copy/refresh interactions after C1 (they were blocked by
  crashes in the walk).

## FEATURE — NVIDIA demo path

### F1. NVIDIA provider support
- Screen: Settings / ModelPicker / auth.
- Symptom (gap): the default model is an NVIDIA nemotron served via
  OpenRouter (`nvidia/nemotron-3-super-120b-a12b:free`); there is no
  first-class `nvidia` provider — no endpoint
  (`https://integrate.api.nvidia.com/v1`), no `NVIDIA_API_KEY` (nvapi-…)
  auth path in the credentials store, no NVIDIA catalog entry for
  `nvidia/nemotron-3.5-lightning-30b-a3b` and friends.
- Expected fix: add `nvidia` to `DefaultEndpointForProvider`
  (`internal/core/config/defaults.go:37-52`), provider list in Settings,
  `NVIDIA_API_KEY` env + secure-store support (`credentials.go`), a
  `getModelsForProvider("nvidia", …)` catalog, and confirm the client
  tolerates the `reasoning_content` streaming delta (it is ignored by the
  current SSE struct — verify with a fixture).
- One line: `internal/core/config/defaults.go:37` +
  `cmd/agent-harness/model_catalog.go:20-70` +
  `cmd/agent-harness/credentials.go:15-80`.

### F2. NVIDIA demo flow
- Screen: demo (scripts/demo/*).
- Expected fix: demo-boot.sh variant that runs against the NVIDIA API with
  `NVIDIA_API_KEY` (nemotron-3.5-lightning-30b-a3b, thinking enabled),
  mock-llm.py as the offline fallback, and a docs page
  (docs/supported_models.md + docs/demo.md) with the free-tier note.
- One line: `scripts/demo/demo-boot.sh` + `scripts/demo/mock-llm.py`.

## Validation protocol (per PR)
1. `make fmt` → `make verify` (structure + go test) → `make test` → `make lint` → `make build`.
2. Real-driver: pty/tmux walk of the changed screen (this ledger's scenarios),
   plus `codex exec` pass where a human-like driver is useful; record the
   outcome in the PR's ## Verification.
3. GitNexus `impact` before editing each symbol; `detect_changes` before
   each commit; report blast radius; HIGH/CRITICAL → stop and surface.
