# Demo

The session shown in the README (`docs/demo-tui.gif`) was recorded with
[VHS](https://github.com/charmbracelet/vhs) from `scripts/demo/tui.tape`:

1. boot via `scripts/demo/demo-boot-live.sh` -- env pins the fireworks
   provider with the GLM model and a scratch session store, so the TUI
   lands straight on the Home tab with a clean transcript
2. Chat tab (`2`) -- the welcome message
3. `/mode auto` -- the agent mode chip flips and a status flash
   announces what changed (Shift+Tab cycles the same ring)
4. the task -- a real turn: the agent runs `ls` through the tool loop
   and answers with a markdown table
5. `Ctrl+C` -- leave

No mock server: the turn is the showcase. The fireworks API key comes
from the encrypted credential store (configure once via `/login`);
nothing is persisted from the recording itself.

`scripts/demo/demo-boot.sh` + `scripts/demo/mock-llm.py` remain for
offline recording: the mock serves an OpenAI-compatible endpoint on
`127.0.0.1:8080` and echoes every received prompt (`PYLOAD: ...`) to
its stdout -- ground truth for verifying a recording typed the right
message.

## Recording

Install VHS (`go install github.com/charmbracelet/vhs@latest`) plus
`ffmpeg` and `ttyd` on PATH, then from the repository root:

```bash
chmod +x scripts/demo/demo-boot-live.sh
vhs scripts/demo/tui.tape
```

and replace `docs/demo-tui.gif`.

## Making new demos

The tape is plain text; copy `scripts/demo/tui.tape` and adjust:

- `Set Width` / `Set Height` / `Set Padding` / `Set FontSize` -- match the
  window to the terminal content so frames never scroll.
- `Sleep` between steps -- the recorder runs in real time; a beat is as long
  as its `Sleep` plus whatever the app takes to respond. Live model turns
  need a generous sleep (45s in the current tape).
- Switch tabs with the digit hotkeys (`1`-`4`), never `Tab`: under the
  recorder, a Tab never switches the view and the first typed characters get
  swallowed by the app.
- VHS cannot send `Shift+Tab`; use `/mode <name>` to showcase the agent
  modes instead.
- `Type "..."` then `Enter` types a command; give `Enter` a `Sleep 800ms`
  before submitting, or the press is dropped mid-typing.
- Record with a clean session store or the session list shows strays from
  every prior run (the live boot wrapper wipes its scratch store for you).
- The demo uses the real binary, so keep the chat welcome path-free: the git
  root renders as its basename by design (see `cmd/agent-harness/project.go`).
