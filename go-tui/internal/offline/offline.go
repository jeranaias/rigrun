// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package offline provides IL5 compliance for air-gapped/offline mode operation.
//
// This implements NIST SP 800-53 controls:
// - SC-7: Boundary Protection (air-gapped environments)
// - SC-8: Transmission Confidentiality (no external transmission)
// - CA-3: System Interconnections (controlled)
package offline

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
)

// =============================================================================
// ERRORS
// =============================================================================

// Error types for offline mode violations.
var (
	// ErrNetworkBlocked is returned when a network operation is attempted in offline mode.
	ErrNetworkBlocked = errors.New("IL5 SC-7: network operation blocked in offline mode")

	// ErrNonLocalhost is returned when attempting to connect to non-localhost in offline mode.
	ErrNonLocalhost = errors.New("IL5 SC-7: only localhost/127.0.0.1 connections allowed in offline mode")

	// ErrCloudBlocked is returned when attempting to use cloud services in offline mode.
	ErrCloudBlocked = errors.New("IL5 SC-8: cloud services disabled in offline mode")

	// ErrWebFetchBlocked is returned when attempting to fetch web content in offline mode.
	ErrWebFetchBlocked = errors.New("IL5 SC-7: web fetch disabled in offline mode")

	// ErrTelemetryBlocked is returned when attempting telemetry/updates in offline mode.
	ErrTelemetryBlocked = errors.New("IL5 CA-3: telemetry and updates disabled in offline mode")

	// ErrInvalidURLScheme is returned when URL scheme is not http or https.
	// CRITICAL FIX: Prevents file://, javascript://, data://, and other dangerous schemes.
	ErrInvalidURLScheme = errors.New("IL5 SC-7: only http and https schemes are allowed")
)

// =============================================================================
// MODE MANAGEMENT
// =============================================================================

// Global offline mode state with thread-safe access.
var (
	offlineMode      bool
	offlineModeMutex sync.RWMutex
)

// SetOfflineMode enables or disables offline mode globally.
// When enabled:
// - ALL outbound network connections are blocked
// - Only local Ollama (localhost/127.0.0.1) is allowed
// - OpenRouter/cloud models are completely disabled
// - WebFetch tool is disabled
// - Telemetry/updates are disabled
func SetOfflineMode(enabled bool) {
	offlineModeMutex.Lock()
	defer offlineModeMutex.Unlock()
	offlineMode = enabled
}

// IsOfflineMode returns true if offline mode is currently enabled.
func IsOfflineMode() bool {
	offlineModeMutex.RLock()
	defer offlineModeMutex.RUnlock()
	return offlineMode
}

// =============================================================================
// URL VALIDATION
// =============================================================================

// IsLocalhost checks if a host string refers to localhost.
// Accepts: "localhost", "127.0.0.1", "::1", "[::1]", and any IPv6 loopback variant.
//
// SECURITY FIX: Uses net.IP.IsLoopback() for comprehensive IPv6 localhost detection.
// This properly handles all IPv6 loopback variants including ::1, 0:0:0:0:0:0:0:1, etc.
func IsLocalhost(host string) bool {
	// Normalize the host (remove port if present)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Remove brackets from IPv6 addresses (e.g., "[::1]" -> "::1")
	host = strings.Trim(host, "[]")
	host = strings.ToLower(host)

	// Check for "localhost" hostname explicitly
	if host == "localhost" {
		return true
	}

	// SECURITY FIX: Use net.IP.IsLoopback() for comprehensive IP detection
	// This properly handles:
	// - IPv4: 127.0.0.1, 127.x.x.x (entire 127.0.0.0/8 range)
	// - IPv6: ::1, 0:0:0:0:0:0:0:1, and all equivalent representations
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return false
}

// ValidateURLForOfflineMode checks if a URL is allowed in offline mode.
// Only localhost URLs with http/https schemes are permitted when offline mode is active.
//
// CRITICAL SECURITY: This function validates URL schemes to prevent:
// - file:// URLs (local file access/exfiltration)
// - javascript:// URLs (XSS in web contexts)
// - data:// URLs (potential data exfiltration)
// - Custom protocol handlers (potential code execution)
//
// Only http:// and https:// are allowed for legitimate network operations.
// SECURITY FIX: URL scheme validation is ALWAYS performed, regardless of offline mode.
func ValidateURLForOfflineMode(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrNetworkBlocked
	}

	// CRITICAL FIX: ALWAYS validate URL scheme regardless of offline mode
	// This prevents dangerous schemes like file://, javascript://, data://, etc.
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidURLScheme
	}

	// Additional localhost check only applies in offline mode
	if IsOfflineMode() {
		host := parsed.Hostname()
		if !IsLocalhost(host) {
			return ErrNonLocalhost
		}
	}

	return nil
}

// ValidateOllamaURL validates that an Ollama URL is allowed in offline mode.
// Returns nil if valid, error if blocked.
func ValidateOllamaURL(baseURL string) error {
	return ValidateURLForOfflineMode(baseURL)
}

// =============================================================================
// FEATURE GUARDS
// =============================================================================

// CheckCloudAllowed returns an error if cloud services are not allowed.
func CheckCloudAllowed() error {
	if IsOfflineMode() {
		return ErrCloudBlocked
	}
	return nil
}

// CheckWebFetchAllowed returns an error if web fetch is not allowed.
func CheckWebFetchAllowed() error {
	if IsOfflineMode() {
		return ErrWebFetchBlocked
	}
	return nil
}

// CheckTelemetryAllowed returns an error if telemetry/updates are not allowed.
func CheckTelemetryAllowed() error {
	if IsOfflineMode() {
		return ErrTelemetryBlocked
	}
	return nil
}

// CheckNetworkAllowed returns an error if any network operation is not allowed.
func CheckNetworkAllowed() error {
	if IsOfflineMode() {
		return ErrNetworkBlocked
	}
	return nil
}

// =============================================================================
// STATUS DISPLAY
// =============================================================================

// StatusIndicator returns the offline mode status indicator for display.
// Returns "OFFLINE MODE" when offline, empty string otherwise.
func StatusIndicator() string {
	if IsOfflineMode() {
		return "OFFLINE MODE"
	}
	return ""
}

// StatusBadge returns a formatted badge for the UI.
// Returns "[OFFLINE]" when offline, empty string otherwise.
func StatusBadge() string {
	if IsOfflineMode() {
		return "[OFFLINE]"
	}
	return ""
}

// ComplianceInfo returns IL5 compliance information for offline mode.
func ComplianceInfo() string {
	if IsOfflineMode() {
		return `IL5 Offline Mode Active
=======================
SC-7:  Boundary Protection - All external network blocked
SC-8:  Transmission Confidentiality - No external data transmission
CA-3:  System Interconnections - Only localhost Ollama allowed

All cloud services, web fetch, and telemetry are DISABLED.`
	}
	return ""
}
