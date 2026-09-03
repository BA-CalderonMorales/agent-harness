# agent-harness live demo — the real deal.
#
# A real turn against a hosted GLM model (fireworks provider): the
# agent reads the repository with a tool call and answers with a
# markdown table. The composer shows the agent mode chip, cycled to
# auto via /mode — tool calls approve without asking.
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

Require ./scripts/demo/demo-boot-live.sh
Type@500ms "./scripts/demo/demo-boot-live.sh"
Enter
Sleep 4s
# 1. Boot: the Home dashboard with git context and the storage
#    footprint line.

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

Type@20ms "Run ls in this repo, then list the top-level directories in a markdown table with one-line descriptions."
Enter
Sleep 45s
# 4. The turn: a real ls tool call, then the markdown table rendered
#    in the transcript.

Sleep 2s
Ctrl+C
Sleep 1s
# 5. Done.
