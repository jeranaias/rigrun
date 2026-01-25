// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection.
package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// EVENT TESTS
// =============================================================================

func TestEvent_ToLogLine(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		contains []string
	}{
		{
			name: "success event",
			event: Event{
				Timestamp: time.Date(2025, 1, 23, 10, 30, 0, 0, time.UTC),
				EventType: "QUERY",
				SessionID: "sess_123",
				Tier:      "local",
				Query:     "test query",
				Tokens:    100,
				Cost:      0.05,
				Success:   true,
			},
			contains: []string{"2025-01-23", "10:30:00", "QUERY", "sess_123", "local", "test query", "100", "0.05", "SUCCESS"},
		},
		{
			name: "failure event with error",
			event: Event{
				Timestamp: time.Date(2025, 1, 23, 10, 30, 0, 0, time.UTC),
				EventType: "QUERY",
				SessionID: "sess_456",
				Success:   false,
				Error:     "connection timeout",
			},
			contains: []string{"ERROR:", "connection timeout"},
		},
		{
			name: "failure event without error",
			event: Event{
				Timestamp: time.Date(2025, 1, 23, 10, 30, 0, 0, time.UTC),
				EventType: "AUTH",
				SessionID: "sess_789",
				Success:   false,
			},
			contains: []string{"FAILURE"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := tc.event.ToLogLine()
			for _, want := range tc.contains {
				if !strings.Contains(line, want) {
					t.Errorf("ToLogLine() = %q, want to contain %q", line, want)
				}
			}
		})
	}
}

func TestEvent_ToJSON(t *testing.T) {
	event := Event{
		Timestamp: time.Date(2025, 1, 23, 10, 30, 0, 0, time.UTC),
		EventType: "QUERY",
		SessionID: "sess_123",
		Tier:      "local",
		Query:     "test query",
		Success:   true,
		Metadata:  map[string]string{"key": "value"},
	}

	jsonStr, err := event.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed Event
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v", err)
	}

	if parsed.EventType != event.EventType {
		t.Errorf("Parsed EventType = %q, want %q", parsed.EventType, event.EventType)
	}
	if parsed.SessionID != event.SessionID {
		t.Errorf("Parsed SessionID = %q, want %q", parsed.SessionID, event.SessionID)
	}
}

// =============================================================================
// PATTERN REDACTOR TESTS
// =============================================================================

func TestPatternRedactor_Redact(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		replace string
		input   string
		want    string
	}{
		{
			name:    "redact OpenAI key",
			pattern: `sk-[a-zA-Z0-9]{20,}`,
			replace: "[OPENAI_KEY_REDACTED]",
			input:   "Using API key: sk-abcdefghijklmnopqrstuvwxyz1234567890",
			want:    "Using API key: [OPENAI_KEY_REDACTED]",
		},
		{
			name:    "redact password",
			pattern: `(?i)(password|passwd|pwd)\s*[=:]\s*\S+`,
			replace: "[PASSWORD_REDACTED]",
			input:   "password=secretvalue123",
			want:    "[PASSWORD_REDACTED]",
		},
		{
			name:    "no match",
			pattern: `sk-[a-zA-Z0-9]{20,}`,
			replace: "[REDACTED]",
			input:   "No sensitive data here",
			want:    "No sensitive data here",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			redactor := NewPatternRedactor(tc.name, regexp.MustCompile(tc.pattern), tc.replace)
			got := redactor.Redact(tc.input)
			if got != tc.want {
				t.Errorf("Redact() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPatternRedactor_Name(t *testing.T) {
	name := "TestRedactor"
	redactor := NewPatternRedactor(name, regexp.MustCompile("test"), "replaced")
	if got := redactor.Name(); got != name {
		t.Errorf("Name() = %q, want %q", got, name)
	}
}

// =============================================================================
// REDACT SECRETS TESTS
// =============================================================================

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OpenAI key",
			input: "API key is sk-1234567890abcdefghijklmnopqrstuvwxyz",
			want:  "API key is [OPENAI_KEY_REDACTED]",
		},
		{
			name:  "OpenRouter key",
			input: "sk-or-v1-" + strings.Repeat("a", 64),
			want:  "[OPENROUTER_KEY_REDACTED]",
		},
		{
			name:  "GitHub token",
			input: "Token: ghp_" + strings.Repeat("a", 36),
			want:  "Token: [GITHUB_TOKEN_REDACTED]",
		},
		{
			name:  "Bearer token",
			input: "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			want:  "Authorization: Bearer [TOKEN_REDACTED]",
		},
		{
			name:  "Anthropic key",
			input: "sk-ant-abc123def456ghi789jkl",
			want:  "[ANTHROPIC_KEY_REDACTED]",
		},
		{
			name:  "no sensitive data",
			input: "This is a normal log message",
			want:  "This is a normal log message",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSecrets(tc.input)
			if got != tc.want {
				t.Errorf("RedactSecrets() = %q, want %q", got, tc.want)
			}
		})
	}
}

// =============================================================================
// LOGGER TESTS
// =============================================================================

func TestNewLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	if !logger.IsEnabled() {
		t.Error("New logger should be enabled")
	}

	if logger.Path() != logPath {
		t.Errorf("Path() = %q, want %q", logger.Path(), logPath)
	}
}

func TestLogger_Log(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Disable halt on failure for testing
	logger.SetHaltOnFailure(false)

	event := Event{
		Timestamp: time.Now(),
		EventType: "TEST_EVENT",
		SessionID: "test_session",
		Success:   true,
		Metadata:  map[string]string{"test": "value"},
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	// Verify log was written
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "TEST_EVENT") {
		t.Error("Log file should contain event type")
	}
	if !strings.Contains(content, "test_session") {
		t.Error("Log file should contain session ID")
	}
}

func TestLogger_LogQuery(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogQuery("sess_123", "local", "What is AI?", 100, 0.05, true)
	if err != nil {
		t.Fatalf("LogQuery() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !strings.Contains(content, "QUERY") {
		t.Error("Log should contain QUERY event type")
	}
}

func TestLogger_LogEvent(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogEvent("sess_123", "CUSTOM_EVENT", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("LogEvent() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !strings.Contains(content, "CUSTOM_EVENT") {
		t.Error("Log should contain custom event type")
	}
}

func TestLogger_Redact(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	// Log with sensitive data
	event := Event{
		Timestamp: time.Now(),
		EventType: "TEST",
		SessionID: "sess_123",
		Query:     "API key is sk-1234567890abcdefghijklmnopqrstuvwxyz",
		Success:   true,
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	content := string(data)
	if strings.Contains(content, "sk-1234567890") {
		t.Error("Log should NOT contain raw API key")
	}
	if !strings.Contains(content, "[OPENAI_KEY_REDACTED]") {
		t.Error("Log should contain redacted placeholder")
	}
}

func TestLogger_AddRedactor(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Add custom redactor
	customRedactor := NewPatternRedactor("custom", regexp.MustCompile(`SECRET_\d+`), "[CUSTOM_REDACTED]")
	logger.AddRedactor(customRedactor)

	// Test redaction
	result := logger.Redact("My SECRET_12345 data")
	if !strings.Contains(result, "[CUSTOM_REDACTED]") {
		t.Error("Custom redactor should work")
	}
}

func TestLogger_SetEnabled(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	logger.SetEnabled(false)
	if logger.IsEnabled() {
		t.Error("Logger should be disabled")
	}

	logger.SetEnabled(true)
	if !logger.IsEnabled() {
		t.Error("Logger should be enabled")
	}
}

// =============================================================================
// AU-5 AUDIT FAILURE RESPONSE TESTS
// =============================================================================

func TestLogger_SetOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	logger.SetOnFailure(func(err error) {
		// Callback would be called on failure
		_ = err
	})

	// Just verify setting callback doesn't panic
	// Actual failure testing is complex due to file system interactions
}

func TestLogger_CircuitBreaker(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Set low threshold for testing
	logger.SetCircuitBreakerThreshold(2)

	// Verify initial state
	if logger.IsCircuitBreakerOpen() {
		t.Error("Circuit breaker should be closed initially")
	}

	if logger.GetFailureCount() != 0 {
		t.Error("Failure count should be 0 initially")
	}
}

func TestLogger_ResetAuditFailure(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Reset should not panic on fresh logger
	logger.ResetAuditFailure()

	if logger.HasAuditFailed() {
		t.Error("HasAuditFailed should be false after reset")
	}
}

func TestLogger_SetHaltOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Default should be true
	if !logger.IsHaltOnFailure() {
		t.Error("HaltOnFailure should be true by default")
	}

	logger.SetHaltOnFailure(false)
	if logger.IsHaltOnFailure() {
		t.Error("HaltOnFailure should be false after setting")
	}
}

// =============================================================================
// ROTATION TESTS
// =============================================================================

func TestLogger_Rotate(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	// Write something first
	logger.LogEvent("sess_123", "PRE_ROTATE", nil)

	// Rotate
	if err := logger.Rotate(); err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}

	// Write after rotation
	logger.LogEvent("sess_456", "POST_ROTATE", nil)

	// Verify new file has new content
	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !strings.Contains(content, "POST_ROTATE") {
		t.Error("New log file should contain post-rotation content")
	}
	if strings.Contains(content, "PRE_ROTATE") {
		t.Error("New log file should NOT contain pre-rotation content")
	}
}

func TestLogger_SetMaxSize(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Set small max size
	logger.SetMaxSize(1024)

	// This just verifies no panic - actual rotation is tested elsewhere
}

// =============================================================================
// CAPACITY TESTS
// =============================================================================

func TestLogger_CheckCapacity(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	usedMB, totalMB, err := logger.CheckCapacity()
	if err != nil {
		t.Fatalf("CheckCapacity() error = %v", err)
	}

	if usedMB < 0 {
		t.Error("UsedMB should be non-negative")
	}
	if totalMB <= 0 {
		t.Error("TotalMB should be positive")
	}
}

func TestLogger_SetCapacityThresholds(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Set custom thresholds
	logger.SetCapacityThresholds(100, 200)
	// This just verifies no panic
}

// =============================================================================
// INTEGRITY TESTS
// =============================================================================

func TestLogger_VerifyIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Verify empty log
	if err := logger.VerifyIntegrity(); err != nil {
		t.Fatalf("VerifyIntegrity() on empty log error = %v", err)
	}
}

// =============================================================================
// SYNC AND CLOSE TESTS
// =============================================================================

func TestLogger_Sync(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	// Write and sync
	logger.LogEvent("sess_123", "TEST", nil)
	if err := logger.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Close again should not panic
	if err := logger.Close(); err != nil {
		t.Fatalf("Second Close() error = %v", err)
	}
}

// =============================================================================
// DEFAULT PATH TESTS
// =============================================================================

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Error("DefaultPath() should not be empty")
	}
	if !strings.HasSuffix(path, "audit.log") {
		t.Errorf("DefaultPath() = %q, should end with audit.log", path)
	}
}

// =============================================================================
// TRUNCATE QUERY TESTS
// =============================================================================

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short query", "Hello", 10, "Hello"},
		{"exact length", "12345", 5, "12345"},
		{"needs truncation", "This is a very long query", 10, "This is..."},
		{"with newlines", "Line1\nLine2\nLine3", 20, "Line1 Line2 Line3"},
		{"empty", "", 10, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateQuery(tc.input, tc.maxLen)
			if len(got) > tc.maxLen {
				t.Errorf("truncateQuery() length = %d, want <= %d", len(got), tc.maxLen)
			}
		})
	}
}

// =============================================================================
// GLOBAL LOGGER TESTS
// =============================================================================

func TestGlobalLogger(t *testing.T) {
	logger := GlobalLogger()
	if logger == nil {
		t.Fatal("GlobalLogger() returned nil")
	}

	// Second call should return same instance
	logger2 := GlobalLogger()
	if logger != logger2 {
		t.Error("GlobalLogger() should return same instance")
	}
}

func TestSetGlobalLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "custom_audit.log")

	customLogger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer customLogger.Close()

	// Save original
	original := GlobalLogger()

	// Set custom logger
	SetGlobalLogger(customLogger)
	defer SetGlobalLogger(original)

	// Verify
	if GlobalLogger().Path() != logPath {
		t.Error("SetGlobalLogger() should update global logger")
	}
}

// =============================================================================
// GLOBAL LOG FUNCTIONS
// =============================================================================

func TestLog_Global(t *testing.T) {
	// Create temp logger
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "global_test.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	// Save and restore original
	original := GlobalLogger()
	SetGlobalLogger(logger)
	defer SetGlobalLogger(original)

	// Test global Log function
	event := Event{
		Timestamp: time.Now(),
		EventType: "GLOBAL_TEST",
		SessionID: "test",
		Success:   true,
	}

	if err := Log(event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}
}

func TestLogQuery_Global(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "global_query.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	original := GlobalLogger()
	SetGlobalLogger(logger)
	defer SetGlobalLogger(original)

	if err := LogQuery("sess_123", "local", "test", 100, 0.01, true); err != nil {
		t.Fatalf("LogQuery() error = %v", err)
	}
}

func TestLogEvent_Global(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "global_event.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	original := GlobalLogger()
	SetGlobalLogger(logger)
	defer SetGlobalLogger(original)

	if err := LogEvent("sess_123", "CUSTOM", map[string]string{"key": "val"}); err != nil {
		t.Fatalf("LogEvent() error = %v", err)
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestLogger_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "concurrent.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				event := Event{
					Timestamp: time.Now(),
					EventType: "CONCURRENT_TEST",
					SessionID: "sess_" + string(rune('A'+id)),
					Success:   true,
				}
				logger.Log(event)
			}
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// ERROR TESTS
// =============================================================================

func TestErrAuditSystemFailed(t *testing.T) {
	if ErrAuditSystemFailed == nil {
		t.Error("ErrAuditSystemFailed should not be nil")
	}
	if !strings.Contains(ErrAuditSystemFailed.Error(), "AU-5") {
		t.Error("ErrAuditSystemFailed should reference AU-5")
	}
}

func TestErrAuditCircuitBreakerOpen(t *testing.T) {
	if ErrAuditCircuitBreakerOpen == nil {
		t.Error("ErrAuditCircuitBreakerOpen should not be nil")
	}
	if !errors.Is(ErrAuditCircuitBreakerOpen, ErrAuditCircuitBreakerOpen) {
		t.Error("Error should match itself")
	}
}

// =============================================================================
// CONVENIENCE METHOD TESTS
// =============================================================================

func TestLogger_LogStartup(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "startup.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogStartup("sess_123", map[string]string{"version": "1.0"})
	if err != nil {
		t.Fatalf("LogStartup() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "STARTUP") {
		t.Error("Log should contain STARTUP event")
	}
}

func TestLogger_LogShutdown(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "shutdown.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogShutdown("sess_123", nil)
	if err != nil {
		t.Fatalf("LogShutdown() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "SHUTDOWN") {
		t.Error("Log should contain SHUTDOWN event")
	}
}

func TestLogger_LogBannerAck(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "banner.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogBannerAck("sess_123")
	if err != nil {
		t.Fatalf("LogBannerAck() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "BANNER_ACK") {
		t.Error("Log should contain BANNER_ACK event")
	}
}

func TestLogger_LogSessionStart(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "session.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogSessionStart("sess_123", map[string]string{"user": "test"})
	if err != nil {
		t.Fatalf("LogSessionStart() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "SESSION_START") {
		t.Error("Log should contain SESSION_START event")
	}
}

func TestLogger_LogSessionEnd(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "session_end.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogSessionEnd("sess_123", nil)
	if err != nil {
		t.Fatalf("LogSessionEnd() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "SESSION_END") {
		t.Error("Log should contain SESSION_END event")
	}
}

func TestLogger_LogTimeout(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "timeout.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogTimeout("sess_123")
	if err != nil {
		t.Fatalf("LogTimeout() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "SESSION_TIMEOUT") {
		t.Error("Log should contain SESSION_TIMEOUT event")
	}
}

func TestLogger_LogQueryWithError(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "query_error.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()
	logger.SetHaltOnFailure(false)

	err = logger.LogQueryWithError("sess_123", "cloud", "test query", 0, 0, "connection failed")
	if err != nil {
		t.Fatalf("LogQueryWithError() error = %v", err)
	}

	data, _ := os.ReadFile(logPath)
	content := string(data)
	if !strings.Contains(content, "ERROR") {
		t.Error("Log should contain ERROR status")
	}
	if !strings.Contains(content, "connection failed") {
		t.Error("Log should contain error message")
	}
}
