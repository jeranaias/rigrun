// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package offline provides IL5 compliance for air-gapped/offline mode operation.
//
// This package implements NIST SP 800-53 controls for secure offline operation
// in DoD IL5 compliant environments. It blocks external network access while
// allowing only authorized local connections.
//
// # NIST SP 800-53 Controls
//
//   - SC-7: Boundary Protection (air-gapped environments)
//   - SC-8: Transmission Confidentiality (no external transmission)
//   - CA-3: System Interconnections (controlled)
//
// # Key Types
//
//   - Mode: Operating mode (online, offline, air-gapped)
//   - Validator: Validates URLs against allowed hosts
//
// # Usage
//
//	// Enable offline mode
//	offline.SetMode(offline.ModeAirGapped)
//
//	// Validate URL before making request
//	if err := offline.ValidateURL(targetURL); err != nil {
//		return err // URL blocked in offline mode
//	}
//
//	// Check current mode
//	if offline.IsOffline() {
//		// Use local models only
//	}
package offline
