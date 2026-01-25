// Package security provides IL5 security controls.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package security

import (
	"crypto/tls"
	"strings"
	"testing"
)

// =============================================================================
// FIPS MODE TESTS (SC-13, IA-7)
// =============================================================================

func TestIsFIPSMode(t *testing.T) {
	// Save original state
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	SetFIPSMode(true)
	if !IsFIPSMode() {
		t.Error("IsFIPSMode() should return true after SetFIPSMode(true)")
	}

	SetFIPSMode(false)
	if IsFIPSMode() {
		t.Error("IsFIPSMode() should return false after SetFIPSMode(false)")
	}
}

func TestSetFIPSMode(t *testing.T) {
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	// Toggle mode multiple times
	for i := 0; i < 3; i++ {
		SetFIPSMode(true)
		if !IsFIPSMode() {
			t.Errorf("Iteration %d: FIPS mode should be true", i)
		}
		SetFIPSMode(false)
		if IsFIPSMode() {
			t.Errorf("Iteration %d: FIPS mode should be false", i)
		}
	}
}

func TestIsFIPSAvailable(t *testing.T) {
	// Just verify it returns a boolean without panic
	available := IsFIPSAvailable()
	t.Logf("FIPS available: %v", available)
	// Result depends on platform, so we just check it doesn't panic
}

// =============================================================================
// ALGORITHM INFO TESTS
// =============================================================================

func TestGetSupportedAlgorithms(t *testing.T) {
	algorithms := GetSupportedAlgorithms()

	if len(algorithms) == 0 {
		t.Fatal("GetSupportedAlgorithms() returned empty list")
	}

	// Should have both FIPS-approved and non-approved
	hasFIPS := false
	hasNonFIPS := false
	for _, alg := range algorithms {
		if alg.FIPSApproved {
			hasFIPS = true
		} else {
			hasNonFIPS = true
		}
	}

	if !hasFIPS {
		t.Error("Should have FIPS-approved algorithms")
	}
	if !hasNonFIPS {
		t.Error("Should have non-FIPS algorithms")
	}
}

func TestGetFIPSApprovedAlgorithms(t *testing.T) {
	algorithms := GetFIPSApprovedAlgorithms()

	if len(algorithms) == 0 {
		t.Fatal("GetFIPSApprovedAlgorithms() returned empty list")
	}

	// All should be FIPS-approved
	for _, alg := range algorithms {
		if !alg.FIPSApproved {
			t.Errorf("Algorithm %s should be FIPS-approved", alg.Name)
		}
	}

	// Verify essential algorithms are present
	essential := []string{"AES-256-GCM", "SHA-256", "HMAC-SHA-256"}
	for _, name := range essential {
		found := false
		for _, alg := range algorithms {
			if alg.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Essential FIPS algorithm %s not found", name)
		}
	}
}

func TestGetAlgorithmsByType(t *testing.T) {
	tests := []struct {
		algType  AlgorithmType
		contains string
	}{
		{AlgorithmTypeSymmetric, "AES"},
		{AlgorithmTypeHash, "SHA"},
		{AlgorithmTypeSignature, "ECDSA"},
		{AlgorithmTypeMAC, "HMAC"},
		{AlgorithmTypeKDF, "PBKDF2"},
		{AlgorithmTypeKeyExch, "ECDH"},
	}

	for _, tc := range tests {
		t.Run(string(tc.algType), func(t *testing.T) {
			algorithms := GetAlgorithmsByType(tc.algType)
			if len(algorithms) == 0 {
				t.Errorf("GetAlgorithmsByType(%s) returned empty list", tc.algType)
				return
			}

			found := false
			for _, alg := range algorithms {
				if strings.Contains(alg.Name, tc.contains) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("GetAlgorithmsByType(%s) should contain algorithm with %s", tc.algType, tc.contains)
			}
		})
	}
}

func TestIsAlgorithmFIPSApproved(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// FIPS-approved algorithms
		{"AES-256-GCM", true},
		{"aes-256-gcm", true}, // Case insensitive
		{"SHA-256", true},
		{"HMAC-SHA-256", true},
		{"ECDSA-P256", true},
		{"RSA-2048", true},
		{"PBKDF2-SHA-256", true},

		// Non-FIPS algorithms
		{"MD5", false},
		{"SHA-1", false},
		{"DES", false},
		{"3DES", false},
		{"RC4", false},

		// Non-existent
		{"UNKNOWN-CIPHER", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsAlgorithmFIPSApproved(tc.name)
			if got != tc.want {
				t.Errorf("IsAlgorithmFIPSApproved(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// =============================================================================
// CRYPTO STATUS TESTS
// =============================================================================

func TestGetCryptoStatus(t *testing.T) {
	status := GetCryptoStatus()

	if status == nil {
		t.Fatal("GetCryptoStatus() returned nil")
	}

	// Check required fields are populated
	if status.Platform == "" {
		t.Error("Status.Platform should not be empty")
	}
	if status.GoVersion == "" {
		t.Error("Status.GoVersion should not be empty")
	}
	if status.TLSVersion == "" {
		t.Error("Status.TLSVersion should not be empty")
	}
	if status.TLSMinVersion == "" {
		t.Error("Status.TLSMinVersion should not be empty")
	}
	if len(status.Algorithms) == 0 {
		t.Error("Status.Algorithms should not be empty")
	}

	t.Logf("Crypto Status: FIPS Mode=%v, Available=%v, Platform=%s",
		status.FIPSMode, status.FIPSAvailable, status.Platform)
}

func TestGetCryptoStatus_FIPSModeWarning(t *testing.T) {
	// Save and restore
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	// Enable FIPS mode
	SetFIPSMode(true)
	status := GetCryptoStatus()

	// If FIPS mode enabled but not available, should have issue
	if status.FIPSMode && !status.FIPSAvailable {
		if len(status.Issues) == 0 {
			t.Error("Should have issues when FIPS mode enabled but not available")
		}
	}
}

// =============================================================================
// FIPS COMPLIANCE VERIFICATION TESTS
// =============================================================================

func TestVerifyFIPSCompliance(t *testing.T) {
	result := VerifyFIPSCompliance()

	if result == nil {
		t.Fatal("VerifyFIPSCompliance() returned nil")
	}

	// Log result for debugging
	t.Logf("FIPS Compliance: Compliant=%v, Issues=%d, Warnings=%d",
		result.Compliant, len(result.Issues), len(result.Warnings))

	// Should have warnings on most systems
	if len(result.Warnings) == 0 {
		t.Log("No warnings - this is unusual unless running on FIPS-validated system")
	}
}

func TestVerifyFIPSCompliance_FIPSModeEnabled(t *testing.T) {
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	SetFIPSMode(true)
	result := VerifyFIPSCompliance()

	// When FIPS mode is enabled but not available, should fail compliance
	if IsFIPSMode() && !IsFIPSAvailable() {
		if result.Compliant {
			t.Error("Should not be compliant when FIPS mode enabled but not available")
		}
		if len(result.Issues) == 0 {
			t.Error("Should have issues when FIPS mode enabled but not available")
		}
	}
}

func TestVerifyFIPSCompliance_FIPSModeDisabled(t *testing.T) {
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	SetFIPSMode(false)
	result := VerifyFIPSCompliance()

	// Should have warning about FIPS mode not enabled
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "FIPS mode is not enabled") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Should warn when FIPS mode is not enabled")
	}
}

// =============================================================================
// TLS VERSION TESTS
// =============================================================================

func TestGetTLSVersionString(t *testing.T) {
	tests := []struct {
		version uint16
		want    string
	}{
		{tls.VersionTLS10, "TLS 1.0"},
		{tls.VersionTLS11, "TLS 1.1"},
		{tls.VersionTLS12, "TLS 1.2"},
		{tls.VersionTLS13, "TLS 1.3"},
		{0x9999, "Unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := getTLSVersionString(tc.version)
			if tc.want == "Unknown" {
				// Unknown versions should return something containing "Unknown"
				if !strings.Contains(got, "Unknown") {
					t.Errorf("getTLSVersionString(0x%04x) = %q, want to contain 'Unknown'", tc.version, got)
				}
			} else {
				// Known versions should contain the version number (e.g., "1.2" from "TLS 1.2")
				parts := strings.Split(tc.want, " ")
				if len(parts) > 1 && !strings.Contains(got, parts[1]) {
					t.Errorf("getTLSVersionString(0x%04x) = %q, want to contain %q", tc.version, got, tc.want)
				}
			}
		})
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		want     uint16
		wantErr  bool
	}{
		{"1.0", tls.VersionTLS10, false},
		{"TLS1.0", tls.VersionTLS10, false},
		{"TLS 1.0", tls.VersionTLS10, false},
		{"1.1", tls.VersionTLS11, false},
		{"TLS 1.1", tls.VersionTLS11, false},
		{"1.2", tls.VersionTLS12, false},
		{"TLS1.2", tls.VersionTLS12, false},
		{"TLS 1.2", tls.VersionTLS12, false},
		{"1.3", tls.VersionTLS13, false},
		{"TLS1.3", tls.VersionTLS13, false},
		{"TLS 1.3", tls.VersionTLS13, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"1.4", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseTLSVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseTLSVersion(%q) expected error", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTLSVersion(%q) error = %v", tc.input, err)
				}
				if got != tc.want {
					t.Errorf("ParseTLSVersion(%q) = 0x%04x, want 0x%04x", tc.input, got, tc.want)
				}
			}
		})
	}
}

// =============================================================================
// CIPHER SUITE TESTS
// =============================================================================

func TestFIPSApprovedCipherSuites(t *testing.T) {
	suites := FIPSApprovedCipherSuites()

	if len(suites) == 0 {
		t.Fatal("FIPSApprovedCipherSuites() returned empty list")
	}

	// Should include TLS 1.3 suites
	hasTLS13 := false
	for _, suite := range suites {
		if suite == tls.TLS_AES_128_GCM_SHA256 || suite == tls.TLS_AES_256_GCM_SHA384 {
			hasTLS13 = true
			break
		}
	}
	if !hasTLS13 {
		t.Error("Should include TLS 1.3 cipher suites")
	}

	// Should include TLS 1.2 suites with GCM
	hasTLS12GCM := false
	for _, suite := range suites {
		if suite == tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 ||
			suite == tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 {
			hasTLS12GCM = true
			break
		}
	}
	if !hasTLS12GCM {
		t.Error("Should include TLS 1.2 GCM cipher suites")
	}

	t.Logf("FIPS-approved cipher suites: %d", len(suites))
}

func TestGetSecureTLSConfig(t *testing.T) {
	cfg := GetSecureTLSConfig()

	if cfg == nil {
		t.Fatal("GetSecureTLSConfig() returned nil")
	}

	// Check minimum TLS version
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = 0x%04x, want 0x%04x (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
	}

	// Check max TLS version
	if cfg.MaxVersion != tls.VersionTLS13 {
		t.Errorf("MaxVersion = 0x%04x, want 0x%04x (TLS 1.3)", cfg.MaxVersion, tls.VersionTLS13)
	}

	// Check cipher suites are set
	if len(cfg.CipherSuites) == 0 {
		t.Error("CipherSuites should not be empty")
	}

	// Check curve preferences
	if len(cfg.CurvePreferences) == 0 {
		t.Error("CurvePreferences should not be empty")
	}

	// Verify P-384 is preferred (more secure)
	if cfg.CurvePreferences[0] != tls.CurveP384 {
		t.Error("P-384 should be the first curve preference")
	}
}

// =============================================================================
// ALGORITHM INFO STRUCT TESTS
// =============================================================================

func TestAlgorithmInfo_Fields(t *testing.T) {
	// Verify algorithm info has required fields
	algorithms := GetFIPSApprovedAlgorithms()

	for _, alg := range algorithms {
		if alg.Name == "" {
			t.Error("Algorithm name should not be empty")
		}
		if alg.Type == "" {
			t.Errorf("Algorithm %s should have a type", alg.Name)
		}
		if alg.Description == "" {
			t.Errorf("Algorithm %s should have a description", alg.Name)
		}
		if alg.Standard == "" {
			t.Errorf("Algorithm %s should have a standard reference", alg.Name)
		}
	}
}

// =============================================================================
// CRYPTO CONFIG TESTS
// =============================================================================

func TestCryptoConfig_Defaults(t *testing.T) {
	cfg := CryptoConfig{
		FIPSMode:           true,
		TLSMinVersion:      "1.2",
		CertificatePinning: true,
		PinnedCertificates: map[string]string{"example.com": "sha256:abc123"},
	}

	if !cfg.FIPSMode {
		t.Error("FIPSMode should be true")
	}
	if cfg.TLSMinVersion != "1.2" {
		t.Error("TLSMinVersion should be 1.2")
	}
	if !cfg.CertificatePinning {
		t.Error("CertificatePinning should be true")
	}
	if len(cfg.PinnedCertificates) != 1 {
		t.Error("Should have one pinned certificate")
	}
}

// =============================================================================
// NON-FIPS ALGORITHM TESTS
// =============================================================================

func TestNonFIPSAlgorithms(t *testing.T) {
	deprecated := []string{"MD5", "SHA-1", "DES", "3DES", "RC4"}

	for _, name := range deprecated {
		if IsAlgorithmFIPSApproved(name) {
			t.Errorf("%s should NOT be FIPS-approved", name)
		}
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestFIPSMode_ConcurrentAccess(t *testing.T) {
	original := IsFIPSMode()
	defer SetFIPSMode(original)

	// Concurrent reads and writes should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				SetFIPSMode(j%2 == 0)
				_ = IsFIPSMode()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// AUDIT LOGGING TESTS
// =============================================================================

func TestLogCryptoFIPSCheck(t *testing.T) {
	// This should not panic even without audit logger
	err := LogCryptoFIPSCheck("test_session", true, nil)
	// Error is acceptable if no audit logger configured
	_ = err
}

func TestLogCertValidation(t *testing.T) {
	err := LogCertValidation("test_session", "example.com", true, "")
	_ = err
}

func TestLogCertPinningMismatch(t *testing.T) {
	err := LogCertPinningMismatch("test_session", "example.com", "sha256:expected", "sha256:actual")
	_ = err
}

// =============================================================================
// CERT STATUS TESTS
// =============================================================================

func TestCertStatus_Fields(t *testing.T) {
	status := CertStatus{
		Subject:         "CN=example.com",
		Issuer:          "CN=Test CA",
		DaysUntilExpiry: 30,
		ChainValid:      true,
		Pinned:          true,
		Fingerprint:     "sha256:abc123",
		SerialNumber:    "123456",
	}

	if status.Subject == "" {
		t.Error("Subject should not be empty")
	}
	if !status.ChainValid {
		t.Error("ChainValid should be true")
	}
	if !status.Pinned {
		t.Error("Pinned should be true")
	}
}

// =============================================================================
// FIPS COMPLIANCE RESULT TESTS
// =============================================================================

func TestFIPSComplianceResult_Fields(t *testing.T) {
	result := FIPSComplianceResult{
		Compliant: true,
		Issues:    []string{"issue1"},
		Warnings:  []string{"warning1", "warning2"},
	}

	if !result.Compliant {
		t.Error("Compliant should be true")
	}
	if len(result.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1", len(result.Issues))
	}
	if len(result.Warnings) != 2 {
		t.Errorf("Warnings count = %d, want 2", len(result.Warnings))
	}
}

// =============================================================================
// ALGORITHM TYPE CONSTANTS TESTS
// =============================================================================

func TestAlgorithmTypeConstants(t *testing.T) {
	types := []AlgorithmType{
		AlgorithmTypeSymmetric,
		AlgorithmTypeHash,
		AlgorithmTypeSignature,
		AlgorithmTypeKDF,
		AlgorithmTypeKeyExch,
		AlgorithmTypeMAC,
	}

	for _, algType := range types {
		if algType == "" {
			t.Error("Algorithm type constant should not be empty")
		}
	}
}
