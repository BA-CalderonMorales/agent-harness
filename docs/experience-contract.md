# Experience Contract

Draft for the sensational release (release/0.3.36). Every interface change
on this branch is judged against this contract. The maintainer is the final
judge of look, feel, and behavior; passing tests never substitute for that
verdict. Inspiration from excellent terminal tools sets the bar; it never
authorizes cloning their appearance.

## Hierarchy

The draft, the streaming response, or the pending decision is always the
easiest thing to find. Status, decoration, and metrics yield space to it:
the composer grows to eight visible lines before scrolling, the transcript
keeps a five-row floor, and supporting chrome collapses first on narrow
panes. Nothing decorative may crowd out an approval decision.

## Density and spacing

Desktop space is useful; phone content stays legible. Resizing preserves
focus and keeps essential actions reachable. Spacing, emphasis, and
terminology agree across views and across loading, error, and recovery
states, so the interface reads as one tool rather than a set of screens.

## Focus and editing modes

Typing and navigate modes are always visibly distinct, with a clear cue
for how to switch: the mode line names the way back in both directions,
and touch panes offer tap-to-type. Normal text entry never requires
shortcut workarounds, and powerful keyboard workflows keep discoverable
alternatives for people who do not know Vim. Mobile essentials never
require obscure modifier chords or a hardware keyboard.

## Discoverability

Displayed hints match actual behavior in the current context — the frozen
desktop footer set is a contract, not a suggestion. Common actions are
reachable from where the user already is, and every overlay, picker, and
modal offers a clear way back to the previous context.

## Responsive layout

Desktop (120x32, 160x48), compact desktop (80x24), phone portrait
(40x24, 50x30), reduced height (40x12), and boundary stress (30x10) are
the fixtures, exercised live and across transitions. Narrow tmux panes
take the mobile path; at extreme sizes the interface offers a usable
compact state or a clear size message with a recovery path. It never
panics, loses a draft, hides an approval, or traps the user. Cells are
measured as cells: ANSI styling, borders, padding, and wide glyphs all
count before clipping.

## Color and theme

Twenty truecolor themes over one thirteen-token palette: switching themes
must be visibly distinct on any interactive terminal, and restrained
everywhere else (pipes, dumb terminals, and standard opt-outs stay
colorless). Meaning never rides on color alone — selection, focus,
status, and errors remain understandable as text and shape. Focus and
selection are always visible in every theme, including while streaming.

## Status, errors, and recovery

Progress reflects real state: slow streams, tool bursts, large output,
empty output, errors, and cancellation all read truthfully, and the final
rendered output agrees with the exported session. Cancellation returns
control promptly and says what was stopped. Drafts, user intent, and
execution state survive restarts, interrupts, and resume; a render
failure never silently becomes a blank screen.

## Verification

Each fix is demonstrated on the desktop and phone walkthroughs
(multiline draft, tool approval, result inspection; read-back during a
stream, cancel, reopen). Color, contrast, and motion claims need terminal
captures or recordings — plain text cannot establish them. Native Termux
and Windows validation is recorded per fix as done or pending; simulation
is labeled as simulation.
