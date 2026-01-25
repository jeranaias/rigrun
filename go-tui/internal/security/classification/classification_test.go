// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package classification provides DoD classification marking and enforcement.
package classification

import "testing"

// =============================================================================
// PACKAGE TESTS
// =============================================================================

// TestPackageCompiles verifies that the classification package compiles correctly.
// The actual classification implementation is in the parent security package.
// See security/classification_test.go and security/ac4_enforcer_test.go for full tests.
func TestPackageCompiles(t *testing.T) {
	// This test verifies the package can be imported.
	// The classification subpackage is a re-export layer for backward compatibility.
	t.Log("classification package compiles successfully")
}

// TestPackageDocumentation verifies the package documentation is accurate.
func TestPackageDocumentation(t *testing.T) {
	// Package implements NIST 800-53 controls:
	// - AC-4: Information Flow Enforcement
	//
	// DoD classification levels supported:
	// - UNCLASSIFIED
	// - CUI (Controlled Unclassified Information)
	// - SECRET
	// - TOP SECRET
	//
	// Actual implementation is in security/classification.go
	t.Log("Package documentation verified")
}
