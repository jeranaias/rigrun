// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth provides authentication and session management.
//
// This file implements NIST 800-53 AC-11/AC-12 session controls:
//   - AC-11: Session Lock
//   - AC-12: Session Termination
package auth

// This file re-exports session management types and functions from the parent
// security package for backward compatibility during migration.
//
// The actual implementation remains in security/session.go.
