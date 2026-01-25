// rbac_integrity_test.go - Tests for NIST 800-53 AU-9 RBAC Integrity Protection
//
// Tests cover:
//   - Signature verification on save/load
//   - Tamper detection (modified data rejected)
//   - Missing/corrupted file handling
//   - Key management (env > file > generate)
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// setupTestRBACManager creates an RBACManager with a temporary storage path.
func setupTestRBACManager(t *testing.T) (*RBACManager, string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_integrity_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storagePath := filepath.Join(tmpDir, "rbac.json")

	// Create manager with custom storage path
	rm, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create RBACManager: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return rm, tmpDir, cleanup
}

// =============================================================================
// SIGNATURE VERIFICATION TESTS
// =============================================================================

func TestRBACIntegrity_SignatureOnSave(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign a role (this triggers save)
	err := rm.AssignRole("admin-user", RBACRoleAdmin, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Read the saved file
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("Failed to read RBAC file: %v", err)
	}

	// Parse and verify signature exists
	var storage rbacStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		t.Fatalf("Failed to unmarshal RBAC data: %v", err)
	}

	if storage.Signature == "" {
		t.Error("AU-9 VIOLATION: RBAC file saved without HMAC signature")
	}

	// Verify signature length (SHA-256 HMAC = 64 hex chars)
	if len(storage.Signature) != 64 {
		t.Errorf("Expected 64-char hex signature, got %d chars", len(storage.Signature))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(storage.Signature)
	if err != nil {
		t.Errorf("Signature is not valid hex: %v", err)
	}
}

func TestRBACIntegrity_SignatureVerificationOnLoad(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign a role
	err := rm.AssignRole("test-user", RBACRoleOperator, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Create a new manager pointing to the same storage
	storagePath := filepath.Join(tmpDir, "rbac.json")
	rm2, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create second RBACManager: %v", err)
	}

	// Verify the role was loaded correctly
	role := rm2.GetUserRole("test-user")
	if role != RBACRoleOperator {
		t.Errorf("Expected role %s, got %s", RBACRoleOperator, role)
	}
}

func TestRBACIntegrity_VerifyRBACIntegrity(t *testing.T) {
	rm, _, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Save some data
	err := rm.AssignRole("auditor-user", RBACRoleAuditor, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Verify integrity
	err = rm.VerifyRBACIntegrity()
	if err != nil {
		t.Errorf("Integrity verification failed for valid file: %v", err)
	}
}

// =============================================================================
// TAMPER DETECTION TESTS
// =============================================================================

func TestRBACIntegrity_DetectTamperedRole(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign a user role
	err := rm.AssignRole("limited-user", RBACRoleUser, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Tamper with the file - change role to admin
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("Failed to read RBAC file: %v", err)
	}

	// Replace "user" role with "admin" (privilege escalation attempt)
	// The JSON format uses ": " with a space, so try both formats
	tamperedData := string(data)
	if strings.Contains(tamperedData, `"role": "user"`) {
		tamperedData = strings.Replace(tamperedData, `"role": "user"`, `"role": "admin"`, 1)
	} else {
		tamperedData = strings.Replace(tamperedData, `"role":"user"`, `"role":"admin"`, 1)
	}

	if tamperedData == string(data) {
		t.Logf("File contents: %s", string(data))
		t.Fatal("Tampering failed - role not found in file")
	}

	err = os.WriteFile(storagePath, []byte(tamperedData), 0600)
	if err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	// Try to load with a new manager - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("AU-9 VIOLATION: Tampered RBAC file was accepted without error")
	}

	// Verify error message indicates integrity failure
	if !strings.Contains(err.Error(), "signature mismatch") && !strings.Contains(err.Error(), "integrity") {
		t.Errorf("Error should mention signature/integrity, got: %v", err)
	}
}

func TestRBACIntegrity_DetectModifiedSignature(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign a role
	err := rm.AssignRole("test-user", RBACRoleOperator, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Read and modify the signature
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("Failed to read RBAC file: %v", err)
	}

	var storage rbacStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Modify signature (attempt to forge)
	storage.Signature = "0000000000000000000000000000000000000000000000000000000000000000"

	tamperedData, _ := json.MarshalIndent(storage, "", "  ")
	err = os.WriteFile(storagePath, tamperedData, 0600)
	if err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	// Try to load - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("AU-9 VIOLATION: File with forged signature was accepted")
	}
}

func TestRBACIntegrity_DetectRemovedSignature(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign a role
	err := rm.AssignRole("test-user", RBACRoleOperator, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Read and remove the signature
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("Failed to read RBAC file: %v", err)
	}

	var storage rbacStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Remove signature
	storage.Signature = ""

	tamperedData, _ := json.MarshalIndent(storage, "", "  ")
	err = os.WriteFile(storagePath, tamperedData, 0600)
	if err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	// Try to load - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("AU-9 VIOLATION: File without signature was accepted")
	}
}

func TestRBACIntegrity_RejectTamperedDoNotFallbackToDefaults(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Assign admin role
	err := rm.AssignRole("admin-user", RBACRoleAdmin, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Tamper with file
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("Failed to read RBAC file: %v", err)
	}

	// Try both JSON formats (with and without space after colon)
	tamperedData := string(data)
	if strings.Contains(tamperedData, `"version": 1`) {
		tamperedData = strings.Replace(tamperedData, `"version": 1`, `"version": 999`, 1)
	} else {
		tamperedData = strings.Replace(tamperedData, `"version":1`, `"version":999`, 1)
	}

	// If version replacement didn't work, try modifying the updated_at timestamp
	if tamperedData == string(data) {
		// Modify any character in the JSON to invalidate signature
		tamperedData = strings.Replace(tamperedData, `"admin-user"`, `"admin-userX"`, 1)
	}

	if tamperedData == string(data) {
		t.Logf("File contents: %s", string(data))
		t.Fatal("Tampering failed - could not modify file")
	}

	err = os.WriteFile(storagePath, []byte(tamperedData), 0600)
	if err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	// Try to create new manager - should FAIL, not fall back to empty defaults
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("AU-9 VIOLATION: Should reject tampered file, not fall back to defaults")
	}
}

// =============================================================================
// MISSING/CORRUPTED FILE TESTS
// =============================================================================

func TestRBACIntegrity_MissingFileAllowed(t *testing.T) {
	// Create temp directory without RBAC file
	tmpDir, err := os.MkdirTemp("", "rbac_missing_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")

	// Create manager - should succeed (missing file is OK for new installations)
	rm, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("NewRBACManager should allow missing file: %v", err)
	}

	// Should have empty roles
	users := rm.ListUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 users for new installation, got %d", len(users))
	}
}

func TestRBACIntegrity_CorruptedJSONRejected(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_corrupt_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")
	keyPath := filepath.Join(tmpDir, ".rbac_hmac_key")

	// Write a valid HMAC key first
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	err = os.WriteFile(keyPath, key, 0600)
	if err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}

	// Write corrupted JSON
	err = os.WriteFile(storagePath, []byte("{invalid json"), 0600)
	if err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	// Create manager - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("Should reject corrupted JSON file")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Error should mention JSON parsing, got: %v", err)
	}
}

func TestRBACIntegrity_TruncatedFileRejected(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Save valid data
	err := rm.AssignRole("test-user", RBACRoleUser, "")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Truncate the file
	storagePath := filepath.Join(tmpDir, "rbac.json")
	data, _ := os.ReadFile(storagePath)
	truncated := data[:len(data)/2] // Cut in half
	err = os.WriteFile(storagePath, truncated, 0600)
	if err != nil {
		t.Fatalf("Failed to write truncated file: %v", err)
	}

	// Try to load - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("Should reject truncated file")
	}
}

// =============================================================================
// KEY MANAGEMENT TESTS
// =============================================================================

func TestRBACIntegrity_KeyFromEnvironment(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_env_key_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")

	// Set environment variable with valid hex key
	testKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv(RBACHMACKeyEnvVar, testKey)
	defer os.Unsetenv(RBACHMACKeyEnvVar)

	// Create manager
	rm, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create manager with env key: %v", err)
	}

	// Verify key was loaded from env (by checking key length)
	if len(rm.hmacKey) != 32 {
		t.Errorf("Expected 32-byte key from env, got %d bytes", len(rm.hmacKey))
	}

	// Verify the key matches
	expectedKey, _ := hex.DecodeString(testKey)
	if !hmac.Equal(rm.hmacKey, expectedKey) {
		t.Error("Key from environment doesn't match expected value")
	}
}

func TestRBACIntegrity_KeyFromFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_file_key_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")
	keyPath := filepath.Join(tmpDir, ".rbac_hmac_key")

	// Write a test key file
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i * 2)
	}
	err = os.WriteFile(keyPath, testKey, 0600)
	if err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// Clear environment variable
	os.Unsetenv(RBACHMACKeyEnvVar)

	// Create manager
	rm, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create manager with file key: %v", err)
	}

	// Verify key was loaded from file
	if !hmac.Equal(rm.hmacKey, testKey) {
		t.Error("Key from file doesn't match expected value")
	}
}

func TestRBACIntegrity_KeyGenerated(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_gen_key_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")
	keyPath := filepath.Join(tmpDir, ".rbac_hmac_key")

	// Clear environment variable
	os.Unsetenv(RBACHMACKeyEnvVar)

	// Ensure no key file exists
	os.Remove(keyPath)

	// Create manager - should generate key
	rm, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Verify key was generated
	if len(rm.hmacKey) != 32 {
		t.Errorf("Expected 32-byte generated key, got %d bytes", len(rm.hmacKey))
	}

	// Verify key file was created
	savedKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Key file should have been created: %v", err)
	}

	if !hmac.Equal(rm.hmacKey, savedKey) {
		t.Error("Saved key doesn't match manager's key")
	}
}

func TestRBACIntegrity_InvalidEnvKeyRejected(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_bad_env_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")

	// Set invalid (non-hex) environment key
	os.Setenv(RBACHMACKeyEnvVar, "not-valid-hex-key")
	defer os.Unsetenv(RBACHMACKeyEnvVar)

	// Create manager - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("Should reject invalid hex key from environment")
	}

	if !strings.Contains(err.Error(), "hex") {
		t.Errorf("Error should mention hex encoding, got: %v", err)
	}
}

func TestRBACIntegrity_WrongSizeEnvKeyRejected(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_short_env_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")

	// Set too-short key (16 bytes instead of 32)
	os.Setenv(RBACHMACKeyEnvVar, "0123456789abcdef0123456789abcdef")
	defer os.Unsetenv(RBACHMACKeyEnvVar)

	// Create manager - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("Should reject wrong-size key from environment")
	}

	if !strings.Contains(err.Error(), "32 bytes") {
		t.Errorf("Error should mention required key size, got: %v", err)
	}
}

func TestRBACIntegrity_WrongSizeKeyFileRejected(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rbac_short_file_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "rbac.json")
	keyPath := filepath.Join(tmpDir, ".rbac_hmac_key")

	// Clear environment
	os.Unsetenv(RBACHMACKeyEnvVar)

	// Write a too-short key file
	err = os.WriteFile(keyPath, []byte("tooshort"), 0600)
	if err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// Create manager - should fail
	_, err = NewRBACManager(WithRBACStoragePath(storagePath))
	if err == nil {
		t.Error("Should reject wrong-size key file")
	}

	if !strings.Contains(err.Error(), "invalid size") {
		t.Errorf("Error should mention invalid size, got: %v", err)
	}
}

// =============================================================================
// HMAC COMPUTATION TESTS
// =============================================================================

func TestRBACIntegrity_HMACDeterministic(t *testing.T) {
	rm, _, cleanup := setupTestRBACManager(t)
	defer cleanup()

	testData := []byte(`{"test":"data","version":1}`)

	// Compute HMAC twice
	hmac1 := rm.computeRBACHMAC(testData)
	hmac2 := rm.computeRBACHMAC(testData)

	if hmac1 != hmac2 {
		t.Error("HMAC should be deterministic for same data and key")
	}
}

func TestRBACIntegrity_HMACDifferentForDifferentData(t *testing.T) {
	rm, _, cleanup := setupTestRBACManager(t)
	defer cleanup()

	data1 := []byte(`{"role":"user"}`)
	data2 := []byte(`{"role":"admin"}`)

	hmac1 := rm.computeRBACHMAC(data1)
	hmac2 := rm.computeRBACHMAC(data2)

	if hmac1 == hmac2 {
		t.Error("HMAC should be different for different data")
	}
}

func TestRBACIntegrity_HMACUsesSHA256(t *testing.T) {
	rm, _, cleanup := setupTestRBACManager(t)
	defer cleanup()

	testData := []byte("test")
	result := rm.computeRBACHMAC(testData)

	// SHA-256 HMAC produces 32 bytes = 64 hex chars
	if len(result) != 64 {
		t.Errorf("Expected 64-char hex (SHA-256), got %d chars", len(result))
	}

	// Verify against standard library
	mac := hmac.New(sha256.New, rm.hmacKey)
	mac.Write(testData)
	expected := hex.EncodeToString(mac.Sum(nil))

	if result != expected {
		t.Errorf("HMAC mismatch: got %s, expected %s", result, expected)
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestRBACIntegrity_ConcurrentSaveLoad(t *testing.T) {
	rm, _, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Run concurrent operations
	done := make(chan bool)
	errors := make(chan error, 100)

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				userID := "user-" + string(rune('A'+id))
				err := rm.AssignRole(userID, RBACRoleUser, "")
				if err != nil {
					errors <- err
				}
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = rm.ListUsers()
				_ = rm.VerifyRBACIntegrity()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

// =============================================================================
// ROUND-TRIP TESTS
// =============================================================================

func TestRBACIntegrity_RoundTrip(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// Create various role assignments
	testCases := []struct {
		userID string
		role   RBACRole
	}{
		{"admin-1", RBACRoleAdmin},
		{"operator-1", RBACRoleOperator},
		{"auditor-1", RBACRoleAuditor},
		{"user-1", RBACRoleUser},
	}

	for _, tc := range testCases {
		err := rm.AssignRole(tc.userID, tc.role, "")
		if err != nil {
			t.Fatalf("Failed to assign role for %s: %v", tc.userID, err)
		}
	}

	// Create new manager and load data
	storagePath := filepath.Join(tmpDir, "rbac.json")
	rm2, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	// Verify all roles survived round-trip
	for _, tc := range testCases {
		role := rm2.GetUserRole(tc.userID)
		if role != tc.role {
			t.Errorf("Round-trip failed for %s: expected %s, got %s", tc.userID, tc.role, role)
		}
	}

	// Verify integrity
	err = rm2.VerifyRBACIntegrity()
	if err != nil {
		t.Errorf("Integrity check failed after round-trip: %v", err)
	}
}

func TestRBACIntegrity_TimestampPreserved(t *testing.T) {
	rm, tmpDir, cleanup := setupTestRBACManager(t)
	defer cleanup()

	// First assign an admin user (required for subsequent assignments with assignedBy)
	err := rm.AssignRole("bootstrap-admin", RBACRoleAdmin, "")
	if err != nil {
		t.Fatalf("Failed to create bootstrap admin: %v", err)
	}

	// Assign role with assignedBy set
	beforeAssign := time.Now()
	err = rm.AssignRole("test-user", RBACRoleUser, "bootstrap-admin")
	if err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}
	afterAssign := time.Now()

	// Load in new manager
	storagePath := filepath.Join(tmpDir, "rbac.json")
	rm2, err := NewRBACManager(WithRBACStoragePath(storagePath))
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	// Check timestamp
	details, exists := rm2.GetUserRoleDetails("test-user")
	if !exists {
		t.Fatal("User not found after reload")
	}

	if details.AssignedAt.Before(beforeAssign) || details.AssignedAt.After(afterAssign) {
		t.Errorf("AssignedAt timestamp not in expected range: %v not in [%v, %v]",
			details.AssignedAt, beforeAssign, afterAssign)
	}

	if details.AssignedBy != "bootstrap-admin" {
		t.Errorf("AssignedBy not preserved: expected 'bootstrap-admin', got '%s'", details.AssignedBy)
	}
}
