package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

// MigrateFromLegacy migrates plaintext credentials to secure storage
func (cm *CredentialManager) MigrateFromLegacy() (*SecureConfig, error) {
	legacyPath := LegacyConfigPath()
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read legacy config: %w", err)
	}

	var fileCfg FileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("failed to parse legacy config: %w", err)
	}

	if fileCfg.APIKey == "" {
		return nil, fmt.Errorf("no API key found in legacy config")
	}

	// Migrate to secure storage
	secureCfg := &SecureConfig{
		Provider: fileCfg.Provider,
		APIKey:   fileCfg.APIKey,
		Model:    fileCfg.Model,
	}

	fmt.Println("Migrating credentials to secure storage...")
	if err := cm.SaveSecure(secureCfg); err != nil {
		return nil, fmt.Errorf("failed to migrate credentials: %w", err)
	}

	// Remove legacy file
	if err := os.Remove(legacyPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove legacy config: %v\n", err)
	}

	fmt.Println("Credentials migrated successfully. Legacy config removed.")
	return secureCfg, nil
}

func (cm *CredentialManager) ClearSecureConfig() error {
	if err := os.Remove(cm.configPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetEncryptionStatus returns the current encryption status
func (cm *CredentialManager) GetEncryptionStatus() EncryptionStatus {
	status := EncryptionStatus{
		Method: EncryptionNone,
	}

	if cm.HasSecureCredentials() {
		status.Method = EncryptionAES256GCM
		status.HasCredentials = true
	} else if cm.HasLegacyCredentials() {
		status.Method = EncryptionPlaintext
		status.HasCredentials = true
	}

	// Check for platform keychain (macOS)
	if runtime.GOOS == "darwin" {
		status.KeychainAvailable = isKeychainAvailable()
	}

	return status
}

// EncryptionStatus represents the current encryption state
type EncryptionStatus struct {
	Method            EncryptionMethod
	HasCredentials    bool
	KeychainAvailable bool
}

// EncryptionMethod represents the encryption method in use
type EncryptionMethod int

const (
	EncryptionNone EncryptionMethod = iota
	EncryptionPlaintext
	EncryptionKeychain
	EncryptionAES256GCM
)

func (e EncryptionMethod) String() string {
	switch e {
	case EncryptionNone:
		return "No credentials stored"
	case EncryptionPlaintext:
		return "Plaintext (insecure - migration recommended)"
	case EncryptionKeychain:
		return "Platform Keychain"
	case EncryptionAES256GCM:
		return "AES-256-GCM"
	default:
		return "Unknown"
	}
}
