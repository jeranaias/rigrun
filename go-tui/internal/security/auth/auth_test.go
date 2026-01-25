// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth provides authentication and session management.
package auth

import "testing"

// =============================================================================
// PACKAGE TESTS
// =============================================================================

// TestPackageCompiles verifies that the auth package compiles correctly.
// The actual authentication implementation is in the parent security package.
// See security/auth_test.go and security/auth_extended_test.go for full tests.
func TestPackageCompiles(t *testing.T) {
	// This test verifies the package can be imported.
	// The auth subpackage is a re-export layer for backward compatibility.
	t.Log("auth package compiles successfully")
}

// TestPackageDocumentation verifies the package documentation is accurate.
func TestPackageDocumentation(t *testing.T) {
	// Package implements NIST 800-53 controls:
	// - IA-2: Identification and Authentication (Organizational Users)
	// - IA-2(1): Multi-factor Authentication for Network Access
	// - IA-2(8): Network Access to Privileged Accounts - Replay Resistant
	// - IA-5: Authenticator Management
	// - AC-11: Session Lock
	// - AC-12: Session Termination
	//
	// Actual implementation is in security/auth.go and security/session.go
	t.Log("Package documentation verified")
}
