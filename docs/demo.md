# Demo

The session shown in the README (`docs/demo-tui.gif`) was recorded with
[VHS](https://github.com/charmbracelet/vhs) from `scripts/demo/tui.tape`:

1. boot via `scripts/demo/demo-boot.sh` -- env pins `provider=local`, so the
   setup wizard is skipped and the TUI lands straight on the Home tab
2. Chat tab (`2`) -- the welcome message and the `Provider ready` status
3. Sessions tab (`3`) -- the session store
4. Settings tab (`4`) -- provider, model, mode, effort (scrolled with `j`)
5. Home tab (`1`) -- the project dashboard
6. `Ctrl+C` -- leave

No agent turn runs: the tab tour is the showcase. `scripts/demo/mock-llm.py`
serves an OpenAI-compatible endpoint on `127.0.0.1:8080` so the readiness
status shows green, and it echoes every received prompt (`PYLOAD: ...`) to its
stdout -- ground truth for verifying a recording typed the right message.

## Recording

Install VHS (`go install github.com/charmbracelet/vhs@latest`) plus `ffmpeg`
and `ttyd` on PATH, then from the repository root with the mock server running
and an empty session store:

```bash
rm -rf ~/.agent-harness/sessions   # first run only: show just the one session
python3 scripts/demo/mock-llm.py   # terminal 1
vhs scripts/demo/tui.tape          # terminal 2
```

and replace `docs/demo-tui.gif`.

## Making new demos

The tape is plain text; copy `scripts/demo/tui.tape` and adjust:

- `Set Width` / `Set Height` / `Set Padding` / `Set FontSize` -- match the
  window to the terminal content so frames never scroll.
- `Sleep` between steps -- the recorder runs in real time; a beat is as long
  as its `Sleep` plus whatever the app takes to respond.
- Switch tabs with the digit hotkeys (`1`-`4`), never `Tab`: under the
  recorder, a Tab never switches the view and the first typed characters get
  swallowed by the app.
- `Type "..."` then `Enter` types a command; give `Enter` a `Sleep 800ms`
  before submitting, or the press is dropped mid-typing.
- Record with a clean session store or the session list shows strays from
  every prior run.
- The demo uses the real binary, so keep the chat welcome path-free: the git
  root renders as its basename by design (see `cmd/agent-harness/project.go`).
