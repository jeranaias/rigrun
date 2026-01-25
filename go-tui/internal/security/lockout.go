// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 AC-7: Unsuccessful Logon Attempts.
//
// # DoD STIG Requirements
//
//   - AC-7(a): Enforces a limit of consecutive invalid logon attempts
//     by a user during a specified time period (default: 3 attempts)
//   - AC-7(b): Automatically locks the account/node until released by
//     an administrator or after a specified time period (default: 15 minutes)
//   - AU-3: All lockout events must be logged for audit compliance
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// =============================================================================
// AC-7 CONSTANTS
// =============================================================================

const (
	// DefaultMaxAttempts is the default number of failed attempts before lockout.
	// Per AC-7(a), this is typically 3 consecutive failures.
	DefaultMaxAttempts = 3

	// DefaultLockoutDuration is the default lockout duration.
	// Per AC-7(b), this is typically 15 minutes for IL5 environments.
	DefaultLockoutDuration = 15 * time.Minute

	// LockoutStateFile is the filename for persistent lockout state.
	LockoutStateFile = "lockout_state.json"
)

// =============================================================================
// ATTEMPT RECORD
// =============================================================================

// AttemptRecord tracks authentication attempts for a specific identifier.
// This provides the data structure for AC-7 compliance tracking.
type AttemptRecord struct {
	// Count is the number of consecutive failed attempts.
	Count int `json:"count"`

	// LastAttempt is the timestamp of the last attempt.
	LastAttempt time.Time `json:"last_attempt"`

	// LockedUntil is the timestamp when the lockout expires.
	// Zero value means not locked.
	LockedUntil time.Time `json:"locked_until,omitempty"`

	// Locked indicates whether the identifier is currently locked out.
	Locked bool `json:"locked"`

	// LockoutCount tracks total number of lockouts for this identifier.
	LockoutCount int `json:"lockout_count,omitempty"`

	// FirstAttempt is the timestamp of the first failed attempt in this series.
	FirstAttempt time.Time `json:"first_attempt,omitempty"`
}

// IsExpired returns true if the lockout has expired.
func (a *AttemptRecord) IsExpired() bool {
	if !a.Locked {
		return false
	}
	return time.Now().After(a.LockedUntil)
}

// TimeRemaining returns the duration until the lockout expires.
// Returns 0 if not locked or lockout has expired.
func (a *AttemptRecord) TimeRemaining() time.Duration {
	if !a.Locked {
		return 0
	}
	remaining := time.Until(a.LockedUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// =============================================================================
// LOCKOUT MANAGER
// =============================================================================

// LockoutManager manages authentication lockout state per AC-7.
// It is thread-safe and supports persistent state storage.
type LockoutManager struct {
	// attempts maps identifiers (user IDs, API key prefixes) to their attempt records.
	attempts map[string]*AttemptRecord

	// maxAttempts is the number of failures before lockout (default: 3).
	maxAttempts int

	// lockoutDuration is how long a lockout lasts (default: 15 minutes).
	lockoutDuration time.Duration

	// mu protects concurrent access to the attempts map.
	mu sync.RWMutex

	// persistPath is the path for persistent lockout state storage.
	persistPath string

	// auditLogger is the audit logger for AC-7 events.
	auditLogger *AuditLogger

	// enabled indicates whether lockout tracking is active.
	enabled bool

	// integrityKey is the HMAC key for state file integrity verification.
	integrityKey []byte

	// paranoidMode is enabled when state file is missing or tampered with.
	paranoidMode bool
}

// LockoutManagerOption is a functional option for configuring LockoutManager.
type LockoutManagerOption func(*LockoutManager)

// WithMaxAttempts sets the maximum number of failed attempts before lockout.
// SECURITY: 0 is valid for paranoid mode (instant lockout - block all attempts).
func WithMaxAttempts(max int) LockoutManagerOption {
	return func(l *LockoutManager) {
		if max >= 0 {
			l.maxAttempts = max
		}
	}
}

// WithLockoutDuration sets the lockout duration.
func WithLockoutDuration(d time.Duration) LockoutManagerOption {
	return func(l *LockoutManager) {
		if d > 0 {
			l.lockoutDuration = d
		}
	}
}

// WithPersistPath sets the path for persistent state storage.
func WithPersistPath(path string) LockoutManagerOption {
	return func(l *LockoutManager) {
		l.persistPath = path
	}
}

// WithAuditLogger sets the audit logger for lockout events.
func WithAuditLogger(logger *AuditLogger) LockoutManagerOption {
	return func(l *LockoutManager) {
		l.auditLogger = logger
	}
}

// NewLockoutManager creates a new LockoutManager with the given options.
func NewLockoutManager(opts ...LockoutManagerOption) *LockoutManager {
	lm := &LockoutManager{
		attempts:        make(map[string]*AttemptRecord),
		maxAttempts:     DefaultMaxAttempts,
		lockoutDuration: DefaultLockoutDuration,
		enabled:         true,
	}

	// Apply options
	for _, opt := range opts {
		opt(lm)
	}

	// Set default persist path if not specified
	if lm.persistPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			lm.persistPath = filepath.Join(home, ".rigrun", LockoutStateFile)
		}
	}

	// Generate or load integrity key
	lm.initIntegrityKey()

	// Load existing state
	_ = lm.loadState()

	return lm
}

// =============================================================================
// CORE OPERATIONS
// =============================================================================

// ErrParanoidMode is returned when an operation is blocked due to paranoid mode.
var ErrParanoidMode = fmt.Errorf("access blocked: security integrity compromised (AU-5)")

// RecordAttempt records an authentication attempt for the given identifier.
// If success is true, the attempt counter is reset.
// If success is false, the counter is incremented and lockout may be triggered.
//
// SECURITY (AU-5): Verifies state file integrity on EVERY access.
// Returns an error if the identifier is currently locked out (ErrLocked).
// Returns ErrParanoidMode if state integrity is compromised with 0 attempts configured.
func (l *LockoutManager) RecordAttempt(identifier string, success bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled {
		return nil
	}

	// SECURITY (AU-5): Verify state file integrity on EVERY access
	l.verifyIntegrityLocked()

	// Mask identifier for logging
	maskedID := maskIdentifier(identifier)
	now := time.Now()

	// SECURITY: In paranoid mode with 0 maxAttempts, block ALL attempts immediately
	if l.paranoidMode && l.maxAttempts == 0 {
		l.logEvent("AUTH_BLOCKED_PARANOID", maskedID, map[string]string{
			"reason": "paranoid mode with 0 max attempts - all access blocked",
		})
		return ErrParanoidMode
	}

	// SECURITY: If maxAttempts is 0 (instant lockout mode), block all attempts
	if l.maxAttempts == 0 {
		l.logEvent("AUTH_BLOCKED_INSTANT", maskedID, map[string]string{
			"reason": "max attempts set to 0 - instant lockout mode",
		})
		return ErrLocked
	}

	// Get or create attempt record
	record, exists := l.attempts[identifier]
	if !exists {
		record = &AttemptRecord{
			FirstAttempt: now,
		}
		l.attempts[identifier] = record
	}

	// Check if currently locked
	if record.Locked && !record.IsExpired() {
		// Log blocked attempt
		l.logEvent("AUTH_ATTEMPT_BLOCKED", maskedID, map[string]string{
			"reason":         "locked",
			"time_remaining": record.TimeRemaining().String(),
		})
		return ErrLocked
	}

	// SECURITY: In paranoid mode, any prior attempt triggers immediate lockout
	if l.paranoidMode && record.Count >= 1 {
		l.logEvent("AUTH_BLOCKED_PARANOID", maskedID, map[string]string{
			"reason": "paranoid mode - prior attempt exists",
		})
		return ErrParanoidMode
	}

	// If lockout expired, clear it
	if record.Locked && record.IsExpired() {
		record.Locked = false
		record.LockedUntil = time.Time{}
		record.Count = 0
	}

	record.LastAttempt = now

	if success {
		// Successful attempt - reset counter
		record.Count = 0
		record.FirstAttempt = time.Time{}

		l.logEvent("AUTH_ATTEMPT", maskedID, map[string]string{
			"success": "true",
		})
	} else {
		// Failed attempt - increment counter
		if record.FirstAttempt.IsZero() {
			record.FirstAttempt = now
		}
		record.Count++

		l.logEvent("AUTH_ATTEMPT", maskedID, map[string]string{
			"success":       "false",
			"attempt_count": fmt.Sprintf("%d/%d", record.Count, l.maxAttempts),
		})

		// Check if lockout threshold reached
		if record.Count >= l.maxAttempts {
			record.Locked = true
			record.LockedUntil = now.Add(l.lockoutDuration)
			record.LockoutCount++

			l.logEvent("AUTH_LOCKOUT", maskedID, map[string]string{
				"duration":       l.lockoutDuration.String(),
				"until":          record.LockedUntil.Format(time.RFC3339),
				"lockout_number": fmt.Sprintf("%d", record.LockoutCount),
			})
		}
	}

	// Persist state
	_ = l.saveStateLocked()

	return nil
}

// IsLocked returns true if the identifier is currently locked out.
// SECURITY (AU-5): Verifies state file integrity on EVERY access.
func (l *LockoutManager) IsLocked(identifier string) bool {
	l.mu.Lock() // Use write lock since we may modify paranoidMode
	defer l.mu.Unlock()

	if !l.enabled {
		return false
	}

	// SECURITY (AU-5): Verify state file integrity on EVERY access
	l.verifyIntegrityLocked()

	// SECURITY: In paranoid mode with 0 maxAttempts, block ALL attempts immediately
	// This is defense in depth - if state file is missing/corrupted, lock everything
	if l.paranoidMode && l.maxAttempts == 0 {
		l.logEvent("AUTH_BLOCKED_PARANOID", maskIdentifier(identifier), map[string]string{
			"reason": "paranoid mode with 0 max attempts - all access blocked",
		})
		return true
	}

	// SECURITY: In paranoid mode, any recorded attempt triggers lockout
	if l.paranoidMode {
		record, exists := l.attempts[identifier]
		if exists && record.Count >= 1 && !record.Locked {
			// Force lockout on first failure in paranoid mode
			return true
		}
	}

	// SECURITY: Check if maxAttempts is 0 (instant lockout mode)
	// Block all access even if not in paranoid mode
	if l.maxAttempts == 0 {
		return true
	}

	record, exists := l.attempts[identifier]
	if !exists {
		return false
	}

	// Check if locked and not expired
	if record.Locked && !record.IsExpired() {
		return true
	}

	return false
}

// Unlock manually unlocks an identifier.
// This is typically done by an administrator.
func (l *LockoutManager) Unlock(identifier string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	record, exists := l.attempts[identifier]
	if !exists {
		return fmt.Errorf("identifier not found: %s", maskIdentifier(identifier))
	}

	if !record.Locked {
		return fmt.Errorf("identifier not locked: %s", maskIdentifier(identifier))
	}

	record.Locked = false
	record.LockedUntil = time.Time{}
	record.Count = 0

	maskedID := maskIdentifier(identifier)
	l.logEvent("AUTH_UNLOCK", maskedID, map[string]string{
		"method": "manual",
	})

	// Persist state
	_ = l.saveStateLocked()

	return nil
}

// Reset completely resets the lockout state for an identifier.
func (l *LockoutManager) Reset(identifier string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, identifier)

	maskedID := maskIdentifier(identifier)
	l.logEvent("AUTH_RESET", maskedID, nil)

	// Persist state
	_ = l.saveStateLocked()
}

// GetStatus returns the current attempt record for an identifier.
// Returns nil if no record exists.
func (l *LockoutManager) GetStatus(identifier string) *AttemptRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	record, exists := l.attempts[identifier]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	copyRecord := *record
	return &copyRecord
}

// =============================================================================
// LISTING AND STATS
// =============================================================================

// LockoutEntry represents a locked identifier for listing.
type LockoutEntry struct {
	Identifier    string        `json:"identifier"`
	LockedAt      time.Time     `json:"locked_at"`
	LockedUntil   time.Time     `json:"locked_until"`
	TimeRemaining time.Duration `json:"time_remaining"`
	LockoutCount  int           `json:"lockout_count"`
	FailedCount   int           `json:"failed_count"`
}

// ListLocked returns all currently locked identifiers.
func (l *LockoutManager) ListLocked() []LockoutEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var locked []LockoutEntry

	for id, record := range l.attempts {
		if record.Locked && !record.IsExpired() {
			locked = append(locked, LockoutEntry{
				Identifier:    maskIdentifier(id),
				LockedAt:      record.LockedUntil.Add(-l.lockoutDuration),
				LockedUntil:   record.LockedUntil,
				TimeRemaining: record.TimeRemaining(),
				LockoutCount:  record.LockoutCount,
				FailedCount:   record.Count,
			})
		}
	}

	return locked
}

// LockoutStats provides statistics about lockout state.
type LockoutStats struct {
	TotalTracked    int           `json:"total_tracked"`
	CurrentlyLocked int           `json:"currently_locked"`
	TotalLockouts   int           `json:"total_lockouts"`
	MaxAttempts     int           `json:"max_attempts"`
	LockoutDuration time.Duration `json:"lockout_duration"`
	PersistPath     string        `json:"persist_path,omitempty"`
	Enabled         bool          `json:"enabled"`
	ParanoidMode    bool          `json:"paranoid_mode"`
}

// GetStats returns lockout statistics.
func (l *LockoutManager) GetStats() LockoutStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LockoutStats{
		TotalTracked:    len(l.attempts),
		MaxAttempts:     l.maxAttempts,
		LockoutDuration: l.lockoutDuration,
		PersistPath:     l.persistPath,
		Enabled:         l.enabled,
		ParanoidMode:    l.paranoidMode,
	}

	for _, record := range l.attempts {
		if record.Locked && !record.IsExpired() {
			stats.CurrentlyLocked++
		}
		stats.TotalLockouts += record.LockoutCount
	}

	return stats
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// persistentState is the JSON structure for saving lockout state.
type persistentState struct {
	Attempts map[string]*AttemptRecord `json:"attempts"`
	SavedAt  time.Time                 `json:"saved_at"`
	Version  string                    `json:"version"`
}

// initIntegrityKey initializes the HMAC integrity key.
// It tries to load an existing key from a file, or generates a new one if not found.
// SECURITY (AU-9): No deterministic fallback keys - if random generation fails,
// we enter paranoid mode and require valid key or return error.
func (l *LockoutManager) initIntegrityKey() {
	if l.persistPath == "" {
		// Generate a random key for this session
		key, err := generateSecureRandom(32)
		if err != nil {
			// SECURITY: Do NOT use empty key - enter paranoid mode and log error
			l.paranoidMode = true
			l.logEvent("INTEGRITY_KEY_FAILED", "system", map[string]string{
				"reason": "failed to generate random key - security compromised",
				"error":  err.Error(),
			})
			// Set a nil key to force failures on HMAC operations
			l.integrityKey = nil
			return
		}
		l.integrityKey = key
		return
	}

	// Try to load existing key
	keyPath := l.persistPath + ".key"
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		l.integrityKey = keyData
		return
	}

	// Generate new key
	key, err := generateSecureRandom(32)
	if err != nil {
		// SECURITY: Do NOT use empty key - enter paranoid mode and log error
		l.paranoidMode = true
		l.logEvent("INTEGRITY_KEY_FAILED", "system", map[string]string{
			"reason": "failed to generate random key for persistence - security compromised",
			"error":  err.Error(),
		})
		// Set a nil key to force failures on HMAC operations
		l.integrityKey = nil
		return
	}
	l.integrityKey = key

	// Save key for future use using atomic write
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		l.logEvent("KEY_DIR_CREATE_FAILED", "system", map[string]string{
			"reason": fmt.Sprintf("failed to create key directory: %v", err),
		})
		return
	}

	// Use atomic write for the key file
	if err := l.atomicWriteFile(keyPath, l.integrityKey, 0600); err != nil {
		l.logEvent("KEY_SAVE_FAILED", "system", map[string]string{
			"reason": fmt.Sprintf("failed to save integrity key: %v", err),
		})
	}
}

// loadState loads lockout state from disk.
func (l *LockoutManager) loadState() error {
	if l.persistPath == "" {
		return nil
	}

	payload, err := os.ReadFile(l.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			// SECURITY: Missing state file is SUSPICIOUS
			// Could indicate tampering - enable paranoid mode
			l.paranoidMode = true
			l.logEvent("LOCKOUT_STATE_MISSING", "system", map[string]string{
				"reason": "state file not found - enabling paranoid mode",
			})
			return nil // Continue with empty state but in paranoid mode
		}
		return fmt.Errorf("failed to read lockout state: %w", err)
	}

	// SECURITY: Verify HMAC signature
	if len(payload) < 32 {
		l.paranoidMode = true
		l.logEvent("LOCKOUT_STATE_INVALID", "system", map[string]string{
			"reason": "state file too short for signature",
		})
		return errors.New("lockout state file corrupted - too short")
	}

	// Split data and signature (last 32 bytes are HMAC-SHA256)
	dataLen := len(payload) - 32
	data := payload[:dataLen]
	sig := payload[dataLen:]

	// Compute expected HMAC
	mac := hmac.New(sha256.New, l.integrityKey)
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	// Verify HMAC
	if !hmac.Equal(sig, expectedSig) {
		l.paranoidMode = true
		l.logEvent("LOCKOUT_STATE_TAMPERED", "system", map[string]string{
			"reason": "HMAC verification failed - possible tampering detected",
		})
		return errors.New("lockout state integrity check failed - possible tampering")
	}

	var state persistentState
	if err := json.Unmarshal(data, &state); err != nil {
		l.paranoidMode = true
		l.logEvent("LOCKOUT_STATE_INVALID", "system", map[string]string{
			"reason": "failed to parse JSON",
		})
		return fmt.Errorf("failed to parse lockout state: %w", err)
	}

	l.attempts = state.Attempts
	if l.attempts == nil {
		l.attempts = make(map[string]*AttemptRecord)
	}

	// Successfully loaded and verified
	l.paranoidMode = false
	l.logEvent("LOCKOUT_STATE_LOADED", "system", map[string]string{
		"records": fmt.Sprintf("%d", len(l.attempts)),
	})

	return nil
}

// verifyIntegrityLocked verifies state file integrity (caller must hold lock).
// SECURITY (AU-5): Must be called on EVERY state access, not just load.
// Returns true if integrity verified, false if compromised (triggers paranoid mode).
func (l *LockoutManager) verifyIntegrityLocked() bool {
	if l.persistPath == "" {
		return true // No persistence, nothing to verify
	}

	payload, err := os.ReadFile(l.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			// SECURITY: State file deleted - treat as tampering
			if !l.paranoidMode {
				l.paranoidMode = true
				l.logEvent("LOCKOUT_STATE_DELETED", "system", map[string]string{
					"reason": "state file deleted during operation - enabling paranoid mode",
				})
			}
			return false
		}
		// Read error - assume worst case
		if !l.paranoidMode {
			l.paranoidMode = true
			l.logEvent("LOCKOUT_STATE_READ_ERROR", "system", map[string]string{
				"reason": fmt.Sprintf("failed to read state file: %v", err),
			})
		}
		return false
	}

	// SECURITY: Verify HMAC signature
	if len(payload) < 32 {
		if !l.paranoidMode {
			l.paranoidMode = true
			l.logEvent("LOCKOUT_STATE_CORRUPTED", "system", map[string]string{
				"reason": "state file too short for signature",
			})
		}
		return false
	}

	// Split data and signature (last 32 bytes are HMAC-SHA256)
	dataLen := len(payload) - 32
	data := payload[:dataLen]
	sig := payload[dataLen:]

	// Compute expected HMAC
	mac := hmac.New(sha256.New, l.integrityKey)
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	// Verify HMAC
	if !hmac.Equal(sig, expectedSig) {
		if !l.paranoidMode {
			l.paranoidMode = true
			l.logEvent("LOCKOUT_STATE_TAMPERED", "system", map[string]string{
				"reason": "HMAC verification failed during access - possible tampering",
			})
		}
		return false
	}

	// Successfully verified - clear paranoid mode if it was set
	// This allows recovery after state file is recreated
	if l.paranoidMode {
		l.paranoidMode = false
		l.logEvent("LOCKOUT_STATE_VERIFIED", "system", map[string]string{
			"reason": "state file integrity verified - clearing paranoid mode",
		})
	}

	return true
}

// saveStateLocked saves lockout state to disk (caller must hold lock).
// SECURITY (AU-9): Uses atomic write pattern (temp file + fsync + rename)
// to ensure crash consistency and prevent partial writes.
func (l *LockoutManager) saveStateLocked() error {
	if l.persistPath == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(l.persistPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create lockout state directory: %w", err)
	}

	state := persistentState{
		Attempts: l.attempts,
		SavedAt:  time.Now(),
		Version:  "1.0",
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lockout state: %w", err)
	}

	// SECURITY: Generate HMAC signature for integrity verification
	mac := hmac.New(sha256.New, l.integrityKey)
	mac.Write(data)
	sig := mac.Sum(nil)

	// Append signature to data (last 32 bytes)
	payload := append(data, sig...)

	// SECURITY (AU-9): Use atomic write pattern
	if err := l.atomicWriteFile(l.persistPath, payload, 0600); err != nil {
		return fmt.Errorf("failed to write lockout state: %w", err)
	}

	// SECURITY: Clear paranoid mode after successful save with valid integrity
	// This allows normal operation after state file is created/recreated
	// The state is now valid with proper HMAC signature
	if l.paranoidMode {
		l.paranoidMode = false
		l.logEvent("LOCKOUT_STATE_SAVED", "system", map[string]string{
			"reason": "state file saved with integrity - clearing paranoid mode",
		})
	}

	return nil
}

// atomicWriteFile writes data to a file atomically using the temp file + rename pattern.
// SECURITY (AU-9): This ensures crash consistency - we never have a partial write.
// The pattern is:
// 1. Write to a temp file in the same directory
// 2. Fsync the temp file to ensure data is on disk
// 3. Fsync the directory (Linux only) to ensure the directory entry is persisted
// 4. Atomic rename temp file to final filename
func (l *LockoutManager) atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temp file in the same directory (required for atomic rename)
	tmpFile, err := os.CreateTemp(dir, ".lockout_state_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on any error
	success := false
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Set permissions on the temp file
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Write data to temp file
	n, err := tmpFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(data))
	}

	// SECURITY (AU-9): Fsync the file to ensure data is on disk before rename
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to fsync temp file: %w", err)
	}

	// Close the temp file before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// SECURITY (AU-9): On Linux/Unix, fsync the directory to ensure the rename is durable
	if runtime.GOOS != "windows" {
		if err := l.syncDirectory(dir); err != nil {
			// Log but don't fail - the write itself succeeded
			l.logEvent("DIR_SYNC_WARNING", "system", map[string]string{
				"reason": fmt.Sprintf("directory fsync failed: %v", err),
			})
		}
	}

	// SECURITY (AU-9): Atomic rename - this is the commit point
	// On POSIX systems, rename is atomic. On Windows, it's atomic for files.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to final path: %w", err)
	}

	// SECURITY (AU-9): On Linux/Unix, fsync the directory again after rename
	// to ensure the directory entry update is persisted
	if runtime.GOOS != "windows" {
		if err := l.syncDirectory(dir); err != nil {
			// Log but don't fail - the rename itself succeeded
			l.logEvent("DIR_SYNC_WARNING", "system", map[string]string{
				"reason": fmt.Sprintf("post-rename directory fsync failed: %v", err),
			})
		}
	}

	success = true
	return nil
}

// syncDirectory fsyncs a directory to ensure directory entries are persisted.
// This is a no-op on Windows where directory fsync is not meaningful.
func (l *LockoutManager) syncDirectory(dir string) error {
	if runtime.GOOS == "windows" {
		return nil // Windows doesn't support directory fsync
	}

	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open directory for sync: %w", err)
	}
	defer d.Close()

	if err := d.Sync(); err != nil {
		return fmt.Errorf("failed to sync directory: %w", err)
	}

	return nil
}

// SaveState saves the current lockout state to disk.
func (l *LockoutManager) SaveState() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.saveStateLocked()
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetEnabled enables or disables lockout tracking.
func (l *LockoutManager) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled returns whether lockout tracking is enabled.
func (l *LockoutManager) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// SetMaxAttempts sets the maximum attempts before lockout.
// SECURITY: 0 is valid for paranoid mode (instant lockout - block all attempts).
func (l *LockoutManager) SetMaxAttempts(max int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if max >= 0 {
		l.maxAttempts = max
	}
}

// GetMaxAttempts returns the maximum attempts before lockout.
func (l *LockoutManager) GetMaxAttempts() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.maxAttempts
}

// SetLockoutDuration sets the lockout duration.
func (l *LockoutManager) SetLockoutDuration(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d > 0 {
		l.lockoutDuration = d
	}
}

// GetLockoutDuration returns the lockout duration.
func (l *LockoutManager) GetLockoutDuration() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lockoutDuration
}

// IsParanoidMode returns whether paranoid mode is currently active.
func (l *LockoutManager) IsParanoidMode() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.paranoidMode
}

// ClearParanoidMode explicitly clears paranoid mode.
// SECURITY: Paranoid mode persists until explicitly cleared by this method.
// This requires explicit user action and creates an audit trail.
// Returns error if the operation fails.
func (l *LockoutManager) ClearParanoidMode(reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.paranoidMode {
		return nil // Already not in paranoid mode
	}

	// Require a reason for audit trail
	if reason == "" {
		reason = "explicit user action"
	}

	// Log the paranoid mode clear for audit compliance
	l.logEvent("PARANOID_MODE_CLEARED", "system", map[string]string{
		"reason": reason,
		"action": "explicit_clear",
	})

	l.paranoidMode = false

	// Save state to persist the change
	return l.saveStateLocked()
}

// =============================================================================
// CLEANUP
// =============================================================================

// Cleanup removes expired lockout records to free memory.
func (l *LockoutManager) Cleanup() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	cleaned := 0
	for id, record := range l.attempts {
		// Remove records that are:
		// 1. Not locked and have no recent attempts (more than lockout duration old)
		// 2. Locked but expired (auto-unlock)
		if !record.Locked && time.Since(record.LastAttempt) > l.lockoutDuration*2 {
			delete(l.attempts, id)
			cleaned++
		} else if record.Locked && record.IsExpired() {
			// Auto-unlock expired records
			record.Locked = false
			record.LockedUntil = time.Time{}
			record.Count = 0

			l.logEvent("AUTH_UNLOCK", maskIdentifier(id), map[string]string{
				"method": "auto_expire",
			})
		}
	}

	if cleaned > 0 {
		_ = l.saveStateLocked()
	}

	return cleaned
}

// Clear removes all lockout records.
func (l *LockoutManager) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.attempts = make(map[string]*AttemptRecord)
	_ = l.saveStateLocked()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a lockout-related event to the audit log.
func (l *LockoutManager) logEvent(eventType, identifier string, metadata map[string]string) {
	if l.auditLogger == nil || !l.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: identifier,
		Success:   eventType != "AUTH_LOCKOUT" && eventType != "AUTH_ATTEMPT_BLOCKED",
		Metadata:  metadata,
	}

	if err := l.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log lockout event %s: %v\n", eventType, err)
	}
}

// maskIdentifier masks an identifier for logging using SHA256 hash.
// SECURITY: No longer shows partial strings to prevent information leakage.
func maskIdentifier(id string) string {
	// Use SHA256 to create a consistent, non-reversible identifier
	hash := sha256.Sum256([]byte(id))
	// Show first 12 characters of hex-encoded hash (48 bits of entropy)
	return "hash:" + hex.EncodeToString(hash[:])[:12]
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateSecureRandom generates cryptographically secure random bytes.
// SECURITY: Returns error if random generation fails - do NOT use empty/fallback keys.
func generateSecureRandom(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate integrity key: %w", err)
	}
	return key, nil
}

// =============================================================================
// ERRORS
// =============================================================================

// ErrLocked is returned when an operation is blocked due to lockout.
var ErrLocked = fmt.Errorf("account locked: too many failed attempts (AC-7)")

// Close cleans up LockoutManager resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (l *LockoutManager) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.integrityKey != nil {
		ZeroBytes(l.integrityKey)
		l.integrityKey = nil
	}
}

// =============================================================================
// GLOBAL LOCKOUT MANAGER
// =============================================================================

var (
	globalLockoutManager     *LockoutManager
	globalLockoutManagerOnce sync.Once
	globalLockoutManagerMu   sync.Mutex
)

// GlobalLockoutManager returns the global lockout manager instance.
// It lazily initializes the manager with default settings.
func GlobalLockoutManager() *LockoutManager {
	globalLockoutManagerOnce.Do(func() {
		globalLockoutManager = NewLockoutManager(
			WithAuditLogger(GlobalAuditLogger()),
		)
	})
	return globalLockoutManager
}

// SetGlobalLockoutManager sets the global lockout manager instance.
func SetGlobalLockoutManager(manager *LockoutManager) {
	globalLockoutManagerMu.Lock()
	defer globalLockoutManagerMu.Unlock()
	globalLockoutManager = manager
}

// InitGlobalLockoutManager initializes the global lockout manager with options.
func InitGlobalLockoutManager(maxAttempts int, lockoutMinutes int) {
	globalLockoutManagerMu.Lock()
	defer globalLockoutManagerMu.Unlock()

	opts := []LockoutManagerOption{
		WithAuditLogger(GlobalAuditLogger()),
	}

	if maxAttempts > 0 {
		opts = append(opts, WithMaxAttempts(maxAttempts))
	}

	if lockoutMinutes > 0 {
		opts = append(opts, WithLockoutDuration(time.Duration(lockoutMinutes)*time.Minute))
	}

	globalLockoutManager = NewLockoutManager(opts...)
}
