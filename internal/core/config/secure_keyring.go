package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// PromptNewPassword prompts for a new password with confirmation
func PromptNewPassword() (string, error) {
	for {
		password, err := PromptPassword("Create master password: ")
		if err != nil {
			return "", err
		}

		if len(password) < 8 {
			fmt.Println("Password must be at least 8 characters.")
			continue
		}

		confirm, err := PromptPassword("Confirm master password: ")
		if err != nil {
			return "", err
		}

		if password != confirm {
			fmt.Println("Passwords do not match.")
			continue
		}

		return password, nil
	}
}

// writeFileSecure writes a file with specific permissions atomically.
// Syncs before rename so a crash cannot leave a truncated file, cleans up
// the temp file on any failure (including rename), and retries transiently
// on Windows where another reader/indexer can briefly hold the target open.
func writeFileSecure(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if err = tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err = tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err = tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err = tmpFile.Close(); err != nil {
		return err
	}

	const attempts = 10
	for attempt := 0; attempt < attempts; attempt++ {
		if err = os.Rename(tmpPath, path); err == nil {
			return nil
		}
		if runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}

// ClearSecureConfig removes all secure credentials

// isKeychainAvailable checks if macOS keychain is available
func isKeychainAvailable() bool {
	// For now, return false as we haven't implemented keychain integration
	// This would require cgo or external commands
	return false
}
