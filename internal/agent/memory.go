package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MemoryLoader discovers and loads project-specific memory files.
// This mirrors Claude Code's CLAUDE.md loading pattern.
type MemoryLoader struct {
	// Nested memory paths already loaded this session (dedup)
	LoadedPaths map[string]bool
}

// NewMemoryLoader creates a fresh memory loader.
func NewMemoryLoader() *MemoryLoader {
	return &MemoryLoader{LoadedPaths: make(map[string]bool)}
}

// LoadForDirectory finds memory files in the given directory and its parents.
// Files are loaded from root-to-leaf so leaf files override parent files.
func (m *MemoryLoader) LoadForDirectory(dir string) ([]MemoryFile, error) {
	var files []MemoryFile
	current := dir

	for {
		for _, name := range memoryFileNames() {
			path := filepath.Join(current, name)
			if _, err := os.Stat(path); err == nil {
				if !m.LoadedPaths[path] {
					content, err := os.ReadFile(path)
					if err == nil {
						files = append(files, MemoryFile{
							Path:    path,
							Content: string(content),
						})
						m.LoadedPaths[path] = true
					}
				}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Reverse so root comes first, leaf comes last
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}

	return files, nil
}

// MemoryFile represents a loaded memory document.
type MemoryFile struct {
	Path    string
	Content string
}

// FormatSystemPrompt appends memory content to the system prompt.
func (m *MemoryFile) FormatSystemPrompt() string {
	return fmt.Sprintf("\n\n<memory file=\"%s\">\n%s\n</memory>", m.Path, m.Content)
}

func memoryFileNames() []string {
	// Priority order: AGENTS.md is checked first, then CLAUDE.md
	return []string{"AGENTS.md", "CLAUDE.md", ".claude.md"}
}

// FindSimilarFile suggests a file path when the exact one doesn't exist.
func FindSimilarFile(target string) string {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var best string
	bestScore := 0
	for _, entry := range entries {
		name := entry.Name()
		score := similarityScore(base, name)
		if score > bestScore {
			bestScore = score
			best = filepath.Join(dir, name)
		}
	}
	return best
}

// similarityScore returns a rough similarity metric (0-100).
func similarityScore(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a == b {
		return 100
	}
	// Simple prefix/suffix matching
	if strings.HasPrefix(b, a) || strings.HasSuffix(b, a) {
		return 50
	}
	return 0
}
