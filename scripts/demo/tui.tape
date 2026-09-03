# agent-harness live demo — the real deal.
#
# A real conversation against a hosted GLM model (fireworks provider):
# a fast first response, then a short tool call reading AGENTS.md to
# say what the repo is about. Closes with the tab tour — Sessions,
# Settings, Home — showing the conversation landed in the session
# store. The composer shows the agent mode chip, set to auto via
# /mode — tool calls approve without asking.
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
# 2. Chat tab: the welcome message.

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

# 6. The tab tour: leave the composer (Esc → normal mode — digit
#    hotkeys switch views there), then walk Sessions, Logs, Settings,
#    Home.
Escape
Sleep 500ms
Type@100ms "3"
Sleep 3s
Type@100ms "4"
Sleep 3s
Type@100ms "5"
Sleep 3s
Type@100ms "1"
Sleep 3s
# 7. Home tab — the tour ends where it started. The tape ends on the
#    TUI: no exit, no shell.
