---
name: debugging-patterns
description: Systematic debugging patterns for root cause analysis. Use when investigating bugs, unexpected behavior, or system failures. Apply to trace through code execution, identify missing connections, and verify fixes end-to-end. Critical for TUI, async, and event-driven systems where symptoms may be far from causes.
---

# Debugging Patterns

> **Purpose:** Move beyond symptom-fixing to true root cause analysis with confidence.

---

## Pattern 1: Chain-Reasoning (The "Skip Forward")

Use this when you find a potential issue but need to verify the fix will actually work end-to-end.

### When to Apply

- Bug has clear symptom but unclear cause
- Multiple components interact (TUI, async, events)
- You suspect a missing connection or broken wire

### Steps

**Step 1: Trace Backwards from Symptom**
```
Symptom: "Chat doesn't show LLM responses"
  ↓
Where should response originate? → Agent loop
  ↓  
What triggers the agent loop? → OnSubmit handler
  ↓
What calls OnSubmit? → Chat delegate
```

**Step 2: Identify the Gap**
```
ChatModel has delegate field ✓
ChatDelegate interface defined ✓
OnSubmit uses delegate ✓
delegate.SetDelegate() called? ✗ FOUND IT
```

**Step 3: Skip Forward - Verify the Chain**
Before fixing, trace what happens IF you fix it:
```
Fix: Add SetChatDelegate()
  ↓
OnSubmit fires → handleUserSubmit()
  ↓
Agent loop starts → Query()
  ↓
Stream events flow → AgentChunkMsg
  ↓
Chat updates → updateOrCreateStreamingMessage()
  ↓
Viewport refreshes → User sees response ✓
```

**Step 4: Confirm No Other Breaks**
Check for other places the chain could fail:
- Is the message channel buffered? (Yes, 64)
- Does chatModel.Update() handle AgentChunkMsg? (Yes, lines 225-232)
- Does refreshViewport() render correctly? (Yes, uses messages slice)

### Example Application

```
Bug: TUI chat accepts input but never shows responses

1. Backward trace:
   - User types, presses Enter
   - chat.go line 176-185 handles KeyEnter
   - Checks m.delegate != nil before calling OnSubmit
   - delegate is nil → returns early

2. Compare with working parts:
   - Sessions view works → has delegate set
   - Settings view works → has delegate set
   - Chat view broken → no delegate set

3. Skip forward verification:
   - If delegate is set, OnSubmit calls handleUserSubmit
   - handleUserSubmit calls handleAgentLoopAsync
   - Agent loop sends AgentChunkMsg via Send()
   - App.Update forwards to chatModel.Update
   - chatModel renders chunks in real-time
   - All pieces exist and connect properly

4. Fix confidence: HIGH
   - Only missing piece is the delegate connection
   - All downstream code verified working
```

---

## Pattern 2: Working vs Broken Comparison

Use this when one part of the system works and another doesn't.

### Method

1. Identify a working equivalent (sessions view, settings view)
2. List the wiring/connections for the working part
3. List the wiring/connections for the broken part
4. Compare to find missing pieces

### Example

```bash
# What was done in agent-harness chat fix
grep -n "SetDelegate" internal/tui/*.go

# Results:
# app.go:98  sessionsModel.SetDelegate(delegate) ✓
# app.go:103 settingsModel.SetDelegate(delegate) ✓
# NO chatModel.SetDelegate() call ✗
```

---

## Pattern 3: Event Flow Tracing

For async/event-driven systems, trace the complete message flow.

### Template

```
[Event Origin] → [Channel/Queue] → [Handler] → [Side Effects]
                    ↓
            [Any filters/transforms?]
                    ↓
            [Any early returns?]
```

### TUI-Specific Checklist

- [ ] Does the sub-model receive the message?
- [ ] Does Update() handle this message type?
- [ ] Are there early returns that skip processing?
- [ ] Does the message modify state?
- [ ] Does the view re-render after state change?
- [ ] Are dimensions/content properly set for viewport?

---

## Pattern 4: Nil Check Audit

Common in Go: interfaces and pointers that are never initialized.

### Quick Audit

```bash
# Find all delegate/interface fields
grep -rn "delegate\|handler\|callback" --include="*.go" | grep -v "_test.go"

# Find where they're set
grep -rn "SetDelegate\|SetHandler" --include="*.go"

# Compare lists - any set without corresponding field?
```

---

## Pattern 5: The Confidence Checklist

Before declaring a fix complete, verify:

- [ ] **Isolated the cause:** Can explain exactly why the bug occurs
- [ ] **Verified the chain:** Traced from fix to expected outcome
- [ ] **No other breaks:** Checked that fix doesn't break other paths
- [ ] **Minimal change:** Fix touches only what's necessary
- [ ] **Tested:** Actually ran the code and verified behavior

---

## Pattern 6: PATH Shadowing Detection

Use when updates appear to succeed but old version persists.

### Symptom
- Update reports success but `version` shows old number
- Which binary varies depending on how you call it
- Different paths return different versions

### Detection
```bash
# Find all binaries in PATH
which -a agent-harness

# Check each version
for bin in $(which -a agent-harness 2>/dev/null); do
    echo "=== $bin ==="
    "$bin" --version 2>/dev/null | head -1
done

# Check PATH precedence
echo "$PATH" | tr ':' '\n' | nl
```

### Common Shadow Locations (Termux)
- `$HOME/buckets/usr/bin/` - Often added to PATH before `$PREFIX/bin`
- `$HOME/.local/bin/` - User-local installs
- `$HOME/` - Direct home directory copies

### Fix
Remove shadowing binaries, keep only canonical location:
```bash
# Canonical Termux location
$PREFIX/bin/agent-harness
```

---

## Extension Points

Add new patterns here as we discover them:

- [ ] **Pattern 7:** State machine validation
- [ ] **Pattern 8:** Resource leak tracking
- [ ] **Pattern 9:** Configuration drift detection

---

## Anti-Patterns to Avoid

### ❌ Symptom Patching

```go
// Don't do this:
if response == "" {
    response = "No response"  // Hides the real bug
}
```

### ❌ Blind Faith

Fixing the first thing you find without verifying the full chain.

### ❌ Over-Logging

Adding print statements everywhere instead of systematic tracing.

---

## Remember

> **The goal is understanding, not just fixing.**
> 
> When you truly understand why a bug exists, the fix is obvious and the confidence is high.

---

## Usage Notes

This skill is intentionally concise. Each pattern is designed to be applied quickly without excessive ceremony. The examples are real patterns used in this codebase - refer to them when similar issues arise.
