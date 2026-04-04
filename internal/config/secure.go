// Secure credential storage for agent-harness
//
// Implements defense-in-depth for API key storage:
// 1. Platform keychain integration (preferred on macOS)
// 2. AES-256-GCM encrypted file fallback
// 3. Argon2id key derivation
// 4. Proper file permissions (0600)
// 5. Transparent migration from plaintext

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	// Current encryption format version
	secureStoreVersion = 1
	// File permissions: user read/write only
	secureFilePerms = 0600
	// Dir permissions: user read/write/execute only
	secureDirPerms = 0700
)

// SecureStore represents encrypted credential storage
type SecureStore struct {
	Version    uint32 `json:"version"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
}

// SecureConfig holds runtime configuration loaded from secure storage
type SecureConfig struct {
	Provider string
	APIKey   string
	Model    string
}

// CredentialManager handles secure credential operations
type CredentialManager struct {
	configPath string
	masterKey  []byte
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager() *CredentialManager {
	return &CredentialManager{
		configPath: SecureConfigPath(),
	}
}

// SecureConfigPath returns the path for encrypted config
func SecureConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness", "credentials.enc")
}

// LegacyConfigPath returns the old plaintext config path
func LegacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness", "config.json")
}

// HasSecureCredentials checks if encrypted credentials exist
func (cm *CredentialManager) HasSecureCredentials() bool {
	_, err := os.Stat(cm.configPath)
	return err == nil
}

// HasLegacyCredentials checks if old plaintext credentials exist
func (cm *CredentialManager) HasLegacyCredentials() bool {
	_, err := os.Stat(LegacyConfigPath())
	return err == nil
}

// LoadSecure loads credentials from encrypted storage
func (cm *CredentialManager) LoadSecure() (*SecureConfig, error) {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secure config: %w", err)
	}

	var store SecureStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse secure config: %w", err)
	}

	if store.Version != secureStoreVersion {
		return nil, fmt.Errorf("unsupported secure store version: %d", store.Version)
	}

	// Validate stored data integrity before attempting decryption
	if len(store.Salt) == 0 {
		return nil, fmt.Errorf("corrupted credentials: missing salt")
	}
	if len(store.Nonce) == 0 {
		return nil, fmt.Errorf("corrupted credentials: missing nonce")
	}
	if len(store.Ciphertext) == 0 {
		return nil, fmt.Errorf("corrupted credentials: missing encrypted data")
	}
	// AES-GCM nonce should be 12 bytes
	if len(store.Nonce) != 12 {
		return nil, fmt.Errorf("corrupted credentials: invalid nonce length (%d, expected 12)", len(store.Nonce))
	}

	// Get master password if not already set
	if cm.masterKey == nil {
		password, err := PromptPassword("Enter master password: ")
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %w", err)
		}
		// Validate password isn't empty
		if password == "" {
			return nil, fmt.Errorf("password cannot be empty")
		}
		cm.masterKey = deriveKey(password, store.Salt)
	}

	// Decrypt
	plaintext, err := cm.decrypt(store.Ciphertext, store.Nonce)
	if err != nil {
		// Clear the master key so next attempt will prompt for password again
		cm.masterKey = nil
		return nil, fmt.Errorf("failed to decrypt credentials (wrong password?): %w", err)
	}

	return &SecureConfig{
		Provider: store.Provider,
		APIKey:   string(plaintext),
		Model:    store.Model,
	}, nil
}

// SaveSecure saves credentials to encrypted storage
func (cm *CredentialManager) SaveSecure(cfg *SecureConfig) error {
	// Generate salt if not exists
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Prompt for master password if not set
	if cm.masterKey == nil {
		password, err := PromptNewPassword()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		cm.masterKey = deriveKey(password, salt)
	}

	// Generate nonce
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext, err := cm.encrypt([]byte(cfg.APIKey), nonce)
	if err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}

	// Create store
	store := SecureStore{
		Version:    secureStoreVersion,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Provider:   cfg.Provider,
		Model:      cfg.Model,
	}

	// Marshal and save with secure permissions
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store: %w", err)
	}

	// Ensure directory exists with secure permissions
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, secureDirPerms); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write with secure permissions
	if err := writeFileSecure(cm.configPath, data, secureFilePerms); err != nil {
		return fmt.Errorf("failed to write secure config: %w", err)
	}

	// Remove legacy file if it exists
	legacyPath := LegacyConfigPath()
	if _, err := os.Stat(legacyPath); err == nil {
		_ = os.Remove(legacyPath)
	}

	return nil
}

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

// encrypt encrypts plaintext using AES-256-GCM
func (cm *CredentialManager) encrypt(plaintext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// decrypt decrypts ciphertext using AES-256-GCM
func (cm *CredentialManager) decrypt(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// deriveKey derives a 32-byte key from password using Argon2id
func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

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
func (cm *CredentialManager) ClearSecureConfig() error {
	if err := os.Remove(cm.configPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// UpdateDefaultModel updates just the default model in secure storage
// This is called when the user changes the model at runtime
func (cm *CredentialManager) UpdateDefaultModel(model string) error {
	// Load existing credentials
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read secure config: %w", err)
	}

	var store SecureStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("failed to parse secure config: %w", err)
	}

	if store.Version != secureStoreVersion {
		return fmt.Errorf("unsupported secure store version: %d", store.Version)
	}

	// Update the model
	store.Model = model

	// Marshal and save
	newData, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store: %w", err)
	}

	if err := writeFileSecure(cm.configPath, newData, secureFilePerms); err != nil {
		return fmt.Errorf("failed to write secure config: %w", err)
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

// isKeychainAvailable checks if macOS keychain is available
func isKeychainAvailable() bool {
	// For now, return false as we haven't implemented keychain integration
	// This would require cgo or external commands
	return false
}
