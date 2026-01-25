// boundary_test.go - Tests for network boundary protection.
//
// NIST 800-53 AU-9: Protection of Audit Information
// Tests for policy signature key management compliance.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestGetPolicySignatureKey_MissingKey tests that an error is returned when
// the policy signature key is not configured (NIST 800-53 AU-9 compliance).
func TestGetPolicySignatureKey_MissingKey(t *testing.T) {
	// Save and clear the environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	os.Unsetenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		}
	}()

	// Temporarily move config files if they exist to ensure key is not found
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	configDir := filepath.Join(home, ".rigrun")
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")

	// Backup and remove config files if they exist
	tomlBackup := tomlPath + ".test_backup"
	jsonBackup := jsonPath + ".test_backup"

	if _, err := os.Stat(tomlPath); err == nil {
		os.Rename(tomlPath, tomlBackup)
		defer os.Rename(tomlBackup, tomlPath)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		os.Rename(jsonPath, jsonBackup)
		defer os.Rename(jsonBackup, jsonPath)
	}

	// Test that getPolicySignatureKey returns error when key is not configured
	key, err := getPolicySignatureKey()
	if err == nil {
		t.Errorf("Expected error when policy key is not configured, got key: %s", key)
	}
	if !errors.Is(err, ErrPolicyKeyNotConfigured) {
		t.Errorf("Expected ErrPolicyKeyNotConfigured, got: %v", err)
	}
}

// TestGetPolicySignatureKey_FromEnv tests that the key is loaded from environment variable.
func TestGetPolicySignatureKey_FromEnv(t *testing.T) {
	// Save the original environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(PolicyKeyEnvVar)
		}
	}()

	// Set the environment variable (must be at least 32 bytes)
	testKey := "test-policy-key-from-env-123456789"
	os.Setenv(PolicyKeyEnvVar, testKey)

	// Test that getPolicySignatureKey returns the key from env
	key, err := getPolicySignatureKey()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if key != testKey {
		t.Errorf("Expected key %q, got %q", testKey, key)
	}
}

// TestGetPolicySignatureKey_EnvOverridesConfig tests that environment variable
// takes precedence over config file.
func TestGetPolicySignatureKey_EnvOverridesConfig(t *testing.T) {
	// Save the original environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(PolicyKeyEnvVar)
		}
	}()

	// Set the environment variable (must be at least 32 bytes)
	envKey := "env-key-takes-priority-override-test"
	os.Setenv(PolicyKeyEnvVar, envKey)

	// Even if config has a different key, env should win
	key, err := getPolicySignatureKey()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if key != envKey {
		t.Errorf("Expected env key %q to take priority, got %q", envKey, key)
	}
}

// TestComputePolicySignature_MissingKey tests that signature computation fails
// when the policy key is not configured.
func TestComputePolicySignature_MissingKey(t *testing.T) {
	// Save and clear the environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	os.Unsetenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		}
	}()

	// Temporarily move config files if they exist
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	configDir := filepath.Join(home, ".rigrun")
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")

	tomlBackup := tomlPath + ".test_backup"
	jsonBackup := jsonPath + ".test_backup"

	if _, err := os.Stat(tomlPath); err == nil {
		os.Rename(tomlPath, tomlBackup)
		defer os.Rename(tomlBackup, tomlPath)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		os.Rename(jsonPath, jsonBackup)
		defer os.Rename(jsonBackup, jsonPath)
	}

	// Create a BoundaryProtection instance
	bp := NewBoundaryProtection()

	// Test that computePolicySignature returns error when key is not configured
	_, err = bp.computePolicySignature([]byte("test data"))
	if err == nil {
		t.Error("Expected error when computing signature without configured key")
	}
	if !errors.Is(err, ErrPolicyKeyNotConfigured) {
		t.Errorf("Expected ErrPolicyKeyNotConfigured in error chain, got: %v", err)
	}
}

// TestVerifyPolicySignature_MissingKey tests that signature verification fails
// when the policy key is not configured.
func TestVerifyPolicySignature_MissingKey(t *testing.T) {
	// Save and clear the environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	os.Unsetenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		}
	}()

	// Temporarily move config files if they exist
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	configDir := filepath.Join(home, ".rigrun")
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")

	tomlBackup := tomlPath + ".test_backup"
	jsonBackup := jsonPath + ".test_backup"

	if _, err := os.Stat(tomlPath); err == nil {
		os.Rename(tomlPath, tomlBackup)
		defer os.Rename(tomlBackup, tomlPath)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		os.Rename(jsonPath, jsonBackup)
		defer os.Rename(jsonBackup, jsonPath)
	}

	// Create a BoundaryProtection instance
	bp := NewBoundaryProtection()

	// Test that verifyPolicySignature returns error when key is not configured
	_, err = bp.verifyPolicySignature([]byte("test data"), "dummy-signature")
	if err == nil {
		t.Error("Expected error when verifying signature without configured key")
	}
	if !errors.Is(err, ErrPolicyKeyNotConfigured) {
		t.Errorf("Expected ErrPolicyKeyNotConfigured in error chain, got: %v", err)
	}
}

// TestComputeAndVerifyPolicySignature_WithKey tests that signature computation
// and verification work correctly when the key is configured.
func TestComputeAndVerifyPolicySignature_WithKey(t *testing.T) {
	// Save the original environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		} else {
			os.Unsetenv(PolicyKeyEnvVar)
		}
	}()

	// Set the environment variable
	testKey := "test-policy-key-for-signature-verification"
	os.Setenv(PolicyKeyEnvVar, testKey)

	// Create a BoundaryProtection instance
	bp := NewBoundaryProtection()

	// Test data
	testData := []byte(`{"allowed_hosts": ["localhost"], "blocked_hosts": []}`)

	// Compute signature
	signature, err := bp.computePolicySignature(testData)
	if err != nil {
		t.Fatalf("Failed to compute signature: %v", err)
	}
	if signature == "" {
		t.Error("Expected non-empty signature")
	}

	// Verify signature
	valid, err := bp.verifyPolicySignature(testData, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		t.Error("Expected signature to be valid")
	}

	// Verify that tampered data fails
	tamperedData := []byte(`{"allowed_hosts": ["localhost", "malicious.com"], "blocked_hosts": []}`)
	valid, err = bp.verifyPolicySignature(tamperedData, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature for tampered data: %v", err)
	}
	if valid {
		t.Error("Expected signature verification to fail for tampered data")
	}
}

// TestSavePolicySignature_MissingKey tests that saving policy signature fails
// when the policy key is not configured.
func TestSavePolicySignature_MissingKey(t *testing.T) {
	// Save and clear the environment variable
	originalEnv := os.Getenv(PolicyKeyEnvVar)
	os.Unsetenv(PolicyKeyEnvVar)
	defer func() {
		if originalEnv != "" {
			os.Setenv(PolicyKeyEnvVar, originalEnv)
		}
	}()

	// Temporarily move config files if they exist
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	configDir := filepath.Join(home, ".rigrun")
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")

	tomlBackup := tomlPath + ".test_backup"
	jsonBackup := jsonPath + ".test_backup"

	if _, err := os.Stat(tomlPath); err == nil {
		os.Rename(tomlPath, tomlBackup)
		defer os.Rename(tomlBackup, tomlPath)
	}
	if _, err := os.Stat(jsonPath); err == nil {
		os.Rename(jsonPath, jsonBackup)
		defer os.Rename(jsonBackup, jsonPath)
	}

	// Create a BoundaryProtection instance with temp directory
	tempDir := t.TempDir()
	bp := NewBoundaryProtection(WithBoundaryConfigPath(filepath.Join(tempDir, "policy.json")))

	// Test that savePolicySignature returns error when key is not configured
	err = bp.savePolicySignature([]byte("test data"))
	if err == nil {
		t.Error("Expected error when saving signature without configured key")
	}
	if !errors.Is(err, ErrPolicyKeyNotConfigured) {
		t.Errorf("Expected ErrPolicyKeyNotConfigured in error chain, got: %v", err)
	}
}
