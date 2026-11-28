package builtin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/fs"
)

func TestFileReadBlocksUncleanDevicePathsDirsAndOversizedFiles(t *testing.T) {
	for _, bad := range []string{"/dev//zero", "/dev/./random", "/proc/self/./fd/0"} {
		if vr := FileReadTool.ValidateInput(map[string]any{"file_path": bad}, tools.Context{}); vr.Valid {
			t.Errorf("ValidateInput(%q) = valid, want blocked", bad)
		}
	}

	dir := t.TempDir()
	if _, err := FileReadTool.Call(map[string]any{"file_path": dir}, tools.Context{}, nil, nil); err == nil {
		t.Fatal("FileReadTool on directory succeeded, want error")
	}

	bigPath := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(bigPath, []byte(strings.Repeat("x", 2048)), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := tools.Context{FileReadingLimits: tools.FileReadingLimits{MaxSizeBytes: 512}}
	if _, err := FileReadTool.Call(map[string]any{"file_path": bigPath}, ctx, nil, nil); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("FileReadTool on oversized file = %v, want too large error", err)
	}
}

func TestFileReadCacheHitDoesNotCorruptStaleTracker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	body := "line1\nline2\nline3\nline4\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.DefaultCache.InvalidatePath(path) })

	// First partial read populates both cache (sliced) and stale tracker (full hash).
	if _, err := FileReadTool.Call(map[string]any{"file_path": path, "offset": 0, "limit": 1}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("first read: %v", err)
	}
	// Second partial read hits cache; must not clobber stale tracker with slice.
	if _, err := FileReadTool.Call(map[string]any{"file_path": path, "offset": 0, "limit": 1}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("cached read: %v", err)
	}
	// Touch mtime without changing content so CheckStale exercises the hash path.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if err := fs.DefaultStaleTracker.CheckStale(path); err != nil {
		t.Fatalf("CheckStale failed after cached partial read: %v", err)
	}
}

func TestFileEditNormalizesCurlyQuotesAndPreservesExecutableMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.sh")
	initial := "#!/bin/sh\necho \u201chello\u201d\n# author=O\u2019Neil\n"
	if err := os.WriteFile(path, []byte(initial), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.DefaultCache.InvalidatePath(path) })

	if _, err := FileReadTool.Call(map[string]any{"file_path": path}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("read: %v", err)
	}
	_, err := FileEditTool.Call(map[string]any{
		"file_path":  path,
		"old_string": `echo "hello"`,
		"new_string": `echo "world"`,
	}, tools.Context{}, nil, nil)
	if err != nil {
		t.Fatalf("edit with curly-quote fallback failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "#!/bin/sh\necho \"world\"\n# author=O\u2019Neil\n"
	if string(got) != want {
		t.Fatalf("edited content = %q, want %q (unrelated curly quote must be preserved)", string(got), want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("executable mode lost after edit: %o", info.Mode().Perm())
		}
	}

	// Verify replace_all=true replaces every normalized match while still
	// preserving unrelated curly quotes elsewhere in the file.
	multiPath := filepath.Join(dir, "multi.txt")
	multiInit := "a=\u201cx\u201d\nb=O\u2019Neil\nc=\u201cx\u201d\n"
	if err := os.WriteFile(multiPath, []byte(multiInit), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fs.DefaultCache.InvalidatePath(multiPath) })
	if _, err := FileReadTool.Call(map[string]any{"file_path": multiPath}, tools.Context{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := FileEditTool.Call(map[string]any{
		"file_path":   multiPath,
		"old_string":  `"x"`,
		"new_string":  `"y"`,
		"replace_all": true,
	}, tools.Context{}, nil, nil); err != nil {
		t.Fatalf("replace_all curly-quote fallback failed: %v", err)
	}
	multiGot, err := os.ReadFile(multiPath)
	if err != nil {
		t.Fatal(err)
	}
	multiWant := "a=\"y\"\nb=O\u2019Neil\nc=\"y\"\n"
	if string(multiGot) != multiWant {
		t.Fatalf("replace_all content = %q, want %q", string(multiGot), multiWant)
	}
}

func TestBashPreservesExitStatusWhenOutputTruncated(t *testing.T) {
	cmd := "i=1; while [ $i -le 450 ]; do echo line-$i; i=$((i+1)); done; exit 3"
	result, err := runBashCommand(context.Background(), cmd, 5000, nil)
	if err != nil {
		t.Fatalf("runBashCommand() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for exit 3")
	}
	out, _ := result.Data.(string)
	if !strings.Contains(out, "[Output truncated:") {
		t.Fatalf("expected truncation notice, got tail: %q", out[max(0, len(out)-120):])
	}
	if !strings.Contains(out, "[exit status:") {
		t.Fatalf("truncation stripped exit status marker: %q", out[max(0, len(out)-120):])
	}
}

func TestTruncateOutputPreservesValidUTF8(t *testing.T) {
	// 4-byte emoji repeated so slicing at raw byte boundary would split a rune.
	input := strings.Repeat("🔥", 20)
	got := truncateOutput(input, 10, 100, "chars", "lines")
	if !utf8.ValidString(got) {
		t.Fatalf("truncateOutput produced invalid UTF-8: %q", got)
	}
}

func TestIsCommonIgnoredDirMatchesGlobsAndWorktrees(t *testing.T) {
	for _, dir := range []string{".claude", ".worktrees", ".next", "mypkg.egg-info", "NODE_MODULES"} {
		if !isCommonIgnoredDir(dir) {
			t.Errorf("isCommonIgnoredDir(%q) = false, want true", dir)
		}
	}
	if isCommonIgnoredDir("internal") {
		t.Error("isCommonIgnoredDir(internal) = true, want false")
	}
}

func TestGrepSkipsBinaryHonorsCancelAndHandlesLongLines(t *testing.T) {
	dir := t.TempDir()
	// Binary file containing the needle after a NUL byte must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte("hdr\x00needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Long line (>64 KiB) that previously broke bufio.NewScanner's default limit.
	longLine := strings.Repeat("a", 80*1024) + "needle"
	if err := os.WriteFile(filepath.Join(dir, "long.txt"), []byte(longLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := GrepTool.Call(map[string]any{"pattern": "needle", "path": dir}, tools.Context{AbortController: context.Background()}, nil, nil)
	if err != nil {
		t.Fatalf("GrepTool.Call() error = %v", err)
	}
	out, _ := res.Data.(string)
	if strings.Contains(out, "bin.dat") {
		t.Fatalf("grep matched inside binary file: %q", out)
	}
	if !strings.Contains(out, "long.txt:1:") {
		t.Fatalf("grep missed match on >64KiB line: %q", out)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GrepTool.Call(map[string]any{"pattern": "needle", "path": dir}, tools.Context{AbortController: canceled}, nil, nil); err == nil {
		t.Fatal("grep ignored canceled context")
	}
}
