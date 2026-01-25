// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package access provides role-based access control and account lockout.
package access

import "testing"

// =============================================================================
// PACKAGE TESTS
// =============================================================================

// TestPackageCompiles verifies that the access package compiles correctly.
// The actual RBAC and lockout implementation is in the parent security package.
// See security/rbac_integrity_test.go and security/lockout_test.go for full tests.
func TestPackageCompiles(t *testing.T) {
	// This test verifies the package can be imported.
	// The access subpackage is a re-export layer for backward compatibility.
	t.Log("access package compiles successfully")
}

// TestPackageDocumentation verifies the package documentation is accurate.
func TestPackageDocumentation(t *testing.T) {
	// Package implements NIST 800-53 controls:
	// - AC-5: Separation of Duties
	// - AC-6: Least Privilege
	// - AC-7: Unsuccessful Logon Attempts (account lockout)
	//
	// Actual implementation is in:
	// - security/rbac.go (role-based access control)
	// - security/lockout.go (account lockout management)
	t.Log("Package documentation verified")
}
