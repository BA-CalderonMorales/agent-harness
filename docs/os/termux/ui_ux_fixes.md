# Termux UI/UX Fixes

> Documentation for Android/Termux-specific UI and input handling patches.

## Summary

The agent-harness includes specialized handling for Termux environments to ensure smooth input/output on mobile devices. These patches address terminal detection, keyboard input, and rendering issues specific to Android terminals.

## Changes Made

### 1. Terminal Detection (internal/ui/input.go)

**Problem**: Original code used hardcoded `return true` for `isatty()` check.

**Solution**: Uses `golang.org/x/term` for proper TTY detection:

```go
func isatty(fd uintptr) bool {
    _, _, err := term.GetSize(int(fd))
    return err == nil
}
```

### 2. Termux Environment Detection (internal/ui/input.go, internal/ui/tui.go)

Detects Termux by checking environment:

```go
isTermux := os.Getenv("TERMUX_VERSION") != "" || 
    strings.Contains(os.Getenv("HOME"), "com.termux")
```

### 3. Raw Terminal Input Path (internal/ui/input.go)

**Problem**: BubbleTea's alt-screen TUI does not work well with mobile keyboards and touch events.

**Solution**: When Termux is detected, uses `readLineTermux()` which:

- Reads directly from stdin using `bufio.Reader.ReadRune()`
- Handles escape sequences for arrow keys manually
- Implements tab completion for slash commands
- Provides history navigation with up/down arrows
- Uses simple `\r\033[K` for line clearing

```go
func (le *LineEditor) readLineTermux() (*ReadOutcome, error) {
    fmt.Print(le.prompt)
    reader := bufio.NewReader(os.Stdin)
    // Handle each rune directly...
}
```

### 4. Clean Prompt (internal/ui/tui.go)

**Problem**: Special characters in prompts may render incorrectly on mobile terminals.

**Solution**: Uses simple `> ` prompt for Termux, avoiding special symbols that might display as boxes or question marks.

### 5. Key Input Handling (internal/ui/tui.go)

**Problem**: `msg.String()` returns key names like "enter" instead of actual characters.

**Solution**: Checks `msg.Type` and handles `tea.KeyRunes` separately:

```go
case tea.KeyRunes:
    m.input += string(msg.Runes)
case tea.KeySpace:
    m.input += " "
```

### 6. Interactive Setup (cmd/agent-harness/main.go)

**Problem**: `fmt.Scanln` may have issues with mobile terminal input.

**Solution**: Uses `bufio.Reader.ReadString('\n')` for all interactive prompts during setup.

## Input Modes

| Mode | Detection | Use Case |
|------|-----------|----------|
| Termux Raw | `TERMUX_VERSION` env or `HOME` contains `com.termux` | Mobile terminals with limited keyboard |
| BubbleTea TUI | TTY detected, not Termux | Desktop terminals with full keyboard |
| Simple | No TTY detected | Pipes, scripts, automation |

## Escape Sequence Handling

The Termux raw input mode handles these sequences:

| Sequence | Meaning | Action |
|----------|---------|--------|
| `\x7f` or `\b` | Backspace | Delete last character |
| `\x09` | Tab | Complete slash command |
| `\x1b[A` | Up arrow | History previous |
| `\x1b[B` | Down arrow | History next |
| `\x03` | Ctrl+C | Cancel/exit |
| `\x04` | Ctrl+D | Exit on empty input |

## Testing in Termux

```bash
# Build for Termux
cd ~/projects/agent-harness
go build -o ./build/agent-harness ./cmd/agent-harness

# Run with verbose output
./build/agent-harness

# Verify Termux detection
TERMUX_VERSION=0.118 ./build/agent-harness
```

## Related Files

- `internal/ui/input.go` - LineEditor with Termux support
- `internal/ui/tui.go` - TUI model with Termux detection
- `cmd/agent-harness/main.go` - Main entry point with Termux-aware setup
- `docs/termux_edge_cases.md` - General Termux portability notes

## Future Improvements

1. **Soft Keyboard Support**: Consider integrating with Termux's soft keyboard API
2. **Gesture Handling**: Swipe gestures for history navigation
3. **Voice Input**: Integration with Android speech-to-text
4. **Haptic Feedback**: Vibration on command completion/errors
