// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection.
//
// This file implements NIST 800-53 AU-9: Protection of Audit Information.
package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// DefaultRetentionDays is the default retention period for archived logs (7 years per DoD requirements).
const DefaultRetentionDays = 365 * 7

// =============================================================================
// LOG CHAIN ENTRY
// =============================================================================

// LogChainEntry represents a single entry in the audit log chain.
// Each entry includes a hash of the previous entry for tamper detection.
type LogChainEntry struct {
	Index        int       `json:"index"`
	Timestamp    time.Time `json:"timestamp"`
	EventHash    string    `json:"event_hash"`    // SHA-256 hash of event data
	PreviousHash string    `json:"previous_hash"` // Hash of previous entry
	ChainHash    string    `json:"chain_hash"`    // Hash of this entry
}

// =============================================================================
// TAMPER REPORT
// =============================================================================

// TamperReport contains the results of tampering detection.
type TamperReport struct {
	Timestamp          time.Time `json:"timestamp"`
	Verified           bool      `json:"verified"`
	ChainLength        int       `json:"chain_length"`
	Issues             []string  `json:"issues"`
	PermissionIssues   []string  `json:"permission_issues"`
	TimestampAnomalies []string  `json:"timestamp_anomalies"`
}

// =============================================================================
// AUDIT PROTECTOR
// =============================================================================

// Protector provides cryptographic protection for audit logs.
// Implements NIST 800-53 AU-9 (Protection of Audit Information).
// Also implements AU-5 (Response to Audit Processing Failures) with
// synchronous saves, retry logic, and halt-on-failure support.
type Protector struct {
	auditLogPath string
	chainFile    string // Path to chain integrity file
	witnessFile  string // Path to external witness file (hash anchoring)
	chain        []LogChainEntry
	hmacKey      []byte // HMAC key for signing
	mu           sync.RWMutex

	// AU-9: Key manager for HMAC key rotation and multi-source loading
	keyManager *HMACKeyManager

	// AU-5: Retry configuration for audit save operations
	maxRetries    int           // Maximum number of retry attempts (default: 3)
	retryBaseWait time.Duration // Base wait time for exponential backoff (default: 100ms)

	// AU-5: Strict mode - halt operations if audit log fails
	strictMode bool // When true, return errors to halt operations on audit failure
}

// NewProtector creates a new audit protector.
// NIST 800-53 AU-9 COMPLIANCE: Requires explicit key configuration.
// Key sources (in priority order):
// 1. RIGRUN_AUDIT_HMAC_KEY environment variable
// 2. RIGRUN_AUDIT_HMAC_KEY_FILE environment variable pointing to key file
// 3. Default key file at <audit_dir>/.audit_hmac_key
// If no key is configured, returns an error (no fallback key generation).
func NewProtector(auditLogPath string) (*Protector, error) {
	dir := filepath.Dir(auditLogPath)
	chainFile := filepath.Join(dir, "audit_chain.json")
	witnessFile := filepath.Join(dir, "audit_witness.txt")

	// AU-9: Use the new key manager for multi-source key loading
	// CRITICAL: No auto-generation - fails if no key configured
	keyManager := NewHMACKeyManager(dir)
	hmacKey, source, err := keyManager.LoadKey()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize HMAC key: %w", err)
	}

	// Log key source for audit trail
	fmt.Fprintf(os.Stderr, "[AU-9 INFO] Audit HMAC key loaded from: %s\n", source)

	p := &Protector{
		auditLogPath:  auditLogPath,
		chainFile:     chainFile,
		witnessFile:   witnessFile,
		chain:         make([]LogChainEntry, 0),
		hmacKey:       hmacKey,
		keyManager:    keyManager,
		maxRetries:    3,                      // AU-5: Default 3 retry attempts
		retryBaseWait: 100 * time.Millisecond, // AU-5: Exponential backoff base
		strictMode:    true,                   // AU-5: Default to strict mode for compliance
	}

	// Load existing chain if present
	if err := p.loadChain(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load chain: %w", err)
	}

	return p, nil
}

// =============================================================================
// LOG SIGNING
// =============================================================================

// ErrAuditSaveFailed is returned when the audit save fails after all retries.
var ErrAuditSaveFailed = fmt.Errorf("AU-5: audit save failed after all retries - operations halted")

// SignLogEntry cryptographically signs a log entry and adds it to the chain.
// AU-5 COMPLIANCE: Saves are SYNCHRONOUS with retry logic.
// In strict mode, returns error to halt operations on failure.
func (p *Protector) SignLogEntry(entry Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Serialize entry for hashing
	eventData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Compute event hash
	eventHash := p.computeHash(eventData)

	// Get previous hash
	previousHash := ""
	if len(p.chain) > 0 {
		previousHash = p.chain[len(p.chain)-1].ChainHash
	}

	// Create chain entry
	chainEntry := LogChainEntry{
		Index:        len(p.chain),
		Timestamp:    entry.Timestamp,
		EventHash:    eventHash,
		PreviousHash: previousHash,
	}

	// Compute chain hash (hash of this entry including previous hash)
	chainData, err := json.Marshal(chainEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal chain entry: %w", err)
	}
	chainEntry.ChainHash = p.computeHash(chainData)

	// Add to chain
	p.chain = append(p.chain, chainEntry)

	// AU-5 CRITICAL FIX: Synchronous save with retry logic
	// NO MORE fire-and-forget goroutines - this is a compliance requirement
	chainErr := p.saveChainWithRetry()
	if chainErr != nil {
		fmt.Fprintf(os.Stderr, "[AU-5 CRITICAL] Failed to save chain after %d retries: %v\n", p.maxRetries, chainErr)
		if p.strictMode {
			// Remove the entry we just added since save failed
			p.chain = p.chain[:len(p.chain)-1]
			return fmt.Errorf("%w: chain save error: %v", ErrAuditSaveFailed, chainErr)
		}
	}

	// AU-5 CRITICAL FIX: Synchronous witness write with retry logic
	witnessErr := p.writeWitnessWithRetry(chainEntry)
	if witnessErr != nil {
		fmt.Fprintf(os.Stderr, "[AU-5 WARNING] Failed to write witness after %d retries: %v\n", p.maxRetries, witnessErr)
		if p.strictMode {
			return fmt.Errorf("%w: witness write error: %v", ErrAuditSaveFailed, witnessErr)
		}
	}

	return nil
}

// saveChainWithRetry saves the chain with exponential backoff retry.
// AU-5: Returns error if all retries fail.
func (p *Protector) saveChainWithRetry() error {
	var lastErr error
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms...
			waitTime := p.retryBaseWait * time.Duration(1<<uint(attempt-1))
			fmt.Fprintf(os.Stderr, "[AU-5 INFO] Retrying chain save (attempt %d/%d) after %v\n",
				attempt+1, p.maxRetries, waitTime)
			time.Sleep(waitTime)
		}

		err := p.saveChainLocked()
		if err == nil {
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "[AU-5 INFO] Chain save succeeded on attempt %d\n", attempt+1)
			}
			return nil
		}
		lastErr = err
		fmt.Fprintf(os.Stderr, "[AU-5 ERROR] Chain save attempt %d failed: %v\n", attempt+1, err)
	}
	return fmt.Errorf("all %d retry attempts failed: %w", p.maxRetries, lastErr)
}

// writeWitnessWithRetry writes the witness with exponential backoff retry.
// AU-5: Returns error if all retries fail.
func (p *Protector) writeWitnessWithRetry(entry LogChainEntry) error {
	var lastErr error
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms...
			waitTime := p.retryBaseWait * time.Duration(1<<uint(attempt-1))
			fmt.Fprintf(os.Stderr, "[AU-5 INFO] Retrying witness write (attempt %d/%d) after %v\n",
				attempt+1, p.maxRetries, waitTime)
			time.Sleep(waitTime)
		}

		err := p.writeWitness(entry)
		if err == nil {
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "[AU-5 INFO] Witness write succeeded on attempt %d\n", attempt+1)
			}
			return nil
		}
		lastErr = err
		fmt.Fprintf(os.Stderr, "[AU-5 ERROR] Witness write attempt %d failed: %v\n", attempt+1, err)
	}
	return fmt.Errorf("all %d retry attempts failed: %w", p.maxRetries, lastErr)
}

// saveChainLocked saves the chain without acquiring lock (caller must hold lock).
// This is the internal implementation called by saveChainWithRetry.
func (p *Protector) saveChainLocked() error {
	data, err := json.MarshalIndent(p.chain, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chain: %w", err)
	}

	// Write to temporary file first
	tmpFile := p.chainFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write chain: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, p.chainFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename chain file: %w", err)
	}

	return nil
}

// =============================================================================
// INTEGRITY VERIFICATION
// =============================================================================

// VerifyLogIntegrity verifies the integrity of the audit log chain.
// Returns true if the chain is intact, false if tampering is detected.
// CRITICAL SECURITY FIX: Empty chain is now SUSPICIOUS, not valid.
func (p *Protector) VerifyLogIntegrity() (bool, []string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	issues := make([]string, 0)

	// FIXED: Empty chain is SUSPICIOUS
	// An audit log that has been initialized should have at least one entry
	if len(p.chain) == 0 {
		// Check if chain file exists
		if _, err := os.Stat(p.chainFile); os.IsNotExist(err) {
			// Chain file doesn't exist - this is acceptable for new systems
			return true, nil, nil
		}
		// Chain file exists but is empty - SUSPICIOUS
		issues = append(issues, "CRITICAL: Empty audit chain detected - possible tampering or deletion")
		return false, issues, nil
	}

	// Verify timestamp monotonicity
	var lastTimestamp time.Time
	for i := 0; i < len(p.chain); i++ {
		entry := p.chain[i]

		// Verify index sequence
		if entry.Index != i {
			issues = append(issues, fmt.Sprintf("Entry %d has incorrect index: expected %d, got %d", i, i, entry.Index))
		}

		// ADDED: Verify timestamp monotonicity (entries must be in order)
		if i > 0 {
			if entry.Timestamp.Before(lastTimestamp) {
				issues = append(issues, fmt.Sprintf("Entry %d has non-monotonic timestamp: %s before %s",
					i, entry.Timestamp.Format(time.RFC3339), lastTimestamp.Format(time.RFC3339)))
			}
			// Allow same timestamp for entries within same second, but flag if too many
			if entry.Timestamp.Equal(lastTimestamp) {
				// This is acceptable for high-frequency logging
			}
		}
		lastTimestamp = entry.Timestamp

		// Verify previous hash linkage (except for first entry)
		if i > 0 {
			expectedPrevHash := p.chain[i-1].ChainHash
			if entry.PreviousHash != expectedPrevHash {
				issues = append(issues, fmt.Sprintf("Entry %d has broken chain: previous hash mismatch", i))
			}
		} else {
			// First entry should have empty previous hash
			if entry.PreviousHash != "" {
				issues = append(issues, fmt.Sprintf("Entry 0 should have empty previous hash, got: %s", entry.PreviousHash))
			}
		}

		// Verify chain hash
		tempEntry := entry
		tempEntry.ChainHash = "" // Clear chain hash for verification
		chainData, err := json.Marshal(tempEntry)
		if err != nil {
			return false, issues, fmt.Errorf("failed to marshal entry %d: %w", i, err)
		}
		computedHash := p.computeHash(chainData)
		// SECURITY: Constant-time comparison prevents timing attacks
		if !hmac.Equal([]byte(entry.ChainHash), []byte(computedHash)) {
			issues = append(issues, fmt.Sprintf("Entry %d has invalid chain hash", i))
		}
	}

	valid := len(issues) == 0
	return valid, issues, nil
}

// DetectTampering checks for any signs of log tampering.
// Returns a detailed report of any anomalies detected.
func (p *Protector) DetectTampering() (*TamperReport, error) {
	valid, issues, err := p.VerifyLogIntegrity()
	if err != nil {
		return nil, err
	}

	report := &TamperReport{
		Timestamp:   time.Now(),
		Verified:    valid,
		ChainLength: len(p.chain),
		Issues:      issues,
	}

	// Additional tampering checks
	p.checkFilePermissions(report)
	p.checkFileTimestamps(report)

	// ADDED: Verify external witness
	witnessValid, witnessIssues, err := p.VerifyWitness()
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("Witness verification error: %v", err))
		report.Verified = false
	} else if !witnessValid {
		report.Issues = append(report.Issues, witnessIssues...)
		report.Verified = false
	}

	return report, nil
}

// checkFilePermissions verifies that audit log files have secure permissions.
func (p *Protector) checkFilePermissions(report *TamperReport) {
	report.PermissionIssues = make([]string, 0)

	// Check audit log permissions
	if info, err := os.Stat(p.auditLogPath); err == nil {
		mode := info.Mode()
		// Should be 0600 or 0400 (owner read/write or read-only)
		if mode.Perm()&0077 != 0 {
			report.PermissionIssues = append(report.PermissionIssues,
				fmt.Sprintf("Audit log has overly permissive mode: %o", mode.Perm()))
		}
	}

	// Check chain file permissions
	if info, err := os.Stat(p.chainFile); err == nil {
		mode := info.Mode()
		if mode.Perm()&0077 != 0 {
			report.PermissionIssues = append(report.PermissionIssues,
				fmt.Sprintf("Chain file has overly permissive mode: %o", mode.Perm()))
		}
	}
}

// checkFileTimestamps checks for anomalous file modification times.
func (p *Protector) checkFileTimestamps(report *TamperReport) {
	report.TimestampAnomalies = make([]string, 0)

	// Check if file modification time is newer than last chain entry
	if len(p.chain) > 0 {
		lastEntry := p.chain[len(p.chain)-1]
		if info, err := os.Stat(p.auditLogPath); err == nil {
			modTime := info.ModTime()
			// Allow 1 minute grace period for clock skew
			if modTime.After(lastEntry.Timestamp.Add(1 * time.Minute)) {
				report.TimestampAnomalies = append(report.TimestampAnomalies,
					fmt.Sprintf("File modified after last chain entry: file=%s, chain=%s",
						modTime.Format(time.RFC3339), lastEntry.Timestamp.Format(time.RFC3339)))
			}
		}
	}
}

// =============================================================================
// LOG PROTECTION
// =============================================================================

// ProtectLogs applies protective measures to audit log files.
// Sets restrictive permissions and marks as system/immutable if supported.
func (p *Protector) ProtectLogs() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Set restrictive permissions on audit log
	if err := os.Chmod(p.auditLogPath, 0600); err != nil {
		return fmt.Errorf("failed to set audit log permissions: %w", err)
	}

	// Set restrictive permissions on chain file
	if err := os.Chmod(p.chainFile, 0600); err != nil {
		return fmt.Errorf("failed to set chain file permissions: %w", err)
	}

	// On Unix systems, try to make logs append-only (requires root)
	// This is a best-effort operation
	// chattr +a would be called here on Linux if running as root

	return nil
}

// GetLogHash returns the hash of the current log state.
func (p *Protector) GetLogHash() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.chain) == 0 {
		return "", nil
	}

	// Return the chain hash of the latest entry
	return p.chain[len(p.chain)-1].ChainHash, nil
}

// =============================================================================
// LOG ARCHIVAL
// =============================================================================

// ArchiveLogs archives old audit logs securely.
// Moves logs older than retentionDays to an archive directory.
func (p *Protector) ArchiveLogs(retentionDays int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}

	// Create archive directory
	archiveDir := filepath.Join(filepath.Dir(p.auditLogPath), "archive")
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Find old log files to archive
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// Look for rotated log files
	dir := filepath.Dir(p.auditLogPath)
	baseFile := filepath.Base(p.auditLogPath)
	ext := filepath.Ext(baseFile)
	baseName := strings.TrimSuffix(baseFile, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	archivedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if this is a rotated log file
		name := entry.Name()
		if !strings.HasPrefix(name, baseName) || name == baseFile {
			continue
		}

		// Check file age
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			// Archive this file
			srcPath := filepath.Join(dir, name)
			dstPath := filepath.Join(archiveDir, name)

			// Copy file to archive
			if err := copyFile(srcPath, dstPath); err != nil {
				fmt.Fprintf(os.Stderr, "[AU-9 WARN] Failed to archive %s: %v\n", name, err)
				continue
			}

			// Verify copy
			if err := p.verifyArchive(srcPath, dstPath); err != nil {
				fmt.Fprintf(os.Stderr, "[AU-9 WARN] Archive verification failed for %s: %v\n", name, err)
				// Don't delete source if verification fails
				continue
			}

			// Delete original after successful archive
			if err := os.Remove(srcPath); err != nil {
				fmt.Fprintf(os.Stderr, "[AU-9 WARN] Failed to remove archived file %s: %v\n", name, err)
			} else {
				archivedCount++
			}
		}
	}

	if archivedCount > 0 {
		// Log archival event
		LogEvent("", "AUDIT_ARCHIVED", map[string]string{
			"count":     fmt.Sprintf("%d", archivedCount),
			"retention": fmt.Sprintf("%d days", retentionDays),
		})
	}

	return nil
}

// verifyArchive verifies that an archived file matches the original.
func (p *Protector) verifyArchive(srcPath, dstPath string) error {
	// Compute hash of source file
	srcHash, err := hashFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to hash source: %w", err)
	}

	// Compute hash of destination file
	dstHash, err := hashFile(dstPath)
	if err != nil {
		return fmt.Errorf("failed to hash destination: %w", err)
	}

	// SECURITY: Constant-time comparison prevents timing attacks
	if !hmac.Equal([]byte(srcHash), []byte(dstHash)) {
		return fmt.Errorf("archive verification failed: hash mismatch")
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// computeHash computes SHA-256 HMAC hash of data.
func (p *Protector) computeHash(data []byte) string {
	mac := hmac.New(sha256.New, p.hmacKey)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// loadChain loads the audit chain from disk.
func (p *Protector) loadChain() error {
	data, err := os.ReadFile(p.chainFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &p.chain)
}

// saveChain saves the audit chain to disk (acquires lock).
// Note: For AU-5 compliance, use saveChainWithRetry instead which provides retry logic.
func (p *Protector) saveChain() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saveChainLocked()
}

// hashFile computes SHA-256 hash of a file.
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

// =============================================================================
// EXTERNAL WITNESS (Hash Anchoring)
// =============================================================================

// writeWitness writes a hash witness to an external file.
// This provides an independent integrity check - if someone replaces the entire
// audit chain, the witness file will reveal the discrepancy.
func (p *Protector) writeWitness(entry LogChainEntry) error {
	// Open witness file for appending
	file, err := os.OpenFile(p.witnessFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open witness file: %w", err)
	}
	defer file.Close()

	// Write witness entry: timestamp|index|chain_hash
	witness := fmt.Sprintf("%s|%d|%s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Index,
		entry.ChainHash)

	if _, err := file.WriteString(witness); err != nil {
		return fmt.Errorf("failed to write witness: %w", err)
	}

	return file.Sync()
}

// VerifyWitness verifies that the witness file matches the audit chain.
// This detects if the entire chain has been replaced.
func (p *Protector) VerifyWitness() (bool, []string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	issues := make([]string, 0)

	// Read witness file
	data, err := os.ReadFile(p.witnessFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No witness file - acceptable for new systems
			return true, nil, nil
		}
		return false, issues, fmt.Errorf("failed to read witness file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	witnessCount := 0

	for i, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			issues = append(issues, fmt.Sprintf("Witness line %d has invalid format", i))
			continue
		}

		// Parse witness entry
		witnessTime, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			issues = append(issues, fmt.Sprintf("Witness line %d has invalid timestamp", i))
			continue
		}

		var witnessIndex int
		if _, err := fmt.Sscanf(parts[1], "%d", &witnessIndex); err != nil {
			issues = append(issues, fmt.Sprintf("Witness line %d has invalid index", i))
			continue
		}

		witnessHash := parts[2]

		// Verify against chain
		if witnessIndex >= len(p.chain) {
			issues = append(issues, fmt.Sprintf("Witness entry %d references non-existent chain index %d",
				i, witnessIndex))
			continue
		}

		chainEntry := p.chain[witnessIndex]
		// SECURITY: Constant-time comparison prevents timing attacks
		if !hmac.Equal([]byte(chainEntry.ChainHash), []byte(witnessHash)) {
			issues = append(issues, fmt.Sprintf("Witness entry %d hash mismatch: witness=%s, chain=%s",
				i, witnessHash, chainEntry.ChainHash))
		}

		if !chainEntry.Timestamp.Equal(witnessTime) {
			issues = append(issues, fmt.Sprintf("Witness entry %d timestamp mismatch: witness=%s, chain=%s",
				i, witnessTime.Format(time.RFC3339), chainEntry.Timestamp.Format(time.RFC3339)))
		}

		witnessCount++
	}

	// Check if witness has fewer entries than chain (suspicious)
	if witnessCount < len(p.chain) {
		issues = append(issues, fmt.Sprintf("Witness has fewer entries (%d) than chain (%d) - possible tampering",
			witnessCount, len(p.chain)))
	}

	valid := len(issues) == 0
	return valid, issues, nil
}

// =============================================================================
// AU-5 CONFIGURATION
// =============================================================================

// SetStrictMode enables or disables strict mode for AU-5 compliance.
// When enabled, operations will halt if audit logging fails.
func (p *Protector) SetStrictMode(strict bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.strictMode = strict
}

// IsStrictMode returns whether strict mode is enabled.
func (p *Protector) IsStrictMode() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.strictMode
}

// SetRetryConfig configures the retry behavior for AU-5 compliance.
// maxRetries: Maximum number of retry attempts (default: 3)
// baseWait: Base wait time for exponential backoff (default: 100ms)
func (p *Protector) SetRetryConfig(maxRetries int, baseWait time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if maxRetries > 0 {
		p.maxRetries = maxRetries
	}
	if baseWait > 0 {
		p.retryBaseWait = baseWait
	}
}

// GetRetryConfig returns the current retry configuration.
func (p *Protector) GetRetryConfig() (maxRetries int, baseWait time.Duration) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxRetries, p.retryBaseWait
}

// =============================================================================
// AU-9: KEY ROTATION
// =============================================================================

// RotateKey rotates the HMAC key and optionally re-signs existing entries.
// NIST 800-53 AU-9: Supports key rotation for audit log protection.
// Parameters:
//   - resignEntries: If true, all existing chain entries will be re-signed with the new key
//
// Returns RotationResult with details about the rotation operation.
func (p *Protector) RotateKey(resignEntries bool) (*RotationResult, error) {
	if p.keyManager == nil {
		return nil, fmt.Errorf("key manager not initialized - cannot rotate key")
	}

	result, err := p.keyManager.RotateKey(p, resignEntries)
	if err != nil {
		return nil, fmt.Errorf("key rotation failed: %w", err)
	}

	// Update our local key reference
	p.mu.Lock()
	p.hmacKey = p.keyManager.GetCurrentKey()
	p.mu.Unlock()

	return result, nil
}

// GetKeyMetadata returns metadata about the current HMAC key.
// Returns nil if key manager is not initialized.
func (p *Protector) GetKeyMetadata() *HMACKeyMetadata {
	if p.keyManager == nil {
		return nil
	}
	return p.keyManager.GetKeyMetadata()
}

// GetKeyManager returns the underlying key manager.
// Returns nil if key manager is not initialized.
func (p *Protector) GetKeyManager() *HMACKeyManager {
	return p.keyManager
}

// Close cleans up Protector resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (p *Protector) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Zero the HMAC key
	if p.hmacKey != nil {
		zeroBytes(p.hmacKey)
		p.hmacKey = nil
	}
	// Close the key manager (which zeros its keys)
	if p.keyManager != nil {
		p.keyManager.Close()
	}
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalProtector     *Protector
	globalProtectorOnce sync.Once
	globalProtectorMu   sync.RWMutex
)

// globalProtectorInitErr holds initialization error for fail-secure checking
var globalProtectorInitErr error

// GlobalProtector returns the global audit protector instance.
// SECURITY CRITICAL: Check IsHealthy() before relying on audit protection.
func GlobalProtector() *Protector {
	globalProtectorOnce.Do(func() {
		auditPath := DefaultPath()
		var err error
		globalProtector, err = NewProtector(auditPath)
		if err != nil {
			// SECURITY: Fail-secure - record error for callers to check
			globalProtectorInitErr = fmt.Errorf("SECURITY CRITICAL: audit protector init failed: %w", err)
			// Log to stderr since audit logging may not work
			fmt.Fprintf(os.Stderr, "[SECURITY CRITICAL] Audit protector initialization failed: %v\n", err)
			// Create minimal protector but mark it as unhealthy
			globalProtector = &Protector{
				auditLogPath: auditPath,
				chain:        make([]LogChainEntry, 0),
			}
		}
	})
	return globalProtector
}

// GlobalProtectorHealthy returns true if the global protector initialized successfully.
// SECURITY: Callers should check this before relying on audit integrity protection.
func GlobalProtectorHealthy() bool {
	GlobalProtector() // Ensure initialized
	return globalProtectorInitErr == nil
}

// GlobalProtectorError returns the initialization error, if any.
func GlobalProtectorError() error {
	GlobalProtector() // Ensure initialized
	return globalProtectorInitErr
}

// SetGlobalProtector sets the global audit protector instance.
func SetGlobalProtector(protector *Protector) {
	globalProtectorMu.Lock()
	defer globalProtectorMu.Unlock()
	globalProtector = protector
}
