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
}

// ReadCache is an LRU cache for file reads to preserve prompt cache tokens.
type ReadCache struct {
	mu      sync.RWMutex
	entries map[ReadCacheKey]ReadCacheEntry
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
	if err != nil || info.ModTime() != key.Mtime {
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
		var oldestTime time.Time
		first := true
		for k, v := range c.entries {
			if first || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
				first = false
			}
		}
		delete(c.entries, oldestKey)
	}

	c.entries[key] = ReadCacheEntry{
		Content:   content,
		Timestamp: time.Now(),
	}
}

// MakeKey creates a cache key from read parameters.
func MakeKey(path string, offset, limit int, info os.FileInfo) ReadCacheKey {
	return ReadCacheKey{
		Path:   path,
		Offset: offset,
		Limit:  limit,
		Mtime:  info.ModTime(),
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
type FileVersion struct {
	Path    string
	Mtime   time.Time
	Hash    string // SHA256 hash of content for content-based comparison
	Content string // Optional: store for Windows timestamp fallback
}

// ComputeHash computes SHA256 hash of content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// StaleWriteTracker tracks file versions to detect concurrent modifications.
type StaleWriteTracker struct {
	mu       sync.RWMutex
	versions map[string]FileVersion
}

// NewStaleWriteTracker creates a new tracker.
func NewStaleWriteTracker() *StaleWriteTracker {
	return &StaleWriteTracker{
		versions: make(map[string]FileVersion),
	}
}

// RecordRead records that a file was read at a specific version.
func (t *StaleWriteTracker) RecordRead(path string, content []byte, info os.FileInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.versions[path] = FileVersion{
		Path:    path,
		Mtime:   info.ModTime(),
		Hash:    ComputeHash(content),
		Content: string(content), // Store for Windows fallback
	}
}

// CheckStale returns an error if the file has been modified since the last read.
// On Windows, falls back to content comparison if timestamps are unreliable.
func (t *StaleWriteTracker) CheckStale(path string) error {
	t.mu.RLock()
	expected, ok := t.versions[path]
	t.mu.RUnlock()

	if !ok {
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
