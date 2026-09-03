# Release 0.3.8 TUI verification: tabs, settings editing, palette.
# Boots the real binary against the mock LLM (scripts/demo/mock-llm.py
# must be running). Records durable evidence to evidence/.
Output evidence/release-tui-verify.gif
Set Shell bash
Set FontSize 14
Set Width 1200
Set Height 720
Set TypingSpeed 30ms

Require ./scripts/demo/demo-boot.sh
Type@500ms "./scripts/demo/demo-boot.sh"
Enter
Sleep 2s

# Tab walk: Home -> Chat -> Sessions -> Settings
Tab
Sleep 600ms
Tab
Sleep 600ms
Tab
Sleep 800ms

# Settings: cycle a choice with arrows (consistent fallback)
Down
Sleep 300ms
Right
Sleep 300ms
Left
Sleep 500ms

# Settings: temperature editor rejects garbage, accepts a number
Down
Down
Down
Down
Sleep 200ms
Enter
Sleep 300ms
Type "abc"
Sleep 300ms
Enter
Sleep 600ms
Escape
Sleep 300ms

# Command palette with honest hint
Ctrl+P
Sleep 600ms
Type "eff"
Sleep 600ms
Escape
Sleep 400ms

# Chat: /cost responds
Type "2"
Sleep 300ms
Type "i"
Sleep 200ms
Type "/cost"
Sleep 300ms
Enter
Sleep 900ms

# Quit
Ctrl+C
Sleep 500ms
