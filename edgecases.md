# Edge Cases, Quirks & Fixes

> Derived from analysis of Claude Code v2.1.88 patterns. These are non-obvious behaviors that must be explicitly designed for.

---

## 1. Tool Execution Edge Cases

### Bash Sibling Abort
When a Bash tool errors, all concurrently running Bash subprocesses must be immediately killed. The turn itself does NOT end - other non-Bash tools may continue.

**Fix:** Implement a batch-scoped `context.Context` that is cancelled on Bash error. The query-level context must remain alive.

### Simulated `sed -i`
BashTool detects `sed -i` in validation and simulates the edit directly rather than executing it. This guarantees the preview matches the actual transformation.

**Fix:** Add a Bash pre-processor that intercepts `sed -i` and routes to `pkg/fs` atomic edit operations.

### Background Task Auto-Promotion
Bash commands exceeding 15 seconds in assistant mode are automatically backgrounded. The tool result must explain this to the model.

**Fix:** Implement a runtime threshold in `tasks.LocalShell` that promotes long-running commands to background tasks mid-flight.

### UNC Path Security
File tools skip filesystem validation for UNC paths (`\\server` or `//server`) to prevent `fs.existsSync()` from triggering SMB authentication dialogs and leaking NTLM credentials.

**Fix:** Reject or defer UNC paths in file tool validation.

### Stale Write Protection
FileEditTool rejects edits if the file was modified since the last read (by user or linter). On Windows, fall back to content comparison because timestamps are unreliable.

**Fix:** Track `(path, mtime, hash)` in a read-file state cache. Validate before every edit.

---

## 2. Permission System Edge Cases

### Auto-Mode Classifier Race
When `mode === 'auto'`, a separate LLM call classifies whether an action is safe. This races against the user potentially sending new input.

**Fix:** Run the classifier in a goroutine. If the user interrupts, cancel the classifier and fall back to `ask` behavior.

### Denial Tracking Threshold
Consecutive denials in auto mode trigger fallback to normal prompting. For async/subagents, this state lives in `ToolUseContext.localDenialTracking` rather than the global store.

**Fix:** Pass a mutable `DenialTrackingState` pointer through `ToolUseContext`.

### Permission Hooks Can Mutate Input
Pre-tool hooks can return `updatedInput`, which skips `backfillObservableInput` re-application. The hook owns the shape of the data from that point forward.

**Fix:** In the execution pipeline, branch after hooks: if `updatedInput` is present, skip backfill and use the hook's output directly.

### MCP Tool Name Collisions
Built-in tools take precedence over MCP tools with the same name. MCP tools use a prefixed name (`mcp__server__tool`) for rule matching but may have unprefixed display names.

**Fix:** Deduplicate after concatenating built-ins + MCP tools, with built-ins winning. Normalize names for permission checks.

---

## 3. Context / Message Edge Cases

### Empty `tool_result` Breaks Capybara v8
Empty tool results cause zero output on Capybara v8. Always return at least a whitespace or explanation string.

**Fix:** Sanitize tool results in the result mapper: if empty, replace with `" "` (single space) or `"(no output)"`.

### Thinking Block Rules
1. Any message with thinking/redacted_thinking must be in a query with `max_thinking_length > 0`
2. A thinking block cannot be the last content block
3. Thinking blocks must be preserved across the entire assistant trajectory (turn + tool_use + tool_result + following assistant message)

**Fix:** Validate message sequences before sending to API. Strip or relocate orphaned thinking blocks.

### Streaming Fallback Tombstones
When streaming falls back to non-streaming, partial assistant messages with thinking blocks have invalid signatures. They must be tombstoned (removed from UI and transcript) before the retry.

**Fix:** Emit `TombstoneMessage` events for all in-flight assistant messages on streaming fallback.

### Prompt Cache Stability
The server places a global cache breakpoint after the last prefix-matched built-in tool. Interleaving MCP tools between built-ins invalidates downstream cache keys.

**Fix:** Sort and concatenate in two partitions: `[sorted built-ins] + [sorted MCP tools]`.

---

## 4. File Reading Edge Cases

### macOS Screenshot Thin Space (U+202F)
macOS screenshot filenames use either a regular space or a thin space before AM/PM. If the exact path does not exist, try the alternate space character.

**Fix:** In `FileReadTool`, if `ENOENT` and filename matches screenshot pattern, attempt path resolution with both space variants.

### Blocked Device Paths
Reading `/dev/zero`, `/dev/random`, `/dev/urandom`, `/dev/full`, `/dev/stdin`, `/dev/tty`, `/dev/console`, `/dev/stdout`, `/dev/stderr`, or `/proc/*/fd/0-2` will hang or produce nonsense.

**Fix:** Maintain a blocklist of device paths and reject reads before I/O.

### File Read Dedup Cache
If the exact same `(path, offset, limit)` is read again and the mtime is unchanged, return a `file_unchanged` stub instead of content. This preserves prompt cache creation tokens.

**Fix:** Implement an LRU cache keyed by `(path, offset, limit, mtime)` in `FileReadTool`.

---

## 5. API / Streaming Edge Cases

### Max Output Tokens Recovery
When the model hits `max_output_tokens`, the loop can retry with a higher limit (up to 3 times). The error must be WITHHELD from SDK consumers until recovery is attempted, because some consumers terminate on any `error` field.

**Fix:** Buffer `max_output_tokens` errors internally. Only yield them if recovery fails after 3 attempts.

### Prompt Too Long Withholding
Similarly, `prompt_too_long` errors are withheld if reactive compact or context collapse might recover. Do not yield the error until recovery paths are exhausted.

**Fix:** Gate error yielding behind recovery attempt status.

### Image Size Errors
Images exceeding token limits throw `ImageSizeError` or `ImageResizeError`. These must be caught and converted to user-facing tool errors, not thrown out of the loop.

**Fix:** Wrap image processing in a typed error boundary in `FileReadTool`.

---

## 6. State / Storage Edge Cases

### Fire-and-Forget Transcript Writes
User messages are written blocking (crash recovery). Assistant messages are fire-and-forget (order-preserving queue). Progress is inline with dedup.

**Fix:** Use a priority/durability queue for transcript persistence: `critical` (blocking), `standard` (queued), `ephemeral` (inline, overwrite previous).

### Content Replacement Budget
Aggregate tool result sizes are bounded per message. Tools with `maxResultSizeChars: Infinity` (like FileRead) are exempt to avoid circular read-persist loops.

**Fix:** Implement a per-turn content replacement tracker. Skip tools explicitly marked as infinite.

### Task Output File Rotation
Tasks write output to disk files. The `outputOffset` field tracks how much has already been consumed by the model, enabling resume.

**Fix:** Every task gets an append-only output file. Track offset in task state.

---

## 7. UI / Interaction Edge Cases

### Interrupt Behavior Variability
Tools declare `interruptBehavior`: `'cancel'` (stop and discard) or `'block'` (keep running, queue new user message). Defaults to `'block'`.

**Fix:** Check `interruptBehavior()` before cancelling a tool on user input.

### Transparent Wrapper Tools
REPLTool is a "transparent wrapper" - it delegates all rendering to inner tool calls. The wrapper itself shows nothing.

**Fix:** Support a `IsTransparentWrapper` flag on tools that suppresses their own UI rendering.

### Tool Progress Deduplication
Progress messages for the same tool use ID must be deduplicated in the transcript. Only the latest progress state is meaningful.

**Fix:** In transcript serialization, collapse consecutive progress messages for the same `toolUseID` into the last one.

---

## 8. Build / Environment Quirks

### Feature Flag Dead Code Elimination
Claude Code uses `feature('FLAG')` as a Bun compile-time intrinsic. In external builds, this always returns `false`, causing 108 modules to be eliminated.

**Fix:** Use Go build tags (`//go:build kairos`) for equivalent compile-time gating.

### `MACRO.VERSION` Injection
Version is injected at bundle time via `MACRO.VERSION`.

**Fix:** Use `go build -ldflags "-X main.Version=X.Y.Z"` for compile-time version injection.

### Lazy Schema Evaluation
Zod schemas are wrapped in `lazySchema(() => ...)` to defer module-load overhead.

**Fix:** Use `sync.Once` or factory functions for schema initialization in Go.
