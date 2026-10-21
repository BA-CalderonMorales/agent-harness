package config

import (
	"fmt"
	"os"
	"runtime"
)

// Credential files are created 0600 and their directory 0700, but a
// restored backup, a copied profile, an archive extraction, or an older
// release can leave them readable by the group or the world. These
// helpers repair that drift on the read path and refuse symlinks, so a
// link planted in the config home cannot redirect a credential read to
// a file the user did not intend.

// readPrivateFile reads a credential-bearing file after confirming it is
// a regular file and tightening group/world permissions.
func readPrivateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlinked credential file %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(path, secureFilePerms); err != nil {
			return nil, fmt.Errorf("failed to tighten permissions on %s: %w", path, err)
		}
	}
	return os.ReadFile(path)
}

// ensurePrivateDir creates the credential directory and tightens an
// existing one that is group/world accessible.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, secureDirPerms); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, secureDirPerms); err != nil {
			return fmt.Errorf("failed to tighten permissions on %s: %w", dir, err)
		}
	}
	return nil
}
