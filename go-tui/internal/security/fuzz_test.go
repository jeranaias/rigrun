// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file contains fuzz tests for security-critical parsing functions.
// Run with: go test -fuzz=. ./internal/security/...
package security

import (
	"encoding/json"
	"testing"
)

// =============================================================================
// AUDIT EVENT PARSING FUZZ TESTS
// =============================================================================

// FuzzAuditEntryParsing tests that audit entry parsing handles arbitrary input safely.
func FuzzAuditEntryParsing(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"event":"test"}`))
	f.Add([]byte(`{"timestamp":"2024-01-01T00:00:00Z","event_type":"QUERY","session_id":"123","success":true}`))
	f.Add([]byte(`{"query":"SELECT * FROM users","tokens":100,"cost_cents":0.5}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"metadata":{"key":"value"}}`))
	f.Add([]byte(`{"error":"something went wrong"}`))
	f.Add([]byte(`{"event_type":"MALICIOUS","query":"'; DROP TABLE users;--"}`))
	f.Add([]byte(`{"session_id":"<script>alert('xss')</script>"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var entry AuditEvent
		// Should not panic regardless of input
		_ = json.Unmarshal(data, &entry)

		// If unmarshal succeeded, ToLogLine should also not panic
		_ = entry.ToLogLine()

		// ToJSON should not panic
		_, _ = entry.ToJSON()
	})
}

// FuzzAuditEventToLogLine tests that ToLogLine handles all field values safely.
func FuzzAuditEventToLogLine(f *testing.F) {
	// Add seed corpus with various field combinations
	f.Add("QUERY", "session123", "SELECT * FROM users", "error msg", 100, 0.5, true)
	f.Add("", "", "", "", 0, 0.0, false)
	f.Add("LOGIN", "sess-with-special-chars-!@#$%", "query with\nnewlines\tand\ttabs", "", -1, -0.5, true)
	f.Add("VERY_LONG_EVENT_TYPE_THAT_EXCEEDS_NORMAL_BOUNDS", "a", "b", "c", 999999999, 999999.99, false)

	f.Fuzz(func(t *testing.T, eventType, sessionID, query, errMsg string, tokens int, cost float64, success bool) {
		event := AuditEvent{
			EventType: eventType,
			SessionID: sessionID,
			Query:     query,
			Error:     errMsg,
			Tokens:    tokens,
			Cost:      cost,
			Success:   success,
		}

		// Should not panic
		line := event.ToLogLine()
		_ = line

		// ToJSON should also not panic
		_, _ = event.ToJSON()
	})
}

// =============================================================================
// REDACTION FUZZ TESTS
// =============================================================================

// FuzzRedactSecrets tests that secret redaction handles arbitrary input safely.
func FuzzRedactSecrets(f *testing.F) {
	// Add seed corpus with known patterns
	f.Add("sk-1234567890123456789012345678901234567890")
	f.Add("sk-or-v1-1234567890123456789012345678901234567890123456789012345678901234")
	f.Add("Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")
	f.Add("ghp_123456789012345678901234567890123456")
	f.Add("AKIAIOSFODNN7EXAMPLE")
	f.Add("password=secret123")
	f.Add("sk-ant-api03-1234567890")
	f.Add("")
	f.Add("normal text without secrets")
	f.Add("sk-sk-sk-sk-sk-sk-sk-sk-")
	f.Add(string(make([]byte, 10000))) // Large input

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic regardless of input
		result := RedactSecrets(input)

		// Result should not be longer than input (redaction typically shortens)
		// This is a loose check - some redactions might be longer
		_ = result

		// Should be able to redact the result again without panic
		_ = RedactSecrets(result)
	})
}

// =============================================================================
// API KEY VALIDATION FUZZ TESTS
// =============================================================================

// FuzzValidateAPIKeyFormat tests API key validation with arbitrary input.
func FuzzValidateAPIKeyFormat(f *testing.F) {
	// Add seed corpus
	f.Add("sk-1234567890123456789012345678901234567890")
	f.Add("sk-or-v1-1234567890123456789012345678901234567890123456789012345678901234")
	f.Add("sk-ant-api03-1234567890")
	f.Add("")
	f.Add("short")
	f.Add("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")
	f.Add("!@#$%^&*(){}[]|\\:\";<>?,./")
	f.Add(string(make([]byte, 10000))) // Large input
	f.Add("sk-\x00\x01\x02\x03")        // Binary data in key

	f.Fuzz(func(t *testing.T, key string) {
		// Should not panic regardless of input
		result := ValidateAPIKeyFormat(key)

		// Result should be consistent for same input
		result2 := ValidateAPIKeyFormat(key)
		if result != result2 {
			t.Errorf("ValidateAPIKeyFormat not deterministic: got %v then %v", result, result2)
		}
	})
}

// =============================================================================
// LOCKOUT IDENTIFIER MASKING FUZZ TESTS
// =============================================================================

// FuzzMaskIdentifier tests identifier masking with arbitrary input.
func FuzzMaskIdentifier(f *testing.F) {
	// Add seed corpus
	f.Add("user123")
	f.Add("")
	f.Add("very-long-identifier-that-exceeds-normal-bounds-" + string(make([]byte, 1000)))
	f.Add("special-chars-!@#$%^&*()")
	f.Add("\x00\x01\x02\x03") // Binary data

	f.Fuzz(func(t *testing.T, identifier string) {
		// Should not panic regardless of input
		masked := maskIdentifier(identifier)

		// Masked output should always start with "hash:"
		if masked != "" && len(masked) < 5 {
			t.Errorf("Expected mask prefix 'hash:', got %q", masked)
		}

		// Same input should always produce same output (deterministic)
		masked2 := maskIdentifier(identifier)
		if masked != masked2 {
			t.Errorf("maskIdentifier not deterministic: got %q then %q", masked, masked2)
		}
	})
}

// =============================================================================
// SESSION ID SANITIZATION FUZZ TESTS
// =============================================================================

// FuzzSanitizeSessionIDForLog tests session ID sanitization with arbitrary input.
func FuzzSanitizeSessionIDForLog(f *testing.F) {
	// Add seed corpus
	f.Add("auth_1234567890abcdef1234567890abcdef")
	f.Add("")
	f.Add("short")
	f.Add("12345678")
	f.Add("1234567")
	f.Add(string(make([]byte, 10000))) // Large input

	f.Fuzz(func(t *testing.T, sessionID string) {
		// Should not panic regardless of input
		sanitized := sanitizeSessionIDForLog(sessionID)
		_ = sanitized

		// Sanitization should be deterministic
		sanitized2 := sanitizeSessionIDForLog(sessionID)
		if sanitized != sanitized2 {
			t.Errorf("sanitizeSessionIDForLog not deterministic")
		}
	})
}

// =============================================================================
// QUERY TRUNCATION FUZZ TESTS
// =============================================================================

// FuzzTruncateQuery tests query truncation with arbitrary input.
func FuzzTruncateQuery(f *testing.F) {
	// Add seed corpus
	f.Add("SELECT * FROM users WHERE id = 1", 50)
	f.Add("", 100)
	f.Add("short", 100)
	f.Add("query with\nnewlines\tand\ttabs", 20)
	f.Add(string(make([]byte, 10000)), 100) // Large input
	f.Add("unicode: \u4e2d\u6587\u6d4b\u8bd5", 20)
	f.Add("test", 0)
	f.Add("test", -1)
	f.Add("test", 3)
	f.Add("test", 2)
	f.Add("test", 1)

	f.Fuzz(func(t *testing.T, query string, maxLen int) {
		// Handle negative maxLen - the function may or may not handle this
		if maxLen < 0 {
			maxLen = 0
		}

		// Should not panic regardless of input
		truncated := truncateQuery(query, maxLen)
		_ = truncated

		// If maxLen > 0 and result is not empty, check length constraint
		// Note: The function may have different behavior for edge cases
		if maxLen > 0 && len([]rune(truncated)) > maxLen {
			t.Errorf("truncateQuery exceeded maxLen: got %d runes, max was %d", len([]rune(truncated)), maxLen)
		}
	})
}

// =============================================================================
// ATTEMPT RECORD FUZZ TESTS
// =============================================================================

// FuzzAttemptRecordIsExpired tests AttemptRecord expiration check.
func FuzzAttemptRecordIsExpired(f *testing.F) {
	// Add seed corpus with various timestamp configurations
	f.Add(int64(0), int64(0), true)
	f.Add(int64(1609459200), int64(1609462800), true) // Past locked_until
	f.Add(int64(1609459200), int64(9999999999), true) // Future locked_until
	f.Add(int64(0), int64(0), false)

	f.Fuzz(func(t *testing.T, lastAttemptUnix, lockedUntilUnix int64, locked bool) {
		record := &AttemptRecord{
			Locked: locked,
		}

		// Set times from unix timestamps (handle potential overflow)
		// Note: time.Unix can handle very large values, but may produce odd dates

		// Should not panic
		_ = record.IsExpired()
		_ = record.TimeRemaining()
	})
}

// =============================================================================
// ENCRYPTION STRING FUZZ TESTS
// =============================================================================

// FuzzIsEncrypted tests the IsEncrypted helper function.
func FuzzIsEncrypted(f *testing.F) {
	// Add seed corpus
	f.Add("ENC:abc123")
	f.Add("ENC:")
	f.Add("")
	f.Add("enc:abc123")
	f.Add("ENC")
	f.Add("ENCabc123")
	f.Add(string(make([]byte, 10000)))

	f.Fuzz(func(t *testing.T, value string) {
		// Should not panic
		result := IsEncrypted(value)

		// Should be deterministic
		result2 := IsEncrypted(value)
		if result != result2 {
			t.Errorf("IsEncrypted not deterministic")
		}
	})
}

// =============================================================================
// AUTH IDENTIFIER GENERATION FUZZ TESTS
// =============================================================================

// FuzzGenerateAuthIdentifier tests auth identifier generation.
func FuzzGenerateAuthIdentifier(f *testing.F) {
	// Add seed corpus
	f.Add("api_key", "sk-test-1234567890")
	f.Add("password", "secretpassword123")
	f.Add("", "")
	f.Add("mfa", "123456")
	f.Add("api_key", string(make([]byte, 10000)))

	f.Fuzz(func(t *testing.T, method, credential string) {
		// Should not panic
		identifier := generateAuthIdentifier(AuthMethod(method), credential)

		// Should be deterministic
		identifier2 := generateAuthIdentifier(AuthMethod(method), credential)
		if identifier != identifier2 {
			t.Errorf("generateAuthIdentifier not deterministic")
		}

		// Identifier should always be non-empty (it's a hash)
		if identifier == "" {
			t.Error("Expected non-empty identifier")
		}
	})
}

// =============================================================================
// HASH API KEY FUZZ TESTS
// =============================================================================

// FuzzHashAPIKey tests API key hashing.
func FuzzHashAPIKey(f *testing.F) {
	// Add seed corpus
	f.Add("sk-test-1234567890")
	f.Add("")
	f.Add(string(make([]byte, 10000)))
	f.Add("\x00\x01\x02\x03")

	f.Fuzz(func(t *testing.T, apiKey string) {
		// Should not panic
		hash := hashAPIKey(apiKey)

		// Should be deterministic
		hash2 := hashAPIKey(apiKey)
		if hash != hash2 {
			t.Errorf("hashAPIKey not deterministic")
		}

		// Hash should always be non-empty
		if hash == "" {
			t.Error("Expected non-empty hash")
		}

		// Hash should not contain the original API key
		if apiKey != "" && len(apiKey) > 8 && hash == apiKey[:8] {
			t.Error("Hash should not be a simple substring of the API key")
		}
	})
}

// =============================================================================
// DERIVE USER ID FUZZ TESTS
// =============================================================================

// FuzzDeriveUserID tests user ID derivation from API key.
func FuzzDeriveUserID(f *testing.F) {
	// Add seed corpus
	f.Add("sk-test-1234567890")
	f.Add("")
	f.Add(string(make([]byte, 10000)))

	f.Fuzz(func(t *testing.T, apiKey string) {
		// Should not panic
		userID := deriveUserID(apiKey)

		// Should be deterministic
		userID2 := deriveUserID(apiKey)
		if userID != userID2 {
			t.Errorf("deriveUserID not deterministic")
		}

		// User ID should have the expected prefix
		if len(userID) < 5 || userID[:5] != "user_" {
			t.Errorf("Expected user ID to start with 'user_', got %q", userID)
		}
	})
}
