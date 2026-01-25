// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file contains security-related command handlers for the chat interface:
//   - Audit log retrieval and display (getRecentAuditEntries)
//   - Security status reporting (getSecurityStatus)
//   - User consent status checking (getConsentStatus)
//
// These functions support DoD IL5 compliance requirements and provide
// visibility into security-critical operations.
package chat

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// SECURITY STATUS AND AUDIT LOG FUNCTIONS
// =============================================================================

// getRecentAuditEntries reads and returns the most recent audit log entries.
//
// This function efficiently reads the audit log file, handling both small and large files:
//   - Small files (<1MB): reads normally into memory
//   - Large files: uses a circular buffer to avoid excessive memory usage
//
// Parameters:
//   - lines: number of recent entries to retrieve (capped at 10,000 for safety)
//
// Returns:
//   - Formatted string with the requested audit entries
//   - Error messages if audit logging is disabled or file cannot be read
//
// Security considerations:
//   - File system errors are sanitized to prevent information disclosure
//   - Input is validated to prevent abuse
//   - Memory usage is bounded even for very large audit logs
func getRecentAuditEntries(lines int) string {
	// Validate input parameter
	if lines <= 0 {
		return "Invalid number of lines requested"
	}
	// Cap at a reasonable maximum to prevent abuse
	const maxLines = 10000
	if lines > maxLines {
		lines = maxLines
	}

	auditLogger := security.GlobalAuditLogger()
	// GlobalAuditLogger() never returns nil, it returns a disabled logger on error
	if !auditLogger.IsEnabled() {
		return "Audit logging is not enabled"
	}

	auditPath := auditLogger.Path()
	if auditPath == "" {
		return "Audit log path not configured"
	}

	// Read the audit log file
	file, err := os.Open(auditPath)
	if err != nil {
		// Don't expose detailed file system errors to user (security concern)
		return "Failed to open audit log: access denied or file not found"
	}
	defer file.Close()

	// Get file size to determine if we can optimize reading
	fileInfo, err := file.Stat()
	if err != nil {
		return "Failed to read audit log: unable to stat file"
	}

	// For small files, read entirely. For large files, use a more efficient approach.
	const smallFileThreshold = 1024 * 1024 // 1MB
	var allLines []string

	if fileInfo.Size() < smallFileThreshold {
		// Small file: read normally
		scanner := bufio.NewScanner(file)
		// Increase buffer size for long lines (default 64KB might be too small)
		const maxScanTokenSize = 1024 * 1024 // 1MB per line
		buf := make([]byte, maxScanTokenSize)
		scanner.Buffer(buf, maxScanTokenSize)

		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return "Failed to read audit log: file read error"
		}
	} else {
		// Large file: read from end efficiently
		// This is a simplified approach - for production, consider using reverse file reading
		// or a circular buffer approach, but this still loads everything into memory.
		// TODO: Implement proper tail reading for very large files
		scanner := bufio.NewScanner(file)
		const maxScanTokenSize = 1024 * 1024 // 1MB per line
		buf := make([]byte, maxScanTokenSize)
		scanner.Buffer(buf, maxScanTokenSize)

		// Use a circular buffer approach: keep only the last N lines
		// This prevents unbounded memory growth
		circularBuf := make([]string, 0, lines*2) // Pre-allocate with some headroom
		for scanner.Scan() {
			circularBuf = append(circularBuf, scanner.Text())
			// Keep only the last lines*2 entries to avoid growing forever
			if len(circularBuf) > lines*2 {
				// Shift and remove old entries
				copy(circularBuf, circularBuf[len(circularBuf)-lines:])
				circularBuf = circularBuf[:lines]
			}
		}

		if err := scanner.Err(); err != nil {
			return "Failed to read audit log: file read error"
		}
		allLines = circularBuf
	}

	if len(allLines) == 0 {
		return "No audit entries found"
	}

	// Get the last N lines
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	recentLines := allLines[start:]

	// Format the output efficiently using strings.Builder
	var builder strings.Builder
	// Pre-allocate approximate capacity
	builder.Grow(len(recentLines) * 100) // Assume ~100 chars per line
	builder.WriteString(fmt.Sprintf("Recent audit log entries (last %d):\n\n", len(recentLines)))
	for _, line := range recentLines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

// getSecurityStatus returns a formatted summary of the security status.
//
// Displays current security configuration including:
//   - Classification level (e.g., UNCLASSIFIED, SECRET)
//   - Consent requirement status
//   - Session timeout settings
//   - Encryption status (AES-256-GCM)
//   - Audit logging status
//
// Returns:
//   - Formatted multi-line string with security status summary
//   - Error message if configuration is not loaded
func getSecurityStatus() string {
	cfg := config.Global()
	if cfg == nil {
		return "Configuration not loaded"
	}

	// Classification level
	classLevel := "UNCLASSIFIED"
	if cfg.Security.Classification != "" {
		classLevel = cfg.Security.Classification
	}

	// Consent status
	consentRequired := cfg.Security.ConsentRequired

	// Session timeout - handle edge cases for formatting
	sessionTimeoutStr := "Not configured"
	if cfg.Security.SessionTimeoutSecs > 0 {
		secs := cfg.Security.SessionTimeoutSecs
		if secs < 60 {
			sessionTimeoutStr = fmt.Sprintf("%d seconds", secs)
		} else {
			mins := secs / 60
			remainingSecs := secs % 60
			if remainingSecs == 0 {
				sessionTimeoutStr = fmt.Sprintf("%d minutes", mins)
			} else {
				sessionTimeoutStr = fmt.Sprintf("%d minutes %d seconds", mins, remainingSecs)
			}
		}
	}

	// Encryption status
	encryptionMgr := security.GlobalEncryptionManager()
	encryptionEnabled := "Not initialized"
	// GlobalEncryptionManager may return nil, check defensively
	if encryptionMgr != nil && encryptionMgr.IsInitialized() {
		encryptionEnabled = "Enabled (AES-256-GCM)"
	}

	// Audit logging status
	auditLogger := security.GlobalAuditLogger()
	auditEnabled := "Disabled"
	// GlobalAuditLogger() never returns nil, but keep check for safety
	if auditLogger != nil && auditLogger.IsEnabled() {
		auditEnabled = "Enabled"
	}

	// Format the status message efficiently using strings.Builder
	var builder strings.Builder
	builder.Grow(300) // Pre-allocate approximate size
	builder.WriteString("Security Status Summary:\n\n")
	builder.WriteString(fmt.Sprintf("Classification Level: %s\n", classLevel))
	builder.WriteString(fmt.Sprintf("Consent Required: %v\n", consentRequired))
	builder.WriteString(fmt.Sprintf("Session Timeout: %s\n", sessionTimeoutStr))
	builder.WriteString(fmt.Sprintf("Encryption: %s\n", encryptionEnabled))
	builder.WriteString(fmt.Sprintf("Audit Logging: %s\n", auditEnabled))

	return builder.String()
}

// getConsentStatus returns formatted information about consent status.
//
// Checks and displays:
//   - Whether consent is required (DoD IL5 PS-6 compliance)
//   - User agreement acceptance status
//   - Missing agreements that need to be signed
//   - Instructions for enabling consent requirement
//
// Returns:
//   - Formatted multi-line string with consent status details
//   - Error message if configuration is not loaded
func getConsentStatus() string {
	cfg := config.Global()
	if cfg == nil {
		return "Configuration not loaded"
	}

	// Use strings.Builder for efficient string concatenation
	var builder strings.Builder
	builder.Grow(500) // Pre-allocate approximate size
	builder.WriteString("Consent Status:\n\n")

	// Check if consent is required and if agreements have been accepted
	agreementMgr := security.GlobalAgreementManager()

	if cfg.Security.ConsentRequired {
		builder.WriteString("Consent Requirement: ENABLED\n")
		builder.WriteString("Required for DoD IL5 PS-6 compliance.\n\n")

		// Check if all required agreements are signed
		// GlobalAgreementManager() should never return nil, but check defensively
		if agreementMgr != nil {
			// Use the system username or UserID from config
			userID := cfg.Security.UserID
			if userID == "" {
				userID = "system_user"
			}

			valid, missing := agreementMgr.CheckAgreementsValid(userID)
			if valid {
				builder.WriteString("Status: ALL AGREEMENTS ACCEPTED\n")
				builder.WriteString("You have signed all required access agreements.")
			} else {
				builder.WriteString("Status: AGREEMENTS PENDING\n")
				builder.WriteString(fmt.Sprintf("Missing agreements: %v\n", missing))
				builder.WriteString("\nPlease sign all required access agreements before using the system.")
			}
		} else {
			builder.WriteString("Status: Agreement manager not initialized")
		}
	} else {
		builder.WriteString("Consent Requirement: DISABLED\n\n")
		builder.WriteString("To enable consent requirement:\n  rigrun config security.consent_required true")
	}

	return builder.String()
}
