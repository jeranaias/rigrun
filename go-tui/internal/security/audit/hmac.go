// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection.
//
// This file implements NIST 800-53 AU-9: HMAC Key Management for Audit Logs.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// NIST 800-53 AU-9: HMAC KEY MANAGEMENT
// =============================================================================

// KeySource indicates where the HMAC key was loaded from.
type KeySource string

const (
	KeySourceEnvVar  KeySource = "environment_variable"
	KeySourceEnvFile KeySource = "env_file_path"
	KeySourceDefault KeySource = "default_key_file"
	KeySourceNone    KeySource = "not_loaded"
)

// HMACKeyMetadata contains metadata about the current HMAC key.
type HMACKeyMetadata struct {
	Source      KeySource `json:"source"`
	KeyPath     string    `json:"key_path,omitempty"`
	KeySize     int       `json:"key_size"`
	LoadedAt    time.Time `json:"loaded_at"`
	RotatedAt   time.Time `json:"rotated_at,omitempty"`
	Fingerprint string    `json:"fingerprint"` // First 8 chars of hash for identification
}

// HMACKeyManager manages HMAC keys for audit log integrity.
// Implements NIST 800-53 AU-9 with secure key loading and rotation support.
type HMACKeyManager struct {
	auditDir string
	key      []byte
	metadata *HMACKeyMetadata
	mu       sync.RWMutex
}

// NewHMACKeyManager creates a new HMAC key manager.
func NewHMACKeyManager(auditDir string) *HMACKeyManager {
	return &HMACKeyManager{
		auditDir: auditDir,
	}
}

// =============================================================================
// KEY LOADING
// =============================================================================

const (
	// HMACKeyEnvVar is the environment variable for the HMAC key (hex-encoded).
	HMACKeyEnvVar = "RIGRUN_AUDIT_HMAC_KEY"

	// HMACKeyFileEnvVar is the environment variable pointing to a key file.
	HMACKeyFileEnvVar = "RIGRUN_AUDIT_HMAC_KEY_FILE"

	// DefaultKeyFileName is the default key file name.
	DefaultKeyFileName = ".audit_hmac_key"

	// KeySize is the HMAC key size in bytes (256 bits).
	KeySize = 32
)

// LoadKey loads the HMAC key from configured sources.
// Priority: 1) Environment variable, 2) Env file path, 3) Default key file
// Returns the key, source, and any error.
// CRITICAL: Does NOT auto-generate keys - fails if no key is found.
func (m *HMACKeyManager) LoadKey() ([]byte, KeySource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Priority 1: Direct environment variable (hex-encoded key)
	if keyHex := os.Getenv(HMACKeyEnvVar); keyHex != "" {
		key, err := hex.DecodeString(keyHex)
		if err != nil {
			return nil, KeySourceNone, fmt.Errorf("AU-9: invalid HMAC key in %s: %w", HMACKeyEnvVar, err)
		}
		if len(key) != KeySize {
			return nil, KeySourceNone, fmt.Errorf("AU-9: HMAC key must be %d bytes, got %d", KeySize, len(key))
		}
		m.key = key
		m.metadata = &HMACKeyMetadata{
			Source:      KeySourceEnvVar,
			KeySize:     len(key),
			LoadedAt:    time.Now(),
			Fingerprint: hex.EncodeToString(key[:4]),
		}
		return key, KeySourceEnvVar, nil
	}

	// Priority 2: Environment variable pointing to key file
	if keyPath := os.Getenv(HMACKeyFileEnvVar); keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, KeySourceNone, fmt.Errorf("AU-9: failed to read HMAC key file %s: %w", keyPath, err)
		}
		if len(key) != KeySize {
			return nil, KeySourceNone, fmt.Errorf("AU-9: HMAC key file must be %d bytes, got %d", KeySize, len(key))
		}
		m.key = key
		m.metadata = &HMACKeyMetadata{
			Source:      KeySourceEnvFile,
			KeyPath:     keyPath,
			KeySize:     len(key),
			LoadedAt:    time.Now(),
			Fingerprint: hex.EncodeToString(key[:4]),
		}
		return key, KeySourceEnvFile, nil
	}

	// Priority 3: Default key file in audit directory
	defaultKeyPath := filepath.Join(m.auditDir, DefaultKeyFileName)
	key, err := os.ReadFile(defaultKeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// AU-9 CRITICAL: Do NOT auto-generate keys
			// The key MUST be explicitly configured for security
			return nil, KeySourceNone, fmt.Errorf(
				"AU-9: No HMAC key configured. Please set one of:\n"+
					"  1. %s environment variable (hex-encoded 32-byte key)\n"+
					"  2. %s environment variable pointing to key file\n"+
					"  3. Create key file at: %s\n"+
					"Generate a key with: openssl rand -out %s 32",
				HMACKeyEnvVar, HMACKeyFileEnvVar, defaultKeyPath, defaultKeyPath)
		}
		return nil, KeySourceNone, fmt.Errorf("AU-9: failed to read default HMAC key file: %w", err)
	}
	if len(key) != KeySize {
		return nil, KeySourceNone, fmt.Errorf("AU-9: default HMAC key file must be %d bytes, got %d", KeySize, len(key))
	}
	m.key = key
	m.metadata = &HMACKeyMetadata{
		Source:      KeySourceDefault,
		KeyPath:     defaultKeyPath,
		KeySize:     len(key),
		LoadedAt:    time.Now(),
		Fingerprint: hex.EncodeToString(key[:4]),
	}
	return key, KeySourceDefault, nil
}

// GetCurrentKey returns the currently loaded key.
// Returns nil if no key is loaded.
func (m *HMACKeyManager) GetCurrentKey() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.key
}

// GetKeyMetadata returns metadata about the current key.
// Returns nil if no key is loaded.
func (m *HMACKeyManager) GetKeyMetadata() *HMACKeyMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.metadata == nil {
		return nil
	}
	// Return a copy to prevent modification
	copy := *m.metadata
	return &copy
}

// Close cleans up HMACKeyManager resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (m *HMACKeyManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.key != nil {
		zeroBytes(m.key)
		m.key = nil
	}
}

// zeroBytes securely zeros sensitive byte slices to prevent memory disclosure.
// SECURITY: Zero key material to prevent memory disclosure via crash dumps.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// =============================================================================
// KEY ROTATION
// =============================================================================

// RotationResult contains the result of a key rotation operation.
type RotationResult struct {
	Success          bool      `json:"success"`
	OldKeyPath       string    `json:"old_key_path,omitempty"`
	NewKeyPath       string    `json:"new_key_path"`
	RotatedAt        time.Time `json:"rotated_at"`
	EntriesResigned  int       `json:"entries_resigned,omitempty"`
	OldKeyFingerprint string   `json:"old_key_fingerprint"`
	NewKeyFingerprint string   `json:"new_key_fingerprint"`
}

// RotateKey generates a new HMAC key and optionally re-signs existing entries.
// The protector parameter is used to re-sign entries if resignEntries is true.
// The old key is backed up before rotation.
func (m *HMACKeyManager) RotateKey(protector *Protector, resignEntries bool) (*RotationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &RotationResult{
		RotatedAt: time.Now(),
	}

	// Generate new key
	newKey := make([]byte, KeySize)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("AU-9: failed to generate new HMAC key: %w", err)
	}

	// Record old key fingerprint
	if m.key != nil {
		result.OldKeyFingerprint = hex.EncodeToString(m.key[:4])
	}
	result.NewKeyFingerprint = hex.EncodeToString(newKey[:4])

	// Backup old key if it exists
	defaultKeyPath := filepath.Join(m.auditDir, DefaultKeyFileName)
	if _, err := os.Stat(defaultKeyPath); err == nil {
		backupPath := filepath.Join(m.auditDir, fmt.Sprintf("%s.%s.bak",
			DefaultKeyFileName, time.Now().Format("20060102_150405")))
		if err := os.Rename(defaultKeyPath, backupPath); err != nil {
			return nil, fmt.Errorf("AU-9: failed to backup old key: %w", err)
		}
		result.OldKeyPath = backupPath
	}

	// Write new key
	if err := os.WriteFile(defaultKeyPath, newKey, 0600); err != nil {
		return nil, fmt.Errorf("AU-9: failed to write new key: %w", err)
	}
	result.NewKeyPath = defaultKeyPath

	// Update internal state
	oldKey := m.key
	m.key = newKey
	m.metadata = &HMACKeyMetadata{
		Source:      KeySourceDefault,
		KeyPath:     defaultKeyPath,
		KeySize:     len(newKey),
		LoadedAt:    time.Now(),
		RotatedAt:   time.Now(),
		Fingerprint: result.NewKeyFingerprint,
	}

	// Re-sign existing entries if requested
	if resignEntries && protector != nil && oldKey != nil {
		count, err := m.resignEntries(protector, oldKey, newKey)
		if err != nil {
			// Log but don't fail - the key is already rotated
			fmt.Fprintf(os.Stderr, "[AU-9 WARN] Failed to re-sign entries: %v\n", err)
		} else {
			result.EntriesResigned = count
		}
	}

	// SECURITY: Zero old key material to prevent memory disclosure
	if oldKey != nil {
		zeroBytes(oldKey)
	}

	result.Success = true
	return result, nil
}

// resignEntries re-signs all chain entries with the new key.
// This is called during key rotation when resignEntries is true.
func (m *HMACKeyManager) resignEntries(protector *Protector, oldKey, newKey []byte) (int, error) {
	protector.mu.Lock()
	defer protector.mu.Unlock()

	// Temporarily swap in new key for hashing
	oldProtectorKey := protector.hmacKey
	protector.hmacKey = newKey

	// Re-compute all chain hashes
	for i := range protector.chain {
		entry := &protector.chain[i]

		// Get previous hash (for entries after first)
		previousHash := ""
		if i > 0 {
			previousHash = protector.chain[i-1].ChainHash
		}
		entry.PreviousHash = previousHash

		// Re-compute chain hash
		tempEntry := *entry
		tempEntry.ChainHash = ""
		chainData, err := json.Marshal(tempEntry)
		if err != nil {
			protector.hmacKey = oldProtectorKey
			return i, fmt.Errorf("failed to marshal entry %d: %w", i, err)
		}
		entry.ChainHash = protector.computeHash(chainData)
	}

	// Save the re-signed chain
	if err := protector.saveChainLocked(); err != nil {
		protector.hmacKey = oldProtectorKey
		return 0, fmt.Errorf("failed to save re-signed chain: %w", err)
	}

	return len(protector.chain), nil
}

// =============================================================================
// KEY GENERATION UTILITY
// =============================================================================

// GenerateAuditHMACKey generates a new HMAC key and saves it to the specified path.
// This is a utility function for initial key setup.
func GenerateAuditHMACKey(keyPath string) error {
	// Generate random key
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	// SECURITY: Zero key material to prevent memory disclosure
	defer zeroBytes(key)

	// Ensure directory exists
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write key with restrictive permissions
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return nil
}
