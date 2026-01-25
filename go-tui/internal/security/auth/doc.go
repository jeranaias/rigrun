// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth provides authentication and session management for IL5 compliance.
//
// This package implements NIST 800-53 IA-* and AC-* controls:
//   - IA-2: Identification and Authentication (Organizational Users)
//   - IA-2(1): Multi-factor Authentication for Network Access
//   - IA-2(8): Network Access to Privileged Accounts - Replay Resistant
//   - IA-5: Authenticator Management
//   - AC-11: Session Lock
//   - AC-12: Session Termination
//
// # Authentication Manager
//
// The Manager provides API key authentication with lockout integration:
//
//	manager := auth.NewManager()
//	session, err := manager.Authenticate(auth.MethodAPIKey, apiKey)
//	if err != nil {
//	    if errors.Is(err, security.ErrLocked) {
//	        // Account is locked out
//	    }
//	    return err
//	}
//
// # Session Management
//
// Sessions are managed per DoD STIG requirements with 15-minute timeout:
//
//	sessionMgr := auth.NewSessionManager(15 * time.Minute)
//	session, err := sessionMgr.StartSession()
//	if err != nil {
//	    return err
//	}
//	defer sessionMgr.EndSession()
//
// # Multi-Factor Authentication (IA-2(1))
//
// MFA support is available using TOTP:
//
//	manager.SetTOTPSecret(userID, secret)
//	err := manager.VerifyMFA(sessionID, totpCode)
package auth
