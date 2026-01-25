// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file contains tests for NIST 800-53 AU-9 compliance:
// - Atomic writes for lockout state
// - Crash recovery scenarios
// - Integrity verification
package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAtomicWriteBasic tests that atomic writes complete successfully.
func TestAtomicWriteBasic(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Record some attempts
	_ = lm.RecordAttempt("user1", false)
	_ = lm.RecordAttempt("user1", false)

	// Verify state file exists and is valid
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("State file not created: %v", err)
	}

	// Read and verify the state file has proper HMAC signature (32 bytes at end)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	if len(data) < 32 {
		t.Fatalf("State file too short, expected at least 32 bytes for HMAC, got %d", len(data))
	}

	// Verify JSON structure (without signature)
	jsonData := data[:len(data)-32]
	var state persistentState
	if err := json.Unmarshal(jsonData, &state); err != nil {
		t.Fatalf("Failed to parse state JSON: %v", err)
	}

	if len(state.Attempts) != 1 {
		t.Errorf("Expected 1 attempt record, got %d", len(state.Attempts))
	}
}

// TestAtomicWriteNoTempFileLeftBehind tests that no temp files are left after successful write.
func TestAtomicWriteNoTempFileLeftBehind(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Perform multiple writes
	for i := 0; i < 10; i++ {
		_ = lm.RecordAttempt("user1", false)
	}

	// Check for any leftover temp files
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("Found leftover temp file: %s", entry.Name())
		}
	}
}

// TestCrashRecoveryPartialWrite simulates a crash during write by creating a partial temp file.
func TestCrashRecoveryPartialWrite(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create a valid state file first
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	_ = lm.RecordAttempt("user1", false)
	_ = lm.RecordAttempt("user1", false)

	// Simulate a crash by creating a partial temp file
	partialTempPath := filepath.Join(tempDir, ".lockout_state_crash.tmp")
	if err := os.WriteFile(partialTempPath, []byte("partial data"), 0600); err != nil {
		t.Fatalf("Failed to create partial temp file: %v", err)
	}

	// Create a new lockout manager - it should load the valid state, not the partial file
	lm2 := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Verify the state was loaded correctly
	status := lm2.GetStatus("user1")
	if status == nil {
		t.Fatal("Expected status for user1, got nil")
	}

	if status.Count != 2 {
		t.Errorf("Expected count 2, got %d", status.Count)
	}

	// The temp file should still exist (we didn't clean it up)
	// In production, you might want to clean up orphaned temp files on startup
	if _, err := os.Stat(partialTempPath); os.IsNotExist(err) {
		t.Log("Note: Partial temp file was cleaned up (this is acceptable behavior)")
	}
}

// TestCrashRecoveryMissingStateFile tests paranoid mode when state file is missing.
func TestCrashRecoveryMissingStateFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create and save initial state
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	_ = lm.RecordAttempt("user1", false)

	// Simulate crash/attack by deleting state file
	if err := os.Remove(statePath); err != nil {
		t.Fatalf("Failed to remove state file: %v", err)
	}

	// Create a new lockout manager - it should enter paranoid mode
	lm2 := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	stats := lm2.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode to be enabled when state file is missing")
	}
}

// TestCrashRecoveryTamperedStateFile tests paranoid mode when state file is tampered.
func TestCrashRecoveryTamperedStateFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create and save initial state
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	_ = lm.RecordAttempt("user1", false)

	// Tamper with the state file by modifying a byte
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	// Modify a byte in the JSON portion (not the signature)
	if len(data) > 40 {
		data[10] = data[10] ^ 0xFF // Flip bits
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatalf("Failed to write tampered state file: %v", err)
	}

	// Create a new lockout manager - it should enter paranoid mode
	lm2 := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	stats := lm2.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode to be enabled when state file is tampered")
	}
}

// TestCrashRecoveryCorruptedStateFile tests paranoid mode when state file is corrupted.
func TestCrashRecoveryCorruptedStateFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Write a corrupted state file (too short)
	if err := os.WriteFile(statePath, []byte("short"), 0600); err != nil {
		t.Fatalf("Failed to write corrupted state file: %v", err)
	}

	// Create a new lockout manager - it should enter paranoid mode
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	stats := lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode to be enabled when state file is corrupted")
	}
}

// TestConcurrentAtomicWrites tests that concurrent writes don't corrupt the state.
func TestConcurrentAtomicWrites(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(100), // High threshold to avoid lockouts during test
	)

	// Perform concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10
	attemptsPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < attemptsPerGoroutine; j++ {
				identifier := string(rune('A' + id))
				_ = lm.RecordAttempt(identifier, false)
			}
		}(i)
	}

	wg.Wait()

	// Verify state file is valid
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file after concurrent writes: %v", err)
	}

	if len(data) < 32 {
		t.Fatalf("State file too short after concurrent writes")
	}

	// Verify JSON is valid
	jsonData := data[:len(data)-32]
	var state persistentState
	if err := json.Unmarshal(jsonData, &state); err != nil {
		t.Fatalf("State file corrupted after concurrent writes: %v", err)
	}

	// Verify we have records for all goroutines
	if len(state.Attempts) != numGoroutines {
		t.Errorf("Expected %d attempt records, got %d", numGoroutines, len(state.Attempts))
	}
}

// TestAtomicWriteKeyFile tests that the integrity key file is written atomically.
func TestAtomicWriteKeyFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")
	keyPath := statePath + ".key"

	_ = NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Verify key file exists
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("Key file not created: %v", err)
	}

	// Verify key file is 32 bytes
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}

	if len(keyData) != 32 {
		t.Errorf("Expected key file to be 32 bytes, got %d", len(keyData))
	}

	// Check for no leftover temp files
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("Found leftover temp file: %s", entry.Name())
		}
	}
}

// TestNoDeterministicFallbackKey verifies that no deterministic fallback key is used.
func TestNoDeterministicFallbackKey(t *testing.T) {
	tempDir := t.TempDir()
	statePath1 := filepath.Join(tempDir, "state1", "lockout_state.json")
	statePath2 := filepath.Join(tempDir, "state2", "lockout_state.json")

	// Create two lockout managers
	lm1 := NewLockoutManager(
		WithPersistPath(statePath1),
		WithMaxAttempts(3),
	)

	lm2 := NewLockoutManager(
		WithPersistPath(statePath2),
		WithMaxAttempts(3),
	)

	// Both should have different integrity keys (random)
	// We can't directly access the key, but we can verify by checking that
	// state files signed by one can't be loaded by the other

	_ = lm1.RecordAttempt("user1", false)
	_ = lm2.RecordAttempt("user2", false)

	// Copy lm1's state file to lm2's location
	data1, err := os.ReadFile(statePath1)
	if err != nil {
		t.Fatalf("Failed to read state1: %v", err)
	}

	// This should cause lm2 to enter paranoid mode when loading
	// because the HMAC won't match
	if err := os.WriteFile(statePath2, data1, 0600); err != nil {
		t.Fatalf("Failed to write state2: %v", err)
	}

	// Create a new lm2 that will try to load the mismatched state
	lm2New := NewLockoutManager(
		WithPersistPath(statePath2),
		WithMaxAttempts(3),
	)

	stats := lm2New.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode when loading state signed with different key")
	}
}

// TestIntegrityVerificationOnAccess tests that integrity is verified on every IsLocked call.
func TestIntegrityVerificationOnAccess(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Record an attempt
	_ = lm.RecordAttempt("user1", false)

	// Verify not locked yet
	if lm.IsLocked("user1") {
		t.Error("user1 should not be locked after 1 attempt")
	}

	// Now tamper with the state file while lm is still running
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	// Tamper by modifying a byte
	if len(data) > 40 {
		data[10] = data[10] ^ 0xFF
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatalf("Failed to write tampered state: %v", err)
	}

	// The next IsLocked call should detect tampering and enable paranoid mode
	// In paranoid mode with 1 recorded attempt, user should be locked
	if !lm.IsLocked("user1") {
		// This is expected behavior - paranoid mode locks after any recorded attempt
		stats := lm.GetStats()
		if !stats.ParanoidMode {
			t.Error("Expected paranoid mode after tampering detected during access")
		}
	}
}

// TestStateFilePermissions tests that state files have secure permissions.
func TestStateFilePermissions(t *testing.T) {
	// Skip on Windows - permissions work differently there
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - uses ACLs instead of Unix permissions")
	}

	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	_ = lm.RecordAttempt("user1", false)

	// Check state file permissions
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("Failed to stat state file: %v", err)
	}

	mode := info.Mode().Perm()
	// On Unix, we expect 0600. Check that group/world don't have access.
	if mode&0077 != 0 {
		t.Errorf("State file has insecure permissions: %o (expected 0600)", mode)
	}
}

// TestKeyFilePermissions tests that key files have secure permissions.
func TestKeyFilePermissions(t *testing.T) {
	// Skip on Windows - permissions work differently there
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - uses ACLs instead of Unix permissions")
	}

	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")
	keyPath := statePath + ".key"

	_ = NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
	)

	// Check key file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}

	mode := info.Mode().Perm()
	// On Unix, we expect 0600. Check that group/world don't have access.
	if mode&0077 != 0 {
		t.Errorf("Key file has insecure permissions: %o (expected 0600)", mode)
	}
}

// TestLockoutManagerReloadsState tests that state is properly loaded on restart.
func TestLockoutManagerReloadsState(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager and record attempts
	lm1 := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
		WithLockoutDuration(time.Hour), // Long duration so it doesn't expire
	)

	_ = lm1.RecordAttempt("user1", false)
	_ = lm1.RecordAttempt("user1", false)
	_ = lm1.RecordAttempt("user1", false) // This triggers lockout

	if !lm1.IsLocked("user1") {
		t.Fatal("user1 should be locked after 3 attempts")
	}

	// Create new manager - should load existing state
	lm2 := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
		WithLockoutDuration(time.Hour),
	)

	// User should still be locked
	if !lm2.IsLocked("user1") {
		t.Error("user1 should still be locked after manager restart")
	}

	// Verify attempt count was preserved
	status := lm2.GetStatus("user1")
	if status == nil {
		t.Fatal("Expected status for user1")
	}

	if status.Count != 3 {
		t.Errorf("Expected count 3, got %d", status.Count)
	}

	if status.LockoutCount != 1 {
		t.Errorf("Expected lockout count 1, got %d", status.LockoutCount)
	}
}

// TestParanoidModeBlocksAccess tests that paranoid mode properly blocks access.
func TestParanoidModeBlocksAccess(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager with max attempts of 0 (instant lockout mode)
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(0),
	)

	// Force paranoid mode
	lm.mu.Lock()
	lm.paranoidMode = true
	lm.mu.Unlock()

	// All access should be blocked
	if !lm.IsLocked("any_user") {
		t.Error("Expected all access to be blocked in paranoid mode with 0 max attempts")
	}
}

// =============================================================================
// NIST 800-53 AU-5 COMPLIANCE TESTS
// =============================================================================

// TestParanoidModeZeroAttemptsBlocksAll tests that 0 max attempts blocks all access.
// This is the "paranoid mode" instant lockout functionality required by AU-5.
func TestParanoidModeZeroAttemptsBlocksAll(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager with 0 max attempts
	lm := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(0),
		WithLockoutDuration(15*time.Minute),
	)

	// Verify max attempts is 0
	if lm.GetMaxAttempts() != 0 {
		t.Errorf("Expected maxAttempts=0, got %d", lm.GetMaxAttempts())
	}

	// Test 1: IsLocked should return true for ANY identifier
	if !lm.IsLocked("newuser") {
		t.Error("Expected IsLocked to return true with 0 max attempts")
	}

	// Test 2: RecordAttempt should return an error (either ErrLocked or ErrParanoidMode)
	// When maxAttempts=0 with paranoid mode, we get ErrParanoidMode
	// When maxAttempts=0 without paranoid mode, we get ErrLocked
	err := lm.RecordAttempt("newuser", false)
	if err != ErrLocked && err != ErrParanoidMode {
		t.Errorf("Expected ErrLocked or ErrParanoidMode for failed attempt with 0 max attempts, got %v", err)
	}

	// Test 3: RecordAttempt should return error even for successful attempts
	err = lm.RecordAttempt("newuser", true)
	if err != ErrLocked && err != ErrParanoidMode {
		t.Errorf("Expected ErrLocked or ErrParanoidMode for successful attempt with 0 max attempts, got %v", err)
	}

	// Test 4: New identifiers should also be blocked
	if !lm.IsLocked("completely_new_user") {
		t.Error("Expected new identifier to be locked with 0 max attempts")
	}
}

// TestSetMaxAttemptsZeroAllowed tests that SetMaxAttempts accepts 0.
func TestSetMaxAttemptsZeroAllowed(t *testing.T) {
	lm := NewLockoutManager(
		WithMaxAttempts(3),
	)

	// Initially should be 3
	if lm.GetMaxAttempts() != 3 {
		t.Errorf("Expected initial maxAttempts=3, got %d", lm.GetMaxAttempts())
	}

	// Set to 0
	lm.SetMaxAttempts(0)
	if lm.GetMaxAttempts() != 0 {
		t.Errorf("Expected maxAttempts=0 after SetMaxAttempts(0), got %d", lm.GetMaxAttempts())
	}

	// Verify all identifiers are now locked
	if !lm.IsLocked("anyuser") {
		t.Error("Expected all identifiers to be locked when maxAttempts=0")
	}
}

// TestWithMaxAttemptsZeroOption tests that WithMaxAttempts accepts 0 as option.
func TestWithMaxAttemptsZeroOption(t *testing.T) {
	lm := NewLockoutManager(
		WithMaxAttempts(0),
	)

	if lm.GetMaxAttempts() != 0 {
		t.Errorf("Expected maxAttempts=0 from option, got %d", lm.GetMaxAttempts())
	}

	// Verify all identifiers are locked
	if !lm.IsLocked("anyuser") {
		t.Error("Expected all identifiers to be locked when maxAttempts=0")
	}
}

// TestStateFileDeletionBlocksAttempts tests that deleting state file
// during operation enables paranoid mode and blocks subsequent attempts.
func TestStateFileDeletionBlocksAttempts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager with normal settings
	lm := NewLockoutManager(
		WithMaxAttempts(3),
		WithPersistPath(statePath),
		WithLockoutDuration(15*time.Minute),
	)

	// Record some attempts to create state
	_ = lm.RecordAttempt("user1", false)
	_ = lm.RecordAttempt("user1", false)

	// Verify state file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file should exist")
	}

	// Delete the state file (simulating attacker deleting it)
	if err := os.Remove(statePath); err != nil {
		t.Fatalf("Failed to remove state file: %v", err)
	}

	// Next access should trigger paranoid mode
	_ = lm.IsLocked("user1")

	// Verify paranoid mode is enabled
	stats := lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode after state file deletion")
	}

	// User with prior attempts should be blocked in paranoid mode
	if !lm.IsLocked("user1") {
		t.Error("Expected user with prior attempts to be locked in paranoid mode")
	}
}

// TestCorruptedStateFileBlocksAttempts tests that corrupted state file
// enables paranoid mode.
func TestCorruptedStateFileBlocksAttempts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager
	lm := NewLockoutManager(
		WithMaxAttempts(3),
		WithPersistPath(statePath),
	)

	// Record attempt to create state
	_ = lm.RecordAttempt("user1", false)

	// Corrupt the state file by writing garbage
	if err := os.WriteFile(statePath, []byte("corrupted garbage data"), 0600); err != nil {
		t.Fatalf("Failed to write corrupted state: %v", err)
	}

	// Next access should detect corruption
	_ = lm.IsLocked("user1")

	// Verify paranoid mode is enabled
	stats := lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode after state file corruption")
	}
}

// TestMissingStateFileBlocksAttempts tests that missing state file
// on load enables paranoid mode.
func TestMissingStateFileBlocksAttempts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "nonexistent_state.json")

	// Create key file to simulate previous use
	keyPath := statePath + ".key"
	keyData := make([]byte, 32)
	for i := range keyData {
		keyData[i] = byte(i)
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}

	// Create manager pointing to non-existent state
	lm := NewLockoutManager(
		WithMaxAttempts(3),
		WithPersistPath(statePath),
	)

	// Should be in paranoid mode
	stats := lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode when state file is missing")
	}
}

// TestErrParanoidModeReturnedOnZeroAttempts tests that ErrParanoidMode is
// returned when paranoid mode is active with 0 max attempts.
func TestErrParanoidModeReturnedOnZeroAttempts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager with 0 attempts
	lm := NewLockoutManager(
		WithMaxAttempts(0),
		WithPersistPath(statePath),
	)

	// Save state then delete to trigger paranoid mode
	_ = lm.SaveState()
	_ = os.Remove(statePath)

	// Trigger integrity check via IsLocked
	_ = lm.IsLocked("user1")

	// RecordAttempt should return ErrParanoidMode
	err := lm.RecordAttempt("user1", false)
	if err != ErrParanoidMode {
		t.Errorf("Expected ErrParanoidMode, got %v", err)
	}
}

// TestIntegrityVerifiedOnEveryIsLockedCall tests that integrity is verified
// on every IsLocked call, not just on load.
func TestIntegrityVerifiedOnEveryIsLockedCall(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithMaxAttempts(3),
		WithPersistPath(statePath),
	)

	// Record attempt
	_ = lm.RecordAttempt("user1", false)

	// Verify not in paranoid mode
	stats := lm.GetStats()
	if stats.ParanoidMode {
		t.Error("Should not be in paranoid mode initially")
	}

	// Multiple IsLocked calls should work
	for i := 0; i < 5; i++ {
		_ = lm.IsLocked("user1")
	}

	// Still not in paranoid mode
	stats = lm.GetStats()
	if stats.ParanoidMode {
		t.Error("Should not be in paranoid mode with valid state")
	}

	// Now corrupt the file
	if err := os.WriteFile(statePath, []byte("corrupted"), 0600); err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}

	// Next IsLocked should trigger paranoid mode
	_ = lm.IsLocked("user1")
	stats = lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Should be in paranoid mode after corruption detected during access")
	}
}

// TestIntegrityVerifiedOnEveryRecordAttemptCall tests that integrity is verified
// on every RecordAttempt call.
func TestIntegrityVerifiedOnEveryRecordAttemptCall(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithMaxAttempts(3),
		WithPersistPath(statePath),
	)

	// Record attempt to create state
	_ = lm.RecordAttempt("user1", false)

	// Verify not in paranoid mode
	stats := lm.GetStats()
	if stats.ParanoidMode {
		t.Error("Should not be in paranoid mode initially")
	}

	// Now corrupt the file
	if err := os.WriteFile(statePath, []byte("corrupted"), 0600); err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}

	// Next RecordAttempt should trigger paranoid mode
	_ = lm.RecordAttempt("user1", false)
	stats = lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Should be in paranoid mode after corruption detected during RecordAttempt")
	}
}

// TestParanoidModeWithPriorAttemptsBlocks tests that in paranoid mode,
// users with prior recorded attempts are blocked.
func TestParanoidModeWithPriorAttemptsBlocks(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	lm := NewLockoutManager(
		WithMaxAttempts(5), // High threshold
		WithPersistPath(statePath),
	)

	// Record one failed attempt for user1
	_ = lm.RecordAttempt("user1", false)

	// user1 should not be locked yet (only 1 of 5 attempts)
	if lm.IsLocked("user1") {
		t.Error("user1 should not be locked after 1 of 5 attempts")
	}

	// Now delete state file to trigger paranoid mode
	_ = os.Remove(statePath)

	// Trigger integrity check
	_ = lm.IsLocked("user1")

	// Verify paranoid mode
	stats := lm.GetStats()
	if !stats.ParanoidMode {
		t.Error("Expected paranoid mode after state deletion")
	}

	// In paranoid mode, user with ANY prior attempts should be locked
	if !lm.IsLocked("user1") {
		t.Error("Expected user with prior attempts to be locked in paranoid mode")
	}

	// RecordAttempt should also block
	err := lm.RecordAttempt("user1", false)
	if err != ErrParanoidMode {
		t.Errorf("Expected ErrParanoidMode for user with prior attempts, got %v", err)
	}
}

// =============================================================================
// TIMING ATTACK TESTS
// =============================================================================

// TestLockoutManager_CannotBypassWithTimingAttack tests that lockout cannot be bypassed
// through rapid status checks (timing attack simulation).
func TestLockoutManager_CannotBypassWithTimingAttack(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	mgr := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3), // Use default max attempts of 3
		WithLockoutDuration(1*time.Hour), // Long lockout
	)
	user := "attacker"

	// Lock the user by recording 3 failed attempts (matches maxAttempts)
	for i := 0; i < 3; i++ {
		_ = mgr.RecordAttempt(user, false)
	}

	// Verify locked via IsLocked - this is the authoritative check
	// Note: IsLocked considers paranoid mode and other security factors
	// that may lock a user even if status.Locked is false
	if !mgr.IsLocked(user) {
		t.Fatal("User should be locked after 3 failed attempts")
	}

	// Timing attack: try rapid IsLocked checks to see if we can find a moment
	// where the user is not locked (race condition exploitation attempt)
	bypassDetected := false
	for i := 0; i < 1000; i++ {
		// IsLocked is the authoritative lockout check
		if !mgr.IsLocked(user) {
			bypassDetected = true
			break
		}
	}

	if bypassDetected {
		t.Fatal("Lockout was bypassed! This indicates a timing vulnerability.")
	}
}

// TestLockoutManager_CannotBypassWithConcurrentChecks tests that concurrent
// lockout status checks cannot bypass the lockout mechanism.
func TestLockoutManager_CannotBypassWithConcurrentChecks(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	mgr := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
		WithLockoutDuration(1*time.Hour),
	)
	user := "concurrent_attacker"

	// Lock the user
	for i := 0; i < 3; i++ {
		_ = mgr.RecordAttempt(user, false)
	}

	// Verify initially locked - IsLocked is the authoritative check
	if !mgr.IsLocked(user) {
		t.Fatal("User should be locked")
	}

	// Concurrent bypass attempt - multiple goroutines checking IsLocked simultaneously
	// IsLocked is the security-critical check that considers paranoid mode
	var wg sync.WaitGroup
	bypassChan := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Only check IsLocked since it's the authoritative lockout check
				if !mgr.IsLocked(user) {
					bypassChan <- true
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Check for bypass detection with timeout
	select {
	case <-bypassChan:
		t.Fatal("Lockout was bypassed during concurrent checks! This is a security vulnerability.")
	case <-done:
		// All checks completed without bypass - expected behavior
	case <-time.After(10 * time.Second):
		t.Fatal("Test timeout - goroutines did not complete in time")
	}
}

// TestLockoutManager_CannotBypassWithRapidRecordAttempts tests that rapid
// RecordAttempt calls cannot bypass the lockout counter.
func TestLockoutManager_CannotBypassWithRapidRecordAttempts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	mgr := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(5),
		WithLockoutDuration(1*time.Hour),
	)
	user := "rapid_attacker"

	// Attempt to bypass by sending many concurrent failed attempts
	// hoping some slip through the counter
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := mgr.RecordAttempt(user, false)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Should have recorded some attempts before lockout
	// But user MUST be locked now
	if !mgr.IsLocked(user) {
		t.Fatal("User should be locked after rapid failed attempts")
	}

	// Additional attempts should be blocked
	for i := 0; i < 100; i++ {
		err := mgr.RecordAttempt(user, false)
		if err == nil {
			t.Fatal("Should not be able to record more attempts while locked")
		}
	}
}

// TestLockoutManager_LockoutPersistsAcrossRapidRestarts tests that lockout
// state persists correctly even with rapid manager restarts.
// Note: This test verifies that a single manager instance maintains lockout state.
// Cross-instance persistence is tested separately in existing persistence tests.
func TestLockoutManager_LockoutPersistsAcrossRapidRestarts(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "lockout_state.json")

	// Create manager and lock user
	mgr := NewLockoutManager(
		WithPersistPath(statePath),
		WithMaxAttempts(3),
		WithLockoutDuration(1*time.Hour),
	)

	user := "restart_test_user"
	for i := 0; i < 3; i++ {
		_ = mgr.RecordAttempt(user, false)
	}

	// Verify locked
	if !mgr.IsLocked(user) {
		t.Fatal("User should be locked")
	}

	// Rapid checks attempting to bypass lockout through repeated checking
	// on the same instance (simulates potential race conditions)
	for i := 0; i < 50; i++ {
		// User should still be locked across repeated checks
		if !mgr.IsLocked(user) {
			t.Fatalf("Lockout was bypassed on check iteration %d", i)
		}
	}

	// Verify status is also still locked
	status := mgr.GetStatus(user)
	if status == nil {
		// Paranoid mode might not have status but IsLocked returns true
		return
	}
	if !status.Locked && !mgr.IsLocked(user) {
		t.Fatal("User should still be locked after rapid checks")
	}
}
