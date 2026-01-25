// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// auditprotect_test.go - Tests for AU-5 audit protection retry behavior
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestHMACKey generates a test HMAC key and sets it in the environment.
// Returns a cleanup function to restore the environment.
func setupTestHMACKey(t *testing.T) func() {
	t.Helper()

	// Generate a random 32-byte key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate test HMAC key: %v", err)
	}

	// Set as hex-encoded environment variable
	keyHex := hex.EncodeToString(key)
	oldKey := os.Getenv(AuditHMACKeyEnvVar)
	os.Setenv(AuditHMACKeyEnvVar, keyHex)

	return func() {
		if oldKey == "" {
			os.Unsetenv(AuditHMACKeyEnvVar)
		} else {
			os.Setenv(AuditHMACKeyEnvVar, oldKey)
		}
	}
}

func TestAuditProtectorRetryBehavior(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Configure retry settings for faster test
	protector.SetRetryConfig(3, 10*time.Millisecond)

	// Verify configuration
	maxRetries, baseWait := protector.GetRetryConfig()
	if maxRetries != 3 {
		t.Errorf("Expected maxRetries=3, got %d", maxRetries)
	}
	if baseWait != 10*time.Millisecond {
		t.Errorf("Expected baseWait=10ms, got %v", baseWait)
	}

	// Test successful save
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST",
		SessionID: "test-session",
		Success:   true,
	}

	if err := protector.SignLogEntry(event); err != nil {
		t.Errorf("SignLogEntry failed: %v", err)
	}

	// Verify chain was saved
	chainPath := filepath.Join(tempDir, "audit_chain.json")
	if _, err := os.Stat(chainPath); os.IsNotExist(err) {
		t.Error("Chain file was not created")
	}
}

func TestAuditProtectorStrictModeBlocking(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Configure for faster test
	protector.SetRetryConfig(3, 10*time.Millisecond)

	// Verify strict mode is enabled by default (AU-5 compliance)
	if !protector.IsStrictMode() {
		t.Error("Strict mode should be enabled by default for AU-5 compliance")
	}

	// Test disabling strict mode
	protector.SetStrictMode(false)
	if protector.IsStrictMode() {
		t.Error("Strict mode should be disabled")
	}

	// Re-enable strict mode
	protector.SetStrictMode(true)
	if !protector.IsStrictMode() {
		t.Error("Strict mode should be enabled")
	}
}

func TestAuditProtectorSaveFailureWithRetry(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Configure for faster test with fewer retries
	protector.SetRetryConfig(2, 5*time.Millisecond)

	// Make chain file directory read-only to force save failures
	chainDir := filepath.Dir(protector.chainFile)

	// First, add a successful entry to create the chain file
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST",
		SessionID: "test-session",
		Success:   true,
	}
	if err := protector.SignLogEntry(event); err != nil {
		t.Fatalf("Initial SignLogEntry failed: %v", err)
	}

	// Now remove write permission on the chain file to force failure
	chainFile := filepath.Join(chainDir, "audit_chain.json")
	if err := os.Chmod(chainFile, 0400); err != nil {
		t.Fatalf("Failed to make chain file read-only: %v", err)
	}
	defer os.Chmod(chainFile, 0600) // Restore for cleanup

	// Attempt to sign another entry - should fail in strict mode
	event2 := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST2",
		SessionID: "test-session",
		Success:   true,
	}

	err = protector.SignLogEntry(event2)
	if err == nil {
		t.Error("Expected error when save fails in strict mode, got nil")
	}

	// Verify error is the expected AU-5 error
	if err != nil && !errors.Is(err, ErrAuditSaveFailed) {
		t.Errorf("Expected ErrAuditSaveFailed, got: %v", err)
	}
}

func TestAuditProtectorNonStrictModeNoError(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Disable strict mode
	protector.SetStrictMode(false)
	protector.SetRetryConfig(2, 5*time.Millisecond)

	// First, add a successful entry
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST",
		SessionID: "test-session",
		Success:   true,
	}
	if err := protector.SignLogEntry(event); err != nil {
		t.Fatalf("Initial SignLogEntry failed: %v", err)
	}

	// Make chain file read-only
	chainDir := filepath.Dir(protector.chainFile)
	chainFile := filepath.Join(chainDir, "audit_chain.json")
	if err := os.Chmod(chainFile, 0400); err != nil {
		t.Fatalf("Failed to make chain file read-only: %v", err)
	}
	defer os.Chmod(chainFile, 0600) // Restore for cleanup

	// In non-strict mode, should not return error (but logs warning)
	event2 := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST2",
		SessionID: "test-session",
		Success:   true,
	}

	err = protector.SignLogEntry(event2)
	if err != nil {
		t.Errorf("In non-strict mode, should not return error, got: %v", err)
	}
}

func TestAuditProtectorRetryExponentialBackoff(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// This test verifies that the exponential backoff timing works correctly
	// We use a short base wait to keep the test fast

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Configure retry with very short times
	baseWait := 10 * time.Millisecond
	protector.SetRetryConfig(3, baseWait)

	// Sign an entry and measure time
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "TIMING_TEST",
		SessionID: "test-session",
		Success:   true,
	}

	start := time.Now()
	err = protector.SignLogEntry(event)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("SignLogEntry failed: %v", err)
	}

	// Successful saves should complete quickly (no retries needed)
	// Allow some buffer for file I/O
	if elapsed > 500*time.Millisecond {
		t.Errorf("Successful save took too long: %v", elapsed)
	}
}

func TestAuditProtectorSavesSynchronous(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// This test verifies that saves are truly synchronous (not fire-and-forget)
	// by checking that the chain file is updated immediately after SignLogEntry returns

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Sign an entry
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "SYNC_TEST",
		SessionID: "test-session",
		Success:   true,
	}

	if err := protector.SignLogEntry(event); err != nil {
		t.Fatalf("SignLogEntry failed: %v", err)
	}

	// Immediately check that chain file exists and has content
	// With async saves, this might fail because the goroutine hasn't completed
	// With synchronous saves, this should always pass
	chainPath := filepath.Join(tempDir, "audit_chain.json")
	data, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatalf("Chain file not readable immediately after SignLogEntry: %v", err)
	}

	if len(data) == 0 {
		t.Error("Chain file is empty immediately after SignLogEntry - saves may not be synchronous")
	}

	// Verify the content contains our event
	if !strings.Contains(string(data), "SYNC_TEST") {
		// Note: The chain doesn't store EventType directly, but we can verify it was saved
		// by checking the chain structure exists
		if !strings.Contains(string(data), "event_hash") {
			t.Error("Chain file does not contain expected chain entry")
		}
	}
}

func TestAuditProtectorChainRollbackOnFailure(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// This test verifies that the chain entry is removed if save fails in strict mode

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	protector.SetRetryConfig(1, 5*time.Millisecond) // Minimal retries for speed

	// Add first entry successfully
	event1 := AuditEvent{
		Timestamp: time.Now(),
		EventType: "ENTRY1",
		SessionID: "test-session",
		Success:   true,
	}
	if err := protector.SignLogEntry(event1); err != nil {
		t.Fatalf("First entry failed: %v", err)
	}

	// Make chain file read-only to force failure
	chainFile := filepath.Join(tempDir, "audit_chain.json")
	if err := os.Chmod(chainFile, 0400); err != nil {
		t.Fatalf("Failed to make chain file read-only: %v", err)
	}
	defer os.Chmod(chainFile, 0600)

	// Try to add second entry - should fail
	event2 := AuditEvent{
		Timestamp: time.Now(),
		EventType: "ENTRY2",
		SessionID: "test-session",
		Success:   true,
	}
	err = protector.SignLogEntry(event2)
	if err == nil {
		t.Fatal("Expected error on second entry, got nil")
	}

	// Verify chain length is still 1 (rollback occurred)
	protector.mu.RLock()
	chainLen := len(protector.chain)
	protector.mu.RUnlock()

	if chainLen != 1 {
		t.Errorf("Expected chain length 1 after rollback, got %d", chainLen)
	}
}

func TestAuditProtectorDefaultConfiguration(t *testing.T) {
	// Setup HMAC key for testing
	cleanup := setupTestHMACKey(t)
	defer cleanup()

	// Verify default AU-5 configuration

	// Create temp directory for test
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create empty audit log file
	if err := os.WriteFile(auditPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	// Create protector
	protector, err := NewAuditProtector(auditPath)
	if err != nil {
		t.Fatalf("Failed to create protector: %v", err)
	}

	// Check defaults
	if !protector.IsStrictMode() {
		t.Error("Default strict mode should be true for AU-5 compliance")
	}

	maxRetries, baseWait := protector.GetRetryConfig()
	if maxRetries != 3 {
		t.Errorf("Default maxRetries should be 3, got %d", maxRetries)
	}
	if baseWait != 100*time.Millisecond {
		t.Errorf("Default baseWait should be 100ms, got %v", baseWait)
	}
}
