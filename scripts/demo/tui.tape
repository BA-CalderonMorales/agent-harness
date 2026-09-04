# agent-harness live demo — the real deal.
#
# A real conversation against a hosted GLM model (fireworks provider):
# a fast first response, then a short tool call reading AGENTS.md to
# say what the repo is about. The tour then shows the Logs tab with the
# live diagnostics stream, the selection marker riding the cursor, the
# detail modal with its exit hint, the level filter, two themes so the
# palette switch is visible, the '?' help overlay, and closes on Home.
#
# Record from the repository root:
#   chmod +x scripts/demo/demo-boot-live.sh
#   vhs scripts/demo/tui.tape
#
# The fireworks API key must be configured once via /login (stored in
# the encrypted credential store); the boot wrapper isolates session
# data so every recording starts fresh.

Output docs/demo-tui.gif
Set FontSize 18
Set FontFamily "DejaVu Sans Mono"
Set Width 1200
Set Height 760
Set Padding 28
Set Margin 0
Set Shell bash

# Boot off the record: the GIF opens on the finished Home dashboard.
Hide
Require ./scripts/demo/demo-boot-live.sh
Type@500ms "./scripts/demo/demo-boot-live.sh"
Enter
Sleep 4s
Show

# 1. The Home dashboard with git context and the storage footprint.

Type@100ms "2"
Sleep 2s
# 2. Chat tab: the two-line welcome and the bullet guidance.

Type@100ms "i"
Sleep 500ms
Type "/mode auto"
Enter
Sleep 2s
# 3. The agent mode chip flips to auto with a status flash: tool
#    calls approve without asking.

Type@20ms "Write a two-sentence haiku about shipping code on a Friday, with a bold title."
Enter
Sleep 22s
# 4. First turn: a fast response with styled markdown — bold, italics.

Type "Read AGENTS.md and tell me in two sentences what this repo is about."
Enter
Sleep 25s
# 5. Second turn: a short tool call, then the summary.

Escape
Sleep 400ms
Type "t"
Sleep 1s
Type "t"
Sleep 1s
# 6. Tool-run collapse on, then off again — the count line and the
#    carets doing their job.

Type "y"
Sleep 1.5s
# 7. 'y' copies the expanded record to the clipboard; the status flash
#    names what went where.

Type@100ms "4"
Sleep 3s
# 8. The Logs tab: the live diagnostics stream — levels, timestamps,
#    source sites, exact file:line callers, and the selection marker
#    on the row the cursor owns.

Type "j"
Sleep 900ms
Type "k"
Sleep 900ms
Type "j"
Sleep 900ms
# 9. The selection moves: the marker follows the cursor and the
#    n/m counter tracks it — in a quiet stream as much as a live one.

Enter
Sleep 2.5s
# 10. The detail modal: the full entry with the exit hint pinned
#     bottom-right.

Type "j"
Sleep 700ms
Escape
Sleep 900ms
# 11. j/k scroll the modal's content; Esc folds it.

Type "f"
Sleep 1.5s
Type "f"
Sleep 1.5s
# 12. The level filter cycles: all → warning+ → error+.

Type@100ms "2"
Sleep 2s
Type@100ms "i"
Sleep 400ms
Type "/theme midnight"
Enter
Sleep 2.5s
Type "/theme paper"
Enter
Sleep 2.5s
Type "/theme default"
Enter
Sleep 2s
# 13. Themes: the palette switches live — midnight, paper, back to the
#     default.

Escape
Sleep 400ms
Type "?"
Sleep 3s
# 14. The help overlay: the scrollable key map (j/k to scroll).

Escape
Sleep 500ms
Type@100ms "1"
Sleep 3s
# 15. Home tab — the tour ends where it started. The tape ends on the
#     TUI: no exit, no shell.
