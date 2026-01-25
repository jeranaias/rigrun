// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides NIST 800-53 SC-28 compliant encryption for data at rest.
//
// This implements Protection of Information at Rest (SC-28) with:
// - AES-256-GCM authenticated encryption
// - PBKDF2-SHA-256 key derivation
// - Platform-specific secure key storage (DPAPI on Windows, Keychain on macOS)
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jeranaias/rigrun-tui/internal/util"
	"golang.org/x/crypto/pbkdf2"
)

// =============================================================================
// SECURITY HELPER FUNCTIONS
// =============================================================================

// ZeroBytes securely zeros sensitive byte slices to prevent memory disclosure.
// SECURITY: Zero key material to prevent memory disclosure via crash dumps.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// =============================================================================
// CONSTANTS
// =============================================================================

// EncryptedPrefix marks a value as encrypted (format: ENC:base64(nonce|ciphertext|tag))
const EncryptedPrefix = "ENC:"

// NonceSize is the size of the nonce/IV for AES-GCM (12 bytes / 96 bits)
const NonceSize = 12

// KeySize is the size of the AES-256 key (32 bytes / 256 bits)
const KeySize = 32

// SaltSize is the size of the salt for key derivation (32 bytes)
const SaltSize = 32

// PBKDF2Iterations is the number of iterations for PBKDF2 key derivation
// OWASP 2023 recommends 600,000+ for PBKDF2-SHA-256 to provide adequate resistance
// against brute-force attacks with modern hardware
const PBKDF2Iterations = 600000

// =============================================================================
// ERRORS
// =============================================================================

var (
	// ErrNotInitialized indicates encryption has not been initialized
	ErrNotInitialized = errors.New("encryption not initialized: run 'rigrun encrypt init'")
	// ErrInvalidCiphertext indicates the ciphertext format is invalid
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	// ErrDecryptionFailed indicates decryption failed (wrong key or tampered data)
	ErrDecryptionFailed = errors.New("decryption failed: authentication tag mismatch")
	// ErrKeyStoreFailed indicates key storage operation failed
	ErrKeyStoreFailed = errors.New("key storage operation failed")
)

// =============================================================================
// ENCRYPTION STATUS
// =============================================================================

// EncryptionStatus represents the current state of encryption.
type EncryptionStatus struct {
	Initialized     bool   `json:"initialized"`
	Algorithm       string `json:"algorithm"`        // "AES-256-GCM"
	KeyDerivation   string `json:"key_derivation"`   // "PBKDF2-SHA-256"
	ConfigEncrypted bool   `json:"config_encrypted"`
	CacheEncrypted  bool   `json:"cache_encrypted"`
	AuditEncrypted  bool   `json:"audit_encrypted"`
	KeyStorePath    string `json:"key_store_path"`
}

// =============================================================================
// ENCRYPTION MANAGER
// =============================================================================

// EncryptionManager provides NIST 800-53 SC-28 compliant encryption for data at rest.
type EncryptionManager struct {
	mu           sync.RWMutex
	keyStore     KeyStore
	cipher       cipher.AEAD
	keyPath      string
	nonceCounter uint64            // Counter for deterministic nonce generation
	usedNonces   map[string]bool   // Track used nonces to prevent reuse
}

// NewEncryptionManager creates a new encryption manager.
// It will use platform-appropriate key storage (DPAPI on Windows, file-based on Unix).
func NewEncryptionManager() (*EncryptionManager, error) {
	keyPath, err := DefaultKeyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine key path: %w", err)
	}

	keyStore := NewKeyStore()

	em := &EncryptionManager{
		keyStore:     keyStore,
		keyPath:      keyPath,
		nonceCounter: 0,
		usedNonces:   make(map[string]bool),
	}

	// Try to initialize if key already exists
	if keyStore.Exists() {
		if err := em.loadKey(); err != nil {
			// Key exists but couldn't load - return manager but not initialized
			return em, nil
		}
	}

	return em, nil
}

// NewEncryptionManagerWithPassword creates a new encryption manager with password-based key derivation.
// This is used when a master password is provided instead of system key storage.
func NewEncryptionManagerWithPassword(password string) (*EncryptionManager, error) {
	keyPath, err := DefaultKeyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine key path: %w", err)
	}

	em := &EncryptionManager{
		keyStore:     NewKeyStore(),
		keyPath:      keyPath,
		nonceCounter: 0,
		usedNonces:   make(map[string]bool),
	}

	// Check if salt file exists
	saltPath := keyPath + ".salt"
	saltData, err := os.ReadFile(saltPath)
	if err != nil {
		return nil, fmt.Errorf("no encryption initialized: salt file not found")
	}

	// Derive key from password
	key := DeriveKey(password, saltData)
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(key)

	// Initialize cipher
	if err := em.initCipher(key); err != nil {
		return nil, fmt.Errorf("failed to initialize cipher: %w", err)
	}

	return em, nil
}

// DefaultKeyPath returns the default path for the master key (~/.rigrun/master.key).
func DefaultKeyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rigrun", "master.key"), nil
}

// =============================================================================
// KEY DERIVATION
// =============================================================================

// GenerateSalt generates a cryptographically secure random salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateMasterKey generates a cryptographically secure random master key.
func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate master key: %w", err)
	}
	return key, nil
}

// DeriveKey derives an encryption key from a password and salt using PBKDF2-SHA-256.
// This implements NIST SP 800-132 Password-Based Key Derivation.
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeySize, sha256.New)
}

// =============================================================================
// INITIALIZATION
// =============================================================================

// Initialize initializes encryption by generating a new master key and storing it securely.
// This should be called once during first-time setup.
func (e *EncryptionManager) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate new master key
	key, err := GenerateMasterKey()
	if err != nil {
		return fmt.Errorf("failed to generate master key: %w", err)
	}
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(key)

	// Store key using platform-specific secure storage
	if err := e.keyStore.Store(key); err != nil {
		return fmt.Errorf("failed to store master key: %w", err)
	}

	// Initialize cipher
	if err := e.initCipher(key); err != nil {
		// Clean up stored key if cipher init fails
		_ = e.keyStore.Delete()
		return fmt.Errorf("failed to initialize cipher: %w", err)
	}

	// Log initialization event
	AuditLogEvent("", "ENCRYPTION_INIT", map[string]string{
		"algorithm":      "AES-256-GCM",
		"key_derivation": "PBKDF2-SHA-256",
	})

	return nil
}

// InitializeWithPassword initializes encryption with a user-provided password.
// The password is used to derive the encryption key via PBKDF2.
func (e *EncryptionManager) InitializeWithPassword(password string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate salt
	salt, err := GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key from password
	key := DeriveKey(password, salt)
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(key)

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	// Store salt to file (the derived key is stored via keyStore)
	saltPath := e.keyPath + ".salt"
	if err := util.AtomicWriteFileWithDir(saltPath, salt, 0600, 0700); err != nil {
		return fmt.Errorf("failed to save salt: %w", err)
	}

	// Store derived key using platform-specific secure storage
	if err := e.keyStore.Store(key); err != nil {
		// Clean up salt file
		_ = os.Remove(saltPath)
		return fmt.Errorf("failed to store encryption key: %w", err)
	}

	// Initialize cipher
	if err := e.initCipher(key); err != nil {
		_ = e.keyStore.Delete()
		_ = os.Remove(saltPath)
		return fmt.Errorf("failed to initialize cipher: %w", err)
	}

	// Log initialization event
	AuditLogEvent("", "ENCRYPTION_INIT", map[string]string{
		"algorithm":      "AES-256-GCM",
		"key_derivation": "PBKDF2-SHA-256",
		"password_based": "true",
	})

	return nil
}

// loadKey loads the master key from secure storage and initializes the cipher.
func (e *EncryptionManager) loadKey() error {
	key, err := e.keyStore.Retrieve()
	if err != nil {
		return fmt.Errorf("failed to retrieve master key: %w", err)
	}
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(key)

	return e.initCipher(key)
}

// initCipher initializes the AES-GCM cipher with the given key.
func (e *EncryptionManager) initCipher(key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	e.cipher = gcm
	return nil
}

// =============================================================================
// STATUS
// =============================================================================

// IsInitialized returns true if encryption has been initialized.
func (e *EncryptionManager) IsInitialized() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cipher != nil
}

// GetStatus returns the current encryption status.
func (e *EncryptionManager) GetStatus() *EncryptionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &EncryptionStatus{
		Initialized:   e.cipher != nil,
		Algorithm:     "AES-256-GCM",
		KeyDerivation: "PBKDF2-SHA-256",
		KeyStorePath:  e.keyPath,
	}

	// Check if config is encrypted
	configPath, err := ConfigPathTOMLForEncryption()
	if err == nil {
		status.ConfigEncrypted = isFileEncrypted(configPath)
	}

	// Check if cache is encrypted
	cachePath := DefaultCachePathForEncryption()
	status.CacheEncrypted = isFileEncrypted(cachePath)

	// Check if audit log is encrypted
	auditPath := DefaultAuditPath()
	status.AuditEncrypted = isFileEncrypted(auditPath + ".enc")

	return status
}

// isFileEncrypted checks if a file contains encrypted content.
func isFileEncrypted(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Check if the file starts with our encryption marker or contains encrypted fields
	content := string(data)
	return strings.Contains(content, EncryptedPrefix)
}

// ConfigPathTOMLForEncryption returns the TOML config path without creating circular import.
func ConfigPathTOMLForEncryption() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rigrun", "config.toml"), nil
}

// DefaultCachePathForEncryption returns the default cache path.
func DefaultCachePathForEncryption() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".rigrun", "cache.json")
	}
	return filepath.Join(home, ".rigrun", "cache.json")
}

// =============================================================================
// ENCRYPTION OPERATIONS
// =============================================================================

// generateUniqueNonce generates a cryptographically unique nonce using a combination
// of random data and a counter to ensure uniqueness even if rand.Reader fails.
func (e *EncryptionManager) generateUniqueNonce() ([]byte, error) {
	const maxAttempts = 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		nonce := make([]byte, NonceSize)

		// First 8 bytes: counter (ensures uniqueness)
		e.nonceCounter++
		for i := 0; i < 8 && i < NonceSize; i++ {
			nonce[i] = byte(e.nonceCounter >> (i * 8))
		}

		// Remaining bytes: random data (ensures unpredictability)
		if NonceSize > 8 {
			randomPart := nonce[8:]
			if _, err := io.ReadFull(rand.Reader, randomPart); err != nil {
				// If random fails, use counter only (still unique but less ideal)
				AuditLogEvent("", "NONCE_RANDOM_FAILURE", map[string]string{
					"error": err.Error(),
					"fallback": "counter-only",
				})
				// Fill with counter-derived bytes instead
				for i := 8; i < NonceSize; i++ {
					nonce[i] = byte((e.nonceCounter >> ((i-8) * 8)) ^ 0xFF)
				}
			}
		}

		// Check for uniqueness (defense in depth)
		nonceStr := string(nonce)
		if !e.usedNonces[nonceStr] {
			e.usedNonces[nonceStr] = true

			// Prevent unbounded memory growth - clear old nonces if map gets too large
			if len(e.usedNonces) > 10000 {
				// Keep only recent half of nonces
				newMap := make(map[string]bool, 5000)
				count := 0
				for k, v := range e.usedNonces {
					if count >= 5000 {
						newMap[k] = v
					}
					count++
				}
				e.usedNonces = newMap
			}

			return nonce, nil
		}

		// Collision detected (should be extremely rare)
		AuditLogEvent("", "NONCE_COLLISION_DETECTED", map[string]string{
			"attempt": fmt.Sprintf("%d", attempt),
		})
	}

	return nil, fmt.Errorf("failed to generate unique nonce after %d attempts", maxAttempts)
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns: nonce || ciphertext || tag
// Uses counter-based nonce generation with uniqueness tracking for security.
func (e *EncryptionManager) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cipher == nil {
		return nil, ErrNotInitialized
	}

	// Generate unique nonce with tracking
	nonce, err := e.generateUniqueNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique nonce: %w", err)
	}

	// Encrypt with authentication tag
	ciphertext := e.cipher.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext encrypted with AES-256-GCM.
// Input format: nonce || ciphertext || tag
func (e *EncryptionManager) Decrypt(ciphertext []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.cipher == nil {
		return nil, ErrNotInitialized
	}

	if len(ciphertext) < NonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce
	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := e.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Log decryption failure as security event
		AuditLogEvent("", "DECRYPTION_FAILURE", map[string]string{
			"error": err.Error(),
		})
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext with ENC: prefix.
func (e *EncryptionManager) EncryptString(plaintext string) (string, error) {
	ciphertext, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64-encoded string with ENC: prefix.
func (e *EncryptionManager) DecryptString(ciphertext string) (string, error) {
	// Check for ENC: prefix
	if !strings.HasPrefix(ciphertext, EncryptedPrefix) {
		// Not encrypted, return as-is
		return ciphertext, nil
	}

	// Decode base64
	encoded := strings.TrimPrefix(ciphertext, EncryptedPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid base64 encoding: %w", err)
	}

	// Decrypt
	plaintext, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a string value is encrypted (has ENC: prefix).
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, EncryptedPrefix)
}

// =============================================================================
// FILE OPERATIONS
// =============================================================================

// EncryptFile encrypts a file and writes to the destination path.
func (e *EncryptionManager) EncryptFile(srcPath, dstPath string) error {
	// Read source file
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Encrypt
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		return err
	}

	// Write encrypted file with ENC: marker at the beginning
	output := []byte(EncryptedPrefix)
	output = append(output, []byte(base64.StdEncoding.EncodeToString(ciphertext))...)

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(dstPath, output, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}

	return nil
}

// DecryptFile decrypts a file and writes to the destination path.
func (e *EncryptionManager) DecryptFile(srcPath, dstPath string) error {
	// Read encrypted file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %w", err)
	}

	// Check for ENC: prefix
	content := string(data)
	if !strings.HasPrefix(content, EncryptedPrefix) {
		return fmt.Errorf("file is not encrypted (missing ENC: prefix)")
	}

	// Decode base64
	encoded := strings.TrimPrefix(content, EncryptedPrefix)
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %w", err)
	}

	// Decrypt
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return err
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(dstPath, plaintext, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write decrypted file: %w", err)
	}

	return nil
}

// =============================================================================
// CONFIG ENCRYPTION
// =============================================================================

// EncryptConfigField encrypts a single config field value.
// Returns the encrypted value with ENC: prefix.
func (e *EncryptionManager) EncryptConfigField(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if IsEncrypted(value) {
		// Already encrypted
		return value, nil
	}
	return e.EncryptString(value)
}

// DecryptConfigField decrypts a single config field value.
// Returns the plaintext value, or the original if not encrypted.
func (e *EncryptionManager) DecryptConfigField(value string) (string, error) {
	if value == "" || !IsEncrypted(value) {
		return value, nil
	}
	return e.DecryptString(value)
}

// =============================================================================
// KEY ROTATION
// =============================================================================

// RotateKey generates a new master key and re-encrypts all data.
// This is a critical operation that should be done with care.
// It re-encrypts all existing encrypted data to prevent data loss.
func (e *EncryptionManager) RotateKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cipher == nil {
		return ErrNotInitialized
	}

	// Save old cipher for re-encryption
	oldCipher := e.cipher

	// Generate new master key
	newKey, err := GenerateMasterKey()
	if err != nil {
		return fmt.Errorf("failed to generate new master key: %w", err)
	}
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(newKey)

	// Initialize new cipher
	if err := e.initCipher(newKey); err != nil {
		return fmt.Errorf("failed to initialize cipher with new key: %w", err)
	}

	// Re-encrypt all existing data with new key
	if err := e.reencryptAllData(oldCipher); err != nil {
		// Rollback on failure - restore old cipher
		e.cipher = oldCipher
		return fmt.Errorf("failed to re-encrypt data, rolled back to old key: %w", err)
	}

	// Store new key (only after successful re-encryption)
	if err := e.keyStore.Store(newKey); err != nil {
		// Attempt to re-encrypt back to old key
		_ = e.reencryptAllData(e.cipher)
		e.cipher = oldCipher
		return fmt.Errorf("failed to store new master key, rolled back: %w", err)
	}

	// Log rotation event
	AuditLogEvent("", "ENCRYPTION_ROTATE", map[string]string{
		"algorithm": "AES-256-GCM",
		"status":    "success",
	})

	return nil
}

// reencryptAllData re-encrypts all encrypted data from old cipher to new cipher.
// This is called during key rotation to prevent data loss.
func (e *EncryptionManager) reencryptAllData(oldCipher cipher.AEAD) error {
	// List of files to re-encrypt
	filesToReencrypt := []string{
		DefaultCachePathForEncryption(),
		DefaultAuditPath() + ".enc",
	}

	// Add config file if it exists
	configPath, err := ConfigPathTOMLForEncryption()
	if err == nil {
		filesToReencrypt = append(filesToReencrypt, configPath)
	}

	for _, filePath := range filesToReencrypt {
		// Check if file exists
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent files
			}
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// Check if file is encrypted
		content := string(data)
		if !strings.HasPrefix(content, EncryptedPrefix) {
			continue // Skip unencrypted files
		}

		// Decode base64
		encoded := strings.TrimPrefix(content, EncryptedPrefix)
		ciphertext, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("invalid base64 in %s: %w", filePath, err)
		}

		// Decrypt with old cipher
		if len(ciphertext) < NonceSize {
			return fmt.Errorf("invalid ciphertext in %s", filePath)
		}
		nonce := ciphertext[:NonceSize]
		oldCiphertext := ciphertext[NonceSize:]
		plaintext, err := oldCipher.Open(nil, nonce, oldCiphertext, nil)
		if err != nil {
			return fmt.Errorf("failed to decrypt %s with old key: %w", filePath, err)
		}

		// Encrypt with new cipher (e.cipher) using unique nonce generation
		newNonce, err := e.generateUniqueNonce()
		if err != nil {
			return fmt.Errorf("failed to generate unique nonce for %s: %w", filePath, err)
		}
		newCiphertext := e.cipher.Seal(newNonce, newNonce, plaintext, nil)

		// RELIABILITY: Atomic write with fsync prevents data loss on crash
		// Write re-encrypted data
		output := []byte(EncryptedPrefix)
		output = append(output, []byte(base64.StdEncoding.EncodeToString(newCiphertext))...)
		if err := util.AtomicWriteFile(filePath, output, 0600); err != nil {
			return fmt.Errorf("failed to write re-encrypted %s: %w", filePath, err)
		}
	}

	return nil
}

// =============================================================================
// CACHE ENCRYPTION
// =============================================================================

// EncryptCache encrypts the cache database file.
func (e *EncryptionManager) EncryptCache() error {
	cachePath := DefaultCachePathForEncryption()

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return fmt.Errorf("cache file not found: %s", cachePath)
	}

	// Check if already encrypted
	if isFileEncrypted(cachePath) {
		return fmt.Errorf("cache is already encrypted")
	}

	// Encrypt in place (to .enc file, then replace)
	encPath := cachePath + ".enc"
	if err := e.EncryptFile(cachePath, encPath); err != nil {
		return fmt.Errorf("failed to encrypt cache: %w", err)
	}

	// Backup original
	backupPath := cachePath + ".bak"
	if err := os.Rename(cachePath, backupPath); err != nil {
		_ = os.Remove(encPath)
		return fmt.Errorf("failed to backup cache: %w", err)
	}

	// Move encrypted file to original location
	if err := os.Rename(encPath, cachePath); err != nil {
		_ = os.Rename(backupPath, cachePath) // Restore backup
		return fmt.Errorf("failed to replace cache with encrypted version: %w", err)
	}

	// Remove backup
	_ = os.Remove(backupPath)

	AuditLogEvent("", "CACHE_ENCRYPTED", nil)
	return nil
}

// DecryptCache decrypts the cache database file.
func (e *EncryptionManager) DecryptCache() error {
	cachePath := DefaultCachePathForEncryption()

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return fmt.Errorf("cache file not found: %s", cachePath)
	}

	// Check if encrypted
	if !isFileEncrypted(cachePath) {
		return fmt.Errorf("cache is not encrypted")
	}

	// Decrypt to temp file
	decPath := cachePath + ".dec"
	if err := e.DecryptFile(cachePath, decPath); err != nil {
		return fmt.Errorf("failed to decrypt cache: %w", err)
	}

	// Backup original
	backupPath := cachePath + ".bak"
	if err := os.Rename(cachePath, backupPath); err != nil {
		_ = os.Remove(decPath)
		return fmt.Errorf("failed to backup encrypted cache: %w", err)
	}

	// Move decrypted file to original location
	if err := os.Rename(decPath, cachePath); err != nil {
		_ = os.Rename(backupPath, cachePath) // Restore backup
		return fmt.Errorf("failed to replace cache with decrypted version: %w", err)
	}

	// Remove backup
	_ = os.Remove(backupPath)

	return nil
}

// =============================================================================
// AUDIT LOG ENCRYPTION
// =============================================================================

// EncryptAuditLog encrypts the audit log file.
func (e *EncryptionManager) EncryptAuditLog() error {
	auditPath := DefaultAuditPath()

	// Check if audit log exists
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		return fmt.Errorf("audit log not found: %s", auditPath)
	}

	// Encrypt to .enc file
	encPath := auditPath + ".enc"
	if err := e.EncryptFile(auditPath, encPath); err != nil {
		return fmt.Errorf("failed to encrypt audit log: %w", err)
	}

	// Keep original for now (in production, would remove after verification)
	AuditLogEvent("", "AUDIT_ENCRYPTED", nil)
	return nil
}

// =============================================================================
// GLOBAL ENCRYPTION MANAGER
// =============================================================================

var (
	globalEncryptionManager     *EncryptionManager
	globalEncryptionManagerOnce sync.Once
	globalEncryptionManagerMu   sync.Mutex
)

// globalEncryptionManagerInitErr holds initialization error for fail-secure checking
var globalEncryptionManagerInitErr error

// GlobalEncryptionManager returns the global encryption manager instance.
// It lazily initializes the manager.
// SECURITY CRITICAL: Check GlobalEncryptionManagerHealthy() before relying on encryption.
func GlobalEncryptionManager() *EncryptionManager {
	globalEncryptionManagerOnce.Do(func() {
		var err error
		globalEncryptionManager, err = NewEncryptionManager()
		if err != nil {
			// SECURITY: Fail-secure - record error for callers to check
			globalEncryptionManagerInitErr = fmt.Errorf("SECURITY CRITICAL: encryption manager init failed: %w", err)
			// Log to stderr
			fmt.Fprintf(os.Stderr, "[SECURITY CRITICAL] Encryption manager initialization failed: %v\n", err)
			// Create non-functional manager but mark it as unhealthy
			globalEncryptionManager = &EncryptionManager{}
		}
	})
	return globalEncryptionManager
}

// GlobalEncryptionManagerHealthy returns true if the global manager initialized successfully.
// SECURITY: Callers should check this before relying on encryption for sensitive data.
func GlobalEncryptionManagerHealthy() bool {
	GlobalEncryptionManager() // Ensure initialized
	return globalEncryptionManagerInitErr == nil
}

// GlobalEncryptionManagerError returns the initialization error, if any.
func GlobalEncryptionManagerError() error {
	GlobalEncryptionManager() // Ensure initialized
	return globalEncryptionManagerInitErr
}

// SetGlobalEncryptionManager sets the global encryption manager instance.
func SetGlobalEncryptionManager(em *EncryptionManager) {
	globalEncryptionManagerMu.Lock()
	defer globalEncryptionManagerMu.Unlock()
	globalEncryptionManager = em
}
