package config

import (
	"crypto/aes"
	"crypto/cipher"
	"golang.org/x/crypto/argon2"
)

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
