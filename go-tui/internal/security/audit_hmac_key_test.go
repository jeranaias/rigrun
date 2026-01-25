// audit_hmac_key_test.go - Tests for NIST 800-53 AU-9 HMAC Key Management
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// setupTestDir creates a temporary directory for testing.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "audit_hmac_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// setEnvVar sets an environment variable and returns a cleanup function.
func setEnvVar(t *testing.T, key, value string) {
	t.Helper()
	oldValue, exists := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, oldValue)
		} else {
			os.Unsetenv(key)
		}
	})
}

// clearEnvVar clears an environment variable and returns a cleanup function.
func clearEnvVar(t *testing.T, key string) {
	t.Helper()
	oldValue, exists := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, oldValue)
		}
	})
}

// =============================================================================
// TESTS: KEY LOADING FROM ENVIRONMENT
// =============================================================================

func TestLoadKey_FromEnvironment_HexEncoded(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	manager := NewAuditHMACKeyManager(dir)

	// Generate a 32-byte key and hex-encode it
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i)
	}
	hexKey := hex.EncodeToString(testKey)
	setEnvVar(t, AuditHMACKeyEnvVar, hexKey)

	// Test
	key, source, err := manager.LoadKey()

	// Verify
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceEnv {
		t.Errorf("expected source %s, got %s", KeySourceEnv, source)
	}
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(key))
	}
	for i, b := range key {
		if b != byte(i) {
			t.Errorf("key byte %d mismatch: expected %d, got %d", i, i, b)
		}
	}
}

func TestLoadKey_FromEnvironment_RawBytes_Rejected(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	manager := NewAuditHMACKeyManager(dir)

	// SECURITY TEST: Raw ASCII bytes should be REJECTED to prevent weak passwords
	// This prevents users from using "mypassword" or other weak strings as keys
	rawKey := "this-is-a-test-key-with-32-chars" // 32 characters but NOT hex-encoded
	setEnvVar(t, AuditHMACKeyEnvVar, rawKey)

	// Test
	_, _, err := manager.LoadKey()

	// Verify - should FAIL because raw bytes are not allowed (must be hex-encoded)
	if err == nil {
		t.Fatal("expected error for raw ASCII bytes, got nil - SECURITY VIOLATION")
	}
	// Error should mention hex-encoding requirement
	errStr := err.Error()
	if !strings.Contains(errStr, "hex-encoded") && !strings.Contains(errStr, "hex") {
		t.Errorf("error should mention hex-encoding requirement, got: %v", err)
	}
}

func TestLoadKey_FromEnvironment_InvalidHex(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	manager := NewAuditHMACKeyManager(dir)

	// SECURITY TEST: Invalid hex should be rejected (e.g., contains 'G' which is not hex)
	invalidHex := "GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG"
	setEnvVar(t, AuditHMACKeyEnvVar, invalidHex)

	// Test
	_, _, err := manager.LoadKey()

	// Verify - should FAIL because the string is not valid hex
	if err == nil {
		t.Fatal("expected error for invalid hex, got nil - SECURITY VIOLATION")
	}
	// Error should mention hex-encoding requirement
	errStr := err.Error()
	if !strings.Contains(errStr, "hex-encoded") && !strings.Contains(errStr, "hex") {
		t.Errorf("error should mention hex-encoding requirement, got: %v", err)
	}
}

func TestLoadKey_FromEnvironment_TooShort(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	manager := NewAuditHMACKeyManager(dir)

	// Use a hex-encoded key that's too short (16 bytes = 32 hex chars, need 32 bytes = 64 hex chars)
	shortKey := hex.EncodeToString([]byte("tooshort"))
	setEnvVar(t, AuditHMACKeyEnvVar, shortKey)

	// Test
	_, _, err := manager.LoadKey()

	// Verify - should fail because key is too short
	if err == nil {
		t.Fatal("expected error for short key, got nil")
	}
	// Error should mention minimum key length
	errStr := err.Error()
	if !strings.Contains(errStr, "32 bytes") && !strings.Contains(errStr, "256 bits") {
		t.Errorf("error should mention minimum key length requirement, got: %v", err)
	}
}

// =============================================================================
// TESTS: KEY LOADING FROM FILE
// =============================================================================

func TestLoadKey_FromFile_DefaultLocation(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	clearEnvVar(t, AuditHMACKeyEnvVar)
	clearEnvVar(t, AuditHMACKeyFileEnvVar)

	// Create a key file at the default location
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i * 2)
	}
	keyFile := filepath.Join(dir, HMACKeyFileName)
	if err := os.WriteFile(keyFile, testKey, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	manager := NewAuditHMACKeyManager(dir)

	// Test
	key, source, err := manager.LoadKey()

	// Verify
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceFile {
		t.Errorf("expected source %s, got %s", KeySourceFile, source)
	}
	for i, b := range key {
		if b != byte(i*2) {
			t.Errorf("key byte %d mismatch: expected %d, got %d", i, i*2, b)
		}
	}
}

func TestLoadKey_FromFile_CustomLocation(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	clearEnvVar(t, AuditHMACKeyEnvVar)

	// Create a key file at a custom location
	customKeyFile := filepath.Join(dir, "custom_key.bin")
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i * 3)
	}
	if err := os.WriteFile(customKeyFile, testKey, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	// Set the custom file path via environment variable
	setEnvVar(t, AuditHMACKeyFileEnvVar, customKeyFile)

	manager := NewAuditHMACKeyManager(dir)

	// Test
	key, source, err := manager.LoadKey()

	// Verify
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceFile {
		t.Errorf("expected source %s, got %s", KeySourceFile, source)
	}
	for i, b := range key {
		if b != byte(i*3) {
			t.Errorf("key byte %d mismatch: expected %d, got %d", i, i*3, b)
		}
	}
}

func TestLoadKey_FromFile_InsecurePermissions(t *testing.T) {
	// Skip on Windows - permissions work differently
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}

	// Setup
	dir := setupTestDir(t)
	clearEnvVar(t, AuditHMACKeyEnvVar)
	clearEnvVar(t, AuditHMACKeyFileEnvVar)

	// Create a key file with insecure permissions (world-readable)
	testKey := make([]byte, 32)
	keyFile := filepath.Join(dir, HMACKeyFileName)
	if err := os.WriteFile(keyFile, testKey, 0644); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	manager := NewAuditHMACKeyManager(dir)

	// Test
	_, _, err := manager.LoadKey()

	// Verify - should fail because permissions are too permissive
	if err == nil {
		t.Fatal("expected error for insecure permissions, got nil")
	}
}

// =============================================================================
// TESTS: NO KEY CONFIGURED (SHOULD FAIL)
// =============================================================================

func TestLoadKey_NoKeyConfigured(t *testing.T) {
	// Setup
	dir := setupTestDir(t)
	clearEnvVar(t, AuditHMACKeyEnvVar)
	clearEnvVar(t, AuditHMACKeyFileEnvVar)

	manager := NewAuditHMACKeyManager(dir)

	// Test - no key file exists, no env var set
	_, source, err := manager.LoadKey()

	// Verify - should fail with ErrNoHMACKeyConfigured
	if err == nil {
		t.Fatal("expected ErrNoHMACKeyConfigured, got nil")
	}
	if source != KeySourceNone {
		t.Errorf("expected source %s, got %s", KeySourceNone, source)
	}
}

// =============================================================================
// TESTS: KEY PRIORITY (ENV > FILE)
// =============================================================================

func TestLoadKey_EnvironmentTakesPriority(t *testing.T) {
	// Setup
	dir := setupTestDir(t)

	// Create both env var and file
	envKey := make([]byte, 32)
	for i := range envKey {
		envKey[i] = 0xAA // Distinctive pattern for env key
	}
	setEnvVar(t, AuditHMACKeyEnvVar, hex.EncodeToString(envKey))

	fileKey := make([]byte, 32)
	for i := range fileKey {
		fileKey[i] = 0xBB // Distinctive pattern for file key
	}
	keyFile := filepath.Join(dir, HMACKeyFileName)
	if err := os.WriteFile(keyFile, fileKey, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	manager := NewAuditHMACKeyManager(dir)

	// Test
	key, source, err := manager.LoadKey()

	// Verify - environment should take priority
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceEnv {
		t.Errorf("expected source %s, got %s", KeySourceEnv, source)
	}
	// Check key is the env key (0xAA pattern)
	for i, b := range key {
		if b != 0xAA {
			t.Errorf("key byte %d should be 0xAA, got 0x%02X", i, b)
		}
	}
}

// =============================================================================
// TESTS: KEY GENERATION UTILITY
// =============================================================================

func TestGenerateAuditHMACKey(t *testing.T) {
	key, err := GenerateAuditHMACKey()
	if err != nil {
		t.Fatalf("GenerateAuditHMACKey failed: %v", err)
	}
	if len(key) != MinHMACKeyLength {
		t.Errorf("expected %d-byte key, got %d bytes", MinHMACKeyLength, len(key))
	}

	// Generate another key and verify they're different (not deterministic)
	key2, err := GenerateAuditHMACKey()
	if err != nil {
		t.Fatalf("GenerateAuditHMACKey failed: %v", err)
	}

	same := true
	for i := range key {
		if key[i] != key2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("two generated keys should not be identical")
	}
}

func TestGenerateAndSaveAuditHMACKey(t *testing.T) {
	dir := setupTestDir(t)
	keyPath := filepath.Join(dir, "generated_key.bin")

	err := GenerateAndSaveAuditHMACKey(keyPath)
	if err != nil {
		t.Fatalf("GenerateAndSaveAuditHMACKey failed: %v", err)
	}

	// Verify file exists and has correct permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}

	// Check permissions on Unix
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
		}
	}

	// Verify key can be loaded
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read key file: %v", err)
	}
	if len(key) != MinHMACKeyLength {
		t.Errorf("expected %d-byte key, got %d bytes", MinHMACKeyLength, len(key))
	}
}

// =============================================================================
// TESTS: KEY ROTATION
// =============================================================================

func TestRotateKey(t *testing.T) {
	// Setup
	dir := setupTestDir(t)

	// Create initial key file
	initialKey := make([]byte, 32)
	for i := range initialKey {
		initialKey[i] = byte(i)
	}
	keyFile := filepath.Join(dir, HMACKeyFileName)
	if err := os.WriteFile(keyFile, initialKey, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	clearEnvVar(t, AuditHMACKeyEnvVar)
	clearEnvVar(t, AuditHMACKeyFileEnvVar)

	manager := NewAuditHMACKeyManager(dir)

	// Load initial key
	_, _, err := manager.LoadKey()
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}

	// Rotate key (without re-signing entries)
	result, err := manager.RotateKey(nil, false)
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}

	// Verify result
	if result.OldKeyFingerprint == "" {
		t.Error("old key fingerprint should not be empty")
	}
	if result.NewKeyFingerprint == "" {
		t.Error("new key fingerprint should not be empty")
	}
	if result.OldKeyFingerprint == result.NewKeyFingerprint {
		t.Error("old and new key fingerprints should be different")
	}

	// Verify new key is different from initial
	newKey := manager.GetCurrentKey()
	for i, b := range newKey {
		if b == byte(i) {
			// If all bytes match the initial pattern, that's a problem
			continue
		} else {
			// Good - at least one byte is different
			return
		}
	}
	t.Error("new key should be different from initial key")
}

func TestGetKeyMetadata(t *testing.T) {
	// Setup
	dir := setupTestDir(t)

	// Create key file
	testKey := make([]byte, 32)
	keyFile := filepath.Join(dir, HMACKeyFileName)
	if err := os.WriteFile(keyFile, testKey, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	clearEnvVar(t, AuditHMACKeyEnvVar)
	clearEnvVar(t, AuditHMACKeyFileEnvVar)

	manager := NewAuditHMACKeyManager(dir)

	// Load key
	_, _, err := manager.LoadKey()
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}

	// Get metadata
	metadata := manager.GetKeyMetadata()

	// Verify
	if metadata == nil {
		t.Fatal("metadata should not be nil")
	}
	if metadata.KeyID == "" {
		t.Error("KeyID should not be empty")
	}
	if metadata.Source != KeySourceFile {
		t.Errorf("expected source %s, got %s", KeySourceFile, metadata.Source)
	}
	if metadata.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}
	if metadata.Rotated {
		t.Error("key should not be marked as rotated initially")
	}
}

// =============================================================================
// TESTS: COMPUTE HMAC
// =============================================================================

func TestComputeHMACWithKey(t *testing.T) {
	key := []byte("test-key-for-hmac-computation!!")
	data := []byte("test data to hash")

	hash1 := computeHMACWithKey(data, key)

	// Verify hash is not empty
	if hash1 == "" {
		t.Error("HMAC hash should not be empty")
	}

	// Verify same input produces same output
	hash2 := computeHMACWithKey(data, key)
	if hash1 != hash2 {
		t.Error("same input should produce same HMAC hash")
	}

	// Verify different key produces different output
	key2 := []byte("different-key-for-hmac-test!!!!")
	hash3 := computeHMACWithKey(data, key2)
	if hash1 == hash3 {
		t.Error("different key should produce different HMAC hash")
	}

	// Verify different data produces different output
	data2 := []byte("different test data")
	hash4 := computeHMACWithKey(data2, key)
	if hash1 == hash4 {
		t.Error("different data should produce different HMAC hash")
	}
}
