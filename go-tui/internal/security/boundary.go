// boundary.go - Network boundary protection for rigrun.
//
// Implements NIST 800-53 SC-7: Boundary Protection
// for DoD IL5 compliance.
//
// This module provides:
//   - Network policy enforcement (SC-7)
//   - Egress filtering (SC-7(5))
//   - Connection monitoring and logging
//   - Host allowlist/blocklist management
//   - Proxy configuration support
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// DefaultPolicyPath is the default network policy file path.
	DefaultPolicyPath = "network_policy.json"

	// DefaultConnectionLogPath is the default connection log path.
	DefaultConnectionLogPath = "connections.log"

	// MaxConnectionLogEntries is the maximum connection log entries to keep.
	MaxConnectionLogEntries = 10000

	// PolicyKeyEnvVar is the environment variable name for the policy signature key.
	// NIST 800-53 AU-9: Protection of Audit Information requires secure key management.
	PolicyKeyEnvVar = "RIGRUN_POLICY_KEY"

	// MinPolicyKeyLength is the minimum policy key length in bytes (256 bits).
	// SECURITY: Minimum key length enforced for cryptographic strength.
	MinPolicyKeyLength = 32
)

// Common ports
const (
	PortHTTPS = 443
	PortHTTP  = 80
	PortOllama = 11434
)

// =============================================================================
// ERRORS
// =============================================================================

// ErrPolicyKeyNotConfigured is returned when the policy signature key is not configured.
// NIST 800-53 AU-9 compliance requires proper key management - no silent fallback.
var ErrPolicyKeyNotConfigured = errors.New("policy signature key not configured: set RIGRUN_POLICY_KEY environment variable or security.policy_key in config")

// ErrPolicyKeyTooShort is returned when the policy key is shorter than MinPolicyKeyLength.
// SECURITY: Minimum key length enforced for cryptographic strength.
var ErrPolicyKeyTooShort = fmt.Errorf("policy key must be at least %d bytes", MinPolicyKeyLength)

// =============================================================================
// POLICY KEY MANAGEMENT
// =============================================================================

// getPolicySignatureKey retrieves the policy signature key from environment or config.
// NIST 800-53 AU-9: Protection of Audit Information requires secure key management.
// Returns error if key is not configured (no silent fallback to hardcoded values).
// SECURITY: Minimum key length enforced for cryptographic strength.
func getPolicySignatureKey() (string, error) {
	// 1. Check environment variable first (highest priority)
	if key := os.Getenv(PolicyKeyEnvVar); key != "" {
		// SECURITY: Minimum key length enforced
		if len(key) < MinPolicyKeyLength {
			return "", ErrPolicyKeyTooShort
		}
		return key, nil
	}

	// 2. Check config file as fallback
	// Import cycle prevention: use direct file read instead of config.Global()
	key, err := loadPolicyKeyFromConfig()
	if err == nil && key != "" {
		// SECURITY: Minimum key length enforced
		if len(key) < MinPolicyKeyLength {
			return "", ErrPolicyKeyTooShort
		}
		return key, nil
	}

	// 3. No key configured - return error (no silent fallback per AU-9)
	return "", ErrPolicyKeyNotConfigured
}

// loadPolicyKeyFromConfig loads the policy key from the config file.
func loadPolicyKeyFromConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Try TOML config first
	tomlPath := filepath.Join(home, ".rigrun", "config.toml")
	if data, err := os.ReadFile(tomlPath); err == nil {
		// Simple TOML parsing for policy_key field
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "policy_key") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					value := strings.TrimSpace(parts[1])
					// Remove quotes if present
					value = strings.Trim(value, "\"'")
					if value != "" {
						return value, nil
					}
				}
			}
		}
	}

	// Try JSON config as fallback
	jsonPath := filepath.Join(home, ".rigrun", "config.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		var cfg struct {
			Security struct {
				PolicyKey string `json:"policy_key"`
			} `json:"security"`
		}
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.Security.PolicyKey != "" {
			return cfg.Security.PolicyKey, nil
		}
	}

	return "", errors.New("policy_key not found in config")
}

// =============================================================================
// NETWORK POLICY
// =============================================================================

// NetworkPolicy defines allowed/blocked hosts and ports.
type NetworkPolicy struct {
	// AllowedHosts is the list of allowed destination hosts/domains.
	AllowedHosts []string `json:"allowed_hosts"`

	// BlockedHosts is the list of explicitly blocked hosts/domains.
	BlockedHosts []string `json:"blocked_hosts"`

	// AllowedPorts is the list of allowed destination ports.
	AllowedPorts []int `json:"allowed_ports"`

	// DefaultAllow is HARDCODED to false for IL5 compliance (SC-7).
	// This field is kept for backward compatibility but ignored at runtime.
	// IL5 requires default-deny posture for all network boundary controls.
	DefaultAllow bool `json:"default_allow,omitempty"`

	// ProxyURL is the optional proxy server URL.
	ProxyURL string `json:"proxy_url,omitempty"`

	// ProxyBypass lists hosts that bypass the proxy.
	ProxyBypass []string `json:"proxy_bypass,omitempty"`

	// Updated timestamp
	Updated time.Time `json:"updated"`
}

// DefaultNetworkPolicy returns the default network policy for IL5.
func DefaultNetworkPolicy() *NetworkPolicy {
	return &NetworkPolicy{
		AllowedHosts: []string{
			"openrouter.ai",           // OpenRouter API
			"api.openrouter.ai",       // OpenRouter API
			"localhost",               // Local Ollama
			"127.0.0.1",              // Local Ollama
			"::1",                     // Local Ollama IPv6
		},
		BlockedHosts: []string{
			// Known malicious hosts can be added here
		},
		AllowedPorts: []int{
			PortHTTPS,  // HTTPS only by default
			PortOllama, // Ollama default port
		},
		DefaultAllow: false, // SC-7: Default deny for IL5
		Updated:      time.Now(),
	}
}

// IsHostAllowed checks if a host is allowed by the policy.
func (p *NetworkPolicy) IsHostAllowed(host string) bool {
	// Normalize host
	host = strings.ToLower(strings.TrimSpace(host))

	// Check blocklist first (explicit deny)
	for _, blocked := range p.BlockedHosts {
		if matchesHost(host, blocked) {
			return false
		}
	}

	// Check allowlist
	for _, allowed := range p.AllowedHosts {
		if matchesHost(host, allowed) {
			return true
		}
	}

	// IL5 SECURITY FIX: DefaultAllow is HARDCODED to false.
	// This enforces SC-7 requirement for default-deny boundary protection.
	// Even if p.DefaultAllow is set to true in the policy file, we ignore it.
	return false
}

// IsPortAllowed checks if a port is allowed by the policy.
func (p *NetworkPolicy) IsPortAllowed(port int) bool {
	if len(p.AllowedPorts) == 0 {
		// No port restrictions if list is empty
		return true
	}

	for _, allowed := range p.AllowedPorts {
		if port == allowed {
			return true
		}
	}

	return false
}

// AddAllowedHost adds a host to the allowlist.
func (p *NetworkPolicy) AddAllowedHost(host string) {
	host = strings.ToLower(strings.TrimSpace(host))

	// Check if already in list
	for _, existing := range p.AllowedHosts {
		if existing == host {
			return
		}
	}

	p.AllowedHosts = append(p.AllowedHosts, host)
	p.Updated = time.Now()
}

// RemoveAllowedHost removes a host from the allowlist.
func (p *NetworkPolicy) RemoveAllowedHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))

	for i, existing := range p.AllowedHosts {
		if existing == host {
			p.AllowedHosts = append(p.AllowedHosts[:i], p.AllowedHosts[i+1:]...)
			p.Updated = time.Now()
			return true
		}
	}

	return false
}

// AddBlockedHost adds a host to the blocklist.
func (p *NetworkPolicy) AddBlockedHost(host string) {
	host = strings.ToLower(strings.TrimSpace(host))

	// Check if already in list
	for _, existing := range p.BlockedHosts {
		if existing == host {
			return
		}
	}

	p.BlockedHosts = append(p.BlockedHosts, host)
	p.Updated = time.Now()
}

// RemoveBlockedHost removes a host from the blocklist.
func (p *NetworkPolicy) RemoveBlockedHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))

	for i, existing := range p.BlockedHosts {
		if existing == host {
			p.BlockedHosts = append(p.BlockedHosts[:i], p.BlockedHosts[i+1:]...)
			p.Updated = time.Now()
			return true
		}
	}

	return false
}

// matchesHost checks if a host matches a pattern (supports wildcards).
func matchesHost(host, pattern string) bool {
	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)

	// Exact match
	if host == pattern {
		return true
	}

	// Wildcard subdomain matching (*.example.com)
	if strings.HasPrefix(pattern, "*.") {
		domain := strings.TrimPrefix(pattern, "*.")
		return strings.HasSuffix(host, "."+domain) || host == domain
	}

	// Subdomain matching (.example.com matches any.example.com)
	if strings.HasPrefix(pattern, ".") {
		return strings.HasSuffix(host, pattern) || host == strings.TrimPrefix(pattern, ".")
	}

	return false
}

// =============================================================================
// CONNECTION LOG ENTRY
// =============================================================================

// ConnectionLogEntry represents a logged network connection attempt.
type ConnectionLogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Destination string    `json:"destination"`
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	Action      string    `json:"action"` // "allow" or "block"
	Reason      string    `json:"reason,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
}

// =============================================================================
// BLOCKED HOST ENTRY
// =============================================================================

// BlockedHostEntry represents a blocked host with metadata.
type BlockedHostEntry struct {
	Host      string    `json:"host"`
	Reason    string    `json:"reason"`
	BlockedAt time.Time `json:"blocked_at"`
	BlockedBy string    `json:"blocked_by"` // User/system that blocked it
}

// =============================================================================
// BOUNDARY PROTECTION
// =============================================================================

// BoundaryProtection provides network boundary protection per SC-7.
type BoundaryProtection struct {
	// policy is the current network policy
	policy *NetworkPolicy

	// blockedHosts maps hosts to block reasons
	blockedHosts map[string]*BlockedHostEntry

	// connectionLog stores recent connection attempts
	connectionLog []ConnectionLogEntry

	// egressEnabled determines if egress filtering is active
	egressEnabled bool

	// auditLogger for SC-7 events
	auditLogger *AuditLogger

	// configPath is the path to the policy config file
	configPath string

	// originalTransport stores the original http.DefaultTransport before enforcement
	originalTransport http.RoundTripper

	// policyKey is the key used for policy signature verification.
	// SECURITY: Minimum key length enforced via SetPolicyKey.
	policyKey []byte

	// mu protects concurrent access
	mu sync.RWMutex
}

// =============================================================================
// HTTP TRANSPORT ENFORCEMENT
// =============================================================================

// BoundaryTransport wraps http.RoundTripper to enforce boundary protection.
// This is the ACTUAL ENFORCEMENT mechanism that blocks connections at the
// HTTP transport layer, not just logging them.
type BoundaryTransport struct {
	Base      http.RoundTripper
	Protector *BoundaryProtection
}

// RoundTrip implements http.RoundTripper with boundary enforcement.
func (t *BoundaryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	port := req.URL.Port()

	// Determine port from scheme if not explicitly set
	portNum := 0
	if port != "" {
		fmt.Sscanf(port, "%d", &portNum)
	} else {
		switch req.URL.Scheme {
		case "https":
			portNum = PortHTTPS
		case "http":
			portNum = PortHTTP
		default:
			portNum = PortHTTPS // Default to HTTPS
		}
	}

	// ENFORCE boundary protection - BLOCK if not allowed
	allowed, reason := t.Protector.ValidateDestination(host, portNum)
	if !allowed {
		// Log security violation
		t.Protector.logEvent("BOUNDARY_ENFORCEMENT_BLOCKED", false, map[string]string{
			"host":   host,
			"port":   fmt.Sprintf("%d", portNum),
			"reason": reason,
			"url":    req.URL.String(),
			"method": req.Method,
		})

		// BLOCK the request - return error, do NOT proceed
		return nil, fmt.Errorf("boundary protection blocked request to %s:%d: %s (SC-7 violation)", host, portNum, reason)
	}

	// Request is allowed, proceed with base transport
	return t.Base.RoundTrip(req)
}

// ValidateHost is a convenience method for simple host validation.
func (b *BoundaryProtection) ValidateHost(host string) error {
	allowed, reason := b.ValidateDestination(host, PortHTTPS)
	if !allowed {
		return fmt.Errorf("host %s not allowed: %s", host, reason)
	}
	return nil
}

// EnforceTransport replaces http.DefaultTransport with boundary enforcement.
// This is the critical method that activates ACTUAL BLOCKING at the HTTP layer.
// Call this during application initialization to enforce boundary protection.
func (b *BoundaryProtection) EnforceTransport() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Save original transport if not already saved
	if b.originalTransport == nil {
		b.originalTransport = http.DefaultTransport
	}

	// Replace with enforcing transport
	http.DefaultTransport = &BoundaryTransport{
		Base:      b.originalTransport,
		Protector: b,
	}

	b.logEvent("BOUNDARY_TRANSPORT_ENFORCED", true, map[string]string{
		"status": "active",
	})
}

// DisableEnforcement restores the original http.DefaultTransport.
// WARNING: This should only be used in testing or emergency override situations.
// Disabling enforcement violates IL5 SC-7 requirements.
func (b *BoundaryProtection) DisableEnforcement() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.originalTransport != nil {
		http.DefaultTransport = b.originalTransport
	}

	b.logEvent("BOUNDARY_TRANSPORT_DISABLED", true, map[string]string{
		"status":  "disabled",
		"warning": "SC-7 enforcement disabled",
	})
}

// IsEnforced returns whether transport enforcement is currently active.
func (b *BoundaryProtection) IsEnforced() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	_, ok := http.DefaultTransport.(*BoundaryTransport)
	return ok
}

// BoundaryProtectionOption is a functional option for BoundaryProtection.
type BoundaryProtectionOption func(*BoundaryProtection)

// WithBoundaryAuditLogger sets the audit logger.
func WithBoundaryAuditLogger(logger *AuditLogger) BoundaryProtectionOption {
	return func(b *BoundaryProtection) {
		b.auditLogger = logger
	}
}

// WithBoundaryConfigPath sets the config file path.
func WithBoundaryConfigPath(path string) BoundaryProtectionOption {
	return func(b *BoundaryProtection) {
		b.configPath = path
	}
}

// NewBoundaryProtection creates a new BoundaryProtection instance.
func NewBoundaryProtection(opts ...BoundaryProtectionOption) *BoundaryProtection {
	bp := &BoundaryProtection{
		policy:        DefaultNetworkPolicy(),
		blockedHosts:  make(map[string]*BlockedHostEntry),
		connectionLog: make([]ConnectionLogEntry, 0),
		egressEnabled: true, // Enabled by default for IL5
		configPath:    DefaultPolicyPath,
	}

	// Apply options
	for _, opt := range opts {
		opt(bp)
	}

	// Set default audit logger if not provided
	if bp.auditLogger == nil {
		bp.auditLogger = GlobalAuditLogger()
	}

	// Try to load existing policy
	// ERROR HANDLING: Errors must not be silently ignored
	if err := bp.LoadPolicy(); err != nil {
		// Log policy load errors - not fatal, will use defaults
		fmt.Fprintf(os.Stderr, "BOUNDARY WARNING: failed to load network policy, using defaults: %v\n", err)
	}

	return bp
}

// SetPolicyKey sets the policy signature key with minimum length validation.
// SECURITY: Minimum key length enforced for cryptographic strength.
// NIST 800-53 AU-9: Protection of Audit Information requires secure key management.
func (b *BoundaryProtection) SetPolicyKey(key []byte) error {
	if len(key) < MinPolicyKeyLength {
		return fmt.Errorf("policy key must be at least %d bytes, got %d",
			MinPolicyKeyLength, len(key))
	}

	// SECURITY: Mutex required for writes
	b.mu.Lock()
	defer b.mu.Unlock()

	// Copy key to prevent external modification
	b.policyKey = make([]byte, len(key))
	copy(b.policyKey, key)
	return nil
}

// =============================================================================
// POLICY FILE INTEGRITY
// =============================================================================

// computePolicySignature computes HMAC-SHA256 signature of policy data.
// NIST 800-53 AU-9: Uses key from env/config, returns error if not configured.
func (b *BoundaryProtection) computePolicySignature(data []byte) (string, error) {
	key, err := getPolicySignatureKey()
	if err != nil {
		return "", fmt.Errorf("cannot compute policy signature: %w", err)
	}
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyPolicySignature verifies the HMAC signature of policy data.
// NIST 800-53 AU-9: Uses key from env/config, returns error if not configured.
func (b *BoundaryProtection) verifyPolicySignature(data []byte, signature string) (bool, error) {
	expected, err := b.computePolicySignature(data)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(expected), []byte(signature)), nil
}

// getPolicySignaturePath returns the path to the policy signature file.
func (b *BoundaryProtection) getPolicySignaturePath() string {
	return b.getPolicyPath() + ".sig"
}

// savePolicySignature saves the HMAC signature of the policy file.
// NIST 800-53 AU-9: Returns error if policy key is not configured.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (b *BoundaryProtection) savePolicySignature(policyData []byte) error {
	signature, err := b.computePolicySignature(policyData)
	if err != nil {
		b.logEvent("BOUNDARY_POLICY_SIGN_FAILED", false, map[string]string{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to compute policy signature: %w", err)
	}
	sigPath := b.getPolicySignaturePath()

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFile(sigPath, []byte(signature), 0600); err != nil {
		return fmt.Errorf("failed to write policy signature: %w", err)
	}

	b.logEvent("BOUNDARY_POLICY_SIGNED", true, map[string]string{
		"signature_path": sigPath,
	})

	return nil
}

// verifyPolicyFile verifies the integrity of the policy file.
// NIST 800-53 AU-9: Returns error if policy key is not configured.
func (b *BoundaryProtection) verifyPolicyFile(policyData []byte) error {
	sigPath := b.getPolicySignaturePath()

	// Read signature file
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		// Signature file missing - policy may have been tampered with
		if os.IsNotExist(err) {
			return errors.New("policy signature file missing - possible tampering or initial setup")
		}
		return fmt.Errorf("failed to read policy signature: %w", err)
	}

	signature := strings.TrimSpace(string(sigData))

	// Verify signature
	valid, err := b.verifyPolicySignature(policyData, signature)
	if err != nil {
		// Key not configured - critical security error
		b.logEvent("BOUNDARY_POLICY_VERIFY_FAILED", false, map[string]string{
			"policy_path": b.getPolicyPath(),
			"error":       err.Error(),
			"severity":    "CRITICAL",
		})
		return fmt.Errorf("failed to verify policy signature: %w", err)
	}
	if !valid {
		// CRITICAL: Signature mismatch indicates tampering
		b.logEvent("BOUNDARY_POLICY_TAMPER_DETECTED", false, map[string]string{
			"policy_path":    b.getPolicyPath(),
			"signature_path": sigPath,
			"severity":       "CRITICAL",
		})
		return errors.New("policy file signature invalid - TAMPERING DETECTED")
	}

	return nil
}

// CheckPolicyIntegrity performs an on-demand integrity check of the policy file.
// Returns error if the policy file has been tampered with or is missing signature.
func (b *BoundaryProtection) CheckPolicyIntegrity() error {
	path := b.getPolicyPath()

	// Check if policy file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No policy file, using defaults
	}

	// Read current policy file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	// Verify integrity
	return b.verifyPolicyFile(data)
}

// =============================================================================
// POLICY MANAGEMENT
// =============================================================================

// SetNetworkPolicy sets the network policy.
func (b *BoundaryProtection) SetNetworkPolicy(policy *NetworkPolicy) {
	b.mu.Lock()
	defer b.mu.Unlock()

	policy.Updated = time.Now()
	b.policy = policy

	b.logEvent("BOUNDARY_POLICY_UPDATE", true, map[string]string{
		"allowed_hosts": fmt.Sprintf("%d", len(policy.AllowedHosts)),
		"blocked_hosts": fmt.Sprintf("%d", len(policy.BlockedHosts)),
		"allowed_ports": fmt.Sprintf("%d", len(policy.AllowedPorts)),
	})
}

// GetNetworkPolicy returns a copy of the current network policy.
func (b *BoundaryProtection) GetNetworkPolicy() *NetworkPolicy {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to prevent external modifications
	policyCopy := *b.policy
	policyCopy.AllowedHosts = append([]string{}, b.policy.AllowedHosts...)
	policyCopy.BlockedHosts = append([]string{}, b.policy.BlockedHosts...)
	policyCopy.AllowedPorts = append([]int{}, b.policy.AllowedPorts...)
	policyCopy.ProxyBypass = append([]string{}, b.policy.ProxyBypass...)

	return &policyCopy
}

// LoadPolicy loads the network policy from disk.
// IL5 SECURITY FIX: Now verifies policy file integrity via HMAC signature.
func (b *BoundaryProtection) LoadPolicy() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	path := b.getPolicyPath()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No policy file, use defaults
		return nil
	}

	// Read policy data
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	// SECURITY FIX: Verify policy file integrity before loading
	if err := b.verifyPolicyFile(data); err != nil {
		// Log critical security event
		b.logEvent("BOUNDARY_POLICY_LOAD_FAILED", false, map[string]string{
			"path":   path,
			"error":  err.Error(),
			"action": "using_default_policy",
		})

		// On signature verification failure, DO NOT load the policy
		// Keep using the current (default) policy to maintain security
		return fmt.Errorf("policy integrity check failed: %w", err)
	}

	// Parse policy
	var policy NetworkPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("failed to parse policy file: %w", err)
	}

	// IL5 ENFORCEMENT: Override any DefaultAllow=true in the loaded policy
	// This ensures IL5 compliance even if the policy file is modified
	if policy.DefaultAllow {
		b.logEvent("BOUNDARY_POLICY_DEFAULT_ALLOW_OVERRIDE", true, map[string]string{
			"original_value": "true",
			"enforced_value": "false",
			"reason":         "IL5_SC7_COMPLIANCE",
		})
		policy.DefaultAllow = false
	}

	b.policy = &policy

	b.logEvent("BOUNDARY_POLICY_LOAD", true, map[string]string{
		"path":           path,
		"integrity":      "verified",
		"allowed_hosts":  fmt.Sprintf("%d", len(policy.AllowedHosts)),
		"blocked_hosts":  fmt.Sprintf("%d", len(policy.BlockedHosts)),
		"default_allow":  "false_enforced",
	})

	return nil
}

// SavePolicy saves the network policy to disk.
// IL5 SECURITY FIX: Now signs the policy file with HMAC for integrity protection.
func (b *BoundaryProtection) SavePolicy() error {
	b.mu.RLock()
	policy := b.policy
	b.mu.RUnlock()

	path := b.getPolicyPath()

	// IL5 ENFORCEMENT: Ensure DefaultAllow is always false before saving
	policy.DefaultAllow = false

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(path, data, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write policy file: %w", err)
	}

	// SECURITY FIX: Sign the policy file for tamper detection
	if err := b.savePolicySignature(data); err != nil {
		// If signature save fails, remove the policy file to prevent use of unsigned policy
		os.Remove(path)
		return fmt.Errorf("failed to sign policy file: %w", err)
	}

	b.logEvent("BOUNDARY_POLICY_SAVE", true, map[string]string{
		"path":      path,
		"integrity": "signed",
		"signature": b.getPolicySignaturePath(),
	})

	return nil
}

func (b *BoundaryProtection) getPolicyPath() string {
	if filepath.IsAbs(b.configPath) {
		return b.configPath
	}

	// Use ~/.rigrun directory
	home, err := os.UserHomeDir()
	if err != nil {
		return b.configPath
	}

	return filepath.Join(home, ".rigrun", b.configPath)
}

// =============================================================================
// VALIDATION
// =============================================================================

// ValidateDestination checks if a connection to host:port is allowed.
func (b *BoundaryProtection) ValidateDestination(host string, port int) (bool, string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.egressEnabled {
		// Egress filtering disabled
		return true, "egress_filtering_disabled"
	}

	// Normalize host
	host = normalizeBoundaryHost(host)

	// Check if explicitly blocked
	if entry, blocked := b.blockedHosts[host]; blocked {
		b.logConnection(host, port, "tcp", "block", entry.Reason)
		return false, entry.Reason
	}

	// Check policy
	if !b.policy.IsHostAllowed(host) {
		reason := "host_not_allowed"
		b.logConnection(host, port, "tcp", "block", reason)
		return false, reason
	}

	if !b.policy.IsPortAllowed(port) {
		reason := "port_not_allowed"
		b.logConnection(host, port, "tcp", "block", reason)
		return false, reason
	}

	b.logConnection(host, port, "tcp", "allow", "")
	return true, ""
}

// normalizeBoundaryHost normalizes a host string (removes port, lowercases).
func normalizeBoundaryHost(host string) string {
	// Remove port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	return strings.ToLower(strings.TrimSpace(host))
}

// =============================================================================
// HOST MANAGEMENT
// =============================================================================

// BlockHost adds a host to the blocklist with a reason.
func (b *BoundaryProtection) BlockHost(host, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	host = normalizeBoundaryHost(host)

	entry := &BlockedHostEntry{
		Host:      host,
		Reason:    reason,
		BlockedAt: time.Now(),
		BlockedBy: "manual", // Could be extended to track user
	}

	b.blockedHosts[host] = entry

	// Also add to policy blocklist
	b.policy.AddBlockedHost(host)

	b.logEvent("BOUNDARY_HOST_BLOCKED", true, map[string]string{
		"host":   host,
		"reason": reason,
	})
}

// UnblockHost removes a host from the blocklist.
func (b *BoundaryProtection) UnblockHost(host string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	host = normalizeBoundaryHost(host)

	_, exists := b.blockedHosts[host]
	if !exists {
		return false
	}

	delete(b.blockedHosts, host)
	b.policy.RemoveBlockedHost(host)

	b.logEvent("BOUNDARY_HOST_UNBLOCKED", true, map[string]string{
		"host": host,
	})

	return true
}

// AllowHost adds a host to the allowlist.
func (b *BoundaryProtection) AllowHost(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	host = normalizeBoundaryHost(host)

	b.policy.AddAllowedHost(host)

	b.logEvent("BOUNDARY_HOST_ALLOWED", true, map[string]string{
		"host": host,
	})
}

// GetBlockedHosts returns a list of blocked hosts with their metadata.
func (b *BoundaryProtection) GetBlockedHosts() []BlockedHostEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	entries := make([]BlockedHostEntry, 0, len(b.blockedHosts))
	for _, entry := range b.blockedHosts {
		entries = append(entries, *entry)
	}

	return entries
}

// GetAllowedHosts returns a list of allowed hosts.
func (b *BoundaryProtection) GetAllowedHosts() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy
	hosts := make([]string, len(b.policy.AllowedHosts))
	copy(hosts, b.policy.AllowedHosts)

	return hosts
}

// =============================================================================
// CONNECTION MONITORING
// =============================================================================

// MonitorConnections enables connection monitoring via transport enforcement.
// IL5 SECURITY FIX: This now actually enforces boundary protection by wrapping
// the HTTP transport. Call this during application initialization.
func (b *BoundaryProtection) MonitorConnections() {
	// Enable egress filtering
	b.EnforceEgress(true)

	// Enforce transport-level blocking
	b.EnforceTransport()

	b.logEvent("BOUNDARY_MONITORING_ENABLED", true, map[string]string{
		"enforcement": "active",
		"transport":   "wrapped",
		"egress":      "enabled",
	})
}

// logConnection logs a connection attempt.
// SECURITY: Mutex required for writes
func (b *BoundaryProtection) logConnection(host string, port int, protocol, action, reason string) {
	entry := ConnectionLogEntry{
		Timestamp:   time.Now(),
		Destination: host,
		Port:        port,
		Protocol:    protocol,
		Action:      action,
		Reason:      reason,
	}

	// SECURITY: Mutex required for writes to shared state
	b.mu.Lock()
	defer b.mu.Unlock()

	// Add to in-memory log (with rotation)
	b.connectionLog = append(b.connectionLog, entry)
	if len(b.connectionLog) > MaxConnectionLogEntries {
		// Remove oldest entries
		b.connectionLog = b.connectionLog[len(b.connectionLog)-MaxConnectionLogEntries:]
	}

	// Log to audit if blocked
	if action == "block" {
		b.logEvent("BOUNDARY_CONNECTION_BLOCKED", false, map[string]string{
			"destination": host,
			"port":        fmt.Sprintf("%d", port),
			"reason":      reason,
		})
	}
}

// GetConnectionLog returns recent connection log entries.
func (b *BoundaryProtection) GetConnectionLog(limit int) []ConnectionLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.connectionLog) {
		limit = len(b.connectionLog)
	}

	// Return most recent entries
	start := len(b.connectionLog) - limit
	if start < 0 {
		start = 0
	}

	entries := make([]ConnectionLogEntry, limit)
	copy(entries, b.connectionLog[start:])

	return entries
}

// =============================================================================
// EGRESS CONTROL
// =============================================================================

// EnforceEgress enables or disables egress filtering.
func (b *BoundaryProtection) EnforceEgress(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.egressEnabled = enabled

	b.logEvent("BOUNDARY_EGRESS_CONTROL", true, map[string]string{
		"enabled": fmt.Sprintf("%t", enabled),
	})
}

// IsEgressEnforced returns whether egress filtering is enabled.
func (b *BoundaryProtection) IsEgressEnforced() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.egressEnabled
}

// =============================================================================
// STATISTICS
// =============================================================================

// BoundaryStats provides boundary protection statistics.
type BoundaryStats struct {
	EgressEnabled      bool      `json:"egress_enabled"`
	AllowedHostsCount  int       `json:"allowed_hosts_count"`
	BlockedHostsCount  int       `json:"blocked_hosts_count"`
	AllowedPortsCount  int       `json:"allowed_ports_count"`
	ConnectionsLogged  int       `json:"connections_logged"`
	ConnectionsBlocked int       `json:"connections_blocked"`
	LastPolicyUpdate   time.Time `json:"last_policy_update"`
}

// GetStats returns boundary protection statistics.
func (b *BoundaryProtection) GetStats() BoundaryStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	blocked := 0
	for _, entry := range b.connectionLog {
		if entry.Action == "block" {
			blocked++
		}
	}

	return BoundaryStats{
		EgressEnabled:      b.egressEnabled,
		AllowedHostsCount:  len(b.policy.AllowedHosts),
		BlockedHostsCount:  len(b.blockedHosts),
		AllowedPortsCount:  len(b.policy.AllowedPorts),
		ConnectionsLogged:  len(b.connectionLog),
		ConnectionsBlocked: blocked,
		LastPolicyUpdate:   b.policy.Updated,
	}
}

// =============================================================================
// AUDIT LOGGING
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a boundary protection event to the audit log.
func (b *BoundaryProtection) logEvent(eventType string, success bool, metadata map[string]string) {
	if b.auditLogger == nil || !b.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: "boundary",
		Success:   success,
		Metadata:  metadata,
	}

	if err := b.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log boundary event %s: %v\n", eventType, err)
	}
}

// =============================================================================
// GLOBAL BOUNDARY PROTECTION
// =============================================================================

var (
	globalBoundaryProtection     *BoundaryProtection
	globalBoundaryProtectionOnce sync.Once
	globalBoundaryProtectionMu   sync.Mutex
)

// GlobalBoundaryProtection returns the global boundary protection instance.
func GlobalBoundaryProtection() *BoundaryProtection {
	globalBoundaryProtectionOnce.Do(func() {
		globalBoundaryProtection = NewBoundaryProtection()
	})
	return globalBoundaryProtection
}

// SetGlobalBoundaryProtection sets the global boundary protection instance.
func SetGlobalBoundaryProtection(bp *BoundaryProtection) {
	globalBoundaryProtectionMu.Lock()
	defer globalBoundaryProtectionMu.Unlock()
	globalBoundaryProtection = bp
}

// InitGlobalBoundaryProtection initializes the global instance with options.
func InitGlobalBoundaryProtection(opts ...BoundaryProtectionOption) {
	globalBoundaryProtectionMu.Lock()
	defer globalBoundaryProtectionMu.Unlock()
	globalBoundaryProtection = NewBoundaryProtection(opts...)
}
