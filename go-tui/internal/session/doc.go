// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package session provides session management with DoD compliance timeout.
//
// This package implements session lifecycle management with automatic
// timeout enforcement per NIST 800-53 AC-12 (Session Termination) for
// DoD IL5 compliance.
//
// # Key Types
//
//   - Manager: Session manager with timeout tracking
//   - TimeoutMsg: Bubble Tea message for session timeout
//   - WarningMsg: Bubble Tea message for timeout warning
//
// # Usage
//
// Create a session manager with 15-minute timeout:
//
//	mgr := session.NewManager(15 * time.Minute)
//	mgr.Start()
//	defer mgr.Stop()
//
// Reset timeout on user activity:
//
//	mgr.ResetTimeout()
//
// Check if session is still valid:
//
//	if mgr.IsExpired() {
//	    // Handle session expiration
//	}
//
// # Compliance
//
// Default timeout is 15 minutes per DoD IL5 requirements.
// Sessions are automatically terminated after the timeout period.
package session
