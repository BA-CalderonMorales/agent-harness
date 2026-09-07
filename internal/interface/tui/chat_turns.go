package tui

import (
	"strconv"
	"strings"
)

// Turn grouping: the agent's tool calls belong to its response, not to
// the transcript at large. A turn renders as one block — the Agent
// header, the turn's tool calls nested beneath it (runs merged,
// expansions opening in place), then the answer bubble. A tool run
// that trails the transcript (still streaming, no answer yet) renders
// standalone and is absorbed when the response lands.

// renderCollapsedMessage renders one message with run merging applied,
// pure: no writing, no separators. next is the caller's next index.
func (m ChatModel) renderCollapsedMessage(msgs []ChatMessage, i int, collapsed bool) (string, int) {
	return m.renderCollapsedMessageAt(msgs, i, collapsed, m.width)
}

// renderCollapsedMessageAt renders one message with run merging at a
// width budget — nested rows live inside the response bubble.
func (m ChatModel) renderCollapsedMessageAt(msgs []ChatMessage, i int, collapsed bool, width int) (string, int) {
	msg := msgs[i]

	if !collapsed || !toolRunIsCollapsible(msg) {
		return m.renderMessageAt(msg, width), i + 1
	}

	// Gather the contiguous run: same turn, same display class (not raw
	// tool name — bash/ls/ls_recursive all render as "Shell", Task 4.2),
	// all final. Same-turn system notes (loop-detected markers) skip
	// past: they render inline in the turn block, not as run breaks
	// (goal 0.3.29 Task 3a).
	j := i + 1
	for j < len(msgs) {
		n := msgs[j]
		if n.Role == "system" && n.Turn != 0 && n.Turn == msg.Turn {
			j++
			continue
		}
		if n.Role != "tool" ||
			n.Turn != msg.Turn ||
			getToolDisplayName(n.ToolName) != getToolDisplayName(msg.ToolName) ||
			!toolRunIsCollapsible(n) {
			break
		}
		j++
	}

	// Expanding any member of a run unfolds the run message-by-message
	// so the expanded record has a visible home.
	if j-i > 1 {
		for k := i; k < j; k++ {
			if m.expandedMessageID != "" && m.expandedMessageID == msgs[k].ID {
				return m.renderMessageAt(msg, width), i + 1
			}
		}
	}

	if j-i == 1 {
		return m.renderMessageAt(msg, width), i + 1
	}

	// The run may carry skipped same-turn system notes: they render as
	// dim rows after the group (block metadata, not call rows), and the
	// members passed to the group renderer are tools only.
	members := make([]ChatMessage, 0, j-i)
	var notes []ChatMessage
	for k := i; k < j; k++ {
		if msgs[k].Role == "system" {
			notes = append(notes, msgs[k])
			continue
		}
		members = append(members, msgs[k])
	}
	rendered := m.renderToolRunAt(members, width)
	for _, n := range notes {
		if text := strings.TrimSpace(n.Content); text != "" {
			rendered += "\n" + HelpDimStyle.Render(fitBlock(width, text))
		}
	}
	return rendered, j
}

// offsetClickRefs shifts block-relative click refs down by n rows —
// the header (or frame) rows rendered above the clickable block.
func offsetClickRefs(refs []clickRef, n int) []clickRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]clickRef, len(refs))
	for i, r := range refs {
		out[i] = clickRef{start: r.start + n, lines: r.lines, msgID: r.msgID}
	}
	return out
}

// indentBlock prefixes every line of a rendered block with a single
// space — the nesting step under the Agent header. Line counts never
// change, so click mapping stays honest.
func indentBlock(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		lines[i] = " " + line
	}
	return strings.Join(lines, "\n")
}

// clickRef is a clickable region inside a rendered block: the row
// offset relative to the block's first row, how many rows, and the
// message the rows resolve to.
type clickRef struct {
	start, lines int
	msgID        string
}

// groupExtent returns the end index (exclusive) of the render group
// starting at i — the same boundary appendTurnGroupTracked renders to:
// a run of tool messages plus the assistant response that follows them,
// or a single message. Shared with the group cache so the signature and
// the render always describe the same block.
func groupExtent(msgs []ChatMessage, i int) int {
	if i < len(msgs) && msgs[i].Role == "tool" {
		j := i
		for j < len(msgs) {
			// Same-turn system notes (loop-detected, notices) are part
			// of the block: skipping them keeps the burst contiguous.
			if msgs[j].Role == "system" && msgs[j].Turn != 0 && msgs[j].Turn == msgs[i].Turn {
				j++
				continue
			}
			if msgs[j].Role != "tool" {
				break
			}
			j++
		}
		if j < len(msgs) && msgs[j].Role == "assistant" {
			return j + 1
		}
		return j
	}
	return i + 1
}

// groupSignature builds the group cache key for msgs[i:j]: every field
// a render reads, plus the model state the live block depends on.
//
// Cost discipline: this runs once per group per frame, so it must be
// O(1) per message — never O(content). Message text enters the key
// through a memoized fingerprint carried on the message itself
// (sigFP/sigLen, sigRFP/sigRLen): recomputed only when the text length
// changed, which catches appends and replacements while making the
// steady-state frame pay a few integer writes per message.
func (m ChatModel) groupSignature(msgs []ChatMessage, i, j int) string {
	var b strings.Builder
	b.Grow(64 * (j - i))
	b.WriteByte('{')
	for k := i; k < j; k++ {
		msg := &msgs[k]
		b.WriteString(msg.ID)
		b.WriteByte('|')
		b.WriteString(msg.Role)
		b.WriteByte('|')
		b.WriteString(msg.ToolName)
		b.WriteByte('|')
		b.WriteString(msg.ToolDisplayName)
		b.WriteByte('|')
		b.WriteString(msg.ToolDetail)
		b.WriteByte('|')
		b.WriteString(string(msg.ToolStatus))
		b.WriteByte('|')
		b.WriteString(strconv.FormatUint(msg.contentFP(), 16))
		b.WriteByte('|')
		b.WriteString(strconv.FormatUint(msg.reasoningFP(), 16))
		b.WriteByte('|')
		b.WriteString(strconv.FormatInt(msg.Timestamp.UnixNano(), 10))
		b.WriteByte('|')
		b.WriteString(strconv.FormatInt(int64(msg.ToolElapsed), 10))
		b.WriteByte('|')
		b.WriteString(strconv.FormatInt(int64(msg.ResponseTime), 10))
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(msg.Turn))
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(msg.IsTool))
		b.WriteByte('|')
		// Parts drive the turn block's tool-row nesting; ToolInputJSON
		// the expanded record. A part change must re-render: IDs and
		// text lengths catch the structural moves.
		b.WriteString(strconv.Itoa(len(msg.Parts)))
		for _, p := range msg.Parts {
			b.WriteByte('~')
			b.WriteString(p.ToolID)
			b.WriteByte('~')
			b.WriteString(strconv.Itoa(len(p.Text)))
		}
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(len(msg.ToolInputJSON)))
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(m.expandedMessageID == msg.ID))
		b.WriteByte(0x1f)
	}
	b.WriteString("w")
	b.WriteString(strconv.Itoa(m.width))
	b.WriteString("c")
	b.WriteString(strconv.FormatBool(m.toolsCollapsed))
	// The live block animates: thinking badge, spinner, streaming text.
	if m.streaming || m.thinking || m.placeholderPending {
		for k := i; k < j; k++ {
			if msgs[k].ID == m.currentStreamingAssistantID {
				b.WriteString("live")
				b.WriteString(strconv.FormatInt(int64(m.elapsed.Seconds()), 10))
				b.WriteString(strconv.Itoa(len(m.streamBuffer)))
				b.WriteString(strconv.Itoa(len(m.thinkingText)))
				b.WriteString(strconv.FormatBool(m.thinkingIsStatus))
				b.WriteString(strconv.FormatBool(m.expandedMessageID == m.currentStreamingAssistantID))
				break
			}
		}
	}
	b.WriteByte('}')
	return b.String()
}

// appendTurnGroupTracked renders the next render group starting at i
// and reports the clickable line ranges within the block (row offsets
// relative to the block's first row). next is the loop's next index.
// The group render memoizes on the group signature: a transcript of n
// groups repaints only groups whose inputs changed since the last
// frame (append, finalize, expand, stream tail) instead of all n.
func (m ChatModel) appendTurnGroupTracked(msgs []ChatMessage, i int, collapsed bool) (string, int, []clickRef) {
	msg := msgs[i]

	// Tool messages buffer into the response that follows them.
	if msg.Role == "tool" {
		j := i
		for j < len(msgs) && msgs[j].Role == "tool" {
			j++
		}
		if j < len(msgs) && msgs[j].Role == "assistant" {
			return m.renderTurnBlock(msgs, i, j, collapsed)
		}
		// Trailing run: still streaming, no response yet — standalone,
		// absorbed when the answer lands.
		return m.renderSingleGroup(msgs, i, collapsed)
	}

	return m.renderSingleGroup(msgs, i, collapsed)
}

// appendTurnGroupCached renders the group at i through the group cache:
// a hit returns the memoized block, a miss renders and memoizes.
// Cache-invalidation correctness rests on groupSignature covering every
// field the render reads (verified by the signature-completeness test)
// and on always consulting the cache with the same extent.
func (m ChatModel) appendTurnGroupCached(msgs []ChatMessage, i int) (string, int, []clickRef) {
	j := groupExtent(msgs, i)
	sig := m.groupSignature(msgs, i, j)
	if cached, ok := groupCacheStore.get(sig); ok {
		return cached.block, j, cached.refs
	}
	rendered, next, refs := m.appendTurnGroupTracked(msgs, i, m.toolsCollapsed)
	if next != j {
		// Signature/extent disagreement: bail out of caching, render
		// wins. (Extent derivation is shared, so this is belt-and-
		// suspenders — the signature test pins it anyway.)
		return rendered, next, refs
	}
	groupCacheStore.put(sig, rendered, refs)
	return rendered, j, refs
}

// renderSingleGroup renders one message through the collapse machinery.
// Assistants render through the tracked path: their click refs (the
// reasoning frame, the preview line) are constructed while rendering —
// estimating rows from the raw text drifts the moment a segment wraps.
func (m ChatModel) renderSingleGroup(msgs []ChatMessage, i int, collapsed bool) (string, int, []clickRef) {
	msg := msgs[i]
	if msg.Role == "assistant" {
		rendered, refs := m.renderAssistantTracked(msg, m.width)
		return rendered, i + 1, refs
	}
	rendered, next := m.renderCollapsedMessage(msgs, i, collapsed)
	var clicks []clickRef
	if msgs[i].IsTool {
		// One clickable block for the whole run, mapped to the run
		// head: expanding any member unfolds the run message-by-
		// message (see renderCollapsedMessageAt), so block-level
		// granularity is the honest click map here.
		clicks = append(clicks, clickRef{start: 0, lines: strings.Count(rendered, "\n") + 1, msgID: msgs[i].ID})
	}
	return rendered, next, clicks
}

// renderTurnBlock renders tools + response as one block: the Agent
// header, then the bubble — prose and tool rows interleaved in the
// order they actually happened, inside the border.
func (m ChatModel) renderTurnBlock(msgs []ChatMessage, i, j int, collapsed bool) (string, int, []clickRef) {
	assistant := msgs[j]

	// Legacy data carries no segmentation: the run nests above the
	// whole answer — never dropped.
	parts := assistant.Parts
	if len(parts) == 0 {
		parts = make([]TurnPart, 0, j-i+1)
		for k := i; k < j; k++ {
			parts = append(parts, TurnPart{ToolID: msgs[k].ID})
		}
		if strings.TrimSpace(assistant.Content) != "" {
			parts = append(parts, TurnPart{Text: assistant.Content})
		}
	}

	// The bubble's inner width: pane minus bubble border, padding, and
	// the nesting step.
	innerWidth := m.width - 8
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Tool rows render in natural arrival order: each call appears at
	// its chronological position through the collapse machinery, so
	// back-to-back same-category calls merge into one group header
	// while interleaved narration splits them — the user watches calls
	// come in naturally, grouped only when a category repeats
	// consecutively (goal 0.3.29 live feedback on Task 3a's spanning).
	toolRow := func(id string) (string, bool) {
		for k := i; k < j; k++ {
			if msgs[k].ID != id {
				continue
			}
			row, _ := m.renderCollapsedMessageAt(msgs, k, collapsed, innerWidth)
			return indentBlock(row), true
		}
		return "", false
	}

	var clicks []clickRef
	b := strings.Builder{}
	b.WriteString(m.renderAssistantHeader(assistant))
	b.WriteString("\n")

	// Same-turn system notes (loop-detected markers, mid-turn notices)
	// render as dim inline rows under the header, before the bubble:
	// chronology-critical text lives in the bubble; the note is block
	// metadata and must not split the tool burst into separate headers
	// (goal 0.3.29 Task 3a, the [Tool loop detected: ...] dogfood case).
	for k := i; k < j; k++ {
		if msgs[k].Role == "system" && msgs[k].Turn == assistant.Turn {
			note := strings.TrimSpace(msgs[k].Content)
			if note != "" {
				b.WriteString(HelpDimStyle.Render(fitBlock(innerWidth, note)))
				b.WriteString("\n")
			}
		}
	}

	width := m.width - 4
	if width < 1 {
		width = 1
	}
	inner, refs := m.assistantInnerContent(assistant, parts, toolRow)
	bubbles := MessageBubbleAssistant.Width(width).Render(inner)
	b.WriteString(bubbles)

	// The inner content is pre-wrapped to the bubble's inner width and
	// the bubble is left-border only: inner rows are bubble rows, and
	// the block carries one header row above the bubble.
	clicks = offsetClickRefs(refs, 1)

	return b.String(), j + 1, clicks
}

// groupCacheStore memoizes rendered turn blocks between frames. A
// marathon transcript re-rendered every message on every agent event
// (and every AddMessage) because refreshViewport walks the whole
// transcript; with the group cache a frame's cost is O(changed groups)
// instead of O(transcript). Keys are the signature strings themselves:
// a map[string] over bounded entries is cheap, collisions impossible,
// and the LRU keeps memory flat (this harness has an OOM history).
const (
	// groupCacheCapacity must exceed the group count of a marathon
	// transcript: a 10k-event session produces 10,000 groups, and at
	// 8192 the LRU evicted entries the next frame needed — the same
	// thrash the markdown cache had (see chat_render_cache.go). 65536
	// covers 10k events 6x over; per-block output stays capped at
	// groupCacheMaxOutput so memory remains bounded.
	groupCacheCapacity  = 65536
	groupCacheMaxOutput = 1 << 20 // 1 MiB per block — beyond that, don't cache
)

type groupCache struct {
	ents map[string]cachedGroup
	lru  []string // signature strings, front = most recent
}

// cachedGroup is one memoized render: the styled block and the click
// refs that belong to it. They live together so eviction removes both.
type cachedGroup struct {
	block string
	refs  []clickRef
}

func newGroupCache() *groupCache {
	return &groupCache{ents: make(map[string]cachedGroup, groupCacheCapacity)}
}

func (c *groupCache) get(sig string) (cachedGroup, bool) {
	g, ok := c.ents[sig]
	return g, ok
}

func (c *groupCache) put(sig, block string, refs []clickRef) {
	if len(block) > groupCacheMaxOutput {
		return
	}
	if _, ok := c.ents[sig]; !ok {
		c.lru = append([]string{sig}, c.lru...)
	}
	c.ents[sig] = cachedGroup{block: block, refs: refs}
	if len(c.lru) > groupCacheCapacity {
		oldest := c.lru[len(c.lru)-1]
		delete(c.ents, oldest)
		c.lru = c.lru[:len(c.lru)-1]
	}
}

var groupCacheStore = newGroupCache()
