package sandbox

import (
	"os"
	"path/filepath"
	"strings"
)

// Config defines path and command restrictions.
type Config struct {
	WorkingDirectory         string
	AdditionalWorkingDirs    []string
	AllowAbsolutePaths       bool
	AllowHomeDirectoryAccess bool
}

// IsPathAllowed checks whether a file path is within permitted directories.
func IsPathAllowed(path string, cfg Config) bool {
	// Clean the path
	path = filepath.Clean(path)

	// Check working directory
	if isUnder(path, cfg.WorkingDirectory) {
		return true
	}

	// Check additional directories
	for _, dir := range cfg.AdditionalWorkingDirs {
		if isUnder(path, dir) {
			return true
		}
	}

	// Check home directory if allowed
	if cfg.AllowHomeDirectoryAccess {
		if home := getHome(); home != "" && isUnder(path, home) {
			return true
		}
	}

	// Allow absolute paths if configured
	if cfg.AllowAbsolutePaths && filepath.IsAbs(path) {
		return true
	}

	return false
}

// IsDangerousCommand detects commands that should require extra scrutiny.
func IsDangerousCommand(cmd string) bool {
	dangerous := []string{
		"rm -rf /",
		"curl | sh",
		"wget | sh",
		"curl | bash",
		"wget | bash",
		":(){ :|:& };:", // fork bomb
		"mkfs",
		"dd if=/dev/zero of=/dev/",
	}
	lower := strings.ToLower(cmd)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

func isUnder(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func getHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
