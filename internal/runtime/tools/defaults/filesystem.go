package defaults

import "os"

// FS defaults for file system operations.
const (
	FSMaxFileSize   int64 = 10 * 1024 * 1024 // 10MB
	FSMaxReadOffset int   = 10000
	FSMaxReadLimit  int   = 10000
)

// FSDangerousPaths are paths that should never be accessed.
var FSDangerousPaths = []string{
	"/dev/zero", "/dev/random", "/dev/urandom",
	"/dev/full", "/dev/stdin", "/dev/tty",
	"/dev/console", "/dev/stdout", "/dev/stderr",
	"/dev/fd/0", "/dev/fd/1", "/dev/fd/2",
	"/proc", "/sys",
}

// FSIsBlockedDevicePath checks if a path is a blocked device.
func FSIsBlockedDevicePath(path string) bool {
	blocked := map[string]bool{
		"/dev/zero": true, "/dev/random": true, "/dev/urandom": true,
		"/dev/full": true, "/dev/stdin": true, "/dev/tty": true,
		"/dev/console": true, "/dev/stdout": true, "/dev/stderr": true,
		"/dev/fd/0": true, "/dev/fd/1": true, "/dev/fd/2": true,
	}
	return blocked[path]
}

// FSIsUNCPath checks if path is a UNC path (security risk).
func FSIsUNCPath(path string) bool {
	return len(path) >= 2 && (path[0:2] == "\\\\" || path[0:2] == "//")
}

// FSIsProcFdPath checks if path is a /proc/*/fd path.
func FSIsProcFdPath(path string) bool {
	return len(path) > 6 && path[0:6] == "/proc/" && contains(path, "/fd/")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// FSFileInfoGetter gets file info.
type FSFileInfoGetter func(path string) (os.FileInfo, error)

// DefaultFSFileInfoGetter uses os.Stat.
var DefaultFSFileInfoGetter FSFileInfoGetter = os.Stat
