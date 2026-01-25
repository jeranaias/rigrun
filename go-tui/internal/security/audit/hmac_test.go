// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection.
//
// This test file covers NIST 800-53 AU-9: HMAC Key Management for Audit Logs.
package audit

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// HMAC KEY MANAGER TESTS
// =============================================================================

func TestHMACKeyManager_LoadKey_FromEnvVar(t *testing.T) {
	// Generate a valid 32-byte key (hex-encoded)
	testKey := make([]byte, KeySize)
	for i := range testKey {
		testKey[i] = byte(i)
	}
	testKeyHex := hex.EncodeToString(testKey)

	// Save and set env var
	originalEnv := os.Getenv(HMACKeyEnvVar)
	os.Setenv(HMACKeyEnvVar, testKeyHex)
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(HMACKeyEnvVar)
		}
	}()

	manager := NewHMACKeyManager(t.TempDir())
	key, source, err := manager.LoadKey()

	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceEnvVar {
		t.Errorf("Expected source %s, got %s", KeySourceEnvVar, source)
	}
	if len(key) != KeySize {
		t.Errorf("Expected key size %d, got %d", KeySize, len(key))
	}
	if string(key) != string(testKey) {
		t.Error("Key content mismatch")
	}

	// Verify metadata
	meta := manager.GetKeyMetadata()
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}
	if meta.Source != KeySourceEnvVar {
		t.Errorf("Expected metadata source %s, got %s", KeySourceEnvVar, meta.Source)
	}
	if meta.KeySize != KeySize {
		t.Errorf("Expected metadata key size %d, got %d", KeySize, meta.KeySize)
	}
}

func TestHMACKeyManager_LoadKey_FromEnvFile(t *testing.T) {
	// Clear direct env var first
	originalEnv := os.Getenv(HMACKeyEnvVar)
	os.Unsetenv(HMACKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		}
	}()

	// Create temp key file
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test_key")
	testKey := make([]byte, KeySize)
	for i := range testKey {
		testKey[i] = byte(i + 100)
	}
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("Failed to write test key file: %v", err)
	}

	// Set env file path
	originalFileEnv := os.Getenv(HMACKeyFileEnvVar)
	os.Setenv(HMACKeyFileEnvVar, keyPath)
	defer func() {
		if originalFileEnv != "" {
			os.Setenv(HMACKeyFileEnvVar, originalFileEnv)
		} else {
			os.Unsetenv(HMACKeyFileEnvVar)
		}
	}()

	manager := NewHMACKeyManager(tempDir)
	key, source, err := manager.LoadKey()

	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceEnvFile {
		t.Errorf("Expected source %s, got %s", KeySourceEnvFile, source)
	}
	if string(key) != string(testKey) {
		t.Error("Key content mismatch")
	}
}

func TestHMACKeyManager_LoadKey_FromDefaultFile(t *testing.T) {
	// Clear env vars
	originalEnv := os.Getenv(HMACKeyEnvVar)
	originalFileEnv := os.Getenv(HMACKeyFileEnvVar)
	os.Unsetenv(HMACKeyEnvVar)
	os.Unsetenv(HMACKeyFileEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		}
		if originalFileEnv != "" {
			os.Setenv(HMACKeyFileEnvVar, originalFileEnv)
		}
	}()

	// Create default key file
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, DefaultKeyFileName)
	testKey := make([]byte, KeySize)
	for i := range testKey {
		testKey[i] = byte(i + 50)
	}
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("Failed to write default key file: %v", err)
	}

	manager := NewHMACKeyManager(tempDir)
	key, source, err := manager.LoadKey()

	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}
	if source != KeySourceDefault {
		t.Errorf("Expected source %s, got %s", KeySourceDefault, source)
	}
	if string(key) != string(testKey) {
		t.Error("Key content mismatch")
	}
}

func TestHMACKeyManager_LoadKey_NoKeyConfigured(t *testing.T) {
	// Clear all env vars
	originalEnv := os.Getenv(HMACKeyEnvVar)
	originalFileEnv := os.Getenv(HMACKeyFileEnvVar)
	os.Unsetenv(HMACKeyEnvVar)
	os.Unsetenv(HMACKeyFileEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		}
		if originalFileEnv != "" {
			os.Setenv(HMACKeyFileEnvVar, originalFileEnv)
		}
	}()

	// Use empty temp dir (no default key file)
	tempDir := t.TempDir()

	manager := NewHMACKeyManager(tempDir)
	_, source, err := manager.LoadKey()

	// Should fail - no key configured
	if err == nil {
		t.Error("Expected error when no key is configured")
	}
	if source != KeySourceNone {
		t.Errorf("Expected source %s, got %s", KeySourceNone, source)
	}
}

func TestHMACKeyManager_LoadKey_InvalidHexKey(t *testing.T) {
	// Set invalid hex key
	originalEnv := os.Getenv(HMACKeyEnvVar)
	os.Setenv(HMACKeyEnvVar, "not-valid-hex!")
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(HMACKeyEnvVar)
		}
	}()

	manager := NewHMACKeyManager(t.TempDir())
	_, _, err := manager.LoadKey()

	if err == nil {
		t.Error("Expected error for invalid hex key")
	}
}

func TestHMACKeyManager_LoadKey_WrongKeySize(t *testing.T) {
	// Set key with wrong size (16 bytes instead of 32)
	shortKey := make([]byte, 16)
	originalEnv := os.Getenv(HMACKeyEnvVar)
	os.Setenv(HMACKeyEnvVar, hex.EncodeToString(shortKey))
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(HMACKeyEnvVar)
		}
	}()

	manager := NewHMACKeyManager(t.TempDir())
	_, _, err := manager.LoadKey()

	if err == nil {
		t.Error("Expected error for wrong key size")
	}
}

func TestHMACKeyManager_GetCurrentKey(t *testing.T) {
	manager := NewHMACKeyManager(t.TempDir())

	// Before loading, key should be nil
	if manager.GetCurrentKey() != nil {
		t.Error("Expected nil key before loading")
	}

	// Load a key
	testKey := make([]byte, KeySize)
	os.Setenv(HMACKeyEnvVar, hex.EncodeToString(testKey))
	defer os.Unsetenv(HMACKeyEnvVar)

	manager.LoadKey()

	// Now key should be available
	if manager.GetCurrentKey() == nil {
		t.Error("Expected key after loading")
	}
}

func TestHMACKeyManager_Close(t *testing.T) {
	manager := NewHMACKeyManager(t.TempDir())

	// Load a key
	testKey := make([]byte, KeySize)
	for i := range testKey {
		testKey[i] = byte(i)
	}
	os.Setenv(HMACKeyEnvVar, hex.EncodeToString(testKey))
	defer os.Unsetenv(HMACKeyEnvVar)

	manager.LoadKey()

	// Close should zero and clear the key
	manager.Close()

	if manager.GetCurrentKey() != nil {
		t.Error("Key should be nil after Close")
	}
}

// =============================================================================
// KEY GENERATION TESTS
// =============================================================================

func TestGenerateAuditHMACKey(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test_key")

	err := GenerateAuditHMACKey(keyPath)
	if err != nil {
		t.Fatalf("GenerateAuditHMACKey failed: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Key file not created: %v", err)
	}

	// Note: Permissions check skipped - Unix-style permissions (0600) don't apply on Windows

	// Verify key size
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}
	if len(key) != KeySize {
		t.Errorf("Expected key size %d, got %d", KeySize, len(key))
	}
}

func TestGenerateAuditHMACKey_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "subdir", "deep", "test_key")

	err := GenerateAuditHMACKey(keyPath)
	if err != nil {
		t.Fatalf("GenerateAuditHMACKey failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("Key file not created: %v", err)
	}
}

// =============================================================================
// KEY ROTATION TESTS
// =============================================================================

func TestHMACKeyManager_RotateKey(t *testing.T) {
	tempDir := t.TempDir()

	// Create initial key file
	initialKey := make([]byte, KeySize)
	for i := range initialKey {
		initialKey[i] = byte(i)
	}
	keyPath := filepath.Join(tempDir, DefaultKeyFileName)
	if err := os.WriteFile(keyPath, initialKey, 0600); err != nil {
		t.Fatalf("Failed to write initial key: %v", err)
	}

	// Clear env vars
	originalEnv := os.Getenv(HMACKeyEnvVar)
	originalFileEnv := os.Getenv(HMACKeyFileEnvVar)
	os.Unsetenv(HMACKeyEnvVar)
	os.Unsetenv(HMACKeyFileEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(HMACKeyEnvVar, originalEnv)
		}
		if originalFileEnv != "" {
			os.Setenv(HMACKeyFileEnvVar, originalFileEnv)
		}
	}()

	// Load the key
	manager := NewHMACKeyManager(tempDir)
	_, _, err := manager.LoadKey()
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}

	// Rotate the key (without re-signing)
	result, err := manager.RotateKey(nil, false)
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}

	if !result.Success {
		t.Error("RotateKey should succeed")
	}
	if result.NewKeyPath != keyPath {
		t.Errorf("Expected new key path %s, got %s", keyPath, result.NewKeyPath)
	}
	if result.NewKeyFingerprint == "" {
		t.Error("Expected non-empty new key fingerprint")
	}
	if result.OldKeyFingerprint == "" {
		t.Error("Expected non-empty old key fingerprint")
	}
	if result.NewKeyFingerprint == result.OldKeyFingerprint {
		t.Error("New and old key fingerprints should differ")
	}

	// Verify old key was backed up
	if result.OldKeyPath == "" {
		t.Error("Expected old key to be backed up")
	}
	if _, err := os.Stat(result.OldKeyPath); err != nil {
		t.Errorf("Old key backup not found: %v", err)
	}

	// Verify new key is different
	newKey := manager.GetCurrentKey()
	if string(newKey) == string(initialKey) {
		t.Error("New key should differ from initial key")
	}
}

// =============================================================================
// SECURITY TESTS
// =============================================================================

func TestZeroBytes(t *testing.T) {
	// Test that zeroBytes properly clears sensitive data
	sensitive := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	zeroBytes(sensitive)

	for i, b := range sensitive {
		if b != 0 {
			t.Errorf("Byte %d not zeroed: got %d", i, b)
		}
	}
}

func TestZeroBytes_EmptySlice(t *testing.T) {
	// Should not panic on empty slice
	zeroBytes([]byte{})
}

// =============================================================================
// METADATA TESTS
// =============================================================================

func TestHMACKeyMetadata_Immutability(t *testing.T) {
	manager := NewHMACKeyManager(t.TempDir())

	// Load a key
	testKey := make([]byte, KeySize)
	os.Setenv(HMACKeyEnvVar, hex.EncodeToString(testKey))
	defer os.Unsetenv(HMACKeyEnvVar)

	manager.LoadKey()

	// Get metadata twice
	meta1 := manager.GetKeyMetadata()
	meta2 := manager.GetKeyMetadata()

	// Modify meta1
	meta1.Source = KeySourceNone
	meta1.KeySize = 0

	// meta2 should be unaffected (it's a copy)
	if meta2.Source == KeySourceNone {
		t.Error("Metadata should be immutable copies")
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestHMACKeyManager_ConcurrentAccess(t *testing.T) {
	manager := NewHMACKeyManager(t.TempDir())

	// Load a key
	testKey := make([]byte, KeySize)
	os.Setenv(HMACKeyEnvVar, hex.EncodeToString(testKey))
	defer os.Unsetenv(HMACKeyEnvVar)

	manager.LoadKey()

	// Concurrent reads should not panic or deadlock
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = manager.GetCurrentKey()
				_ = manager.GetKeyMetadata()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
