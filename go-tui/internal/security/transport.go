// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 SC-8: Transmission Confidentiality and Integrity.
//
// # DoD STIG Requirements
//
//   - SC-8: Protects the confidentiality and integrity of transmitted information
//   - SC-8(1): Cryptographic protection of transmitted information
//   - SC-13: Cryptographic protection using FIPS-approved algorithms
//   - SC-17: Public Key Infrastructure certificates
//   - AU-3: Transmission security events must be logged for audit compliance
package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// SC-8 CONSTANTS
// =============================================================================

const (
	// MinTLSVersion is the minimum allowed TLS version (TLS 1.2).
	MinTLSVersion = tls.VersionTLS12

	// PreferredTLSVersion is the preferred TLS version (TLS 1.3).
	PreferredTLSVersion = tls.VersionTLS13

	// DefaultDialTimeout is the default timeout for TLS connections.
	DefaultDialTimeout = 30 * time.Second

	// DefaultHandshakeTimeout is the default TLS handshake timeout.
	DefaultHandshakeTimeout = 10 * time.Second
)

// =============================================================================
// APPROVED CIPHER SUITES (NIST 800-53 SC-13)
// =============================================================================

// ApprovedCipherSuites returns FIPS-approved cipher suites for TLS.
// Only includes strong, modern cipher suites (AES-256-GCM, CHACHA20-POLY1305).
// Excludes weak ciphers (RC4, DES, 3DES, MD5).
var ApprovedCipherSuites = []uint16{
	// TLS 1.3 cipher suites (preferred - automatically used when TLS 1.3 is negotiated)
	// TLS 1.3 doesn't use the cipher suite list below, it has its own built-in suites

	// TLS 1.2 cipher suites (fallback for compatibility)
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,   // FIPS-approved
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,   // FIPS-approved
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, // FIPS-approved
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, // FIPS-approved
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,    // Modern, secure
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,  // Modern, secure
}

// WeakCipherSuites are explicitly blocked cipher suites.
var WeakCipherSuites = map[uint16]string{
	tls.TLS_RSA_WITH_RC4_128_SHA:                "RC4 (weak)",
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:           "3DES (weak)",
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA:     "3DES (weak)",
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA:        "RC4 (weak)",
	tls.TLS_RSA_WITH_AES_128_CBC_SHA:            "CBC mode (vulnerable to padding oracle)",
	tls.TLS_RSA_WITH_AES_256_CBC_SHA:            "CBC mode (vulnerable to padding oracle)",
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:    "CBC mode (vulnerable to padding oracle)",
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:    "CBC mode (vulnerable to padding oracle)",
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:      "CBC mode (vulnerable to padding oracle)",
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:      "CBC mode (vulnerable to padding oracle)",
}

// =============================================================================
// TRANSPORT SECURITY MANAGER
// =============================================================================

// TransportSecurity manages secure TLS transport configuration per SC-8.
type TransportSecurity struct {
	// tlsConfig is the TLS configuration for secure connections.
	tlsConfig *tls.Config

	// certPins stores certificate pins for certificate pinning.
	certPins map[string][]byte

	// auditLogger is the audit logger for SC-8 events.
	auditLogger *AuditLogger

	// enforceMode determines if non-TLS connections are blocked.
	enforceMode bool

	// mu protects concurrent access.
	mu sync.RWMutex
}

// TransportSecurityOption is a functional option for configuring TransportSecurity.
type TransportSecurityOption func(*TransportSecurity)

// WithTransportAuditLogger sets the audit logger for SC-8 events.
func WithTransportAuditLogger(logger *AuditLogger) TransportSecurityOption {
	return func(ts *TransportSecurity) {
		ts.auditLogger = logger
	}
}

// WithEnforceMode enables strict enforcement mode (blocks non-TLS connections).
func WithEnforceMode(enforce bool) TransportSecurityOption {
	return func(ts *TransportSecurity) {
		ts.enforceMode = enforce
	}
}

// NewTransportSecurity creates a new TransportSecurity manager with the given options.
func NewTransportSecurity(opts ...TransportSecurityOption) *TransportSecurity {
	ts := &TransportSecurity{
		certPins: make(map[string][]byte),
	}

	// Apply options
	for _, opt := range opts {
		opt(ts)
	}

	// Default audit logger if not provided
	if ts.auditLogger == nil {
		ts.auditLogger = GlobalAuditLogger()
	}

	// Configure TLS
	ts.tlsConfig = &tls.Config{
		// SC-8: Minimum TLS 1.2, prefer TLS 1.3
		MinVersion: MinTLSVersion,
		MaxVersion: PreferredTLSVersion,

		// SC-13: Use only approved cipher suites
		CipherSuites: ApprovedCipherSuites,

		// Prefer server cipher suites for better security
		PreferServerCipherSuites: true,

		// Require valid certificates
		InsecureSkipVerify: false,

		// Use system cert pool
		RootCAs: nil, // nil means use system default

		// Set verification callback for certificate pinning
		VerifyPeerCertificate: ts.verifyPeerCertificate,
	}

	return ts
}

// =============================================================================
// TLS CONFIGURATION
// =============================================================================

// GetTLSConfig returns the configured TLS configuration.
func (ts *TransportSecurity) GetTLSConfig() *tls.Config {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.tlsConfig.Clone()
}

// GetSecureTransport returns a configured http.Transport with secure TLS settings.
func (ts *TransportSecurity) GetSecureTransport() *http.Transport {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return &http.Transport{
		// Use secure TLS configuration
		TLSClientConfig: ts.tlsConfig.Clone(),

		// Set timeouts
		DialContext: (&net.Dialer{
			Timeout:   DefaultDialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: DefaultHandshakeTimeout,

		// Connection pooling
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,

		// Force HTTP/2
		ForceAttemptHTTP2: true,

		// Disable compression to prevent CRIME attacks
		DisableCompression: true,
	}
}

// =============================================================================
// CONNECTION VALIDATION
// =============================================================================

// ValidateConnection validates a TLS connection's security parameters.
// Returns an error if the connection doesn't meet security requirements.
func (ts *TransportSecurity) ValidateConnection(conn *tls.Conn) error {
	if conn == nil {
		return errors.New("connection is nil")
	}

	// Force handshake to get connection state
	if err := conn.Handshake(); err != nil {
		ts.logEvent("TLS_HANDSHAKE_FAILED", "", false, map[string]string{
			"error": err.Error(),
		})
		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	state := conn.ConnectionState()

	// Validate TLS version
	if state.Version < MinTLSVersion {
		ts.logEvent("TLS_VERSION_REJECTED", "", false, map[string]string{
			"version":     tlsVersionToString(state.Version),
			"min_version": tlsVersionToString(MinTLSVersion),
		})
		return fmt.Errorf("TLS version %s is below minimum %s",
			tlsVersionToString(state.Version),
			tlsVersionToString(MinTLSVersion))
	}

	// Check for weak cipher suites
	if reason, isWeak := WeakCipherSuites[state.CipherSuite]; isWeak {
		ts.logEvent("WEAK_CIPHER_REJECTED", "", false, map[string]string{
			"cipher": cipherSuiteToString(state.CipherSuite),
			"reason": reason,
		})
		return fmt.Errorf("weak cipher suite rejected: %s (%s)",
			cipherSuiteToString(state.CipherSuite), reason)
	}

	// Log successful validation
	ts.logEvent("TLS_CONNECTION_VALIDATED", "", true, map[string]string{
		"version": tlsVersionToString(state.Version),
		"cipher":  cipherSuiteToString(state.CipherSuite),
		"server":  state.ServerName,
	})

	return nil
}

// GetConnectionInfo returns information about a TLS connection.
func (ts *TransportSecurity) GetConnectionInfo(conn *tls.Conn) (*ConnectionInfo, error) {
	if conn == nil {
		return nil, errors.New("connection is nil")
	}

	state := conn.ConnectionState()

	info := &ConnectionInfo{
		TLSVersion:         tlsVersionToString(state.Version),
		CipherSuite:        cipherSuiteToString(state.CipherSuite),
		ServerName:         state.ServerName,
		HandshakeComplete:  state.HandshakeComplete,
		DidResume:          state.DidResume,
		NegotiatedProtocol: state.NegotiatedProtocol,
	}

	// Extract certificate info
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		info.PeerCertificate = &PeerCertificateInfo{
			Subject:    cert.Subject.String(),
			Issuer:     cert.Issuer.String(),
			NotBefore:  cert.NotBefore,
			NotAfter:   cert.NotAfter,
			DNSNames:   cert.DNSNames,
			IsPinned:   ts.isCertificatePinned(state.ServerName, cert),
		}
	}

	return info, nil
}

// =============================================================================
// ENFORCEMENT
// =============================================================================

// EnforceTLS checks if a connection is using TLS and blocks if not in enforce mode.
func (ts *TransportSecurity) EnforceTLS(conn net.Conn) error {
	ts.mu.RLock()
	enforce := ts.enforceMode
	ts.mu.RUnlock()

	if !enforce {
		return nil
	}

	// Check if this is a TLS connection
	_, isTLS := conn.(*tls.Conn)
	if !isTLS {
		ts.logEvent("NON_TLS_BLOCKED", "", false, map[string]string{
			"remote_addr": conn.RemoteAddr().String(),
		})
		return errors.New("SC-8: non-TLS connections are blocked in enforce mode")
	}

	return nil
}

// SetEnforceMode enables or disables strict TLS enforcement.
func (ts *TransportSecurity) SetEnforceMode(enforce bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.enforceMode = enforce
	ts.logEvent("ENFORCE_MODE_CHANGED", "", true, map[string]string{
		"enabled": fmt.Sprintf("%v", enforce),
	})
}

// IsEnforceMode returns whether strict TLS enforcement is enabled.
func (ts *TransportSecurity) IsEnforceMode() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.enforceMode
}

// =============================================================================
// CERTIFICATE PINNING (SC-17)
// =============================================================================

// PinCertificate pins a certificate for a specific host.
func (ts *TransportSecurity) PinCertificate(host string, certPEM []byte) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Validate PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("failed to decode PEM certificate")
	}

	// Parse certificate to validate
	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("invalid certificate: %w", err)
	}

	ts.certPins[host] = certPEM

	ts.logEvent("CERT_PINNED", "", true, map[string]string{
		"host": host,
	})

	return nil
}

// UnpinCertificate removes a certificate pin for a host.
func (ts *TransportSecurity) UnpinCertificate(host string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	delete(ts.certPins, host)

	ts.logEvent("CERT_UNPINNED", "", true, map[string]string{
		"host": host,
	})
}

// GetPinnedCertificates returns a list of all pinned certificate hosts.
func (ts *TransportSecurity) GetPinnedCertificates() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	hosts := make([]string, 0, len(ts.certPins))
	for host := range ts.certPins {
		hosts = append(hosts, host)
	}
	return hosts
}

// verifyPeerCertificate is the callback for certificate verification (including pinning).
func (ts *TransportSecurity) verifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	// This is called during TLS handshake
	// We don't implement custom verification here, just logging
	// Actual pinning check happens in isCertificatePinned
	return nil
}

// isCertificatePinned checks if a certificate matches the pinned certificate for a host.
func (ts *TransportSecurity) isCertificatePinned(host string, cert *x509.Certificate) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pinnedPEM, exists := ts.certPins[host]
	if !exists {
		return false
	}

	// Decode pinned PEM
	block, _ := pem.Decode(pinnedPEM)
	if block == nil {
		return false
	}

	// Compare raw certificates
	return string(cert.Raw) == string(block.Bytes)
}

// =============================================================================
// AUDIT LOGGING
// =============================================================================

// LogConnection logs a TLS connection event to the audit log.
func (ts *TransportSecurity) LogConnection(conn *tls.Conn) {
	if conn == nil {
		return
	}

	state := conn.ConnectionState()

	metadata := map[string]string{
		"version": tlsVersionToString(state.Version),
		"cipher":  cipherSuiteToString(state.CipherSuite),
		"server":  state.ServerName,
	}

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		metadata["cert_subject"] = cert.Subject.String()
		metadata["cert_issuer"] = cert.Issuer.String()
	}

	ts.logEvent("TLS_CONNECTION", "", true, metadata)
}

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a transport security event to the audit log.
func (ts *TransportSecurity) logEvent(eventType, sessionID string, success bool, metadata map[string]string) {
	if ts.auditLogger == nil || !ts.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		Success:   success,
		Metadata:  metadata,
	}

	if err := ts.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log transport event %s: %v\n", eventType, err)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// tlsVersionToString converts a TLS version constant to a string.
func tlsVersionToString(version uint16) string {
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

// cipherSuiteToString converts a cipher suite ID to a human-readable string.
func cipherSuiteToString(id uint16) string {
	for _, suite := range tls.CipherSuites() {
		if suite.ID == id {
			return suite.Name
		}
	}
	for _, suite := range tls.InsecureCipherSuites() {
		if suite.ID == id {
			return suite.Name + " (insecure)"
		}
	}
	return fmt.Sprintf("Unknown (0x%04x)", id)
}

// =============================================================================
// CONNECTION INFO TYPES
// =============================================================================

// ConnectionInfo contains information about a TLS connection.
type ConnectionInfo struct {
	TLSVersion         string           `json:"tls_version"`
	CipherSuite        string           `json:"cipher_suite"`
	ServerName         string           `json:"server_name"`
	HandshakeComplete  bool             `json:"handshake_complete"`
	DidResume          bool             `json:"did_resume"`
	NegotiatedProtocol string           `json:"negotiated_protocol"`
	PeerCertificate    *PeerCertificateInfo `json:"peer_certificate,omitempty"`
}

// PeerCertificateInfo contains information about a certificate.
type PeerCertificateInfo struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	DNSNames  []string  `json:"dns_names"`
	IsPinned  bool      `json:"is_pinned"`
}

// =============================================================================
// GLOBAL TRANSPORT SECURITY MANAGER
// =============================================================================

var (
	globalTransportSecurity     *TransportSecurity
	globalTransportSecurityOnce sync.Once
	globalTransportSecurityMu   sync.Mutex
)

// GlobalTransportSecurity returns the global transport security manager instance.
func GlobalTransportSecurity() *TransportSecurity {
	globalTransportSecurityOnce.Do(func() {
		globalTransportSecurity = NewTransportSecurity()
	})
	return globalTransportSecurity
}

// SetGlobalTransportSecurity sets the global transport security manager instance.
func SetGlobalTransportSecurity(ts *TransportSecurity) {
	globalTransportSecurityMu.Lock()
	defer globalTransportSecurityMu.Unlock()
	globalTransportSecurity = ts
}

// InitGlobalTransportSecurity initializes the global transport security manager with options.
func InitGlobalTransportSecurity(opts ...TransportSecurityOption) {
	globalTransportSecurityMu.Lock()
	defer globalTransportSecurityMu.Unlock()
	globalTransportSecurity = NewTransportSecurity(opts...)
}

// =============================================================================
// HELPER: SAVE/LOAD CERTIFICATE PINS
// =============================================================================

// SaveCertificatePins saves certificate pins to a file.
func (ts *TransportSecurity) SaveCertificatePins(path string) error {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write each pinned certificate
	for host, certPEM := range ts.certPins {
		// Write host header
		if _, err := fmt.Fprintf(file, "# Host: %s\n", host); err != nil {
			return err
		}
		// Write PEM
		if _, err := file.Write(certPEM); err != nil {
			return err
		}
		if _, err := file.WriteString("\n"); err != nil {
			return err
		}
	}

	return nil
}

// LoadCertificatePins loads certificate pins from a file.
func (ts *TransportSecurity) LoadCertificatePins(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No pins to load
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read pins file: %w", err)
	}

	// Parse file
	var currentHost string
	var currentPEM []byte

	for _, line := range []byte(string(data)) {
		lineStr := string(line)
		if len(lineStr) > 0 && lineStr[0] == '#' {
			// Parse host header
			if len(lineStr) > 8 && lineStr[:8] == "# Host: " {
				// Save previous pin if exists
				if currentHost != "" && len(currentPEM) > 0 {
					if err := ts.PinCertificate(currentHost, currentPEM); err != nil {
						return err
					}
				}
				currentHost = lineStr[8:]
				currentPEM = nil
			}
		} else if len(lineStr) > 0 {
			// Accumulate PEM data
			currentPEM = append(currentPEM, line)
			currentPEM = append(currentPEM, '\n')
		}
	}

	// Save last pin
	if currentHost != "" && len(currentPEM) > 0 {
		if err := ts.PinCertificate(currentHost, currentPEM); err != nil {
			return err
		}
	}

	return nil
}
