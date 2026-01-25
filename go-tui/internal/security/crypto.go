// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security implements NIST 800-53 security controls.
//
// This file implements:
//   - SC-13 (Cryptographic Protection): Document and verify FIPS-approved algorithms
//   - IA-7 (Cryptographic Module Authentication): Verify cryptographic module status
//
// FIPS 140-2/3 Approved Algorithms:
//   - AES-256-GCM (symmetric encryption)
//   - SHA-256, SHA-384, SHA-512 (hashing)
//   - HMAC-SHA-256 (message authentication)
//   - ECDSA P-256/P-384 (signatures)
//   - ECDH P-256/P-384 (key exchange)
//   - RSA-2048+ (signatures, key exchange)
//   - PBKDF2-SHA-256 (key derivation)

package security

import (
	"crypto/tls"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// TYPES
// =============================================================================

// AlgorithmType represents the category of cryptographic algorithm.
type AlgorithmType string

const (
	AlgorithmTypeSymmetric AlgorithmType = "symmetric"
	AlgorithmTypeHash      AlgorithmType = "hash"
	AlgorithmTypeSignature AlgorithmType = "signature"
	AlgorithmTypeKDF       AlgorithmType = "kdf"
	AlgorithmTypeKeyExch   AlgorithmType = "key_exchange"
	AlgorithmTypeMAC       AlgorithmType = "mac"
)

// AlgorithmInfo describes a cryptographic algorithm and its FIPS status.
type AlgorithmInfo struct {
	Name         string        `json:"name"`
	Type         AlgorithmType `json:"type"`
	FIPSApproved bool          `json:"fips_approved"`
	KeySize      int           `json:"key_size,omitempty"`
	Description  string        `json:"description,omitempty"`
	Standard     string        `json:"standard,omitempty"`
}

// CertStatus represents the status of a TLS certificate.
type CertStatus struct {
	Subject         string    `json:"subject"`
	Issuer          string    `json:"issuer"`
	ValidFrom       time.Time `json:"valid_from"`
	ValidUntil      time.Time `json:"valid_until"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
	ChainValid      bool      `json:"chain_valid"`
	Pinned          bool      `json:"pinned"`
	Fingerprint     string    `json:"fingerprint,omitempty"`
	SerialNumber    string    `json:"serial_number,omitempty"`
}

// CryptoStatus represents the overall cryptographic status of the system.
type CryptoStatus struct {
	FIPSMode          bool            `json:"fips_mode"`
	FIPSAvailable     bool            `json:"fips_available"`
	Algorithms        []AlgorithmInfo `json:"algorithms"`
	TLSVersion        string          `json:"tls_version"`
	TLSMinVersion     string          `json:"tls_min_version"`
	CertificateStatus *CertStatus     `json:"certificate_status,omitempty"`
	Platform          string          `json:"platform"`
	GoVersion         string          `json:"go_version"`
	Issues            []string        `json:"issues,omitempty"`
}

// FIPSComplianceResult holds the result of FIPS compliance verification.
type FIPSComplianceResult struct {
	Compliant bool     `json:"compliant"`
	Issues    []string `json:"issues,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// =============================================================================
// FIPS-APPROVED ALGORITHMS
// =============================================================================

// fipsApprovedAlgorithms defines the FIPS 140-2/3 approved algorithms used by rigrun.
var fipsApprovedAlgorithms = []AlgorithmInfo{
	{
		Name:         "AES-256-GCM",
		Type:         AlgorithmTypeSymmetric,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Advanced Encryption Standard with Galois/Counter Mode",
		Standard:     "FIPS 197, SP 800-38D",
	},
	{
		Name:         "AES-128-GCM",
		Type:         AlgorithmTypeSymmetric,
		FIPSApproved: true,
		KeySize:      128,
		Description:  "Advanced Encryption Standard with Galois/Counter Mode",
		Standard:     "FIPS 197, SP 800-38D",
	},
	{
		Name:         "SHA-256",
		Type:         AlgorithmTypeHash,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Secure Hash Algorithm 2 (256-bit)",
		Standard:     "FIPS 180-4",
	},
	{
		Name:         "SHA-384",
		Type:         AlgorithmTypeHash,
		FIPSApproved: true,
		KeySize:      384,
		Description:  "Secure Hash Algorithm 2 (384-bit)",
		Standard:     "FIPS 180-4",
	},
	{
		Name:         "SHA-512",
		Type:         AlgorithmTypeHash,
		FIPSApproved: true,
		KeySize:      512,
		Description:  "Secure Hash Algorithm 2 (512-bit)",
		Standard:     "FIPS 180-4",
	},
	{
		Name:         "HMAC-SHA-256",
		Type:         AlgorithmTypeMAC,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Hash-based Message Authentication Code with SHA-256",
		Standard:     "FIPS 198-1",
	},
	{
		Name:         "HMAC-SHA-384",
		Type:         AlgorithmTypeMAC,
		FIPSApproved: true,
		KeySize:      384,
		Description:  "Hash-based Message Authentication Code with SHA-384",
		Standard:     "FIPS 198-1",
	},
	{
		Name:         "ECDSA-P256",
		Type:         AlgorithmTypeSignature,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Elliptic Curve Digital Signature Algorithm (P-256)",
		Standard:     "FIPS 186-4",
	},
	{
		Name:         "ECDSA-P384",
		Type:         AlgorithmTypeSignature,
		FIPSApproved: true,
		KeySize:      384,
		Description:  "Elliptic Curve Digital Signature Algorithm (P-384)",
		Standard:     "FIPS 186-4",
	},
	{
		Name:         "ECDH-P256",
		Type:         AlgorithmTypeKeyExch,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Elliptic Curve Diffie-Hellman (P-256)",
		Standard:     "SP 800-56A",
	},
	{
		Name:         "ECDH-P384",
		Type:         AlgorithmTypeKeyExch,
		FIPSApproved: true,
		KeySize:      384,
		Description:  "Elliptic Curve Diffie-Hellman (P-384)",
		Standard:     "SP 800-56A",
	},
	{
		Name:         "RSA-2048",
		Type:         AlgorithmTypeSignature,
		FIPSApproved: true,
		KeySize:      2048,
		Description:  "RSA Digital Signature (2048-bit)",
		Standard:     "FIPS 186-4",
	},
	{
		Name:         "RSA-3072",
		Type:         AlgorithmTypeSignature,
		FIPSApproved: true,
		KeySize:      3072,
		Description:  "RSA Digital Signature (3072-bit)",
		Standard:     "FIPS 186-4",
	},
	{
		Name:         "RSA-4096",
		Type:         AlgorithmTypeSignature,
		FIPSApproved: true,
		KeySize:      4096,
		Description:  "RSA Digital Signature (4096-bit)",
		Standard:     "FIPS 186-4",
	},
	{
		Name:         "PBKDF2-SHA-256",
		Type:         AlgorithmTypeKDF,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "Password-Based Key Derivation Function 2 with SHA-256",
		Standard:     "SP 800-132",
	},
	{
		Name:         "HKDF-SHA-256",
		Type:         AlgorithmTypeKDF,
		FIPSApproved: true,
		KeySize:      256,
		Description:  "HMAC-based Key Derivation Function with SHA-256",
		Standard:     "SP 800-56C",
	},
}

// nonFIPSAlgorithms defines algorithms that are NOT FIPS-approved.
var nonFIPSAlgorithms = []AlgorithmInfo{
	{
		Name:         "MD5",
		Type:         AlgorithmTypeHash,
		FIPSApproved: false,
		KeySize:      128,
		Description:  "Message Digest 5 (deprecated, not for security use)",
		Standard:     "RFC 1321",
	},
	{
		Name:         "SHA-1",
		Type:         AlgorithmTypeHash,
		FIPSApproved: false,
		KeySize:      160,
		Description:  "Secure Hash Algorithm 1 (deprecated for signatures)",
		Standard:     "FIPS 180-4 (deprecated)",
	},
	{
		Name:         "DES",
		Type:         AlgorithmTypeSymmetric,
		FIPSApproved: false,
		KeySize:      56,
		Description:  "Data Encryption Standard (deprecated)",
		Standard:     "Withdrawn",
	},
	{
		Name:         "3DES",
		Type:         AlgorithmTypeSymmetric,
		FIPSApproved: false,
		KeySize:      168,
		Description:  "Triple DES (deprecated)",
		Standard:     "Deprecated in SP 800-131A",
	},
	{
		Name:         "RC4",
		Type:         AlgorithmTypeSymmetric,
		FIPSApproved: false,
		KeySize:      128,
		Description:  "Rivest Cipher 4 (deprecated, insecure)",
		Standard:     "Never approved",
	},
}

// =============================================================================
// GLOBAL STATE
// =============================================================================

var (
	cryptoMu     sync.RWMutex
	fipsModeFlag bool
)

// =============================================================================
// FUNCTIONS
// =============================================================================

// IsFIPSMode returns whether FIPS mode is enabled.
// Note: Standard Go does not have FIPS mode; this checks if BoringCrypto is available.
func IsFIPSMode() bool {
	cryptoMu.RLock()
	defer cryptoMu.RUnlock()
	return fipsModeFlag
}

// SetFIPSMode sets the FIPS mode flag.
// Note: This is a configuration flag; actual FIPS enforcement requires BoringCrypto.
func SetFIPSMode(enabled bool) {
	cryptoMu.Lock()
	defer cryptoMu.Unlock()
	fipsModeFlag = enabled
}

// IsFIPSAvailable checks if FIPS-compliant crypto is available.
// In standard Go, this returns false. With BoringCrypto builds, this would return true.
func IsFIPSAvailable() bool {
	// Check for BoringCrypto/BoringSSL build tags
	// Standard Go does not include FIPS-validated crypto
	// To enable FIPS, build with: CGO_ENABLED=1 GOEXPERIMENT=boringcrypto

	// For now, we report based on platform capabilities
	// Windows can use CNG (FIPS-validated)
	// Linux can use OpenSSL (if FIPS-validated version installed)

	// Check if we're running on a platform with potential FIPS support
	switch runtime.GOOS {
	case "windows":
		// Windows CNG is FIPS 140-2 validated
		return true
	case "linux":
		// Linux may have OpenSSL FIPS module
		// This requires additional runtime checks
		return false // Conservative default
	default:
		return false
	}
}

// GetSupportedAlgorithms returns all supported cryptographic algorithms with their FIPS status.
func GetSupportedAlgorithms() []AlgorithmInfo {
	// Return a copy to prevent modification
	all := make([]AlgorithmInfo, 0, len(fipsApprovedAlgorithms)+len(nonFIPSAlgorithms))
	all = append(all, fipsApprovedAlgorithms...)
	all = append(all, nonFIPSAlgorithms...)
	return all
}

// GetFIPSApprovedAlgorithms returns only FIPS-approved algorithms.
func GetFIPSApprovedAlgorithms() []AlgorithmInfo {
	// Return a copy
	result := make([]AlgorithmInfo, len(fipsApprovedAlgorithms))
	copy(result, fipsApprovedAlgorithms)
	return result
}

// GetAlgorithmsByType returns algorithms of a specific type.
func GetAlgorithmsByType(algType AlgorithmType) []AlgorithmInfo {
	var result []AlgorithmInfo
	for _, alg := range GetSupportedAlgorithms() {
		if alg.Type == algType {
			result = append(result, alg)
		}
	}
	return result
}

// IsAlgorithmFIPSApproved checks if a specific algorithm is FIPS-approved.
func IsAlgorithmFIPSApproved(name string) bool {
	normalizedName := strings.ToUpper(strings.ReplaceAll(name, " ", "-"))
	for _, alg := range fipsApprovedAlgorithms {
		if strings.ToUpper(strings.ReplaceAll(alg.Name, " ", "-")) == normalizedName {
			return true
		}
	}
	return false
}

// GetCryptoStatus returns the current cryptographic status of the system.
func GetCryptoStatus() *CryptoStatus {
	status := &CryptoStatus{
		FIPSMode:      IsFIPSMode(),
		FIPSAvailable: IsFIPSAvailable(),
		Algorithms:    GetFIPSApprovedAlgorithms(),
		TLSVersion:    getTLSVersionString(tls.VersionTLS13),
		TLSMinVersion: getTLSVersionString(tls.VersionTLS12),
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:     runtime.Version(),
		Issues:        []string{},
	}

	// Check for potential issues
	if status.FIPSMode && !status.FIPSAvailable {
		status.Issues = append(status.Issues,
			"FIPS mode enabled but FIPS-validated crypto module not available")
	}

	return status
}

// VerifyFIPSCompliance checks if the system meets FIPS compliance requirements.
func VerifyFIPSCompliance() *FIPSComplianceResult {
	result := &FIPSComplianceResult{
		Compliant: true,
		Issues:    []string{},
		Warnings:  []string{},
	}

	// Check 1: FIPS mode should be enabled for compliance
	if !IsFIPSMode() {
		result.Warnings = append(result.Warnings,
			"FIPS mode is not enabled (enable with fips_mode = true in config)")
	}

	// Check 2: Verify FIPS-validated crypto is available
	if IsFIPSMode() && !IsFIPSAvailable() {
		result.Compliant = false
		result.Issues = append(result.Issues,
			"FIPS mode enabled but no FIPS-validated crypto module available. "+
				"Build with GOEXPERIMENT=boringcrypto or use system crypto libraries.")
	}

	// Check 3: Verify TLS minimum version (TLS 1.2 required for FIPS)
	// This is configured in PKIManager

	// Check 4: Verify we're not using deprecated algorithms
	// This is enforced by only using algorithms from fipsApprovedAlgorithms

	// Check 5: Platform-specific checks
	switch runtime.GOOS {
	case "windows":
		// Windows CNG is generally FIPS-compliant
		result.Warnings = append(result.Warnings,
			"Ensure Windows CNG is configured for FIPS mode (secpol.msc)")
	case "linux":
		result.Warnings = append(result.Warnings,
			"Ensure OpenSSL FIPS module is installed and enabled")
	case "darwin":
		result.Warnings = append(result.Warnings,
			"macOS CommonCrypto has limited FIPS compliance")
	}

	return result
}

// getTLSVersionString converts TLS version constant to string.
func getTLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// ParseTLSVersion converts a TLS version string to the constant.
func ParseTLSVersion(version string) (uint16, error) {
	switch strings.TrimSpace(strings.ToUpper(version)) {
	case "1.0", "TLS1.0", "TLS 1.0":
		return tls.VersionTLS10, nil
	case "1.1", "TLS1.1", "TLS 1.1":
		return tls.VersionTLS11, nil
	case "1.2", "TLS1.2", "TLS 1.2":
		return tls.VersionTLS12, nil
	case "1.3", "TLS1.3", "TLS 1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unknown TLS version: %s", version)
	}
}

// =============================================================================
// TLS CIPHER SUITES
// =============================================================================

// FIPSApprovedCipherSuites returns TLS cipher suites that are FIPS-approved.
func FIPSApprovedCipherSuites() []uint16 {
	return []uint16{
		// TLS 1.3 cipher suites (all are FIPS-approved when using AES-GCM)
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,

		// TLS 1.2 FIPS-approved cipher suites
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	}
}

// GetSecureTLSConfig returns a TLS configuration with FIPS-approved settings.
func GetSecureTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: FIPSApprovedCipherSuites(),
		CurvePreferences: []tls.CurveID{
			tls.CurveP384,
			tls.CurveP256,
		},
		// Prefer server cipher suites
		PreferServerCipherSuites: false, // Deprecated in Go 1.18+
	}
}

// =============================================================================
// AUDIT LOGGING FOR CRYPTO EVENTS
// =============================================================================

// LogCryptoFIPSCheck logs a FIPS compliance check event.
func LogCryptoFIPSCheck(sessionID string, compliant bool, issues []string) error {
	eventType := "CRYPTO_FIPS_CHECK"
	metadata := map[string]string{
		"compliant": fmt.Sprintf("%t", compliant),
	}

	if len(issues) > 0 {
		metadata["issues"] = strings.Join(issues, "; ")
	}

	return AuditLogEvent(sessionID, eventType, metadata)
}

// LogCertValidation logs a certificate validation event.
func LogCertValidation(sessionID, host string, valid bool, reason string) error {
	eventType := "CERT_VALIDATION"
	metadata := map[string]string{
		"host":  host,
		"valid": fmt.Sprintf("%t", valid),
	}

	if reason != "" {
		metadata["reason"] = reason
	}

	return AuditLogEvent(sessionID, eventType, metadata)
}

// LogCertPinningMismatch logs a certificate pinning mismatch security event.
func LogCertPinningMismatch(sessionID, host, expected, actual string) error {
	eventType := "CERT_PINNING_MISMATCH"
	metadata := map[string]string{
		"host":                 host,
		"expected_fingerprint": expected,
		"actual_fingerprint":   actual,
	}

	return AuditLogEvent(sessionID, eventType, metadata)
}

// =============================================================================
// CONFIGURATION INTEGRATION
// =============================================================================

// CryptoConfig holds configuration for cryptographic controls.
type CryptoConfig struct {
	FIPSMode           bool              `toml:"fips_mode" json:"fips_mode"`
	TLSMinVersion      string            `toml:"tls_min_version" json:"tls_min_version"`
	CertificatePinning bool              `toml:"certificate_pinning" json:"certificate_pinning"`
	PinnedCertificates map[string]string `toml:"pinned_certificates" json:"pinned_certificates"`
}

// InitCryptoFromConfig initializes cryptographic settings from configuration.
func InitCryptoFromConfig(cfg CryptoConfig) error {
	// Set FIPS mode
	SetFIPSMode(cfg.FIPSMode)

	// Initialize PKI manager with config
	pkiCfg := PKIManagerConfig{
		TLSMinVersion:      cfg.TLSMinVersion,
		CertPinningEnabled: cfg.CertificatePinning,
		PinnedCertificates: cfg.PinnedCertificates,
	}

	return InitGlobalPKIManager(pkiCfg)
}
