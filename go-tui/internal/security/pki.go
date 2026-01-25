// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security implements NIST 800-53 security controls.
//
// This file implements SC-17 (PKI Certificates):
//   - TLS certificate validation for HTTPS connections
//   - Certificate chain verification
//   - Certificate expiration checking
//   - Optional certificate pinning
//   - CAC/PKI client certificate authentication (IA-2(12))

package security

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// ERRORS
// =============================================================================

var (
	// ErrCertExpired indicates the certificate has expired.
	ErrCertExpired = errors.New("certificate has expired")

	// ErrCertNotYetValid indicates the certificate is not yet valid.
	ErrCertNotYetValid = errors.New("certificate is not yet valid")

	// ErrCertChainInvalid indicates the certificate chain could not be verified.
	ErrCertChainInvalid = errors.New("certificate chain verification failed")

	// ErrCertPinMismatch indicates the certificate fingerprint does not match the pinned value.
	ErrCertPinMismatch = errors.New("certificate fingerprint does not match pinned value")

	// ErrNoCertificates indicates no certificates were presented.
	ErrNoCertificates = errors.New("no certificates presented by server")

	// ErrCertRevoked indicates the certificate has been revoked (CRL check).
	ErrCertRevoked = errors.New("certificate has been revoked")

	// ErrCRLUnavailable indicates CRL could not be retrieved for validation.
	ErrCRLUnavailable = errors.New("CRL unavailable for certificate validation")

	// ErrNoClientCertificate indicates no client certificate was provided.
	ErrNoClientCertificate = errors.New("no client certificate provided")

	// ErrCertNotTrusted indicates the certificate is not from a trusted CA.
	ErrCertNotTrusted = errors.New("certificate is not from a trusted CA")
)

// =============================================================================
// PKI MANAGER
// =============================================================================

// PKIManager manages PKI certificates and TLS configurations.
// Implements NIST 800-53 SC-17 (PKI Certificates).
type PKIManager struct {
	mu sync.RWMutex

	// pinnedCerts maps hostname to certificate fingerprint (SHA-256)
	pinnedCerts map[string]string

	// trustStore is the path to a custom CA bundle (empty = system default)
	trustStore string

	// tlsMinVersion is the minimum TLS version to accept
	tlsMinVersion uint16

	// tlsMaxVersion is the maximum TLS version to accept
	tlsMaxVersion uint16

	// certPinningEnabled controls whether certificate pinning is enforced
	certPinningEnabled bool

	// certValidationCache caches recent certificate validations
	certValidationCache map[string]*CertStatus

	// cacheExpiry is how long to cache certificate validations
	cacheExpiry time.Duration
}

// PKIManagerConfig holds configuration for PKIManager.
type PKIManagerConfig struct {
	TLSMinVersion      string            `toml:"tls_min_version" json:"tls_min_version"`
	CertPinningEnabled bool              `toml:"certificate_pinning" json:"certificate_pinning"`
	PinnedCertificates map[string]string `toml:"pinned_certificates" json:"pinned_certificates"`
	TrustStore         string            `toml:"trust_store" json:"trust_store"`
}

// =============================================================================
// CAC/PKI CLIENT CERTIFICATE AUTHENTICATION (IA-2(12))
// =============================================================================

// PKIConfig holds certificate-based authentication settings.
// This implements IA-2(12): Personal Identity Verification (CAC/PIV).
type PKIConfig struct {
	// Enabled indicates whether certificate authentication is enabled.
	Enabled bool `json:"enabled" toml:"enabled"`

	// TrustedCACerts is a list of paths to trusted CA certificate files.
	// These CAs are used to validate client certificates.
	TrustedCACerts []string `json:"trusted_ca_certs" toml:"trusted_ca_certs"`

	// RequireClientCert indicates whether client certificates are required.
	// If true, TLS connections without valid client certs will be rejected.
	RequireClientCert bool `json:"require_client_cert" toml:"require_client_cert"`

	// ValidateCRL indicates whether to check certificate revocation lists.
	// This is required for IL5 compliance but may impact performance.
	ValidateCRL bool `json:"validate_crl" toml:"validate_crl"`

	// CRLURLs is a list of CRL distribution point URLs for revocation checking.
	// If empty and ValidateCRL is true, CRLs are retrieved from certificate extensions.
	CRLURLs []string `json:"crl_urls" toml:"crl_urls"`

	// CRLCacheDuration specifies how long to cache CRL data.
	// Default: 1 hour. Set to 0 to disable caching.
	CRLCacheDuration time.Duration `json:"crl_cache_duration" toml:"crl_cache_duration"`

	// AllowedIssuers restricts which CAs can issue valid client certificates.
	// If empty, any certificate from TrustedCACerts is accepted.
	AllowedIssuers []string `json:"allowed_issuers" toml:"allowed_issuers"`
}

// CertificateInfo contains information extracted from a client certificate.
// This is used for authentication and audit logging.
type CertificateInfo struct {
	// Subject is the certificate subject DN (Distinguished Name).
	Subject string `json:"subject"`

	// Issuer is the certificate issuer DN.
	Issuer string `json:"issuer"`

	// SerialNumber is the certificate serial number.
	SerialNumber string `json:"serial_number"`

	// NotBefore is the certificate validity start time.
	NotBefore time.Time `json:"not_before"`

	// NotAfter is the certificate validity end time.
	NotAfter time.Time `json:"not_after"`

	// Fingerprint is the SHA-256 fingerprint of the certificate.
	Fingerprint string `json:"fingerprint"`

	// CAC-specific fields extracted from certificate extensions
	// These are optional and may not be present in all certificates.

	// EDIPI is the DoD ID number from CAC (10-digit EDI Personal Identifier).
	// Extracted from the Subject Alternative Name or CN.
	EDIPI string `json:"edipi,omitempty"`

	// Email is the email address from the certificate.
	Email string `json:"email,omitempty"`

	// CommonName is the CN field from the subject.
	CommonName string `json:"common_name,omitempty"`

	// Organization is the O field from the subject.
	Organization string `json:"organization,omitempty"`

	// OrganizationalUnit is the OU field from the subject.
	OrganizationalUnit string `json:"organizational_unit,omitempty"`

	// DNSNames contains Subject Alternative Names (DNS entries).
	DNSNames []string `json:"dns_names,omitempty"`
}

// NewPKIManager creates a new PKIManager with default settings.
func NewPKIManager() *PKIManager {
	return &PKIManager{
		pinnedCerts:         make(map[string]string),
		tlsMinVersion:       tls.VersionTLS12,
		tlsMaxVersion:       tls.VersionTLS13,
		certPinningEnabled:  false,
		certValidationCache: make(map[string]*CertStatus),
		cacheExpiry:         5 * time.Minute,
	}
}

// NewPKIManagerWithConfig creates a PKIManager with the specified configuration.
func NewPKIManagerWithConfig(cfg PKIManagerConfig) (*PKIManager, error) {
	pm := NewPKIManager()

	// Parse TLS minimum version
	if cfg.TLSMinVersion != "" {
		version, err := ParseTLSVersion(cfg.TLSMinVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS min version: %w", err)
		}
		pm.tlsMinVersion = version
	}

	// Set certificate pinning
	pm.certPinningEnabled = cfg.CertPinningEnabled

	// Copy pinned certificates
	if cfg.PinnedCertificates != nil {
		for host, fingerprint := range cfg.PinnedCertificates {
			pm.pinnedCerts[normalizeHost(host)] = strings.ToLower(fingerprint)
		}
	}

	// Set trust store
	pm.trustStore = cfg.TrustStore

	return pm, nil
}

// =============================================================================
// TLS CONFIGURATION
// =============================================================================

// GetTLSConfig returns a secure TLS configuration for HTTPS connections.
func (p *PKIManager) GetTLSConfig() *tls.Config {
	p.mu.RLock()
	defer p.mu.RUnlock()

	config := &tls.Config{
		MinVersion:   p.tlsMinVersion,
		MaxVersion:   p.tlsMaxVersion,
		CipherSuites: FIPSApprovedCipherSuites(),
		CurvePreferences: []tls.CurveID{
			tls.CurveP384,
			tls.CurveP256,
		},
	}

	// Add custom verification if pinning is enabled
	if p.certPinningEnabled && len(p.pinnedCerts) > 0 {
		config.VerifyPeerCertificate = p.createPinVerifier()
	}

	return config
}

// GetTLSConfigForHost returns a TLS configuration for a specific host.
// This allows per-host customization like certificate pinning.
func (p *PKIManager) GetTLSConfigForHost(host string) *tls.Config {
	config := p.GetTLSConfig()

	// If we have a pinned certificate for this host, add verification
	normalizedHost := normalizeHost(host)
	p.mu.RLock()
	pinnedFingerprint, hasPinned := p.pinnedCerts[normalizedHost]
	p.mu.RUnlock()

	if hasPinned {
		config.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return ErrNoCertificates
			}

			// Compute fingerprint of the leaf certificate
			fingerprint := sha256.Sum256(rawCerts[0])
			actualFingerprint := hex.EncodeToString(fingerprint[:])

			if actualFingerprint != pinnedFingerprint {
				// Log the mismatch as a security event
				LogCertPinningMismatch("PKI", normalizedHost, pinnedFingerprint, actualFingerprint)
				return fmt.Errorf("%w: host=%s", ErrCertPinMismatch, normalizedHost)
			}

			return nil
		}
	}

	return config
}

// createPinVerifier creates a certificate pinning verifier function.
func (p *PKIManager) createPinVerifier() func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// Get the server name from the verified chains
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return nil // No verified chains, let normal verification handle it
		}

		cert := verifiedChains[0][0]
		host := cert.Subject.CommonName

		// Check all DNS names
		hosts := append([]string{host}, cert.DNSNames...)

		for _, h := range hosts {
			normalizedHost := normalizeHost(h)
			p.mu.RLock()
			pinnedFingerprint, hasPinned := p.pinnedCerts[normalizedHost]
			p.mu.RUnlock()

			if hasPinned {
				fingerprint := sha256.Sum256(rawCerts[0])
				actualFingerprint := hex.EncodeToString(fingerprint[:])

				if actualFingerprint != pinnedFingerprint {
					LogCertPinningMismatch("PKI", normalizedHost, pinnedFingerprint, actualFingerprint)
					return fmt.Errorf("%w: host=%s", ErrCertPinMismatch, normalizedHost)
				}
			}
		}

		return nil
	}
}

// =============================================================================
// CERTIFICATE VALIDATION
// =============================================================================

// ValidateCertificate validates the TLS certificate for a host.
func (p *PKIManager) ValidateCertificate(host string) (*CertStatus, error) {
	normalizedHost := normalizeHost(host)

	// Check cache first
	p.mu.RLock()
	if cached, ok := p.certValidationCache[normalizedHost]; ok {
		// Check if cache entry is still valid
		if time.Since(cached.ValidFrom) < p.cacheExpiry {
			p.mu.RUnlock()
			return cached, nil
		}
	}
	p.mu.RUnlock()

	// Connect and get certificate
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		normalizedHost+":443",
		&tls.Config{
			MinVersion: p.tlsMinVersion,
			MaxVersion: p.tlsMaxVersion,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", host, err)
	}
	defer conn.Close()

	// Get peer certificates
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, ErrNoCertificates
	}

	cert := state.PeerCertificates[0]
	now := time.Now()

	// Calculate fingerprint
	fingerprint := sha256.Sum256(cert.Raw)
	fingerprintHex := hex.EncodeToString(fingerprint[:])

	// Build status
	status := &CertStatus{
		Subject:         cert.Subject.CommonName,
		Issuer:          cert.Issuer.CommonName,
		ValidFrom:       cert.NotBefore,
		ValidUntil:      cert.NotAfter,
		DaysUntilExpiry: int(cert.NotAfter.Sub(now).Hours() / 24),
		ChainValid:      len(state.VerifiedChains) > 0,
		Fingerprint:     fingerprintHex,
		SerialNumber:    cert.SerialNumber.String(),
	}

	// Check pinning status
	p.mu.RLock()
	pinnedFingerprint, isPinned := p.pinnedCerts[normalizedHost]
	p.mu.RUnlock()

	status.Pinned = isPinned
	if isPinned && pinnedFingerprint != fingerprintHex {
		return status, fmt.Errorf("%w: expected %s, got %s", ErrCertPinMismatch, pinnedFingerprint, fingerprintHex)
	}

	// Check validity period
	if now.Before(cert.NotBefore) {
		return status, ErrCertNotYetValid
	}
	if now.After(cert.NotAfter) {
		return status, ErrCertExpired
	}

	// Cache the result
	p.mu.Lock()
	p.certValidationCache[normalizedHost] = status
	p.mu.Unlock()

	return status, nil
}

// CheckCertExpiry returns the number of days until the certificate expires.
// Returns a negative number if already expired.
func (p *PKIManager) CheckCertExpiry(host string) (int, error) {
	status, err := p.ValidateCertificate(host)
	if err != nil && !errors.Is(err, ErrCertExpired) && !errors.Is(err, ErrCertPinMismatch) {
		return 0, err
	}

	return status.DaysUntilExpiry, nil
}

// =============================================================================
// CERTIFICATE PINNING
// =============================================================================

// PinCertificate pins a certificate fingerprint for a host.
func (p *PKIManager) PinCertificate(host string, fingerprint string) error {
	normalizedHost := normalizeHost(host)
	normalizedFingerprint := strings.ToLower(strings.ReplaceAll(fingerprint, ":", ""))

	// Validate fingerprint format (SHA-256 = 64 hex chars)
	if len(normalizedFingerprint) != 64 {
		return fmt.Errorf("invalid fingerprint length: expected 64 hex chars, got %d", len(normalizedFingerprint))
	}

	// Validate hex encoding
	if _, err := hex.DecodeString(normalizedFingerprint); err != nil {
		return fmt.Errorf("invalid fingerprint format: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pinnedCerts[normalizedHost] = normalizedFingerprint
	return nil
}

// UnpinCertificate removes a certificate pin for a host.
func (p *PKIManager) UnpinCertificate(host string) {
	normalizedHost := normalizeHost(host)

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.pinnedCerts, normalizedHost)
}

// IsPinned returns whether a host has a pinned certificate.
func (p *PKIManager) IsPinned(host string) bool {
	normalizedHost := normalizeHost(host)

	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.pinnedCerts[normalizedHost]
	return ok
}

// GetPinnedFingerprint returns the pinned fingerprint for a host.
func (p *PKIManager) GetPinnedFingerprint(host string) (string, bool) {
	normalizedHost := normalizeHost(host)

	p.mu.RLock()
	defer p.mu.RUnlock()

	fingerprint, ok := p.pinnedCerts[normalizedHost]
	return fingerprint, ok
}

// GetPinnedHosts returns all hosts with pinned certificates.
func (p *PKIManager) GetPinnedHosts() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]string, len(p.pinnedCerts))
	for host, fp := range p.pinnedCerts {
		result[host] = fp
	}
	return result
}

// =============================================================================
// CERTIFICATE CHAIN VERIFICATION
// =============================================================================

// VerifyCertificateChain verifies a certificate chain against the system roots.
func VerifyCertificateChain(certs []*x509.Certificate) error {
	if len(certs) == 0 {
		return ErrNoCertificates
	}

	// Get system cert pool
	roots, err := x509.SystemCertPool()
	if err != nil {
		// Fall back to empty pool on some systems
		roots = x509.NewCertPool()
	}

	// Build intermediate pool
	intermediates := x509.NewCertPool()
	for i := 1; i < len(certs); i++ {
		intermediates.AddCert(certs[i])
	}

	// Verify the leaf certificate
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	_, err = certs[0].Verify(opts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCertChainInvalid, err)
	}

	return nil
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetTLSMinVersion sets the minimum TLS version.
func (p *PKIManager) SetTLSMinVersion(version uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tlsMinVersion = version
}

// GetTLSMinVersion returns the minimum TLS version.
func (p *PKIManager) GetTLSMinVersion() uint16 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tlsMinVersion
}

// SetCertPinningEnabled enables or disables certificate pinning.
func (p *PKIManager) SetCertPinningEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.certPinningEnabled = enabled
}

// IsCertPinningEnabled returns whether certificate pinning is enabled.
func (p *PKIManager) IsCertPinningEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.certPinningEnabled
}

// ClearCache clears the certificate validation cache.
func (p *PKIManager) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.certValidationCache = make(map[string]*CertStatus)
}

// =============================================================================
// CLIENT CERTIFICATE VALIDATION (IA-2(12))
// =============================================================================

// ValidateClientCertificate validates a client certificate for authentication.
// This implements IA-2(12): Personal Identity Verification.
//
// Validation steps:
//  1. Check certificate expiration (NotBefore/NotAfter)
//  2. Verify certificate chain against trusted CAs
//  3. Check certificate revocation (CRL) if configured
//  4. Extract certificate information for authentication
//
// Returns CertificateInfo on success, or an error describing the validation failure.
func ValidateClientCertificate(cert *x509.Certificate, config *PKIConfig) (*CertificateInfo, error) {
	if cert == nil {
		return nil, ErrNoClientCertificate
	}

	if config == nil {
		return nil, errors.New("PKI configuration is nil")
	}

	// Step 1: Check expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return nil, fmt.Errorf("%w: certificate not valid until %s", ErrCertNotYetValid, cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("%w: certificate expired on %s", ErrCertExpired, cert.NotAfter)
	}

	// Step 2: Verify against trusted CAs
	if err := verifyAgainstTrustedCAs(cert, config); err != nil {
		return nil, err
	}

	// Step 3: Check CRL if configured
	if config.ValidateCRL {
		if err := checkCertificateRevocation(cert, config); err != nil {
			return nil, err
		}
	}

	// Step 4: Extract certificate information
	info := extractCertificateInfo(cert)

	return info, nil
}

// verifyAgainstTrustedCAs verifies a certificate against the configured trusted CAs.
func verifyAgainstTrustedCAs(cert *x509.Certificate, config *PKIConfig) error {
	// Build CA pool from configured trusted CAs
	caPool := x509.NewCertPool()

	if len(config.TrustedCACerts) == 0 {
		// Fall back to system cert pool if no custom CAs configured
		systemPool, err := x509.SystemCertPool()
		if err != nil {
			return fmt.Errorf("failed to load system cert pool: %w", err)
		}
		caPool = systemPool
	} else {
		// Load each trusted CA certificate
		for _, caPath := range config.TrustedCACerts {
			caCert, err := os.ReadFile(caPath)
			if err != nil {
				return fmt.Errorf("failed to read CA certificate %s: %w", caPath, err)
			}

			// Try to parse as PEM
			block, _ := pem.Decode(caCert)
			if block != nil {
				caCert = block.Bytes
			}

			// Parse the certificate
			ca, err := x509.ParseCertificate(caCert)
			if err != nil {
				return fmt.Errorf("failed to parse CA certificate %s: %w", caPath, err)
			}

			caPool.AddCert(ca)
		}
	}

	// Verify the certificate chain
	opts := x509.VerifyOptions{
		Roots:       caPool,
		CurrentTime: time.Now(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	chains, err := cert.Verify(opts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCertNotTrusted, err)
	}

	// Check if issuer is in allowed list (if configured)
	if len(config.AllowedIssuers) > 0 {
		issuerDN := cert.Issuer.String()
		allowed := false
		for _, allowedIssuer := range config.AllowedIssuers {
			if issuerDN == allowedIssuer || strings.Contains(issuerDN, allowedIssuer) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("certificate issuer %s is not in allowed issuers list", issuerDN)
		}
	}

	// Ensure we have at least one valid chain
	if len(chains) == 0 {
		return ErrCertChainInvalid
	}

	return nil
}

// checkCertificateRevocation checks if a certificate has been revoked using CRL.
// This is required for IL5 compliance per NIST 800-53 IA-5(2)(a).
func checkCertificateRevocation(cert *x509.Certificate, config *PKIConfig) error {
	// Get CRL URLs (from config or certificate)
	crlURLs := config.CRLURLs
	if len(crlURLs) == 0 {
		// Extract CRL distribution points from certificate
		crlURLs = cert.CRLDistributionPoints
	}

	if len(crlURLs) == 0 {
		// No CRL URLs available - this is a warning condition
		// For strict IL5 compliance, you may want to return an error here
		return fmt.Errorf("%w: no CRL distribution points available", ErrCRLUnavailable)
	}

	// Check each CRL
	for _, crlURL := range crlURLs {
		crl, err := fetchCRL(crlURL, config.CRLCacheDuration)
		if err != nil {
			// Log warning but continue - try other CRLs
			continue
		}

		// Check if certificate is revoked
		// Note: Go 1.19+ uses RevokedCertificates field directly on RevocationList
		for _, revokedCert := range crl.RevokedCertificates {
			if revokedCert.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return fmt.Errorf("%w: serial number %s revoked on %s",
					ErrCertRevoked, cert.SerialNumber, revokedCert.RevocationTime)
			}
		}

		// Found valid CRL and cert not revoked - success
		return nil
	}

	// Could not retrieve any CRL
	return fmt.Errorf("%w: failed to retrieve CRL from any distribution point", ErrCRLUnavailable)
}

// crlCache stores downloaded CRLs to avoid repeated downloads.
var (
	crlCache   = make(map[string]*crlCacheEntry)
	crlCacheMu sync.RWMutex
)

type crlCacheEntry struct {
	crl        *x509.RevocationList
	fetchedAt  time.Time
	expiration time.Duration
}

// fetchCRL retrieves a CRL from a URL with caching support.
func fetchCRL(url string, cacheDuration time.Duration) (*x509.RevocationList, error) {
	// Check cache first
	crlCacheMu.RLock()
	entry, exists := crlCache[url]
	crlCacheMu.RUnlock()

	if exists && time.Since(entry.fetchedAt) < entry.expiration {
		return entry.crl, nil
	}

	// Fetch CRL
	var crlData []byte
	var err error

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// HTTP/HTTPS URL
		resp, fetchErr := http.Get(url)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch CRL from %s: %w", url, fetchErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch CRL from %s: HTTP %d", url, resp.StatusCode)
		}

		crlData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRL from %s: %w", url, err)
		}
	} else if strings.HasPrefix(url, "file://") {
		// Local file URL
		filePath := strings.TrimPrefix(url, "file://")
		crlData, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRL file %s: %w", filePath, err)
		}
	} else {
		return nil, fmt.Errorf("unsupported CRL URL scheme: %s", url)
	}

	// Try to parse as PEM first
	block, _ := pem.Decode(crlData)
	if block != nil {
		crlData = block.Bytes
	}

	// Parse CRL
	crl, err := x509.ParseRevocationList(crlData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CRL from %s: %w", url, err)
	}

	// Cache the CRL
	if cacheDuration > 0 {
		crlCacheMu.Lock()
		crlCache[url] = &crlCacheEntry{
			crl:        crl,
			fetchedAt:  time.Now(),
			expiration: cacheDuration,
		}
		crlCacheMu.Unlock()
	}

	return crl, nil
}

// extractCertificateInfo extracts relevant information from a certificate.
func extractCertificateInfo(cert *x509.Certificate) *CertificateInfo {
	info := &CertificateInfo{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		SerialNumber: cert.SerialNumber.String(),
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		Fingerprint:  ComputeCertFingerprint(cert),
		DNSNames:     cert.DNSNames,
	}

	// Extract common fields from subject
	if len(cert.Subject.CommonName) > 0 {
		info.CommonName = cert.Subject.CommonName
	}
	if len(cert.Subject.Organization) > 0 {
		info.Organization = cert.Subject.Organization[0]
	}
	if len(cert.Subject.OrganizationalUnit) > 0 {
		info.OrganizationalUnit = cert.Subject.OrganizationalUnit[0]
	}

	// Extract email from subject or SAN
	if len(cert.EmailAddresses) > 0 {
		info.Email = cert.EmailAddresses[0]
	}

	// Try to extract EDIPI from Subject Alternative Name or CN
	// EDIPI is typically a 10-digit number in DoD CAC certificates
	info.EDIPI = extractEDIPI(cert)

	return info
}

// extractEDIPI attempts to extract the EDIPI (DoD ID) from a certificate.
// This is a best-effort extraction and may not work for all CAC formats.
func extractEDIPI(cert *x509.Certificate) string {
	// Try to find 10-digit number in CN
	cn := cert.Subject.CommonName
	if edipi := extractDigits(cn, 10); edipi != "" {
		return edipi
	}

	// Try email address (sometimes EDIPI.last@mil)
	if len(cert.EmailAddresses) > 0 {
		email := cert.EmailAddresses[0]
		parts := strings.Split(email, "@")
		if len(parts) > 0 {
			if edipi := extractDigits(parts[0], 10); edipi != "" {
				return edipi
			}
		}
	}

	// Could also check Subject Alternative Names, OIDs, etc.
	// This is a simplified implementation

	return ""
}

// extractDigits extracts a sequence of exactly n digits from a string.
func extractDigits(s string, n int) string {
	var digits []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
			if len(digits) == n {
				return string(digits)
			}
		} else if len(digits) > 0 {
			// Reset if we hit a non-digit after starting to collect
			digits = nil
		}
	}
	return ""
}

// GetTLSConfigForClientAuth returns a TLS config for server-side client certificate authentication.
// This should be used when configuring a TLS server to require client certificates.
func (p *PKIManager) GetTLSConfigForClientAuth(config *PKIConfig) (*tls.Config, error) {
	if config == nil || !config.Enabled {
		return nil, errors.New("PKI client auth is not enabled")
	}

	tlsConfig := p.GetTLSConfig()

	// Set client auth policy
	if config.RequireClientCert {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	}

	// Build client CA pool
	if len(config.TrustedCACerts) > 0 {
		clientCAPool := x509.NewCertPool()
		for _, caPath := range config.TrustedCACerts {
			caCert, err := os.ReadFile(caPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read client CA certificate %s: %w", caPath, err)
			}

			if !clientCAPool.AppendCertsFromPEM(caCert) {
				// Try to parse as DER
				block, _ := pem.Decode(caCert)
				if block != nil {
					caCert = block.Bytes
				}
				ca, err := x509.ParseCertificate(caCert)
				if err != nil {
					return nil, fmt.Errorf("failed to parse client CA certificate %s: %w", caPath, err)
				}
				clientCAPool.AddCert(ca)
			}
		}
		tlsConfig.ClientCAs = clientCAPool
	}

	return tlsConfig, nil
}

// =============================================================================
// HELPERS
// =============================================================================

// normalizeHost normalizes a hostname for consistent lookup.
func normalizeHost(host string) string {
	// Remove port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Lowercase
	return strings.ToLower(strings.TrimSpace(host))
}

// ComputeCertFingerprint computes the SHA-256 fingerprint of a certificate.
func ComputeCertFingerprint(cert *x509.Certificate) string {
	fingerprint := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(fingerprint[:])
}

// FormatFingerprint formats a fingerprint with colons for display.
func FormatFingerprint(fingerprint string) string {
	var parts []string
	for i := 0; i < len(fingerprint); i += 2 {
		end := i + 2
		if end > len(fingerprint) {
			end = len(fingerprint)
		}
		parts = append(parts, fingerprint[i:end])
	}
	return strings.ToUpper(strings.Join(parts, ":"))
}

// =============================================================================
// GLOBAL PKI MANAGER
// =============================================================================

var (
	globalPKIManager     *PKIManager
	globalPKIManagerOnce sync.Once
	globalPKIManagerMu   sync.Mutex
)

// GlobalPKIManager returns the global PKI manager instance.
func GlobalPKIManager() *PKIManager {
	globalPKIManagerOnce.Do(func() {
		globalPKIManager = NewPKIManager()
	})
	return globalPKIManager
}

// SetGlobalPKIManager sets the global PKI manager instance.
func SetGlobalPKIManager(pm *PKIManager) {
	globalPKIManagerMu.Lock()
	defer globalPKIManagerMu.Unlock()
	globalPKIManager = pm
}

// InitGlobalPKIManager initializes the global PKI manager with configuration.
func InitGlobalPKIManager(cfg PKIManagerConfig) error {
	pm, err := NewPKIManagerWithConfig(cfg)
	if err != nil {
		return err
	}

	SetGlobalPKIManager(pm)
	return nil
}
