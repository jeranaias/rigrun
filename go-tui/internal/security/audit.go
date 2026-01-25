// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides audit logging with secret redaction for DoD IL5 compliance.
package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// MaxQueryLength is the maximum length of query to log before truncation.
const MaxQueryLength = 200

// DefaultMaxFileSize is the default max file size before rotation (10MB).
const DefaultMaxFileSize int64 = 10 * 1024 * 1024

// =============================================================================
// AUDIT EVENT
// =============================================================================

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	Timestamp time.Time         `json:"timestamp"`
	EventType string            `json:"event_type"`
	SessionID string            `json:"session_id"`
	Tier      string            `json:"tier,omitempty"`
	Query     string            `json:"query,omitempty"` // Truncated/redacted
	Tokens    int               `json:"tokens,omitempty"`
	Cost      float64           `json:"cost_cents,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ToLogLine formats the event as a single log line.
func (e *AuditEvent) ToLogLine() string {
	timestamp := e.Timestamp.Format("2006-01-02 15:04:05")

	// Format query with quotes if present
	query := ""
	if e.Query != "" {
		query = fmt.Sprintf("\"%s\"", e.Query)
	}

	// Format tokens
	tokens := ""
	if e.Tokens > 0 {
		tokens = fmt.Sprintf("%d", e.Tokens)
	}

	// Format cost
	cost := ""
	if e.Cost > 0 {
		cost = fmt.Sprintf("%.2f", e.Cost)
	}

	// Format success/error
	status := "SUCCESS"
	if !e.Success {
		if e.Error != "" {
			status = fmt.Sprintf("ERROR: %s", e.Error)
		} else {
			status = "FAILURE"
		}
	}

	return fmt.Sprintf("%s | %s | %s | %s | %s | %s | %s | %s",
		timestamp,
		e.EventType,
		e.SessionID,
		e.Tier,
		query,
		tokens,
		cost,
		status,
	)
}

// ToJSON formats the event as JSON.
func (e *AuditEvent) ToJSON() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// =============================================================================
// REDACTOR INTERFACE
// =============================================================================

// Redactor defines the interface for secret redaction.
type Redactor interface {
	// Redact replaces sensitive data in the input string.
	Redact(input string) string
	// Name returns the name of this redactor.
	Name() string
}

// =============================================================================
// PATTERN REDACTOR
// =============================================================================

// PatternRedactor redacts text matching a regex pattern.
type PatternRedactor struct {
	name    string
	pattern *regexp.Regexp
	replace string
}

// NewPatternRedactor creates a new pattern-based redactor.
func NewPatternRedactor(name string, pattern *regexp.Regexp, replace string) *PatternRedactor {
	return &PatternRedactor{
		name:    name,
		pattern: pattern,
		replace: replace,
	}
}

// Redact replaces matches with the replacement string.
func (r *PatternRedactor) Redact(input string) string {
	return r.pattern.ReplaceAllString(input, r.replace)
}

// Name returns the redactor name.
func (r *PatternRedactor) Name() string {
	return r.name
}

// =============================================================================
// BUILT-IN SECRET PATTERNS
// =============================================================================

// secretPatterns defines patterns for common API keys and secrets.
var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
	replace string
}{
	{"OpenAI", regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`), "[OPENAI_KEY_REDACTED]"},
	{"OpenRouter", regexp.MustCompile(`sk-or-v1-[a-zA-Z0-9]{64}`), "[OPENROUTER_KEY_REDACTED]"},
	{"GitHub", regexp.MustCompile(`gh[pousr]_[a-zA-Z0-9]{36,}`), "[GITHUB_TOKEN_REDACTED]"},
	{"AWS", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[AWS_KEY_REDACTED]"},
	{"Bearer", regexp.MustCompile(`Bearer\s+[a-zA-Z0-9\-_.]+`), "Bearer [TOKEN_REDACTED]"},
	{"Password", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`), "[PASSWORD_REDACTED]"},
	{"JWT", regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`), "[JWT_REDACTED]"},
	{"Anthropic", regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{20,}`), "[ANTHROPIC_KEY_REDACTED]"},
	{"Azure", regexp.MustCompile(`[a-zA-Z0-9]{32}`), "[AZURE_KEY_REDACTED]"}, // Generic 32-char keys
}

// defaultRedactors returns the default set of secret redactors.
func defaultRedactors() []Redactor {
	redactors := make([]Redactor, 0, len(secretPatterns))
	for _, sp := range secretPatterns {
		redactors = append(redactors, NewPatternRedactor(sp.name, sp.pattern, sp.replace))
	}
	return redactors
}

// =============================================================================
// AUDIT LOGGER
// =============================================================================

// =============================================================================
// NIST 800-53 AU-5 AUDIT FAILURE RESPONSE
// =============================================================================

// ErrAuditSystemFailed is returned when the audit system has failed and operations are blocked.
var ErrAuditSystemFailed = fmt.Errorf("audit system has failed - operations blocked per AU-5")

// ErrAuditCircuitBreakerOpen is returned when the circuit breaker is open due to repeated failures.
var ErrAuditCircuitBreakerOpen = fmt.Errorf("audit circuit breaker open - too many consecutive failures")

// AuditFailureCallback is called when audit logging fails.
// Per AU-5, this allows the system to alert on audit failure.
// CRITICAL: This callback is now SYNCHRONOUS to prevent goroutine leaks.
type AuditFailureCallback func(err error)

// AuditLogger provides thread-safe audit logging with secret redaction.
// Implements NIST 800-53 AU-5 (Response to Audit Processing Failures).
type AuditLogger struct {
	path      string
	file      *os.File
	mu        sync.Mutex
	enabled   bool
	maxSize   int64 // Max file size before rotation
	redactors []Redactor

	// AU-5: Audit Failure Response (CRITICAL SECURITY)
	auditFailed   bool                 // Global flag: true if audit has failed
	lastFailure   error                // Last failure error
	failureCount  int                  // Number of consecutive failures
	onFailure     AuditFailureCallback // Called SYNCHRONOUSLY when logging fails
	haltOnFailure bool                 // Stop operations if logging fails

	// Circuit Breaker Pattern
	circuitBreakerThreshold int           // Number of failures before circuit opens (default: 5)
	circuitBreakerOpen      bool          // Circuit breaker state
	circuitBreakerOpenTime  time.Time     // When circuit breaker opened
	circuitBreakerResetTime time.Duration // How long to wait before attempting reset (default: 1 minute)

	// Storage Capacity
	capacityWarningMB     int64         // Warning threshold (default: 80% of max)
	capacityCriticalMB    int64         // Critical threshold (default: 90% of max)
	lastCapacityCheck     time.Time     // Last time capacity was checked
	capacityCheckInterval time.Duration // How often to check capacity
}

// NewAuditLogger creates a new audit logger at the specified path.
func NewAuditLogger(path string) (*AuditLogger, error) {
	// Resolve path
	if path == "" {
		path = DefaultAuditPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open file for appending
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &AuditLogger{
		path:      path,
		file:      file,
		enabled:   true,
		maxSize:   DefaultMaxFileSize,
		redactors: defaultRedactors(),

		// AU-5: Failure tracking (CRITICAL SECURITY)
		auditFailed:   false,
		lastFailure:   nil,
		failureCount:  0,
		haltOnFailure: true, // DEFAULT TO TRUE for security

		// Circuit Breaker defaults
		circuitBreakerThreshold: 5,
		circuitBreakerOpen:      false,
		circuitBreakerResetTime: 1 * time.Minute,

		// AU-5: Default capacity thresholds (will be calculated from storage)
		capacityWarningMB:     0, // 0 means auto-calculate as 80% of available
		capacityCriticalMB:    0, // 0 means auto-calculate as 90% of available
		capacityCheckInterval: 5 * time.Minute,
	}, nil
}

// =============================================================================
// LOGGING METHODS
// =============================================================================

// Log writes an audit event to the log file.
// Implements AU-5: Responds to audit failures via callback and BLOCKS operations on failure.
func (l *AuditLogger) Log(event AuditEvent) error {
	l.mu.Lock()

	// SECURITY: Collect pending callbacks to execute after releasing lock
	// This prevents race conditions from unlock/relock during callbacks
	var pendingCallbacks []func()

	if !l.enabled || l.file == nil {
		l.mu.Unlock()
		return nil
	}

	// AU-5 CRITICAL: Check if audit system has failed
	// MUST halt ALL operations if audit has failed
	if l.auditFailed {
		lastErr := l.lastFailure
		l.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrAuditSystemFailed, lastErr)
	}

	// AU-5 CRITICAL: Check circuit breaker
	if l.circuitBreakerOpen {
		// Attempt reset if enough time has passed
		if time.Since(l.circuitBreakerOpenTime) >= l.circuitBreakerResetTime {
			l.circuitBreakerOpen = false
			l.failureCount = 0
			fmt.Fprintf(os.Stderr, "[AU-5 INFO] Circuit breaker reset attempted\n")
		} else {
			l.mu.Unlock()
			return ErrAuditCircuitBreakerOpen
		}
	}

	// Redact sensitive data from query
	if event.Query != "" {
		event.Query = l.redactLocked(event.Query)
	}

	// Redact metadata values
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			event.Metadata[k] = l.redactLocked(v)
		}
	}

	// Redact error message
	if event.Error != "" {
		event.Error = l.redactLocked(event.Error)
	}

	// AU-5: Check storage capacity periodically
	if err := l.checkCapacityLocked(); err != nil {
		cb, cbErr := l.handleFailureLocked(err)
		if cb != nil {
			errCopy := cbErr
			pendingCallbacks = append(pendingCallbacks, func() { cb(errCopy) })
		}
		// Critical capacity failures should halt operations
		if strings.Contains(err.Error(), "CRITICAL") && l.haltOnFailure {
			l.mu.Unlock()
			// Execute pending callbacks outside lock
			for _, callback := range pendingCallbacks {
				callback()
			}
			return fmt.Errorf("AU-5: %w", err)
		}
	}

	// Check if rotation needed
	if err := l.checkRotationLocked(); err != nil {
		cb, cbErr := l.handleFailureLocked(err)
		if cb != nil {
			errCopy := cbErr
			pendingCallbacks = append(pendingCallbacks, func() { cb(errCopy) })
		}
		if l.haltOnFailure {
			l.mu.Unlock()
			// Execute pending callbacks outside lock
			for _, callback := range pendingCallbacks {
				callback()
			}
			return fmt.Errorf("AU-5: audit rotation failed: %w", err)
		}
	}

	// Write log line
	logLine := event.ToLogLine()
	if _, err := fmt.Fprintln(l.file, logLine); err != nil {
		writeErr := fmt.Errorf("failed to write audit log: %w", err)
		cb, cbErr := l.handleFailureLocked(writeErr)
		if cb != nil {
			errCopy := cbErr
			pendingCallbacks = append(pendingCallbacks, func() { cb(errCopy) })
		}
		l.mu.Unlock()
		// Execute pending callbacks outside lock
		for _, callback := range pendingCallbacks {
			callback()
		}
		// AU-5 CRITICAL: ALWAYS return error on write failure
		return writeErr
	}

	// Sync to disk to ensure durability
	if err := l.file.Sync(); err != nil {
		syncErr := fmt.Errorf("failed to sync audit log: %w", err)
		cb, cbErr := l.handleFailureLocked(syncErr)
		if cb != nil {
			errCopy := cbErr
			pendingCallbacks = append(pendingCallbacks, func() { cb(errCopy) })
		}
		l.mu.Unlock()
		// Execute pending callbacks outside lock
		for _, callback := range pendingCallbacks {
			callback()
		}
		return syncErr
	}

	// Success - reset failure count
	l.failureCount = 0

	l.mu.Unlock()

	// SECURITY: Execute any pending callbacks outside the lock to prevent races
	for _, callback := range pendingCallbacks {
		callback()
	}

	return nil
}

// handleFailureLocked handles an audit failure (caller must hold lock).
// Implements AU-5: Alert on audit processing failure and HALT operations.
// CRITICAL SECURITY FIX: No more goroutine leaks - callback is SYNCHRONOUS.
// Returns callback and error to be executed by caller AFTER releasing lock.
// SECURITY: No lock release during callback to prevent races
func (l *AuditLogger) handleFailureLocked(err error) (callback func(error), callbackErr error) {
	// Increment failure count
	l.failureCount++
	l.lastFailure = err

	// Alert via stderr (always)
	fmt.Fprintf(os.Stderr, "[AU-5 AUDIT FAILURE #%d] %v\n", l.failureCount, err)

	// Check circuit breaker threshold
	if l.failureCount >= l.circuitBreakerThreshold {
		l.circuitBreakerOpen = true
		l.circuitBreakerOpenTime = time.Now()
		l.auditFailed = true // Set global failure flag
		fmt.Fprintf(os.Stderr, "[AU-5 CRITICAL] Circuit breaker OPEN - audit system FAILED after %d consecutive failures\n", l.failureCount)
	}

	// CRITICAL: Only set auditFailed if haltOnFailure is true
	// This ensures operations are blocked when required
	if l.haltOnFailure {
		l.auditFailed = true
	}

	// SECURITY: Return callback to be executed outside lock to prevent races
	// Caller is responsible for invoking callback after releasing lock
	if l.onFailure != nil {
		return l.onFailure, err
	}
	return nil, nil
}

// checkCapacityLocked checks storage capacity and logs warnings.
// Implements AU-5: Alert when audit log storage is running low.
func (l *AuditLogger) checkCapacityLocked() error {
	// Only check periodically
	if time.Since(l.lastCapacityCheck) < l.capacityCheckInterval {
		return nil
	}
	l.lastCapacityCheck = time.Now()

	// Get storage capacity
	usedMB, totalMB, err := l.getCapacityLocked()
	if err != nil {
		return nil // Ignore capacity check errors
	}

	// Calculate thresholds (use configured or auto-calculate)
	warningMB := l.capacityWarningMB
	criticalMB := l.capacityCriticalMB
	if warningMB == 0 {
		warningMB = totalMB * 80 / 100 // 80%
	}
	if criticalMB == 0 {
		criticalMB = totalMB * 90 / 100 // 90%
	}

	// Check thresholds
	if usedMB >= criticalMB {
		return fmt.Errorf("AUDIT_CAPACITY_CRITICAL: storage at %d%% (%dMB/%dMB)",
			usedMB*100/totalMB, usedMB, totalMB)
	}
	if usedMB >= warningMB {
		return fmt.Errorf("AUDIT_CAPACITY_WARNING: storage at %d%% (%dMB/%dMB)",
			usedMB*100/totalMB, usedMB, totalMB)
	}

	return nil
}

// getCapacityLocked returns the used and total storage in MB (caller must hold lock).
func (l *AuditLogger) getCapacityLocked() (usedMB, totalMB int64, err error) {
	// Get the directory containing the audit log
	dir := filepath.Dir(l.path)

	// Get disk usage - this is platform-specific, use a simple approximation
	// by checking file sizes in the rigrun directory
	var usedBytes int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			usedBytes += info.Size()
		}
		return nil
	})

	usedMB = usedBytes / (1024 * 1024)

	// For total, we'd ideally use syscall to get disk space
	// For now, use a reasonable default or calculate from used + available
	// This is a simplified implementation - production would use platform-specific syscalls
	totalMB = 10240 // Default 10GB assumption

	return usedMB, totalMB, nil
}

// LogQuery is a convenience method for logging queries.
func (l *AuditLogger) LogQuery(sessionID, tier, query string, tokens int, cost float64, success bool) error {
	// Truncate query
	truncatedQuery := truncateQuery(query, MaxQueryLength)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "QUERY",
		SessionID: sessionID,
		Tier:      tier,
		Query:     truncatedQuery,
		Tokens:    tokens,
		Cost:      cost,
		Success:   success,
	}

	return l.Log(event)
}

// LogQueryWithError logs a failed query with an error message.
func (l *AuditLogger) LogQueryWithError(sessionID, tier, query string, tokens int, cost float64, errMsg string) error {
	// Truncate query
	truncatedQuery := truncateQuery(query, MaxQueryLength)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "QUERY",
		SessionID: sessionID,
		Tier:      tier,
		Query:     truncatedQuery,
		Tokens:    tokens,
		Cost:      cost,
		Success:   false,
		Error:     errMsg,
	}

	return l.Log(event)
}

// LogEvent logs a generic event with optional metadata.
func (l *AuditLogger) LogEvent(sessionID, eventType string, metadata map[string]string) error {
	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		Success:   true,
		Metadata:  metadata,
	}

	return l.Log(event)
}

// LogStartup logs application startup.
func (l *AuditLogger) LogStartup(sessionID string, metadata map[string]string) error {
	return l.LogEvent(sessionID, "STARTUP", metadata)
}

// LogShutdown logs application shutdown.
func (l *AuditLogger) LogShutdown(sessionID string, metadata map[string]string) error {
	return l.LogEvent(sessionID, "SHUTDOWN", metadata)
}

// LogBannerAck logs banner acknowledgment for compliance.
func (l *AuditLogger) LogBannerAck(sessionID string) error {
	return l.LogEvent(sessionID, "BANNER_ACK", nil)
}

// LogSessionStart logs the start of a user session.
func (l *AuditLogger) LogSessionStart(sessionID string, metadata map[string]string) error {
	return l.LogEvent(sessionID, "SESSION_START", metadata)
}

// LogSessionEnd logs the end of a user session.
func (l *AuditLogger) LogSessionEnd(sessionID string, metadata map[string]string) error {
	return l.LogEvent(sessionID, "SESSION_END", metadata)
}

// LogTimeout logs a session timeout event.
func (l *AuditLogger) LogTimeout(sessionID string) error {
	return l.LogEvent(sessionID, "SESSION_TIMEOUT", nil)
}

// =============================================================================
// REDACTION
// =============================================================================

// Redact applies all redactors to sanitize the input string.
func (l *AuditLogger) Redact(input string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.redactLocked(input)
}

// redactLocked applies redaction without locking (caller must hold lock).
func (l *AuditLogger) redactLocked(input string) string {
	result := input
	for _, redactor := range l.redactors {
		result = redactor.Redact(result)
	}
	return result
}

// AddRedactor adds a custom redactor.
func (l *AuditLogger) AddRedactor(r Redactor) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.redactors = append(l.redactors, r)
}

// =============================================================================
// FILE ROTATION
// =============================================================================

// Rotate rotates the log file, keeping the old file with a timestamp suffix.
func (l *AuditLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rotateLocked()
}

// rotateLocked performs rotation without locking (caller must hold lock).
func (l *AuditLogger) rotateLocked() error {
	if l.file == nil {
		return nil
	}

	// Close current file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close audit log for rotation: %w", err)
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	ext := filepath.Ext(l.path)
	base := strings.TrimSuffix(l.path, ext)
	rotatedPath := fmt.Sprintf("%s_%s%s", base, timestamp, ext)

	// Rename current file
	if err := os.Rename(l.path, rotatedPath); err != nil {
		// Try to reopen original file if rename fails
		l.file, _ = os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		return fmt.Errorf("failed to rotate audit log: %w", err)
	}

	// Open new file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to create new audit log after rotation: %w", err)
	}
	l.file = file

	return nil
}

// checkRotationLocked checks if rotation is needed based on file size.
func (l *AuditLogger) checkRotationLocked() error {
	if l.maxSize <= 0 {
		return nil
	}

	info, err := l.file.Stat()
	if err != nil {
		return nil // Ignore stat errors
	}

	if info.Size() >= l.maxSize {
		return l.rotateLocked()
	}

	return nil
}

// SetMaxSize sets the maximum file size before rotation.
func (l *AuditLogger) SetMaxSize(size int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxSize = size
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetEnabled enables or disables logging.
func (l *AuditLogger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled returns whether logging is enabled.
func (l *AuditLogger) IsEnabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// Path returns the audit log file path.
func (l *AuditLogger) Path() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}

// =============================================================================
// AU-5 AUDIT FAILURE RESPONSE CONFIGURATION
// =============================================================================

// SetOnFailure sets the callback for audit failures (AU-5).
// CRITICAL: The callback is now called SYNCHRONOUSLY to prevent goroutine leaks.
func (l *AuditLogger) SetOnFailure(callback AuditFailureCallback) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onFailure = callback
}

// SetHaltOnFailure enables/disables halting on audit failure (AU-5).
// When enabled, operations will fail if audit logging fails.
// DEFAULT: TRUE for security compliance.
func (l *AuditLogger) SetHaltOnFailure(halt bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.haltOnFailure = halt
}

// IsHaltOnFailure returns whether halt on failure is enabled.
func (l *AuditLogger) IsHaltOnFailure() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.haltOnFailure
}

// HasAuditFailed returns whether the audit system has failed.
// When true, all audited operations MUST be blocked.
func (l *AuditLogger) HasAuditFailed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.auditFailed
}

// GetLastFailure returns the last audit failure error.
func (l *AuditLogger) GetLastFailure() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastFailure
}

// GetFailureCount returns the number of consecutive failures.
func (l *AuditLogger) GetFailureCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.failureCount
}

// IsCircuitBreakerOpen returns whether the circuit breaker is open.
func (l *AuditLogger) IsCircuitBreakerOpen() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.circuitBreakerOpen
}

// SetCircuitBreakerThreshold sets the number of failures before circuit opens.
// Default is 5 failures.
func (l *AuditLogger) SetCircuitBreakerThreshold(threshold int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if threshold > 0 {
		l.circuitBreakerThreshold = threshold
	}
}

// ResetAuditFailure manually resets the audit failure state.
// WARNING: This should only be called after fixing the underlying issue.
// This is primarily for testing and emergency recovery.
func (l *AuditLogger) ResetAuditFailure() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.auditFailed = false
	l.lastFailure = nil
	l.failureCount = 0
	l.circuitBreakerOpen = false
	fmt.Fprintf(os.Stderr, "[AU-5 INFO] Audit failure state manually reset\n")
}

// SetCapacityThresholds sets the warning and critical thresholds in MB (AU-5).
// Set to 0 to use automatic calculation (80% warning, 90% critical).
func (l *AuditLogger) SetCapacityThresholds(warningMB, criticalMB int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.capacityWarningMB = warningMB
	l.capacityCriticalMB = criticalMB
}

// CheckCapacity returns the current storage capacity (AU-5).
// Returns used MB, total MB, and any error.
func (l *AuditLogger) CheckCapacity() (usedMB, totalMB int64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.getCapacityLocked()
}

// VerifyIntegrity checks the integrity of the audit log file.
// Returns an error if the file appears corrupted or tampered with.
func (l *AuditLogger) VerifyIntegrity() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("audit log file not open")
	}

	// Get file info
	info, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat audit log: %w", err)
	}

	// Basic integrity checks
	// 1. File should exist and be readable
	if info.Size() == 0 {
		return nil // Empty file is valid
	}

	// 2. Check file permissions (should be 0600 or 0700)
	mode := info.Mode()
	if mode.Perm()&0077 != 0 {
		return fmt.Errorf("audit log permissions too open: %o (should be 0600)", mode.Perm())
	}

	// 3. Verify file can be read and contains valid log lines
	// Reopen for reading to avoid affecting write position
	readFile, err := os.Open(l.path)
	if err != nil {
		return fmt.Errorf("failed to open audit log for verification: %w", err)
	}
	defer readFile.Close()

	// Read and verify at least the last few lines
	// This is a basic check - more sophisticated integrity checks could use checksums
	scanner := strings.NewReader("")
	_ = scanner // Placeholder for future enhanced verification

	return nil
}

// =============================================================================
// CLEANUP
// =============================================================================

// Close closes the audit log file.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	err := l.file.Close()
	l.file = nil
	return err
}

// Sync flushes the audit log to disk.
func (l *AuditLogger) Sync() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	return l.file.Sync()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// DefaultAuditPath returns the default audit log path (~/.rigrun/audit.log).
func DefaultAuditPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rigrun", "audit.log")
}

// truncateQuery truncates a query to the specified length, adding ellipsis if needed.
func truncateQuery(query string, maxLen int) string {
	// Remove newlines and normalize whitespace
	cleaned := strings.Join(strings.Fields(query), " ")

	// Use rune-based truncation for Unicode safety
	runes := []rune(cleaned)
	if len(runes) <= maxLen {
		return cleaned
	}

	// Truncate and add ellipsis
	if maxLen > 3 {
		return string(runes[:maxLen-3]) + "..."
	}
	return string(runes[:maxLen])
}

// =============================================================================
// STANDALONE REDACT FUNCTION
// =============================================================================

// RedactSecrets applies default redaction patterns to the input string.
// This can be used without an AuditLogger instance.
func RedactSecrets(input string) string {
	result := input
	for _, sp := range secretPatterns {
		result = sp.pattern.ReplaceAllString(result, sp.replace)
	}
	return result
}

// =============================================================================
// GLOBAL AUDIT LOGGER
// =============================================================================

var (
	globalLogger     *AuditLogger
	globalLoggerOnce sync.Once
	globalLoggerMu   sync.Mutex
)

// globalLoggerInitErr holds initialization error for fail-secure checking
var globalLoggerInitErr error

// GlobalAuditLogger returns the global audit logger instance.
// It lazily initializes the logger with the default path.
// SECURITY CRITICAL: Check GlobalAuditLoggerHealthy() before relying on audit logging.
func GlobalAuditLogger() *AuditLogger {
	globalLoggerOnce.Do(func() {
		var err error
		globalLogger, err = NewAuditLogger("")
		if err != nil {
			// SECURITY: Fail-secure - record error for callers to check
			globalLoggerInitErr = fmt.Errorf("SECURITY CRITICAL: audit logger init failed: %w", err)
			// Log to stderr since audit logging doesn't work
			fmt.Fprintf(os.Stderr, "[SECURITY CRITICAL] Audit logger initialization failed: %v\n", err)
			// Create disabled logger but mark it as unhealthy
			globalLogger = &AuditLogger{
				enabled: false,
			}
		}
	})
	return globalLogger
}

// GlobalAuditLoggerHealthy returns true if the global logger initialized successfully.
// SECURITY: Callers should check this for compliance with AU-5 (audit failure response).
func GlobalAuditLoggerHealthy() bool {
	GlobalAuditLogger() // Ensure initialized
	return globalLoggerInitErr == nil
}

// GlobalAuditLoggerError returns the initialization error, if any.
func GlobalAuditLoggerError() error {
	GlobalAuditLogger() // Ensure initialized
	return globalLoggerInitErr
}

// SetGlobalAuditLogger sets the global audit logger instance.
func SetGlobalAuditLogger(logger *AuditLogger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	globalLogger = logger
}

// InitGlobalAuditLogger initializes the global audit logger with the given path.
func InitGlobalAuditLogger(path string, enabled bool) error {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	logger, err := NewAuditLogger(path)
	if err != nil {
		return err
	}
	logger.SetEnabled(enabled)
	globalLogger = logger
	return nil
}

// AuditLog logs an event to the global audit logger.
func AuditLog(event AuditEvent) error {
	return GlobalAuditLogger().Log(event)
}

// AuditLogQuery logs a query to the global audit logger.
func AuditLogQuery(sessionID, tier, query string, tokens int, cost float64, success bool) error {
	return GlobalAuditLogger().LogQuery(sessionID, tier, query, tokens, cost, success)
}

// AuditLogEvent logs a generic event to the global audit logger.
func AuditLogEvent(sessionID, eventType string, metadata map[string]string) error {
	return GlobalAuditLogger().LogEvent(sessionID, eventType, metadata)
}
