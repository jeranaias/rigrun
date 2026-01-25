// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package offline provides IL5 compliance for air-gapped/offline mode operation.
//
// NIST SP 800-53 controls tested:
// - SC-7: Boundary Protection (air-gapped environments)
// - SC-8: Transmission Confidentiality (no external transmission)
// - CA-3: System Interconnections (controlled)
package offline

import (
	"errors"
	"testing"
)

// =============================================================================
// MODE MANAGEMENT TESTS
// =============================================================================

func TestSetOfflineMode(t *testing.T) {
	// Save original state
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(true)
	if !IsOfflineMode() {
		t.Error("IsOfflineMode should return true after SetOfflineMode(true)")
	}

	SetOfflineMode(false)
	if IsOfflineMode() {
		t.Error("IsOfflineMode should return false after SetOfflineMode(false)")
	}
}

func TestIsOfflineMode_ThreadSafe(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	// Concurrent access should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				SetOfflineMode(j%2 == 0)
				_ = IsOfflineMode()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// LOCALHOST DETECTION TESTS (SECURITY CRITICAL)
// =============================================================================

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		host   string
		expect bool
	}{
		// Valid localhost variants
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"::1", true},
		{"[::1]", true},
		{"[::1]:8080", true},

		// Non-localhost (MUST be blocked in offline mode)
		{"google.com", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"0.0.0.0", false},
		{"api.openai.com", false},
		{"api.anthropic.com", false},

		// Edge cases
		{"", false},
		{"localhost.localdomain", false}, // Not exactly "localhost"
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := IsLocalhost(tc.host)
			if result != tc.expect {
				t.Errorf("IsLocalhost(%q) = %v, want %v", tc.host, result, tc.expect)
			}
		})
	}
}

func TestIsLocalhost_IPv4Loopback(t *testing.T) {
	// The entire 127.0.0.0/8 range should be localhost
	tests := []string{
		"127.0.0.1",
		"127.0.0.2",
		"127.1.2.3",
		"127.255.255.255",
	}

	for _, host := range tests {
		if !IsLocalhost(host) {
			t.Errorf("IsLocalhost(%q) should be true for 127.x.x.x range", host)
		}
	}
}

// =============================================================================
// URL VALIDATION TESTS (SECURITY CRITICAL)
// =============================================================================

func TestValidateURLForOfflineMode_SchemeValidation(t *testing.T) {
	// CRITICAL: Dangerous schemes must ALWAYS be blocked regardless of offline mode
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	dangerousURLs := []string{
		"file:///etc/passwd",
		"file://C:/Windows/System32/config/SAM",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"ftp://ftp.example.com",
		"gopher://gopher.example.com",
	}

	// Test with offline mode OFF
	SetOfflineMode(false)
	for _, url := range dangerousURLs {
		err := ValidateURLForOfflineMode(url)
		if !errors.Is(err, ErrInvalidURLScheme) {
			t.Errorf("ValidateURLForOfflineMode(%q) should block dangerous scheme (offline=false), got %v", url, err)
		}
	}

	// Test with offline mode ON
	SetOfflineMode(true)
	for _, url := range dangerousURLs {
		err := ValidateURLForOfflineMode(url)
		if !errors.Is(err, ErrInvalidURLScheme) {
			t.Errorf("ValidateURLForOfflineMode(%q) should block dangerous scheme (offline=true), got %v", url, err)
		}
	}
}

func TestValidateURLForOfflineMode_OfflineModeBlocking(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(true)

	// External URLs should be blocked in offline mode
	externalURLs := []string{
		"https://api.openai.com/v1/chat",
		"https://api.anthropic.com/v1/messages",
		"https://openrouter.ai/api/v1/chat",
		"http://192.168.1.1:8080/api",
	}

	for _, url := range externalURLs {
		err := ValidateURLForOfflineMode(url)
		if !errors.Is(err, ErrNonLocalhost) {
			t.Errorf("ValidateURLForOfflineMode(%q) should be blocked in offline mode, got %v", url, err)
		}
	}

	// Localhost URLs should be allowed
	localhostURLs := []string{
		"http://localhost:11434/api/generate",
		"http://127.0.0.1:11434/api/generate",
		"https://localhost:8080/api",
	}

	for _, url := range localhostURLs {
		err := ValidateURLForOfflineMode(url)
		if err != nil {
			t.Errorf("ValidateURLForOfflineMode(%q) should be allowed in offline mode, got %v", url, err)
		}
	}
}

func TestValidateURLForOfflineMode_OnlineMode(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)

	// External URLs should be allowed when not in offline mode
	validURLs := []string{
		"https://api.openai.com/v1/chat",
		"https://api.anthropic.com/v1/messages",
		"http://localhost:11434/api/generate",
	}

	for _, url := range validURLs {
		err := ValidateURLForOfflineMode(url)
		if err != nil {
			t.Errorf("ValidateURLForOfflineMode(%q) should be allowed in online mode, got %v", url, err)
		}
	}
}

func TestValidateOllamaURL(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(true)

	// Valid Ollama URLs
	validURLs := []string{
		"http://localhost:11434",
		"http://127.0.0.1:11434",
	}

	for _, url := range validURLs {
		err := ValidateOllamaURL(url)
		if err != nil {
			t.Errorf("ValidateOllamaURL(%q) should be valid, got %v", url, err)
		}
	}

	// Invalid Ollama URLs (remote hosts)
	invalidURLs := []string{
		"http://ollama.example.com:11434",
		"http://192.168.1.100:11434",
	}

	for _, url := range invalidURLs {
		err := ValidateOllamaURL(url)
		if err == nil {
			t.Errorf("ValidateOllamaURL(%q) should be blocked in offline mode", url)
		}
	}
}

// =============================================================================
// FEATURE GUARD TESTS
// =============================================================================

func TestCheckCloudAllowed(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if err := CheckCloudAllowed(); err != nil {
		t.Errorf("CheckCloudAllowed should return nil in online mode, got %v", err)
	}

	SetOfflineMode(true)
	if err := CheckCloudAllowed(); !errors.Is(err, ErrCloudBlocked) {
		t.Errorf("CheckCloudAllowed should return ErrCloudBlocked in offline mode, got %v", err)
	}
}

func TestCheckWebFetchAllowed(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if err := CheckWebFetchAllowed(); err != nil {
		t.Errorf("CheckWebFetchAllowed should return nil in online mode, got %v", err)
	}

	SetOfflineMode(true)
	if err := CheckWebFetchAllowed(); !errors.Is(err, ErrWebFetchBlocked) {
		t.Errorf("CheckWebFetchAllowed should return ErrWebFetchBlocked in offline mode, got %v", err)
	}
}

func TestCheckTelemetryAllowed(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if err := CheckTelemetryAllowed(); err != nil {
		t.Errorf("CheckTelemetryAllowed should return nil in online mode, got %v", err)
	}

	SetOfflineMode(true)
	if err := CheckTelemetryAllowed(); !errors.Is(err, ErrTelemetryBlocked) {
		t.Errorf("CheckTelemetryAllowed should return ErrTelemetryBlocked in offline mode, got %v", err)
	}
}

func TestCheckNetworkAllowed(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if err := CheckNetworkAllowed(); err != nil {
		t.Errorf("CheckNetworkAllowed should return nil in online mode, got %v", err)
	}

	SetOfflineMode(true)
	if err := CheckNetworkAllowed(); !errors.Is(err, ErrNetworkBlocked) {
		t.Errorf("CheckNetworkAllowed should return ErrNetworkBlocked in offline mode, got %v", err)
	}
}

// =============================================================================
// STATUS DISPLAY TESTS
// =============================================================================

func TestStatusIndicator(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if indicator := StatusIndicator(); indicator != "" {
		t.Errorf("StatusIndicator should be empty in online mode, got %q", indicator)
	}

	SetOfflineMode(true)
	if indicator := StatusIndicator(); indicator != "OFFLINE MODE" {
		t.Errorf("StatusIndicator should be 'OFFLINE MODE' in offline mode, got %q", indicator)
	}
}

func TestStatusBadge(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if badge := StatusBadge(); badge != "" {
		t.Errorf("StatusBadge should be empty in online mode, got %q", badge)
	}

	SetOfflineMode(true)
	if badge := StatusBadge(); badge != "[OFFLINE]" {
		t.Errorf("StatusBadge should be '[OFFLINE]' in offline mode, got %q", badge)
	}
}

func TestComplianceInfo(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)

	SetOfflineMode(false)
	if info := ComplianceInfo(); info != "" {
		t.Errorf("ComplianceInfo should be empty in online mode")
	}

	SetOfflineMode(true)
	info := ComplianceInfo()
	if info == "" {
		t.Error("ComplianceInfo should not be empty in offline mode")
	}
	// Check for NIST control references
	if !contains(info, "SC-7") || !contains(info, "SC-8") || !contains(info, "CA-3") {
		t.Error("ComplianceInfo should mention NIST controls SC-7, SC-8, CA-3")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// SECURITY ADVERSARIAL TESTS
// =============================================================================

func TestValidateURLForOfflineMode_Adversarial(t *testing.T) {
	original := IsOfflineMode()
	defer SetOfflineMode(original)
	SetOfflineMode(true)

	// Adversarial attempts to bypass localhost check
	adversarialURLs := []struct {
		url    string
		reason string
	}{
		{"http://localhost.evil.com:11434", "subdomain of evil.com"},
		{"http://127.0.0.1.evil.com:11434", "IP-like subdomain"},
		{"http://evil.com#localhost", "fragment injection"},
		{"http://evil.com?host=localhost", "query injection"},
		{"http://localhost@evil.com", "userinfo injection"},
	}

	for _, tc := range adversarialURLs {
		err := ValidateURLForOfflineMode(tc.url)
		if err == nil {
			t.Errorf("ValidateURLForOfflineMode(%q) should be blocked (%s)", tc.url, tc.reason)
		}
	}
}

// =============================================================================
// ERROR TYPE TESTS
// =============================================================================

func TestErrorMessages(t *testing.T) {
	errors := []error{
		ErrNetworkBlocked,
		ErrNonLocalhost,
		ErrCloudBlocked,
		ErrWebFetchBlocked,
		ErrTelemetryBlocked,
		ErrInvalidURLScheme,
	}

	for _, err := range errors {
		if err.Error() == "" {
			t.Errorf("Error %v should have non-empty message", err)
		}
		// All errors should mention IL5 compliance
		if !contains(err.Error(), "IL5") {
			t.Errorf("Error %q should mention IL5 compliance", err.Error())
		}
	}
}
