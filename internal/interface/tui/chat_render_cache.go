package tui

import (
	"container/list"
	"hash/fnv"
	"sync"
)

// Render cache for markdown output. Every frame used to re-run glamour
// for every message in the transcript — cost grew with transcript
// length × frame rate and produced severe lag on long conversations.
// renderMarkdown is a pure function of (content, width), so its output
// memoizes exactly; streaming messages miss naturally (content changes
// per chunk) and width changes miss via the key.
//
// The cache covers both render paths: glamour on desktop and
// renderTermuxMarkdown on mobile, because both live inside
// renderMarkdown.
//
// Bounded LRU: marathon sessions must not grow memory without limit
// (the harness has an OOM history — see styles_code_test.go's
// fitBlockCode incident). A hard cap on per-entry output size keeps a
// pathological giant record from dominating the budget.

const (
	renderCacheCapacity  = 4096    // entries
	renderCacheMaxOutput = 1 << 20 // 1 MiB per entry — beyond that, don't cache
)

type renderCacheEntry struct {
	hash   uint64
	width  int
	output string
}

type renderCache struct {
	mu    sync.Mutex
	ents  map[uint64]*list.Element // hash -> element holding key+entry
	order *list.List               // LRU front = most recent
}

func newRenderCache() *renderCache {
	return &renderCache{
		ents:  make(map[uint64]*list.Element, renderCacheCapacity),
		order: list.New(),
	}
}

// contentHash is FNV-1a 64. A collision would serve a wrong render, but
// at 64-bit the accidental risk over the few thousand distinct strings
// in a session is negligible.
func contentHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func (c *renderCache) get(content string, width int) (string, bool) {
	if c == nil || len(content) == 0 {
		return "", false
	}
	key := contentHash(content)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.ents[key]; ok {
		e := el.Value.(*renderCacheEntry)
		if e.width == width {
			c.order.MoveToFront(el)
			return e.output, true
		}
		// Width mismatch: report a miss; put() replaces the entry.
		// Reads stay side-effect free.
	}
	return "", false
}

func (c *renderCache) put(content string, width int, output string) {
	if c == nil || len(content) == 0 || len(output) > renderCacheMaxOutput {
		return
	}
	key := contentHash(content)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.ents[key]; ok {
		el.Value.(*renderCacheEntry).width = width
		el.Value.(*renderCacheEntry).output = output
		c.order.MoveToFront(el)
		return
	}
	c.ents[key] = c.order.PushFront(&renderCacheEntry{hash: key, width: width, output: output})
	for c.order.Len() > renderCacheCapacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.ents, oldest.Value.(*renderCacheEntry).hash)
	}
}
