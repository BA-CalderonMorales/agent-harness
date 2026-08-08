# agent-harness interactive demo -- a tour of every tab.
# Boot the TUI (bypassing the setup wizard) and walk Home > Chat >
# Sessions > Settings, then leave. No agent turn: the tab tour is the
# showcase, and the mock LLM still powers the green readiness status.
#
# Record from the repository root with the mock server running and an
# empty session store:
#   rm -rf ~/.agent-harness/sessions  (terminal 1, before recording)
#   python3 scripts/demo/mock-llm.py  (terminal 1)
#   vhs scripts/demo/tui.tape         (terminal 2)
#
# Note: views switch via the digit hotkeys (2=Chat, 3=Sessions,
# 4=Settings, 1=Home) - they arrive as clean single keypresses under
# the recorder; send Tab at your own risk, it mangles typed input.

Output docs/demo-tui.gif
Set FontSize 18
Set FontFamily "DejaVu Sans Mono"
Set Width 1200
Set Height 760
Set Padding 28
Set Margin 0
Set Shell bash

Type "./scripts/demo/demo-boot.sh"
Enter
Sleep 7s
# 1. boot via the demo wrapper: env pins provider=local, so the setup
#    wizard is skipped and the TUI lands straight on the Home tab
#    with the mock LLM ready

Type "2"
Sleep 4s
# 2. Chat tab: welcome message (Agent Harness, mode, git)
#    under the Provider ready status

Type "3"
Sleep 4s
# 3. Sessions tab: session store and resume list

Type "4"
Sleep 3s
# 4. Settings tab: provider, model, mode, effort

Type "j"
Sleep 2s
Type "1"
Sleep 4s
# 5. Home tab: project dashboard and quick actions

Type "2"
Sleep 3s
Ctrl+C
Sleep 2s
# 6. done
