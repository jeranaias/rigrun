// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package access provides role-based access control and account lockout.
//
// This package implements NIST 800-53 AC-* controls:
//   - AC-5: Separation of Duties
//   - AC-6: Least Privilege
//   - AC-7: Unsuccessful Logon Attempts
//
// # Role-Based Access Control (RBAC)
//
// RBAC provides separation of duties with four roles:
//
//   - Admin: Full access to all operations, user management, and audit logs
//   - Operator: Run queries, manage configuration, execute tools
//   - Auditor: Read-only access to logs, reports, and configuration
//   - User: Read-only access, view own history only
//
// Usage:
//
//	manager, err := access.NewRBACManager()
//	if err != nil {
//	    return err
//	}
//
//	// Assign role
//	err = manager.AssignRole("user123", access.RoleOperator, "admin1")
//
//	// Check permission
//	if manager.CheckPermission("user123", access.PermRunQuery) {
//	    // Allow operation
//	}
//
// # Account Lockout (AC-7)
//
// Lockout manager tracks failed authentication attempts:
//
//	lockout := access.NewLockoutManager()
//
//	// Record failed attempt
//	err := lockout.RecordAttempt("user123", false)
//	if errors.Is(err, access.ErrLocked) {
//	    // Account is locked
//	}
//
//	// Check if locked
//	if lockout.IsLocked("user123") {
//	    // Deny access
//	}
package access
