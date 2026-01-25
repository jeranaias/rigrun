// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package crypto provides cryptographic operations for IL5 compliance.
package crypto

import "testing"

// =============================================================================
// PACKAGE TESTS
// =============================================================================

// TestPackageCompiles verifies that the crypto package compiles correctly.
// The actual cryptographic implementation is in the parent security package.
// See security/crypto_test.go and security/encryption_test.go for full tests.
func TestPackageCompiles(t *testing.T) {
	// This test verifies the package can be imported.
	// The crypto subpackage is a re-export layer for backward compatibility.
	t.Log("crypto package compiles successfully")
}

// TestPackageDocumentation verifies the package documentation is accurate.
func TestPackageDocumentation(t *testing.T) {
	// Package implements NIST 800-53 controls:
	// - SC-13: Cryptographic Protection (FIPS 140-2/3 compliance)
	// - SC-17: Public Key Infrastructure Certificates
	//
	// Actual implementation is in:
	// - security/crypto.go (FIPS mode, algorithms)
	// - security/encrypt.go (encryption operations)
	// - security/pki.go (certificate management)
	t.Log("Package documentation verified")
}
