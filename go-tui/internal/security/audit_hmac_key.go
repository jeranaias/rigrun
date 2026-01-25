// audit_hmac_key.go - NIST 800-53 AU-9: Audit HMAC Key Management
//
// Implements secure key management for audit log HMAC signing with support
// for multiple key sources (environment, file) and key rotation.
//
// Key Source Priority:
// 1. Environment variable (RIGRUN_AUDIT_HMAC_KEY) - preferred for production
// 2. File-based key with strict permissions - acceptable with warning
// 3. FAIL - no hardcoded/deterministic fallback keys
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// AuditHMACKeyEnvVar is the environment variable name for the HMAC key.
const AuditHMACKeyEnvVar = "RIGRUN_AUDIT_HMAC_KEY"

// AuditHMACKeyFileEnvVar is the environment variable for key file path.
const AuditHMACKeyFileEnvVar = "RIGRUN_AUDIT_HMAC_KEY_FILE"

// MinHMACKeyLength is the minimum key length in bytes (256 bits).
const MinHMACKeyLength = 32

// HMACKeyFileName is the default key file name.
const HMACKeyFileName = ".audit_hmac_key"

// =============================================================================
// KEY SOURCE TYPE
// =============================================================================

// KeySource identifies where the HMAC key was loaded from.
type KeySource string

const (
	// KeySourceEnv indicates key was loaded from environment variable.
	KeySourceEnv KeySource = "environment"
	// KeySourceFile indicates key was loaded from a file.
	KeySourceFile KeySource = "file"
	// KeySourceNone indicates no key was found.
	KeySourceNone KeySource = "none"
)

// =============================================================================
// ERRORS
// =============================================================================

// ErrNoHMACKeyConfigured is returned when no HMAC key is available.
var ErrNoHMACKeyConfigured = fmt.Errorf("no audit HMAC key configured - set %s environment variable or provide key file", AuditHMACKeyEnvVar)

// ErrInvalidHMACKey is returned when the key is invalid (too short, malformed).
var ErrInvalidHMACKey = fmt.Errorf("invalid HMAC key: must be at least %d bytes", MinHMACKeyLength)

// ErrKeyFilePermissions is returned when key file has insecure permissions.
var ErrKeyFilePermissions = fmt.Errorf("key file has insecure permissions - must be 0600 or more restrictive")

// =============================================================================
// KEY METADATA
// =============================================================================

// HMACKeyMetadata stores information about a key for rotation tracking.
type HMACKeyMetadata struct {
	KeyID       string    `json:"key_id"`                 // Unique identifier for the key
	CreatedAt   time.Time `json:"created_at"`             // When key was generated/loaded
	Source      KeySource `json:"source"`                 // Where key came from
	Fingerprint string    `json:"fingerprint"`            // First 8 chars of SHA-256 hash for identification
	Rotated     bool      `json:"rotated"`                // Whether this key has been rotated out
	RotatedAt   time.Time `json:"rotated_at,omitempty"`   // When key was rotated
}

// =============================================================================
// HMAC KEY MANAGER
// =============================================================================

// AuditHMACKeyManager manages HMAC keys for audit log signing.
// Supports multiple key sources and key rotation per NIST 800-53 AU-9.
type AuditHMACKeyManager struct {
	mu              sync.RWMutex
	currentKey      []byte           // Current active key
	currentMetadata *HMACKeyMetadata // Metadata for current key
	previousKeys    [][]byte         // Previous keys for verification during rotation
	keyDir          string           // Directory for key storage
	metadataFile    string           // Path to key metadata file
}

// NewAuditHMACKeyManager creates a new HMAC key manager.
// The keyDir is where key files and metadata are stored.
func NewAuditHMACKeyManager(keyDir string) *AuditHMACKeyManager {
	return &AuditHMACKeyManager{
		keyDir:       keyDir,
		metadataFile: filepath.Join(keyDir, ".audit_key_metadata.json"),
		previousKeys: make([][]byte, 0),
	}
}

// LoadKey loads the HMAC key from available sources in priority order:
// 1. Environment variable (RIGRUN_AUDIT_HMAC_KEY)
// 2. File specified by RIGRUN_AUDIT_HMAC_KEY_FILE
// 3. Default file location in keyDir
// Returns error if no key is configured (no fallback generation).
func (m *AuditHMACKeyManager) LoadKey() ([]byte, KeySource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Priority 1: Environment variable (preferred for production/KMS integration)
	// SECURITY: If the env var is set but invalid, we must fail immediately
	// rather than falling through to file sources. This prevents silent
	// acceptance of weak keys.
	if key, err := m.loadFromEnv(); err == nil {
		m.currentKey = key
		m.currentMetadata = &HMACKeyMetadata{
			KeyID:       generateKeyID(),
			CreatedAt:   time.Now(),
			Source:      KeySourceEnv,
			Fingerprint: computeKeyFingerprint(key),
		}
		return key, KeySourceEnv, nil
	} else if os.Getenv(AuditHMACKeyEnvVar) != "" {
		// SECURITY: Env var was set but invalid - fail immediately with clear error
		return nil, KeySourceNone, err
	}

	// Priority 2: File specified by environment variable
	if keyFilePath := os.Getenv(AuditHMACKeyFileEnvVar); keyFilePath != "" {
		if key, err := m.loadFromFile(keyFilePath); err == nil {
			m.logFileKeyWarning(keyFilePath)
			m.currentKey = key
			m.currentMetadata = &HMACKeyMetadata{
				KeyID:       generateKeyID(),
				CreatedAt:   time.Now(),
				Source:      KeySourceFile,
				Fingerprint: computeKeyFingerprint(key),
			}
			return key, KeySourceFile, nil
		} else {
			// File was specified but couldn't be loaded - this is an error
			return nil, KeySourceNone, fmt.Errorf("failed to load key from %s: %w", keyFilePath, err)
		}
	}

	// Priority 3: Default file location
	defaultKeyFile := filepath.Join(m.keyDir, HMACKeyFileName)
	if key, err := m.loadFromFile(defaultKeyFile); err == nil {
		m.logFileKeyWarning(defaultKeyFile)
		m.currentKey = key
		m.currentMetadata = &HMACKeyMetadata{
			KeyID:       generateKeyID(),
			CreatedAt:   time.Now(),
			Source:      KeySourceFile,
			Fingerprint: computeKeyFingerprint(key),
		}
		return key, KeySourceFile, nil
	}

	// CRITICAL: No fallback key generation - fail if no key configured
	// This is required for NIST 800-53 AU-9 compliance
	return nil, KeySourceNone, ErrNoHMACKeyConfigured
}

// loadFromEnv loads the key from the environment variable.
func (m *AuditHMACKeyManager) loadFromEnv() ([]byte, error) {
	keyStr := os.Getenv(AuditHMACKeyEnvVar)
	if keyStr == "" {
		return nil, fmt.Errorf("environment variable %s not set", AuditHMACKeyEnvVar)
	}

	// SECURITY: ONLY accept hex-encoded keys to prevent weak password usage
	// Reject raw ASCII bytes that could be weak passwords (e.g., "mypassword")
	key, err := hex.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("HMAC key must be hex-encoded (64+ hex characters for 256+ bits): %w", err)
	}

	// Validate minimum key length (256 bits = 32 bytes)
	if len(key) < MinHMACKeyLength {
		return nil, fmt.Errorf("HMAC key must be at least %d bytes (256 bits), got %d bytes", MinHMACKeyLength, len(key))
	}

	return key, nil
}

// loadFromFile loads the key from a file with permissions check.
func (m *AuditHMACKeyManager) loadFromFile(path string) ([]byte, error) {
	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("key file not found: %w", err)
	}

	// Check permissions (should be 0600 or more restrictive)
	if err := m.checkFilePermissions(info, path); err != nil {
		return nil, err
	}

	// Read key
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Validate key length
	if len(key) < MinHMACKeyLength {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidHMACKey, len(key))
	}

	return key, nil
}

// checkFilePermissions verifies the key file has secure permissions.
// SECURITY: ACL verification is mandatory, not optional
func (m *AuditHMACKeyManager) checkFilePermissions(info os.FileInfo, path string) error {
	// On Windows, use ACL verification (implemented in audit_hmac_key_windows.go)
	if runtime.GOOS == "windows" {
		// SECURITY: ACL verification is mandatory on Windows
		if err := verifyWindowsACL(path); err != nil {
			// SECURITY: Do NOT proceed with insecure permissions
			return fmt.Errorf("key file has insecure permissions: %w", err)
		}
		return nil
	}

	// Unix permissions check
	mode := info.Mode().Perm()
	// Key file should be 0600 (owner read/write only) or 0400 (owner read only)
	// Any group or other permissions are insecure
	if mode&0077 != 0 {
		return fmt.Errorf("%w: file %s has mode %o, should be 0600 or 0400", ErrKeyFilePermissions, path, mode)
	}

	return nil
}

// logFileKeyWarning logs a warning when using file-based keys.
func (m *AuditHMACKeyManager) logFileKeyWarning(path string) {
	fmt.Fprintf(os.Stderr, "[AU-9 WARN] Using file-based HMAC key from %s\n", path)
	fmt.Fprintf(os.Stderr, "[AU-9 WARN] For production, prefer environment variable %s or KMS integration\n", AuditHMACKeyEnvVar)
}

// =============================================================================
// KEY ROTATION
// =============================================================================

// RotationResult contains the results of a key rotation operation.
type RotationResult struct {
	OldKeyFingerprint string    `json:"old_key_fingerprint"`
	NewKeyFingerprint string    `json:"new_key_fingerprint"`
	RotatedAt         time.Time `json:"rotated_at"`
	EntriesResigned   int       `json:"entries_resigned"`
	Errors            []string  `json:"errors,omitempty"`
}

// RotateKey generates a new key and optionally re-signs existing entries.
// The old key is preserved to verify existing signatures during transition.
// Set resignEntries to true to re-sign all existing audit chain entries.
func (m *AuditHMACKeyManager) RotateKey(protector *AuditProtector, resignEntries bool) (*RotationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentKey == nil {
		return nil, fmt.Errorf("no current key to rotate from")
	}

	result := &RotationResult{
		OldKeyFingerprint: computeKeyFingerprint(m.currentKey),
		RotatedAt:         time.Now(),
		Errors:            make([]string, 0),
	}

	// Generate new key
	newKey := make([]byte, MinHMACKeyLength)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return nil, fmt.Errorf("failed to generate new key: %w", err)
	}

	result.NewKeyFingerprint = computeKeyFingerprint(newKey)

	// Preserve old key for verification
	m.previousKeys = append(m.previousKeys, m.currentKey)

	// Mark old key as rotated in metadata
	if m.currentMetadata != nil {
		m.currentMetadata.Rotated = true
		m.currentMetadata.RotatedAt = time.Now()
	}

	// Update to new key
	oldKey := m.currentKey
	m.currentKey = newKey
	m.currentMetadata = &HMACKeyMetadata{
		KeyID:       generateKeyID(),
		CreatedAt:   time.Now(),
		Source:      KeySourceFile, // Rotated keys are always file-based
		Fingerprint: result.NewKeyFingerprint,
	}

	// Re-sign existing entries if requested
	if resignEntries && protector != nil {
		resigned, errs := m.resignChainEntries(protector, oldKey, newKey)
		result.EntriesResigned = resigned
		result.Errors = append(result.Errors, errs...)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	// Save new key to file
	keyFile := filepath.Join(m.keyDir, HMACKeyFileName)
	if err := util.AtomicWriteFile(keyFile, newKey, 0600); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to save new key: %v", err))
	}

	// Save metadata
	if err := m.saveMetadata(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to save metadata: %v", err))
	}

	// Log the rotation event
	AuditLogEvent("", "AUDIT_KEY_ROTATED", map[string]string{
		"old_fingerprint":  result.OldKeyFingerprint,
		"new_fingerprint":  result.NewKeyFingerprint,
		"entries_resigned": fmt.Sprintf("%d", result.EntriesResigned),
	})

	return result, nil
}

// resignChainEntries re-signs all chain entries with the new key.
func (m *AuditHMACKeyManager) resignChainEntries(protector *AuditProtector, oldKey, newKey []byte) (int, []string) {
	protector.mu.Lock()
	defer protector.mu.Unlock()

	resigned := 0
	errors := make([]string, 0)

	// Re-compute all chain hashes with new key
	for i := range protector.chain {
		entry := &protector.chain[i]

		// Store original hash for audit trail
		originalHash := entry.ChainHash

		// Recompute chain hash with new key
		entry.ChainHash = ""
		chainData, err := json.Marshal(entry)
		if err != nil {
			errors = append(errors, fmt.Sprintf("entry %d: failed to marshal: %v", i, err))
			continue
		}

		// Compute new hash
		entry.ChainHash = computeHMACWithKey(chainData, newKey)

		// Update previous hash for next entry
		if i+1 < len(protector.chain) {
			protector.chain[i+1].PreviousHash = entry.ChainHash
		}

		resigned++

		// Log each re-signing for audit trail (truncate hashes for log)
		origTrunc := originalHash
		if len(origTrunc) > 16 {
			origTrunc = origTrunc[:16] + "..."
		}
		newTrunc := entry.ChainHash
		if len(newTrunc) > 16 {
			newTrunc = newTrunc[:16] + "..."
		}
		AuditLogEvent("", "AUDIT_ENTRY_RESIGNED", map[string]string{
			"entry_index":   fmt.Sprintf("%d", i),
			"original_hash": origTrunc,
			"new_hash":      newTrunc,
		})
	}

	// Save the updated chain
	if err := protector.saveChain(); err != nil {
		errors = append(errors, fmt.Sprintf("failed to save chain: %v", err))
	}

	return resigned, errors
}

// GetCurrentKey returns the current active key.
func (m *AuditHMACKeyManager) GetCurrentKey() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentKey
}

// Close cleans up AuditHMACKeyManager resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (m *AuditHMACKeyManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Zero current key
	if m.currentKey != nil {
		ZeroBytes(m.currentKey)
		m.currentKey = nil
	}
	// Zero all previous keys
	for i := range m.previousKeys {
		if m.previousKeys[i] != nil {
			ZeroBytes(m.previousKeys[i])
		}
	}
	m.previousKeys = nil
}

// ZeroKey zeros the current key material without closing the manager.
// SECURITY: Use this when you want to clear key material but continue using the manager.
func (m *AuditHMACKeyManager) ZeroKey() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentKey != nil {
		ZeroBytes(m.currentKey)
		m.currentKey = nil
	}
}

// GetKeyMetadata returns metadata about the current key.
func (m *AuditHMACKeyManager) GetKeyMetadata() *HMACKeyMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentMetadata == nil {
		return nil
	}
	// Return a copy
	metaCopy := *m.currentMetadata
	return &metaCopy
}

// GetPreviousKeys returns previous keys for signature verification.
func (m *AuditHMACKeyManager) GetPreviousKeys() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return copies
	result := make([][]byte, len(m.previousKeys))
	for i, k := range m.previousKeys {
		keyCopy := make([]byte, len(k))
		copy(keyCopy, k)
		result[i] = keyCopy
	}
	return result
}

// saveMetadata saves key metadata to file.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (m *AuditHMACKeyManager) saveMetadata() error {
	if m.currentMetadata == nil {
		return nil
	}

	data, err := json.MarshalIndent(m.currentMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFile(m.metadataFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// LoadMetadata loads key metadata from file.
func (m *AuditHMACKeyManager) LoadMetadata() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No metadata file is OK
		}
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata HMACKeyMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	m.currentMetadata = &metadata
	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateKeyID generates a unique key identifier.
func generateKeyID() string {
	id := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("key_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("key_%s", hex.EncodeToString(id))
}

// computeKeyFingerprint computes a fingerprint of the key for identification.
// SECURITY: Fingerprint uses hash, not raw key material
func computeKeyFingerprint(key []byte) string {
	// Use SHA-256 hash of the key to derive fingerprint
	// This prevents exposing any raw key bytes while still allowing identification
	h := sha256.Sum256(key)
	// Return first 8 hex chars of hash (not raw key!)
	return hex.EncodeToString(h[:4])
}

// computeHMACWithKey computes HMAC-SHA256 with the given key.
func computeHMACWithKey(data, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// =============================================================================
// GLOBAL KEY MANAGER
// =============================================================================

var (
	globalKeyManager     *AuditHMACKeyManager
	globalKeyManagerOnce sync.Once
	globalKeyManagerMu   sync.RWMutex
)

// GlobalAuditHMACKeyManager returns the global HMAC key manager instance.
func GlobalAuditHMACKeyManager() *AuditHMACKeyManager {
	globalKeyManagerOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		keyDir := filepath.Join(home, ".rigrun")
		globalKeyManager = NewAuditHMACKeyManager(keyDir)
	})
	return globalKeyManager
}

// SetGlobalAuditHMACKeyManager sets the global key manager instance.
func SetGlobalAuditHMACKeyManager(manager *AuditHMACKeyManager) {
	globalKeyManagerMu.Lock()
	defer globalKeyManagerMu.Unlock()
	globalKeyManager = manager
}

// =============================================================================
// KEY GENERATION UTILITY
// =============================================================================

// GenerateAuditHMACKey generates a new random HMAC key.
// This should be used for initial key setup, not called automatically.
func GenerateAuditHMACKey() ([]byte, error) {
	key := make([]byte, MinHMACKeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateAndSaveAuditHMACKey generates a new key and saves it to the specified path.
// This is a utility function for initial setup.
func GenerateAndSaveAuditHMACKey(path string) error {
	key, err := GenerateAuditHMACKey()
	if err != nil {
		return err
	}
	// SECURITY: Zero key material to prevent memory disclosure
	defer ZeroBytes(key)

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(path, key, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[AU-9 INFO] Generated new HMAC key at %s\n", path)
	fmt.Fprintf(os.Stderr, "[AU-9 INFO] Key fingerprint: %s\n", computeKeyFingerprint(key))
	fmt.Fprintf(os.Stderr, "[AU-9 INFO] For production, set %s environment variable instead\n", AuditHMACKeyEnvVar)

	return nil
}
