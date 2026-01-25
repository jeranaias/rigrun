// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package network provides network boundary protection and transport security.
//
// This package implements NIST 800-53 SC-* controls:
//   - SC-7: Boundary Protection
//   - SC-8: Transmission Confidentiality and Integrity
//
// # Boundary Protection (SC-7)
//
// Network boundary protection enforces allowlist/blocklist policies:
//
//	protection, err := network.NewBoundaryProtection(config)
//	if err != nil {
//	    return err
//	}
//
//	// Validate a destination before connecting
//	if err := protection.ValidateDestination("api.example.com:443"); err != nil {
//	    // Destination is blocked
//	}
//
//	// Get a transport with boundary enforcement
//	transport := protection.GetBoundaryTransport()
//
// # Transport Security (SC-8)
//
// Transport security ensures TLS 1.2+ with FIPS-approved cipher suites:
//
//	security := network.NewTransportSecurity()
//
//	// Get a secure HTTP transport
//	transport := security.GetSecureTransport()
//
//	// Validate connection security
//	info, err := security.ValidateConnection("api.example.com:443")
//
// # Network Policy
//
// Network policies define allowed and blocked destinations:
//
//	policy := &network.NetworkPolicy{
//	    Allowlist: []string{"api.openai.com", "api.anthropic.com"},
//	    Blocklist: []string{"evil.com"},
//	    EgressFilteringEnabled: true,
//	}
//
// # Certificate Pinning
//
// Certificate pinning is supported for high-security connections:
//
//	security.PinCertificate("api.example.com", fingerprint)
//	if security.IsCertPinned("api.example.com") {
//	    // Connection will verify certificate fingerprint
//	}
package network
