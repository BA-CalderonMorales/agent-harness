# Termux Edge Cases for agent-harness

> Documenting Android/Termux-specific behaviors discovered during portability testing.

---

## Summary

The agent-harness project **builds and runs successfully** on Termux with minimal adaptations. Most code is portable Go that handles the environment correctly.

---

## Edge Cases Identified

### 1. /tmp Directory Restricted

**Issue**: `/tmp` is not writable in Termux. Using it for build output or temporary files fails.

**Evidence**:
```
open /tmp/agent-harness-test: permission denied
```

**Fix**: Build to project-local directories:
```bash
go build -o ./build/agent-harness ./cmd/agent-harness
```

**Status**: Code already uses proper temp handling; build scripts need adjustment.

---

### 2. Shell Path Resolution

**Issue**: `/bin/sh` exists but is Android's system shell (limited). Termux's `sh` is at `$PREFIX/bin/sh`.

**Evidence**:
```bash
$ ls -la /bin/sh
-rwxr-xr-x 1 root shell 318608 Dec 31  2021 /bin/sh

$ which sh
/data/data/com.termux/files/usr/bin/sh
```

**Fix**: Code correctly uses `exec.LookPath("sh")` which resolves to the proper shell.

**Status**: No changes needed.

---

### 3. TUI Mode Uncertainty

**Issue**: BubbleTea TUI may have issues with mobile keyboards and touch events.

**Risk Level**: Medium

**Mitigation**: 
- Default to CLI mode in Termux
- Test TUI separately with `--tui` flag
- Document keyboard limitations

**Status**: Not tested yet; flagged for investigation.

---

### 4. Background Process Management

**Issue**: Android's OOM killer may terminate background processes aggressively.

**Risk**: Long-running bash commands (>15s) may be killed.

**Current Code**: Has 60s default timeout; may need mobile-specific tuning.

**Mitigation**: 
- Keep operations shorter
- Implement retry logic
- Save state for resumable operations

---

### 5. Network Reliability

**Issue**: Mobile networks have variable latency and may drop.

**Current Code**: 120s HTTP timeout; streaming SSE.

**Risk**: High latency on poor connections; timeouts on large model responses.

**Mitigation**:
- Consider increasing timeout for mobile
- Implement request retry with backoff
- Cache responses where possible

---

### 6. Battery/Doze Mode

**Issue**: Android Doze mode throttles background network and CPU when screen is off.

**Impact**: Long agent sessions may stall when device sleeps.

**Mitigation**:
- Keep screen on during sessions (acquire wakelock)
- Break work into smaller chunks
- Expect interruptions; design for resumability

---

### 7. Storage Access

**Issue**: Termux has isolated storage by default.

**Current Behavior**: Works within `~/projects/` directory.

**Limitation**: Cannot access `/sdcard` without `termux-setup-storage`.

**Status**: Acceptable for development workflow.

---

## Verified Working

| Feature | Status | Notes |
|---------|--------|-------|
| Go build | ✅ | Native android/arm64 compilation |
| Shell exec | ✅ | Uses correct Termux shell |
| File I/O | ✅ | Standard Go file operations work |
| HTTP/HTTPS | ✅ | Uses Android network stack |
| Config loading | ✅ | `$HOME/.config/` accessible |
| Skills loading | ✅ | Local `.agent-harness/skills/` works |
| Memory files | ✅ | AGENTS.md discovery functional |

---

## Recommended Runtime Command

```bash
cd ~/projects/agent-harness && ./scripts/run-termux.sh
```

Or for manual control:
```bash
cd ~/projects/agent-harness
go build -o ./build/agent-harness ./cmd/agent-harness
export OPENROUTER_API_KEY="sk-or-v1-..."
./build/agent-harness
```

---

## Mobile UX Recommendations

1. **Prefer CLI over TUI** on mobile keyboards
2. **Use slash commands** (`/help`, `/compact`, `/model`) for quick actions
3. **Keep responses concise** - mobile screens are small
4. **Save work frequently** - interruptions are common

---

## Skill Created

Created: `.agent-harness/skills/termux-mobile-dev/SKILL.md`

This skill is automatically loaded by agent-harness when running from the project directory, providing Termux-specific context to the LLM.
