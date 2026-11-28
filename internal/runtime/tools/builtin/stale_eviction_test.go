package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
)

func TestEvictedReadBaselineBlocksWriteAndEditUntilReRead(t *testing.T) {
	origTracker := fs.DefaultStaleTracker
	fs.DefaultStaleTracker = fs.NewStaleWriteTracker()
	t.Cleanup(func() { fs.DefaultStaleTracker = origTracker })

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("initial content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.DefaultCache.InvalidatePath(target) })

	if _, err := FileReadTool.Call(map[string]any{"file_path": target}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("initial read: %v", err)
	}

	// Evict target's baseline by recording more than the default 1024 tracked reads.
	otherInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1030; i++ {
		fs.DefaultStaleTracker.RecordRead(filepath.Join(dir, fmt.Sprintf("other-%d.txt", i)), []byte("x"), otherInfo)
	}

	// Modify target externally after its baseline was evicted.
	if err := os.WriteFile(target, []byte("external change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	later := otherInfo.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(target, later, later); err != nil {
		t.Fatal(err)
	}

	if _, err := FileWriteTool.Call(map[string]any{
		"file_path": target,
		"content":   "agent overwrite\n",
	}, tools.Context{}, nil, nil); err == nil {
		t.Fatal("FileWriteTool succeeded after read baseline eviction, want stale/re-read error")
	}

	if _, err := FileEditTool.Call(map[string]any{
		"file_path":  target,
		"old_string": "external change",
		"new_string": "agent edit",
	}, tools.Context{}, nil, nil); err == nil {
		t.Fatal("FileEditTool succeeded after read baseline eviction, want stale/re-read error")
	}

	// External change must still be intact on disk.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "external change\n" {
		t.Fatalf("target content = %q, want external change preserved", string(got))
	}

	// Verify that re-reading an UNCHANGED evicted file that is still present in
	// DefaultCache bypasses the cache, restores the baseline, and unblocks edit.
	cachedPath := filepath.Join(dir, "cached-unchanged.txt")
	if err := os.WriteFile(cachedPath, []byte("hello unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.DefaultCache.InvalidatePath(cachedPath) })
	if _, err := FileReadTool.Call(map[string]any{"file_path": cachedPath}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("initial read cachedPath: %v", err)
	}
	for i := 0; i < 1030; i++ {
		fs.DefaultStaleTracker.RecordRead(filepath.Join(dir, fmt.Sprintf("evict-cached-%d.txt", i)), []byte("x"), otherInfo)
	}
	if !fs.DefaultStaleTracker.NeedsFreshRead(cachedPath) {
		t.Fatal("expected cachedPath to need a fresh read after eviction")
	}
	if _, err := FileReadTool.Call(map[string]any{"file_path": cachedPath}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("re-read cachedPath: %v", err)
	}
	if _, err := FileEditTool.Call(map[string]any{
		"file_path":  cachedPath,
		"old_string": "hello unchanged",
		"new_string": "hello edited",
	}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("edit after re-reading cached evicted file failed: %v", err)
	}
}
