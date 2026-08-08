package buckets

import (
	"os"
	"path/filepath"
	"strings"
)

func (fs *FileSystemBucket) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(fs.basePath, path)
}

func (fs *FileSystemBucket) isBlocked(path string) bool {
	for _, blocked := range fs.blockedPaths {
		if path == blocked || strings.HasPrefix(path, blocked+"/") {
			return true
		}
	}
	return false
}

func (fs *FileSystemBucket) listRecursive(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		relPath, _ := filepath.Rel(fs.basePath, filepath.Join(dir, entry.Name()))
		files = append(files, relPath)
		if entry.IsDir() {
			subFiles := fs.listRecursive(filepath.Join(dir, entry.Name()))
			files = append(files, subFiles...)
		}
	}
	return files
}

func (fs *FileSystemBucket) applyOffsetLimit(content string, offset, limit int) string {
	lines := strings.Split(content, "\n")
	start := offset
	if start < 0 {
		start = 0
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	if start < end {
		return strings.Join(lines[start:end], "\n")
	}
	return ""
}

func (fs *FileSystemBucket) replaceOnce(s, old, new string) string {
	idx := strings.Index(s, old)
	if idx == -1 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func (fs *FileSystemBucket) getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
