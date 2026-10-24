# The journey

0.4.0 makes the harness a place you can read rather than a sequence of
tabs, and it makes the agent's loop visible.

This document is the spec: what the journey is, what it refuses to
become, and how each piece is checked. It exists so the work stays
coherent across releases and so an agent picking this up knows what
"done" means.

## The shape of it

```
home base ──► chat ──► the loop ──► back to home base
   │             │          │
   │             │          └─ the human can interrupt at any beat
   │             └─ a turn is four beats, and you can see which one
   └─ the workspace, the provider, and the recent sessions, drawn
```

The beats are the ones an agent loop already has:

| Beat | What happens | What the user sees |
|---|---|---|
| Ask | The model decides what it needs | The question, if it asks one |
| Plan | The plan is written | The plan, before anything changes |
| Review | The user approves the risky parts | What is waiting, and on what |
| Try | The work runs | The tools, the diffs, the results |

## The rules it lives by

- **The gears turn in view.** The user can see which beat the agent is
  in and what it is waiting for. A harness that hides its state makes
  the user guess, and guessing is the expensive part.
- **The human holds the controls.** The agent waits where a person
  would want it to wait: before it changes something, and before it
  decides on its own. Steering is discoverable, not a hidden chord.
- **The plain path always works.** Every element degrades to text. The
  flourish is an enhancement over a workbench that already works, never
  the other way around.
- **Motion means something, and it can be turned off.**
  `AGENT_HARNESS_REDUCED_MOTION=1` (or `reduced_motion: true` in
  settings) holds every animated element still. The information stays;
  only the movement goes.
- **Nothing decorative.** The sketch draws the workspace, the provider,
  and the sessions because those are the facts. It is not a wallpaper
  with a status line on top.
- **Every beat is checked.** The terminal harness drives the journey and
  the frames are goldens.

## What each piece is

### The loop track

A quiet line in the chat frame: Ask → Plan → Review → Try, with the
current beat marked. It is derived from state the app already tracks
(thinking, streaming, tool calls, approvals), not from a new model of
the turn. A key jumps to a beat in the transcript, so the track is also
a way to navigate.

### Visible human-in-the-loop

When the agent waits for an approval, a plan review, or an answer, the
frame says so — "waits here until you approve" — and the way to
redirect it (steer, answer, reject) is on screen. Today the steer queue
exists but is undiscoverable, and the approval dialog appears without
saying which beat it belongs to.

### Home base as a sketch

The Home pane becomes a drawing of the workspace, the provider, and the
recent sessions, in the same spirit as a notebook page: continuous
strokes, restrained accents, no glow, the marks you care about. It is
the same information the pane has now, drawn so the eye finds it.

### The paper plane

A delegated sub-agent is a flight between places: it leaves, it works,
it comes back with an answer. Today a delegation is a spinner and a
row. The plane makes the delegation visible, and it is the same
sub-agent machinery 0.3.42 made configurable.

### Reduced motion

A setting, an environment variable, and a promise: with it on, nothing
in the interface moves. The elapsed clock still counts, because that is
information, not decoration.

## How it is checked

- The terminal harness (`internal/testharness/ttui`) drives the whole
  journey at three pane sizes: home base, chat, a beat transition, an
  approval, a steer, and the way back.
- Golden frames cover the overlays the journey adds.
- Reduced motion has its own tests: the frames must not change between
  ticks.
- Nothing may clip the frame at 40x15, 80x24, or 120x30.

## What it is not

- Not a dashboard. The Home pane is a place, not a report.
- Not a rewrite of the transcript. The record stays plain and readable.
- Not gated on animation, color, or a capable terminal.
- Not a place to hide a slow operation behind a flourish.

## Where it came from

The taste is borrowed from a sibling project, `my-life-as-a-dev`: a
notebook page that became a small world you can walk around, with a
paper plane between four places, a plain workbench always available
underneath it, reduced-motion support, and e2e quality gates before
anything deploys. The lesson worth keeping is the order of things: the
plain path first, the world on top, and the checks that keep both
honest.
