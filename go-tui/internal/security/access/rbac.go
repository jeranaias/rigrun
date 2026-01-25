// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package access provides role-based access control and account lockout.
//
// This file implements AC-5 (Separation of Duties) and AC-6 (Least Privilege).
package access

// This file re-exports RBAC types and functions from the parent
// security package for backward compatibility during migration.
//
// The actual implementation remains in security/rbac.go.
