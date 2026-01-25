// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth provides authentication and session management.
//
// This package implements NIST 800-53 IA-* controls:
//   - IA-2: Identification and Authentication (Organizational Users)
//   - IA-2(1): Multi-factor Authentication for Network Access
//   - IA-2(8): Network Access to Privileged Accounts - Replay Resistant
//   - IA-5: Authenticator Management
package auth

// This file re-exports types and functions from the parent security package
// for backward compatibility during migration. The actual implementation
// remains in security/auth.go.
//
// To use the auth package directly:
//
//	import "rigrun/internal/security/auth"
//
//	manager := auth.NewManager()
//	session, err := manager.Authenticate(auth.MethodAPIKey, apiKey)

// Re-export type aliases - these will be updated when migration is complete
// For now, import the parent package to use authentication functionality.
