// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security implements NIST 800-53 security controls for IL5 compliance.
//
// This package provides comprehensive security functionality for DoD environments,
// implementing controls required for Impact Level 5 (IL5) authorization.
//
// # Package Organization
//
// The security package is organized into focused subpackages:
//
//   - audit: Audit logging, integrity protection, and review (AU-*)
//   - auth: Authentication and session management (IA-*, AC-11/12)
//   - crypto: Cryptographic operations and PKI (SC-13, SC-17, IA-7)
//   - access: RBAC and account lockout (AC-5, AC-6, AC-7)
//   - classification: Classification marking and enforcement (AC-4)
//   - network: Boundary protection and transport security (SC-7, SC-8)
//
// # NIST 800-53 Controls Implemented
//
// Access Control (AC):
//   - AC-4: Information Flow Enforcement
//   - AC-5: Separation of Duties
//   - AC-6: Least Privilege
//   - AC-7: Unsuccessful Logon Attempts
//   - AC-11: Session Lock
//   - AC-12: Session Termination
//
// Audit and Accountability (AU):
//   - AU-2: Audit Events
//   - AU-3: Content of Audit Records
//   - AU-5: Response to Audit Processing Failures
//   - AU-6: Audit Review, Analysis, and Reporting
//   - AU-9: Protection of Audit Information
//
// Identification and Authentication (IA):
//   - IA-2: Identification and Authentication
//   - IA-2(1): Multi-factor Authentication
//   - IA-5: Authenticator Management
//   - IA-7: Cryptographic Module Authentication
//
// System and Communications Protection (SC):
//   - SC-7: Boundary Protection
//   - SC-8: Transmission Confidentiality and Integrity
//   - SC-13: Cryptographic Protection
//   - SC-17: Public Key Infrastructure Certificates
//
// # Backward Compatibility
//
// This file provides type aliases and re-exports for backward compatibility.
// Existing code importing "rigrun/internal/security" will continue to work.
//
// New code is encouraged to import specific subpackages:
//
//	import "rigrun/internal/security/audit"
//	import "rigrun/internal/security/auth"
//	import "rigrun/internal/security/access"
//
// # Usage Examples
//
// Audit logging:
//
//	logger, err := security.NewAuditLogger("")
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
//	logger.LogQuery(sessionID, tier, query, tokens, cost, success)
//
// Authentication:
//
//	authMgr := security.NewAuthManager()
//	session, err := authMgr.Authenticate(security.AuthMethodAPIKey, apiKey)
//
// RBAC:
//
//	rbacMgr := security.GlobalRBACManager()
//	if rbacMgr.CheckPermission(userID, security.PermRunQuery) {
//	    // Allow operation
//	}
//
// Classification:
//
//	level := security.ParseClassification("SECRET//NOFORN")
//	banner := security.RenderTopBanner(level)
package security

// =============================================================================
// BACKWARD COMPATIBILITY RE-EXPORTS
// =============================================================================

// This section provides backward compatibility for code that imports the
// security package directly. Types and functions are re-exported from their
// new subpackage locations.

// NOTE: ZeroBytes is defined in encrypt.go to avoid redeclaration.
// NOTE: AuditEvent is defined in audit.go to avoid redeclaration.

// =============================================================================
// TYPE ALIASES REMOVED - See individual source files
// =============================================================================

// The original type aliases for AuditEvent and ZeroBytes have been removed
// to avoid redeclaration errors. Import the types directly from their
// defining files (audit.go, encrypt.go).

// Note: Additional type aliases would be added here as migration progresses.
// For now, the original implementations in this package continue to work.
