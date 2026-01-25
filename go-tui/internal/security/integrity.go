// Package security provides integrity verification for NIST 800-53 SI-7 compliance.
//
// Implements Software, Firmware, and Information Integrity controls including:
// - Binary integrity verification (self-checksum)
// - Configuration file integrity verification
// - Audit log integrity verification
// - Baseline checksum management
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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// DefaultChecksumFile is the default path for the checksums file.
const DefaultChecksumFile = "checksums.json"

// ChecksumAlgorithm is the algorithm used for checksums.
const ChecksumAlgorithm = "SHA-256"

// BaselineKeyFile is the filename for the baseline HMAC key.
const BaselineKeyFile = ".baseline_key"

// BaselineSignatureSize is the size of HMAC-SHA256 signature in bytes.
const BaselineSignatureSize = 32

// =============================================================================
// ERRORS
// =============================================================================

var (
	// ErrBaselineUnsigned indicates the baseline file lacks a valid signature.
	ErrBaselineUnsigned = errors.New("baseline file is unsigned or signature is missing")

	// ErrBaselineTampered indicates the baseline signature verification failed.
	ErrBaselineTampered = errors.New("baseline signature verification failed - possible tampering detected")

	// ErrBaselineCorrupted indicates the baseline file is corrupted.
	ErrBaselineCorrupted = errors.New("baseline file is corrupted")
)

// =============================================================================
// CHECKSUM RECORD
// =============================================================================

// ChecksumRecord represents a single file's checksum information.
type ChecksumRecord struct {
	Path       string    `json:"path"`
	Algorithm  string    `json:"algorithm"`
	Checksum   string    `json:"checksum"`
	Size       int64     `json:"size"`
	VerifiedAt time.Time `json:"verified_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChecksumBaseline contains all tracked file checksums.
type ChecksumBaseline struct {
	Version   string                    `json:"version"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Records   map[string]ChecksumRecord `json:"records"`
}

// SignedBaseline wraps ChecksumBaseline with HMAC signature for AU-9 compliance.
// The signature protects against tampering and ensures integrity verification.
type SignedBaseline struct {
	Baseline  ChecksumBaseline `json:"baseline"`
	Signature string           `json:"signature"` // HMAC-SHA256 signature (hex encoded)
	SignedAt  time.Time        `json:"signed_at"`
}

// IntegrityResult represents the result of an integrity check.
type IntegrityResult struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
}

// =============================================================================
// INTEGRITY MANAGER
// =============================================================================

// IntegrityManager handles SI-7 integrity verification operations.
type IntegrityManager struct {
	checksumFile string
	baseline     *ChecksumBaseline
	hmacKey      []byte // HMAC key for signing baselines (AU-9 compliance)
	mu           sync.RWMutex
}

// NewIntegrityManager creates a new IntegrityManager with the specified checksum file path.
// If path is empty, uses the default path (~/.rigrun/checksums.json).
// AU-9 COMPLIANCE: Now requires HMAC key for baseline signing and verification.
func NewIntegrityManager(checksumFile string) (*IntegrityManager, error) {
	if checksumFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		checksumFile = filepath.Join(home, ".rigrun", DefaultChecksumFile)
	}

	// AU-9: Load or generate HMAC key for baseline signing
	dir := filepath.Dir(checksumFile)
	hmacKey, err := loadOrGenerateBaselineKey(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize baseline signing key: %w", err)
	}

	im := &IntegrityManager{
		checksumFile: checksumFile,
		hmacKey:      hmacKey,
	}

	// Try to load existing baseline with signature verification
	if err := im.loadBaseline(); err != nil {
		// Check if this is a security-relevant error
		if errors.Is(err, ErrBaselineTampered) || errors.Is(err, ErrBaselineUnsigned) {
			// Log security event for tampering detection
			AuditLogEvent("", "BASELINE_TAMPER_DETECTED", map[string]string{
				"path":  checksumFile,
				"error": err.Error(),
			})
			return nil, err
		}

		// Check if file is corrupted (file exists but cannot be parsed)
		if errors.Is(err, ErrBaselineCorrupted) {
			AuditLogEvent("", "BASELINE_CORRUPTED", map[string]string{
				"path":  checksumFile,
				"error": err.Error(),
			})
			return nil, err
		}

		// If file doesn't exist (contains "read" or "no such file"), create new baseline
		// Otherwise, the file exists but has some other issue
		if _, statErr := os.Stat(checksumFile); statErr == nil {
			// File exists but couldn't be loaded - treat as corruption
			return nil, fmt.Errorf("%w: %v", ErrBaselineCorrupted, err)
		}

		// Create a new baseline if file doesn't exist
		im.baseline = &ChecksumBaseline{
			Version:   "1.0",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Records:   make(map[string]ChecksumRecord),
		}
	}

	return im, nil
}

// NewIntegrityManagerUnsigned creates an IntegrityManager that allows unsigned baselines.
// WARNING: This should only be used for migration from older versions.
// Use NewIntegrityManager for production use.
func NewIntegrityManagerUnsigned(checksumFile string) (*IntegrityManager, error) {
	if checksumFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		checksumFile = filepath.Join(home, ".rigrun", DefaultChecksumFile)
	}

	// Load or generate HMAC key
	dir := filepath.Dir(checksumFile)
	hmacKey, err := loadOrGenerateBaselineKey(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize baseline signing key: %w", err)
	}

	im := &IntegrityManager{
		checksumFile: checksumFile,
		hmacKey:      hmacKey,
	}

	// Try to load existing baseline (allow unsigned for migration)
	if err := im.loadBaselineUnsigned(); err != nil {
		im.baseline = &ChecksumBaseline{
			Version:   "1.0",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Records:   make(map[string]ChecksumRecord),
		}
	}

	return im, nil
}

// =============================================================================
// CHECKSUM OPERATIONS
// =============================================================================

// ComputeChecksum computes the SHA-256 checksum of a file.
func (im *IntegrityManager) ComputeChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifyFile verifies a single file against its stored checksum.
func (im *IntegrityManager) VerifyFile(path string) (bool, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if we have a record for this file
	record, exists := im.baseline.Records[absPath]
	if !exists {
		return false, fmt.Errorf("no checksum record for file: %s", absPath)
	}

	// Compute current checksum
	currentChecksum, err := im.ComputeChecksum(absPath)
	if err != nil {
		return false, err
	}

	// Compare
	return currentChecksum == record.Checksum, nil
}

// VerifyAll verifies all files in the baseline and returns results.
func (im *IntegrityManager) VerifyAll() ([]IntegrityResult, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var results []IntegrityResult

	for path, record := range im.baseline.Records {
		result := IntegrityResult{
			Path:     path,
			Expected: record.Checksum,
		}

		// Compute current checksum
		currentChecksum, err := im.ComputeChecksum(path)
		if err != nil {
			result.Error = err.Error()
			result.Valid = false
		} else {
			result.Actual = currentChecksum
			result.Valid = currentChecksum == record.Checksum
		}

		results = append(results, result)
	}

	return results, nil
}

// AddFile adds or updates a file in the baseline.
func (im *IntegrityManager) AddFile(path string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Compute checksum
	checksum, err := im.ComputeChecksum(absPath)
	if err != nil {
		return err
	}

	// Add/update record
	im.baseline.Records[absPath] = ChecksumRecord{
		Path:       absPath,
		Algorithm:  ChecksumAlgorithm,
		Checksum:   checksum,
		Size:       info.Size(),
		VerifiedAt: time.Now(),
		UpdatedAt:  time.Now(),
	}

	im.baseline.UpdatedAt = time.Now()

	return nil
}

// RemoveFile removes a file from the baseline.
func (im *IntegrityManager) RemoveFile(path string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	delete(im.baseline.Records, absPath)
	im.baseline.UpdatedAt = time.Now()

	return nil
}

// =============================================================================
// BINARY VERIFICATION (SI-7)
// =============================================================================

// VerifyBinary performs self-verification of the running binary.
// This is a key SI-7 control for software integrity.
func (im *IntegrityManager) VerifyBinary() (bool, error) {
	// Get the path to the running executable
	execPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	return im.VerifyFile(execPath)
}

// GetBinaryPath returns the path to the running binary.
func GetBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks
	return filepath.EvalSymlinks(execPath)
}

// UpdateBinaryBaseline updates the baseline checksum for the current binary.
func (im *IntegrityManager) UpdateBinaryBaseline() error {
	execPath, err := GetBinaryPath()
	if err != nil {
		return err
	}

	return im.AddFile(execPath)
}

// =============================================================================
// CONFIG VERIFICATION (SI-7)
// =============================================================================

// VerifyConfig verifies the integrity of configuration files.
func (im *IntegrityManager) VerifyConfig() ([]IntegrityResult, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var results []IntegrityResult

	// Get config directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".rigrun")

	// Check common config files
	configFiles := []string{
		filepath.Join(configDir, "config.toml"),
		filepath.Join(configDir, "config.json"),
	}

	for _, path := range configFiles {
		// Skip if file doesn't exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		result := IntegrityResult{
			Path: path,
		}

		// Check if we have a baseline record
		record, exists := im.baseline.Records[path]
		if !exists {
			result.Error = "no baseline checksum (file may be new)"
			result.Valid = false
			results = append(results, result)
			continue
		}

		result.Expected = record.Checksum

		// Compute current checksum
		currentChecksum, err := im.ComputeChecksum(path)
		if err != nil {
			result.Error = err.Error()
			result.Valid = false
		} else {
			result.Actual = currentChecksum
			result.Valid = currentChecksum == record.Checksum
		}

		results = append(results, result)
	}

	return results, nil
}

// UpdateConfigBaseline updates the baseline checksums for config files.
func (im *IntegrityManager) UpdateConfigBaseline() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".rigrun")

	// Add common config files
	configFiles := []string{
		filepath.Join(configDir, "config.toml"),
		filepath.Join(configDir, "config.json"),
	}

	for _, path := range configFiles {
		// Skip if file doesn't exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		if err := im.AddFile(path); err != nil {
			return fmt.Errorf("failed to add %s to baseline: %w", path, err)
		}
	}

	return im.saveBaseline()
}

// =============================================================================
// AUDIT LOG VERIFICATION (SI-7)
// =============================================================================

// VerifyAuditLog verifies the integrity of the audit log.
func (im *IntegrityManager) VerifyAuditLog() (*IntegrityResult, error) {
	auditPath := DefaultAuditPath()

	// Check if audit log exists
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		return &IntegrityResult{
			Path:  auditPath,
			Valid: true, // Empty/missing log is valid
		}, nil
	}

	result := &IntegrityResult{
		Path: auditPath,
	}

	// Check if we have a baseline record
	im.mu.RLock()
	record, exists := im.baseline.Records[auditPath]
	im.mu.RUnlock()

	if !exists {
		// No baseline for audit log - this is expected since it changes frequently
		// Just verify the file is readable
		if _, err := im.ComputeChecksum(auditPath); err != nil {
			result.Error = err.Error()
			result.Valid = false
		} else {
			result.Valid = true
		}
		return result, nil
	}

	result.Expected = record.Checksum

	// Compute current checksum
	currentChecksum, err := im.ComputeChecksum(auditPath)
	if err != nil {
		result.Error = err.Error()
		result.Valid = false
	} else {
		result.Actual = currentChecksum
		result.Valid = currentChecksum == record.Checksum
	}

	return result, nil
}

// =============================================================================
// BASELINE MANAGEMENT
// =============================================================================

// UpdateBaseline updates the entire baseline with current file checksums.
func (im *IntegrityManager) UpdateBaseline() error {
	// Update binary
	if err := im.UpdateBinaryBaseline(); err != nil {
		// Log but don't fail - binary might not be tracked
		fmt.Fprintf(os.Stderr, "Warning: Could not update binary baseline: %v\n", err)
	}

	// Update config files
	if err := im.UpdateConfigBaseline(); err != nil {
		return fmt.Errorf("failed to update config baseline: %w", err)
	}

	return im.saveBaseline()
}

// loadBaseline loads the checksum baseline from disk with signature verification.
// AU-9 COMPLIANCE: Rejects unsigned or tampered baselines.
func (im *IntegrityManager) loadBaseline() error {
	data, err := os.ReadFile(im.checksumFile)
	if err != nil {
		return fmt.Errorf("failed to read checksum file: %w", err)
	}

	// Try to parse as signed baseline first
	var signed SignedBaseline
	if err := json.Unmarshal(data, &signed); err != nil {
		return fmt.Errorf("%w: %v", ErrBaselineCorrupted, err)
	}

	// Check if signature is present
	if signed.Signature == "" {
		return ErrBaselineUnsigned
	}

	// Verify HMAC signature
	if !im.verifyBaselineSignature(&signed) {
		return ErrBaselineTampered
	}

	im.baseline = &signed.Baseline
	return nil
}

// loadBaselineUnsigned loads baseline allowing unsigned files (for migration).
func (im *IntegrityManager) loadBaselineUnsigned() error {
	data, err := os.ReadFile(im.checksumFile)
	if err != nil {
		return fmt.Errorf("failed to read checksum file: %w", err)
	}

	// Try to parse as signed baseline first
	var signed SignedBaseline
	if err := json.Unmarshal(data, &signed); err == nil && signed.Signature != "" {
		// Signed baseline - verify signature
		if !im.verifyBaselineSignature(&signed) {
			return ErrBaselineTampered
		}
		im.baseline = &signed.Baseline
		return nil
	}

	// Try to parse as legacy unsigned baseline
	var baseline ChecksumBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return fmt.Errorf("failed to parse checksum file: %w", err)
	}

	im.baseline = &baseline
	return nil
}

// saveBaseline saves the checksum baseline to disk with HMAC signature.
// AU-9 COMPLIANCE: Uses atomic writes (temp + fsync + rename) and signs the baseline.
func (im *IntegrityManager) saveBaseline() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(im.checksumFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create checksum directory: %w", err)
	}

	// Create signed baseline
	signed := SignedBaseline{
		Baseline: *im.baseline,
		SignedAt: time.Now(),
	}

	// Compute HMAC signature
	signed.Signature = im.computeBaselineSignature(&signed)

	// Marshal signed baseline
	data, err := json.MarshalIndent(signed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline: %w", err)
	}

	// AU-9: Atomic write using temp file + fsync + rename
	tmpFile := im.checksumFile + ".tmp"

	// Write to temporary file
	file, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write data
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to write baseline data: %w", err)
	}

	// Fsync to ensure data is flushed to disk
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to sync baseline file: %w", err)
	}

	// Close file before rename
	if err := file.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, im.checksumFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename baseline file: %w", err)
	}

	return nil
}

// Save persists the current baseline to disk.
func (im *IntegrityManager) Save() error {
	return im.saveBaseline()
}

// GetBaseline returns the current baseline (read-only copy).
func (im *IntegrityManager) GetBaseline() ChecksumBaseline {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Return a copy
	copy := ChecksumBaseline{
		Version:   im.baseline.Version,
		CreatedAt: im.baseline.CreatedAt,
		UpdatedAt: im.baseline.UpdatedAt,
		Records:   make(map[string]ChecksumRecord),
	}

	for k, v := range im.baseline.Records {
		copy.Records[k] = v
	}

	return copy
}

// GetChecksumFile returns the path to the checksum file.
func (im *IntegrityManager) GetChecksumFile() string {
	return im.checksumFile
}

// =============================================================================
// STARTUP VERIFICATION (SI-7)
// =============================================================================

// PerformStartupCheck performs all integrity checks at startup.
// Returns true if all checks pass, false if any fail.
// This implements SI-7 startup verification control.
func (im *IntegrityManager) PerformStartupCheck() (bool, []IntegrityResult) {
	var allResults []IntegrityResult
	allValid := true

	// 1. Verify binary integrity
	binaryValid, err := im.VerifyBinary()
	binaryPath, _ := GetBinaryPath()
	binaryResult := IntegrityResult{
		Path:  binaryPath,
		Valid: binaryValid,
	}
	if err != nil {
		binaryResult.Error = err.Error()
		// Don't fail on missing baseline - just warn
		if err.Error() != fmt.Sprintf("no checksum record for file: %s", binaryPath) {
			binaryResult.Valid = false
			allValid = false
		}
	}
	allResults = append(allResults, binaryResult)

	// 2. Verify config integrity
	configResults, err := im.VerifyConfig()
	if err == nil {
		for _, r := range configResults {
			allResults = append(allResults, r)
			if !r.Valid && r.Error != "no baseline checksum (file may be new)" {
				allValid = false
			}
		}
	}

	return allValid, allResults
}

// =============================================================================
// GLOBAL INTEGRITY MANAGER
// =============================================================================

var (
	globalIntegrityManager     *IntegrityManager
	globalIntegrityManagerOnce sync.Once
	globalIntegrityManagerMu   sync.Mutex
)

// GlobalIntegrityManager returns the global integrity manager instance.
func GlobalIntegrityManager() *IntegrityManager {
	globalIntegrityManagerOnce.Do(func() {
		var err error
		globalIntegrityManager, err = NewIntegrityManager("")
		if err != nil {
			// Create a minimal manager on failure
			globalIntegrityManager = &IntegrityManager{
				baseline: &ChecksumBaseline{
					Records: make(map[string]ChecksumRecord),
				},
			}
		}
	})
	return globalIntegrityManager
}

// SetGlobalIntegrityManager sets the global integrity manager instance.
func SetGlobalIntegrityManager(manager *IntegrityManager) {
	globalIntegrityManagerMu.Lock()
	defer globalIntegrityManagerMu.Unlock()
	globalIntegrityManager = manager
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// GetSystemInfo returns basic system information for integrity reports.
func GetSystemInfo() map[string]string {
	return map[string]string{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"go":       runtime.Version(),
		"cpus":     fmt.Sprintf("%d", runtime.NumCPU()),
		"hostname": getHostname(),
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// =============================================================================
// BASELINE SIGNING (AU-9 COMPLIANCE)
// =============================================================================

// loadOrGenerateBaselineKey loads or generates the HMAC key for baseline signing.
// The key is stored securely with restrictive permissions (0600).
// NOTE: Caller is responsible for zeroing the returned key when no longer needed
// via ZeroBytes(). The key is stored in IntegrityManager.hmacKey and zeroed in Close().
func loadOrGenerateBaselineKey(dir string) ([]byte, error) {
	keyFile := filepath.Join(dir, BaselineKeyFile)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	// Try to load existing key
	data, err := os.ReadFile(keyFile)
	if err == nil && len(data) == BaselineSignatureSize {
		return data, nil
	}

	// Generate new 256-bit key
	key := make([]byte, BaselineSignatureSize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save key with restrictive permissions using atomic write
	tmpFile := keyFile + ".tmp"
	if err := os.WriteFile(tmpFile, key, 0600); err != nil {
		// SECURITY: Zero key material on failure to prevent memory disclosure
		ZeroBytes(key)
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, keyFile); err != nil {
		os.Remove(tmpFile)
		// SECURITY: Zero key material on failure to prevent memory disclosure
		ZeroBytes(key)
		return nil, fmt.Errorf("failed to rename key file: %w", err)
	}

	return key, nil
}

// Close cleans up IntegrityManager resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (im *IntegrityManager) Close() {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.hmacKey != nil {
		ZeroBytes(im.hmacKey)
		im.hmacKey = nil
	}
}

// computeBaselineSignature computes HMAC-SHA256 signature of the baseline.
// The signature covers the baseline content and signing timestamp.
func (im *IntegrityManager) computeBaselineSignature(signed *SignedBaseline) string {
	// Marshal baseline content for signing (without signature field)
	baselineData, err := json.Marshal(signed.Baseline)
	if err != nil {
		return ""
	}

	// Include signing timestamp in signature computation
	signData := append(baselineData, []byte(signed.SignedAt.Format(time.RFC3339Nano))...)

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, im.hmacKey)
	mac.Write(signData)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyBaselineSignature verifies the HMAC signature of a signed baseline.
// Returns true if signature is valid, false otherwise.
func (im *IntegrityManager) verifyBaselineSignature(signed *SignedBaseline) bool {
	// Decode stored signature
	storedSig, err := hex.DecodeString(signed.Signature)
	if err != nil {
		return false
	}

	// Compute expected signature
	expectedSigHex := im.computeBaselineSignature(signed)
	expectedSig, err := hex.DecodeString(expectedSigHex)
	if err != nil {
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal(storedSig, expectedSig)
}

// VerifyBaselineIntegrity verifies that the current baseline file has a valid signature.
// This can be called to check baseline integrity without loading a new manager.
func (im *IntegrityManager) VerifyBaselineIntegrity() error {
	data, err := os.ReadFile(im.checksumFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No baseline file is acceptable
		}
		return fmt.Errorf("failed to read baseline: %w", err)
	}

	var signed SignedBaseline
	if err := json.Unmarshal(data, &signed); err != nil {
		return fmt.Errorf("%w: %v", ErrBaselineCorrupted, err)
	}

	if signed.Signature == "" {
		return ErrBaselineUnsigned
	}

	if !im.verifyBaselineSignature(&signed) {
		return ErrBaselineTampered
	}

	return nil
}

// SignAndSaveBaseline explicitly signs and saves the current baseline.
// This is useful for converting an unsigned baseline to a signed one.
func (im *IntegrityManager) SignAndSaveBaseline() error {
	return im.saveBaseline()
}

// BaselineSignatureStatus returns information about the baseline's signature status.
type BaselineSignatureStatus struct {
	IsSigned   bool      `json:"is_signed"`
	IsValid    bool      `json:"is_valid"`
	SignedAt   time.Time `json:"signed_at,omitempty"`
	Error      string    `json:"error,omitempty"`
	FilePath   string    `json:"file_path"`
	FileExists bool      `json:"file_exists"`
}

// GetBaselineSignatureStatus returns the current signature status of the baseline file.
func (im *IntegrityManager) GetBaselineSignatureStatus() BaselineSignatureStatus {
	status := BaselineSignatureStatus{
		FilePath: im.checksumFile,
	}

	// Check if file exists
	data, err := os.ReadFile(im.checksumFile)
	if err != nil {
		if os.IsNotExist(err) {
			status.FileExists = false
			return status
		}
		status.FileExists = true
		status.Error = err.Error()
		return status
	}
	status.FileExists = true

	// Try to parse as signed baseline
	var signed SignedBaseline
	if err := json.Unmarshal(data, &signed); err != nil {
		status.Error = "failed to parse baseline: " + err.Error()
		return status
	}

	// Check signature presence
	if signed.Signature == "" {
		status.IsSigned = false
		return status
	}

	status.IsSigned = true
	status.SignedAt = signed.SignedAt

	// Verify signature
	if im.verifyBaselineSignature(&signed) {
		status.IsValid = true
	} else {
		status.IsValid = false
		status.Error = "signature verification failed"
	}

	return status
}
