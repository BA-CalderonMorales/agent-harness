package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// ReadCacheKey identifies a cached file read operation.
type ReadCacheKey struct {
	Path   string
	Offset int
	Limit  int
	Mtime  time.Time
}

// ReadCacheEntry stores cached read results.
type ReadCacheEntry struct {
	Content   string
	Timestamp time.Time
	seq       uint64
}

// ReadCache is an LRU cache for file reads to preserve prompt cache tokens.
type ReadCache struct {
	mu      sync.RWMutex
	entries map[ReadCacheKey]ReadCacheEntry
	nextSeq uint64
	maxSize int
}

// NewReadCache creates a new file read cache.
func NewReadCache(maxSize int) *ReadCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ReadCache{
		entries: make(map[ReadCacheKey]ReadCacheEntry),
		maxSize: maxSize,
	}
}

// Get retrieves cached content if it exists and is fresh.
func (c *ReadCache) Get(key ReadCacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}

	// Verify file hasn't changed on disk
	info, err := os.Stat(key.Path)
	if err != nil || !info.ModTime().UTC().Equal(key.Mtime) {
		return "", false
	}

	return entry.Content, true
}

// Set stores content in the cache.
func (c *ReadCache) Set(key ReadCacheKey, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize {
		var oldestKey ReadCacheKey
		var oldestSeq uint64
		first := true
		for k, v := range c.entries {
			if first || v.seq < oldestSeq {
				oldestKey = k
				oldestSeq = v.seq
				first = false
			}
		}
		delete(c.entries, oldestKey)
	}

	c.nextSeq++
	c.entries[key] = ReadCacheEntry{
		Content:   content,
		Timestamp: time.Now(),
		seq:       c.nextSeq,
	}
}

// MakeKey creates a cache key from read parameters. Mtime is normalized to
// UTC to strip any Go monotonic clock reading before use as a map key.
func MakeKey(path string, offset, limit int, info os.FileInfo) ReadCacheKey {
	return ReadCacheKey{
		Path:   path,
		Offset: offset,
		Limit:  limit,
		Mtime:  info.ModTime().UTC(),
	}
}

// InvalidatePath removes all cache entries for a given path.
func (c *ReadCache) InvalidatePath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if key.Path == path {
			delete(c.entries, key)
		}
	}
}

// Global cache instance
var DefaultCache = NewReadCache(100)

// FileVersion tracks file metadata for stale-write protection.
// Only the SHA-256 hash and mtime are retained; storing full file bodies
// kept an unbounded copy of every file read in memory for the whole session.
type FileVersion struct {
	Path       string
	Mtime      time.Time
	Hash       string // SHA256 hash of content for content-based comparison
	RecordedAt time.Time
	seq        uint64
}

// ComputeHash computes SHA256 hash of content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

const (
	defaultMaxTrackedFiles = 1024
	defaultMaxEvictedPaths = 4096
	evictedFilterWords     = 1024 // 65,536 bits = 8 KiB fixed cap
	evictedFilterBits      = evictedFilterWords * 64
)

// StaleWriteTracker tracks file versions to detect concurrent modifications.
type StaleWriteTracker struct {
	mu             sync.RWMutex
	versions       map[string]FileVersion
	evicted        map[string]struct{}
	evictedOrder   []string
	evictedFilter  [evictedFilterWords]uint64
	hasSpilled     bool
	nextSeq        uint64
	maxSize        int
	maxEvictedSize int
}

// NewStaleWriteTracker creates a new tracker.
func NewStaleWriteTracker() *StaleWriteTracker {
	return &StaleWriteTracker{
		versions:       make(map[string]FileVersion),
		evicted:        make(map[string]struct{}),
		maxSize:        defaultMaxTrackedFiles,
		maxEvictedSize: defaultMaxEvictedPaths,
	}
}

func (t *StaleWriteTracker) recordEvicted(path string) {
	if _, already := t.evicted[path]; already {
		return
	}
	maxEvicted := t.maxEvictedSize
	if maxEvicted <= 0 {
		maxEvicted = defaultMaxEvictedPaths
	}
	for len(t.evicted) >= maxEvicted && len(t.evictedOrder) > 0 {
		oldest := t.evictedOrder[0]
		t.evictedOrder = t.evictedOrder[1:]
		if _, present := t.evicted[oldest]; !present {
			continue
		}
		delete(t.evicted, oldest)
		t.addEvictedFilter(oldest)
	}
	t.evicted[path] = struct{}{}
	t.evictedOrder = append(t.evictedOrder, path)
	if len(t.evictedOrder) > maxEvicted*2 {
		compacted := make([]string, 0, len(t.evicted))
		for _, p := range t.evictedOrder {
			if _, present := t.evicted[p]; present {
				compacted = append(compacted, p)
			}
		}
		t.evictedOrder = compacted
	}
}

func (t *StaleWriteTracker) addEvictedFilter(path string) {
	b1, b2 := evictedFilterIndices(path)
	t.evictedFilter[b1/64] |= 1 << (b1 % 64)
	t.evictedFilter[b2/64] |= 1 << (b2 % 64)
	t.hasSpilled = true
}

func (t *StaleWriteTracker) isEvictedLocked(path string) bool {
	if _, ok := t.evicted[path]; ok {
		return true
	}
	if !t.hasSpilled {
		return false
	}
	b1, b2 := evictedFilterIndices(path)
	return (t.evictedFilter[b1/64]&(1<<(b1%64))) != 0 &&
		(t.evictedFilter[b2/64]&(1<<(b2%64))) != 0
}

func evictedFilterIndices(path string) (uint64, uint64) {
	sum := sha256.Sum256([]byte(path))
	h1 := uint64(sum[0]) | uint64(sum[1])<<8 | uint64(sum[2])<<16 | uint64(sum[3])<<24
	h2 := uint64(sum[4]) | uint64(sum[5])<<8 | uint64(sum[6])<<16 | uint64(sum[7])<<24
	return h1 % evictedFilterBits, h2 % evictedFilterBits
}

// RecordRead records that a file was read at a specific version.
func (t *StaleWriteTracker) RecordRead(path string, content []byte, info os.FileInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.versions == nil {
		t.versions = make(map[string]FileVersion)
	}
	if t.evicted == nil {
		t.evicted = make(map[string]struct{})
	}
	maxSize := t.maxSize
	if maxSize <= 0 {
		maxSize = defaultMaxTrackedFiles
	}
	if _, exists := t.versions[path]; !exists && len(t.versions) >= maxSize {
		var oldestPath string
		var oldestSeq uint64
		first := true
		for p, v := range t.versions {
			if first || v.seq < oldestSeq {
				oldestPath = p
				oldestSeq = v.seq
				first = false
			}
		}
		delete(t.versions, oldestPath)
		t.recordEvicted(oldestPath)
	}

	delete(t.evicted, path)
	t.nextSeq++
	t.versions[path] = FileVersion{
		Path:       path,
		Mtime:      info.ModTime().UTC(),
		Hash:       ComputeHash(content),
		RecordedAt: time.Now(),
		seq:        t.nextSeq,
	}
}

// NeedsFreshRead reports whether path has an evicted baseline and therefore
// must bypass the read cache so re-reading restores a fresh stale-write baseline.
func (t *StaleWriteTracker) NeedsFreshRead(path string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if _, ok := t.versions[path]; ok {
		return false
	}
	return t.isEvictedLocked(path)
}

// CheckStale returns an error if the file has been modified since the last read,
// or if its read baseline was evicted and a fresh read is required.
// On Windows, falls back to content comparison if timestamps are unreliable.
func (t *StaleWriteTracker) CheckStale(path string) error {
	t.mu.RLock()
	expected, ok := t.versions[path]
	wasEvicted := !ok && t.isEvictedLocked(path)
	t.mu.RUnlock()

	if !ok {
		if wasEvicted {
			return fmt.Errorf("read baseline was evicted; re-read %s before editing or writing", path)
		}
		// No prior read recorded - allow the write
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat file for stale check: %w", err)
	}

	// Primary check: mtime unchanged
	if info.ModTime().Equal(expected.Mtime) {
		return nil
	}

	// On Windows or if mtime differs, check content hash
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file for stale check: %w", err)
	}

	currentHash := ComputeHash(content)
	if currentHash == expected.Hash {
		// File unchanged despite timestamp difference
		return nil
	}

	return fmt.Errorf("file was modified since last read (stale write protection)")
}

// Global tracker instance
var DefaultStaleTracker = NewStaleWriteTracker()
