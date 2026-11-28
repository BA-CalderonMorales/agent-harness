package fs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCacheHitMissInvalidateAndEvict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	cache := NewReadCache(2)
	key := MakeKey(path, 0, 10, info)
	cache.Set(key, "hello")

	got, ok := cache.Get(key)
	if !ok || got != "hello" {
		t.Fatalf("Get() = (%q, %v), want (hello, true)", got, ok)
	}

	// Updating mtime on disk invalidates the cached lookup.
	later := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("Get() hit after file mtime changed")
	}

	// Explicit path invalidation drops all keys for that path.
	cache.Set(key, "hello")
	cache.InvalidatePath(path)
	if len(cache.entries) != 0 {
		t.Fatalf("entries after InvalidatePath = %d, want 0", len(cache.entries))
	}

	// Eviction keeps cache bounded at maxSize.
	for i := 0; i < 5; i++ {
		cache.Set(ReadCacheKey{Path: filepath.Join(dir, string(rune('a'+i))), Mtime: time.Now().UTC()}, "v")
	}
	if len(cache.entries) > 2 {
		t.Fatalf("cache grew to %d entries, want <= 2", len(cache.entries))
	}
}

func TestStaleWriteTrackerDetectsModificationsAndStaysBounded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tracker := NewStaleWriteTracker()
	tracker.maxSize = 3
	tracker.RecordRead(path, []byte("original"), info)

	if err := tracker.CheckStale(path); err != nil {
		t.Fatalf("CheckStale(unchanged) = %v, want nil", err)
	}

	// Same content with newer mtime passes via hash fallback.
	later := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if err := tracker.CheckStale(path); err != nil {
		t.Fatalf("CheckStale(same content, newer mtime) = %v, want nil", err)
	}

	// Different content with newer mtime fails stale check.
	if err := os.WriteFile(path, []byte("modified externally"), 0o600); err != nil {
		t.Fatal(err)
	}
	evenLater := later.Add(2 * time.Second)
	if err := os.Chtimes(path, evenLater, evenLater); err != nil {
		t.Fatal(err)
	}
	if err := tracker.CheckStale(path); err == nil {
		t.Fatal("CheckStale(modified) = nil, want stale write error")
	}

	// Bounded size prevents unbounded session memory growth, while evicted
	// baselines still fail CheckStale with a re-read error rather than acting
	// like never-read files.
	for i := 0; i < 10; i++ {
		tracker.RecordRead(filepath.Join(dir, string(rune('a'+i))), []byte("x"), info)
	}
	if len(tracker.versions) > 3 {
		t.Fatalf("tracker stored %d entries, want <= 3", len(tracker.versions))
	}
	if err := tracker.CheckStale(path); err == nil {
		t.Fatal("CheckStale(evicted path) = nil, want reread-required error")
	}
	// A path that was never read is still allowed.
	if err := tracker.CheckStale(filepath.Join(dir, "never-read.txt")); err != nil {
		t.Fatalf("CheckStale(never-read) = %v, want nil", err)
	}
	// Re-reading clears the evicted state and restores normal tracking.
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	curInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	tracker.RecordRead(path, cur, curInfo)
	if err := tracker.CheckStale(path); err != nil {
		t.Fatalf("CheckStale(re-read path) = %v, want nil", err)
	}
}
