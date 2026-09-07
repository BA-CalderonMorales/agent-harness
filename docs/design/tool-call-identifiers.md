# Design: Stable Short Identifiers for Tool Calls (Task 3c)

**Status:** design (goal 0.3.29 Task 3c) — Slice 1 landed.
**Origin:** 0.3.28/0.3.29 dogfooding: finding the full untruncated
command for a tool call means either expanding the record in chat
(losing scroll position) or reading wrapped text in the transcript.

## Problem

A marathon turn fires dozens of shell calls. When one of them matters
later ("which command removed that file?"), the user has no fast path
from the chat row to the full record:

- The chat row truncates the command to its width budget (Task 3b).
- Expanding in chat works but moves the transcript under you.
- The Logs tab carries the full picture (audit entries, permission
  decisions) but is a different stream with no join key — you cannot
  ask "show me the log rows for THAT shell call".

## Design

A stable, short, human-communicable identifier rendered on every tool
row in chat, and filterable in Logs.

### Identifier derivation

`#a1b2` — the first four hex characters of the FNV-1a 64 hash of the
tool-use ID (the same ID the agent loop already assigns and the audit
log already records as `ToolCallID`). Properties:

- Stable across session reloads (the ID persists in the session file).
- Collision odds: 4 hex chars = 65,536 values; a single turn rarely
  exceeds ~50 calls (birthday collision ≈ 2% at 50, felt as "two rows
  share a tag" once every ~50 turns — acceptable; see collisions).
- Short enough to type into a filter box on mobile.

```go
// chat.go (new helper, ~6 lines)
func shortToolTag(toolID string) string {
    if toolID == "" {
        return ""
    }
    h := fnv.New64a()
    h.Write([]byte(toolID))
    return fmt.Sprintf("#%04x", h.Sum64()&0xffff)
}
```

### Rendering (chat) — Slice 1, landed

The tool summary row carries the tag right-aligned next to the
duration: `▸ 04:15:23 ✓ bash  $ git status … 0.6s  #a1b2`. Rules:

- Truncated rows (Task 3b) always show the tag — it is the pointer to
  the full text, so it must survive truncation.
- Group headers carry the tag of their FIRST member; sub-rows do not
  each carry tags (row noise). Expanding a member reveals its own tag.
- Click mapping unchanged: the tag renders inside the existing row.

### Rendering (Logs) — Slice 2, pending

- Tool-related diag entries gain the tag in their table row (derived,
  not stored — the ID is already recorded).
- New filter state on LogsModel: free-text substring match over the
  rendered row, toggled by `/`. Typing `#a1b2` narrows to that call's
  rows; Esc clears.

### The fast path

1. See `#a1b2` on the chat row.
2. Open Logs, type `#a1b2`.
3. Read the full command, permission decision, audit trail.

No expansion needed in chat; the transcript never moves.

## Slices

1. **Slice 1 (landed):** `shortToolTag` + chat-row rendering
   (summary rows + group headers) + tests.
2. **Slice 2:** Logs filter + tag column on tool-related entries.
3. **Slice 3:** collision handling — if two live groups share a tag,
   extend the later one to five chars (render-time, deterministic).

## Test plan

- Tag derivation: deterministic, 4 hex chars, stable across calls,
  empty-ID returns empty.
- Chat row: tag present on truncated single rows, on group headers,
  absent on group sub-rows; tag is the row's last visible token.
- Width: rows with tags fit their render budget (Task 3b contract).
- Logs filter (slice 2): substring match narrows; Esc clears; day and
  level filters compose.

## Non-goals

- Not a link/click-through (Logs is a separate tab; the tag is a
  copyable filter key, not a hyperref).
- Not persisted storage — always derived from ToolID, so a tag scheme
  change never migrates data.
