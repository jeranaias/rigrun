// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection for IL5 compliance.
//
// This package implements NIST 800-53 AU-* controls:
//   - AU-2: Audit Events - Defines auditable events
//   - AU-3: Content of Audit Records - Ensures records contain required information
//   - AU-5: Response to Audit Processing Failures - Handles audit system failures
//   - AU-6: Audit Review, Analysis, and Reporting - Provides review capabilities
//   - AU-9: Protection of Audit Information - Cryptographic integrity protection
//
// # Components
//
// Logger - Thread-safe audit logging with secret redaction
//
//	logger, err := audit.NewLogger("/path/to/audit.log")
//	if err != nil {
//	    return err
//	}
//	defer logger.Close()
//
//	// Log events
//	logger.LogQuery(sessionID, tier, query, tokens, cost, success)
//	logger.LogEvent(sessionID, "CONFIG_CHANGE", metadata)
//
// Protector - Cryptographic protection and tamper detection
//
//	protector, err := audit.NewProtector("/path/to/audit.log")
//	if err != nil {
//	    return err
//	}
//
//	// Sign entries and verify integrity
//	protector.SignLogEntry(event)
//	valid, issues, err := protector.VerifyLogIntegrity()
//
// Reviewer - Audit log review and analysis
//
//	reviewer := audit.NewReviewer("/path/to/audit.log", nil)
//	result, err := reviewer.Review()
//	report := reviewer.GenerateReport(result)
//
// # Security Considerations
//
// All audit operations are designed with security in mind:
//   - Secret redaction prevents API keys and passwords from being logged
//   - HMAC-SHA256 integrity protection detects tampering
//   - Strict file permissions (0600) protect audit files
//   - Failure callbacks enable alerting on audit system failures
package audit
