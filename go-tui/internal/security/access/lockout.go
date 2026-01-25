// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package access provides role-based access control and account lockout.
//
// This file implements AC-7 (Unsuccessful Logon Attempts).
package access

// This file re-exports lockout types and functions from the parent
// security package for backward compatibility during migration.
//
// The actual implementation remains in security/lockout.go.
