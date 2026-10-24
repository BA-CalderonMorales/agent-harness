# Terminal testing

The TUI's tests assert model state, which is the right level for
behavior and the wrong level for a terminal. A frame can be clipped,
wider than the pane, or missing chrome while the model looks perfect.

`internal/testharness/ttui` closes that gap: it renders the real model,
records every frame, and lets a test assert on the characters a terminal
would have received.

## Using the harness

```go
app := NewApp()
h := ttui.New(t, app)
h.Resize(80, 24)

h.Send(QuestionRequestMsg{Request: req})

h.AssertFitsWidth(80)              // no line wider than the pane
h.AssertContains("staging")         // the frame shows what it should
h.AssertNotContains("\x1b]52")     // and nothing it should not
ttui.AssertGolden(t, "question-dialog", h.Text())
```

The harness is generic over `tea.Model` and deliberately does not import
the TUI package, so the TUI's own tests can use it without an import
cycle.

| Method | What it does |
|---|---|
| `Send(msg)` | Delivers a message and records the frame it rendered |
| `Type(text)` | Sends each rune as its own key press |
| `Press(name)` | Sends a named key (`enter`, `esc`, `up`, `ctrl+w`, ...) |
| `Resize(w, h)` | Tells the model the pane changed size |
| `View()` / `Text()` | The last frame, raw and ANSI-stripped |
| `Frames()` | Every recorded frame, ANSI-stripped |
| `AssertFitsWidth(w)` | Fails when any rendered line is wider than the pane |
| `AssertContains` / `AssertNotContains` | Frame content checks |

## Golden files

Goldens live in the calling package's `testdata/golden/` directory and
hold the normalized frame: ANSI removed, trailing space trimmed, and
timestamps, durations, ids, and temp paths replaced by placeholders.

Regenerate them after an intentional rendering change:

```bash
go test ./... -update
```

The golden diff in the pull request is the review artifact: a rendering
change should look like a rendering change, not like a mystery.

## What belongs here

- Frame invariants: nothing wider than the pane, chrome present, the
  expected rows visible.
- Overlay behavior: a modal owns the keyboard, Esc backs out one level,
  a cancelled turn closes the modal.
- Golden frames for the overlays whose content the test controls
  (question, approval). Frames that read the environment — the home
  view shows the workspace path and a provider probe — stay on invariant
  checks instead of goldens, because a golden that depends on the
  machine is a flaky test wearing a suit.

## What does not belong here

Model logic. Tool execution, permissions, and session state belong to
their own packages' tests; the terminal harness is for what the terminal
receives.
