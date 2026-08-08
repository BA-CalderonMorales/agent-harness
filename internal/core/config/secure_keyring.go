package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// writeFileSecure writes a file with specific permissions atomically
func writeFileSecure(path string, data []byte, perm os.FileMode) error {
	// Create temp file in same directory
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on error
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// Set permissions before writing (security: prevent race condition)
	if err = tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}

	// Write data
	if _, err = tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err = tmpFile.Close(); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, path)
}

// ClearSecureConfig removes all secure credentials

// isKeychainAvailable checks if macOS keychain is available
func isKeychainAvailable() bool {
	// For now, return false as we haven't implemented keychain integration
	// This would require cgo or external commands
	return false
}
