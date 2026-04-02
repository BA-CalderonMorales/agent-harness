# Termux Mobile Development Environment

> Domain-specific guidance for running agent-harness on Termux (Android/Linux sandbox).

---

## Environment Characteristics

- **Platform**: Android (Samsung S26 Ultra) via Termux
- **Go Version**: 1.26.1 (android/arm64)
- **Shell**: `/data/data/com.termux/files/usr/bin/bash`
- **Home**: `/data/data/com.termux/files/home`
- **Prefix**: `/data/data/com.termux/files/usr`
- **Temp**: Use `$PREFIX/tmp` instead of `/tmp`

---

## Edge Cases & Workarounds

### 1. Filesystem Paths

Termux uses non-standard paths. Do NOT hardcode:
- `/bin/sh` → Use `$PREFIX/bin/sh` or `exec.LookPath("sh")`
- `/tmp` → Use `$PREFIX/tmp` or `$TMPDIR`
- `/usr/bin/*` → Use `$PREFIX/bin/*`

The project correctly uses `exec.LookPath("sh")` which resolves properly.

### 2. Shell Execution

Android's process model differs from standard Linux:
- Background processes may be killed by Android OOM
- Signal handling (SIGTERM, SIGKILL) behaves differently
- Process groups may not work as expected

**Mitigation**: Keep commands foreground when possible; use shorter timeouts.

### 3. Network Stack

- Certificate bundle at `$PREFIX/etc/tls/cert.pem`
- DNS resolution uses Android's resolver
- IPv6 may be preferred over IPv4 on some carriers

**Mitigation**: Go's `net/http` handles this correctly; no changes needed.

### 4. TUI Rendering (BubbleTea)

Termux terminal capabilities:
- Supports 256 colors and truecolor
- Mouse events: Supported but may conflict with touch gestures
- Alternate screen buffer: Works correctly

**Potential Issues**:
- Touch scrolling may interfere with TUI mouse events
- Keyboard input: Gboard/Samsung Keyboard may not send all escape sequences
- Use `--tui` flag with caution; CLI mode is more reliable on mobile

### 5. Battery & Resource Constraints

Android power management:
- Background processes throttled when screen off
- Doze mode may interrupt long-running operations
- Network requests may be delayed when device is idle

**Mitigation**: Keep operations short; expect interruptions.

### 6. File System Permissions

- Termux has isolated storage by design
- Access to `/sdcard` requires `termux-setup-storage`
- Cannot access other apps' data directories
- Cannot bind to ports < 1024 (no root)

### 7. Build Environment

- Native compilation works (Go 1.26.1 available)
- CGO enabled by default for Android
- Cross-compilation FROM Termux TO desktop may be limited

---

## Testing Checklist

Before declaring Termux support "production ready":

- [ ] Basic query with OpenRouter API succeeds
- [ ] Bash tool executes simple commands
- [ ] File read/write in project directory works
- [ ] Glob and grep tools function
- [ ] Long-running commands timeout properly
- [ ] TUI mode renders (if supported)
- [ ] Memory files (AGENTS.md) load correctly
- [ ] Config file saves to `$HOME/.config/agent-harness/`

---

## Mobile UX Considerations

1. **Input**: Typing on mobile keyboards is slow. Consider:
   - Slash commands for common operations
   - History recall (↑ key or `/history`)
   - Short aliases

2. **Output**: Mobile screens are narrow:
   - Keep output concise
   - Avoid wide tables
   - Use progress indicators for long operations

3. **Connectivity**: Mobile networks are unreliable:
   - Implement retry logic for API calls
   - Cache responses when possible
   - Handle timeouts gracefully

---

## Known Working Configuration

```bash
# Environment setup
export OPENROUTER_API_KEY="sk-or-v1-..."
export AGENT_HARNESS_MODEL="anthropic/claude-3.5-sonnet"

# Build and run
cd ~/projects/agent-harness
go build -o ./build/agent-harness ./cmd/agent-harness
./build/agent-harness
```
