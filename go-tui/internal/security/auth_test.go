// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file contains tests for NIST 800-53 IA-2 compliance:
// - Session management
// - Concurrent access safety
// - Session expiry
package security

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// SESSION TESTS
// =============================================================================

// TestSession_ConcurrentRefresh tests that concurrent calls to Refresh and IsExpired
// do not cause race conditions or panics.
func TestSession_ConcurrentRefresh(t *testing.T) {
	// Create a session with a long expiry
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_session_1",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.Refresh()
			_ = session.IsExpired()
		}()
	}
	wg.Wait()
	// Should not panic or have race
}

// TestSession_ConcurrentIsValid tests concurrent IsValid calls.
func TestSession_ConcurrentIsValid(t *testing.T) {
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_session_2",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = session.IsValid()
			_ = session.TimeRemaining()
		}()
	}
	wg.Wait()
}

// TestSession_ConcurrentReadWrite tests concurrent read and write operations.
func TestSession_ConcurrentReadWrite(t *testing.T) {
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_session_3",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.Refresh()
		}()
	}

	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = session.IsExpired()
			_ = session.IsValid()
			_ = session.TimeRemaining()
		}()
	}

	wg.Wait()
}

// =============================================================================
// AUTH MANAGER SESSION EXPIRY TESTS
// =============================================================================

// TestAuthManager_SessionExpiry tests that sessions expire correctly.
func TestAuthManager_SessionExpiry(t *testing.T) {
	// Create AuthManager with very short session timeout using existing WithSessionDuration
	mgr := NewAuthManager(WithSessionDuration(100 * time.Millisecond))

	// Create a session manually since we need to control timing
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "auth_test_expiry",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(100 * time.Millisecond),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	// Should be valid initially
	require.False(t, session.IsExpired(), "Session should not be expired initially")

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)
	require.True(t, session.IsExpired(), "Session should be expired after timeout")

	// Test with mgr - we need a way to create a session
	// For this, use the internal mechanism
	mgr.mu.Lock()
	session2 := &AuthSession{
		UserID:          "user2",
		SessionID:       "auth_test_expiry_2",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(50 * time.Millisecond),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}
	mgr.sessions[session2.SessionID] = session2
	mgr.userSessions[session2.UserID] = session2.SessionID
	mgr.mu.Unlock()

	// Should be retrievable initially
	retrieved := mgr.GetSession(session2.SessionID)
	require.NotNil(t, retrieved, "Session should be retrievable before expiry")

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Should not be retrievable after expiry
	retrieved = mgr.GetSession(session2.SessionID)
	require.Nil(t, retrieved, "Session should not be retrievable after expiry")
}

// TestAuthManager_MaxSessionDuration tests that sessions cannot exceed maximum duration.
func TestAuthManager_MaxSessionDuration(t *testing.T) {
	// Create a session that was authenticated more than MaxAuthSessionDuration ago
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "auth_test_max_duration",
		AuthenticatedAt: time.Now().Add(-13 * time.Hour), // 13 hours ago (exceeds 12 hour max)
		ExpiresAt:       time.Now().Add(1 * time.Hour),   // Still has explicit time left
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	// Should be expired due to absolute session timeout
	require.True(t, session.IsExpired(), "Session should be expired due to max session duration")
}

// TestAuthManager_ConcurrentSessions tests concurrent session operations.
func TestAuthManager_ConcurrentSessions(t *testing.T) {
	mgr := NewAuthManager()

	var wg sync.WaitGroup

	// Create sessions concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID, err := generateSessionID()
			if err != nil {
				t.Errorf("Failed to generate session ID: %v", err)
				return
			}
			session := &AuthSession{
				UserID:          "user" + string(rune('A'+id%26)),
				SessionID:       sessionID,
				AuthenticatedAt: time.Now(),
				ExpiresAt:       time.Now().Add(30 * time.Minute),
				AuthMethod:      AuthMethodAPIKey,
				LastActivity:    time.Now(),
			}
			mgr.mu.Lock()
			mgr.sessions[session.SessionID] = session
			mgr.userSessions[session.UserID] = session.SessionID
			mgr.mu.Unlock()
		}(i)
	}

	// Read sessions concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.GetStats()
			_ = mgr.ListSessions()
		}()
	}

	wg.Wait()
}

// TestAuthManager_SessionRotation tests session ID rotation.
func TestAuthManager_SessionRotation(t *testing.T) {
	mgr := NewAuthManager()

	// Create an initial session
	oldSessionID, err := generateSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       oldSessionID,
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	mgr.mu.Lock()
	mgr.sessions[oldSessionID] = session
	mgr.userSessions[session.UserID] = oldSessionID
	mgr.mu.Unlock()

	// Rotate session ID
	newSessionID, err := mgr.RotateSessionID(oldSessionID, "privilege_escalation")
	require.NoError(t, err)
	require.NotEqual(t, oldSessionID, newSessionID)

	// Old session ID should not work
	oldSession := mgr.GetSession(oldSessionID)
	require.Nil(t, oldSession)

	// New session ID should work
	newSession := mgr.GetSession(newSessionID)
	require.NotNil(t, newSession)
	require.Equal(t, "user1", newSession.UserID)
}

// TestAuthManager_Logout tests session logout.
func TestAuthManager_Logout(t *testing.T) {
	mgr := NewAuthManager()

	// Create a session
	sessionID, err := generateSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       sessionID,
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	mgr.mu.Lock()
	mgr.sessions[sessionID] = session
	mgr.userSessions[session.UserID] = sessionID
	mgr.mu.Unlock()

	// Verify session exists
	require.NotNil(t, mgr.GetSession(sessionID))

	// Logout
	mgr.Logout(sessionID)

	// Verify session is gone
	require.Nil(t, mgr.GetSession(sessionID))
	require.Nil(t, mgr.GetUserSession("user1"))
}

// TestAuthManager_LogoutUser tests user logout.
func TestAuthManager_LogoutUser(t *testing.T) {
	mgr := NewAuthManager()

	// Create a session
	sessionID, err := generateSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       sessionID,
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	mgr.mu.Lock()
	mgr.sessions[sessionID] = session
	mgr.userSessions[session.UserID] = sessionID
	mgr.mu.Unlock()

	// Verify session exists
	require.NotNil(t, mgr.GetUserSession("user1"))

	// Logout user
	mgr.LogoutUser("user1")

	// Verify session is gone
	require.Nil(t, mgr.GetSession(sessionID))
	require.Nil(t, mgr.GetUserSession("user1"))
}

// TestAuthManager_Cleanup tests expired session cleanup.
func TestAuthManager_Cleanup(t *testing.T) {
	mgr := NewAuthManager()

	// Create an expired session
	expiredSessionID, err := generateSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}
	expiredSession := &AuthSession{
		UserID:          "expired_user",
		SessionID:       expiredSessionID,
		AuthenticatedAt: time.Now().Add(-1 * time.Hour),
		ExpiresAt:       time.Now().Add(-30 * time.Minute), // Already expired
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now().Add(-1 * time.Hour),
	}

	// Create a valid session
	validSessionID, err := generateSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}
	validSession := &AuthSession{
		UserID:          "valid_user",
		SessionID:       validSessionID,
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(30 * time.Minute),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now(),
	}

	mgr.mu.Lock()
	mgr.sessions[expiredSessionID] = expiredSession
	mgr.userSessions[expiredSession.UserID] = expiredSessionID
	mgr.sessions[validSessionID] = validSession
	mgr.userSessions[validSession.UserID] = validSessionID
	mgr.mu.Unlock()

	// Run cleanup
	cleaned := mgr.Cleanup()
	require.Equal(t, 1, cleaned, "Should have cleaned 1 expired session")

	// Verify expired session is gone
	require.Nil(t, mgr.GetSession(expiredSessionID))

	// Verify valid session still exists
	require.NotNil(t, mgr.GetSession(validSessionID))
}

// =============================================================================
// API KEY VALIDATION TESTS
// =============================================================================

// TestValidateAPIKeyFormat tests API key format validation.
func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "Valid OpenRouter key",
			key:      "sk-or-v1-1234567890123456789012345678901234567890123456789012345678901234",
			expected: true,
		},
		{
			name:     "Valid OpenAI key",
			key:      "sk-1234567890123456789012345678901234567890",
			expected: true,
		},
		{
			name:     "Valid Anthropic key",
			key:      "sk-ant-1234567890123456789012345678901234567890",
			expected: true,
		},
		{
			name:     "Too short key",
			key:      "sk-short",
			expected: false,
		},
		{
			name:     "Empty key",
			key:      "",
			expected: false,
		},
		{
			name:     "Generic valid key",
			key:      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
			expected: true,
		},
		{
			name:     "Key with invalid characters (no sk- prefix)",
			key:      "@#$%^&*()!@#$%^&*()!@#$%^&*()",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAPIKeyFormat(tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// SESSION ID GENERATION TESTS
// =============================================================================

// TestGenerateSessionID tests session ID generation uniqueness.
func TestGenerateSessionID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate many session IDs and ensure uniqueness
	for i := 0; i < 1000; i++ {
		id, err := generateSessionID()
		require.NoError(t, err, "Failed to generate session ID")

		// Should have correct prefix
		require.True(t, len(id) > len(AuthSessionIDPrefix))
		require.Equal(t, AuthSessionIDPrefix, id[:len(AuthSessionIDPrefix)])

		// Should be unique
		require.False(t, ids[id], "Duplicate session ID generated")
		ids[id] = true
	}
}

// TestGenerateSessionID_Concurrent tests concurrent session ID generation.
func TestGenerateSessionID_Concurrent(t *testing.T) {
	var mu sync.Mutex
	ids := make(map[string]bool)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := generateSessionID()
			if err != nil {
				t.Errorf("Failed to generate session ID: %v", err)
				return
			}

			mu.Lock()
			require.False(t, ids[id], "Duplicate session ID generated concurrently")
			ids[id] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
}

// =============================================================================
// AUTH STATS TESTS
// =============================================================================

// TestAuthManager_GetStats tests statistics retrieval.
func TestAuthManager_GetStats(t *testing.T) {
	mgr := NewAuthManager(WithSessionDuration(1 * time.Hour))

	// Create some sessions
	for i := 0; i < 5; i++ {
		sessionID, err := generateSessionID()
		if err != nil {
			t.Fatalf("Failed to generate session ID: %v", err)
		}
		session := &AuthSession{
			UserID:          "user" + string(rune('A'+i)),
			SessionID:       sessionID,
			AuthenticatedAt: time.Now(),
			ExpiresAt:       time.Now().Add(30 * time.Minute),
			AuthMethod:      AuthMethodAPIKey,
			LastActivity:    time.Now(),
		}
		mgr.mu.Lock()
		mgr.sessions[sessionID] = session
		mgr.userSessions[session.UserID] = sessionID
		mgr.mu.Unlock()
	}

	// Create some expired sessions
	for i := 0; i < 3; i++ {
		sessionID, err := generateSessionID()
		if err != nil {
			t.Fatalf("Failed to generate session ID: %v", err)
		}
		session := &AuthSession{
			UserID:          "expired" + string(rune('A'+i)),
			SessionID:       sessionID,
			AuthenticatedAt: time.Now().Add(-2 * time.Hour),
			ExpiresAt:       time.Now().Add(-1 * time.Hour), // Expired
			AuthMethod:      AuthMethodAPIKey,
			LastActivity:    time.Now().Add(-2 * time.Hour),
		}
		mgr.mu.Lock()
		mgr.sessions[sessionID] = session
		mgr.userSessions[session.UserID] = sessionID
		mgr.mu.Unlock()
	}

	stats := mgr.GetStats()
	require.Equal(t, 5, stats.ActiveSessions, "Should have 5 active sessions")
	require.Equal(t, 3, stats.ExpiredSessions, "Should have 3 expired sessions")
	require.Equal(t, 1*time.Hour, stats.SessionDuration)
}

// =============================================================================
// MFA TESTS
// =============================================================================

// TestAuthManager_MFAStatus tests MFA status.
func TestAuthManager_MFAStatus(t *testing.T) {
	// MFA disabled
	mgr1 := NewAuthManager(WithMFAEnabled(false))
	require.False(t, mgr1.IsMFARequired())
	require.Contains(t, mgr1.MFAStatus(), "not enabled")

	// MFA enabled
	mgr2 := NewAuthManager(WithMFAEnabled(true))
	require.True(t, mgr2.IsMFARequired())
	require.Contains(t, mgr2.MFAStatus(), "enabled")
}

// TestAuthManager_SetTOTPSecret tests TOTP secret management.
func TestAuthManager_SetTOTPSecret(t *testing.T) {
	mgr := NewAuthManager()

	// Should fail with empty user ID
	err := mgr.SetTOTPSecret("", "secret")
	require.Error(t, err)

	// Should fail with empty secret
	err = mgr.SetTOTPSecret("user1", "")
	require.Error(t, err)

	// Should succeed with valid inputs
	err = mgr.SetTOTPSecret("user1", "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)

	// Verify secret was stored
	mgr.mu.RLock()
	secret, exists := mgr.totpSecrets["user1"]
	mgr.mu.RUnlock()
	require.True(t, exists)
	require.Equal(t, "JBSWY3DPEHPK3PXP", secret)
}

// =============================================================================
// NIL SESSION TESTS
// =============================================================================

// TestAuthSession_NilSafety tests that nil session handling is safe.
func TestAuthSession_NilSafety(t *testing.T) {
	var session *AuthSession = nil

	// IsValid should handle nil gracefully
	require.False(t, session.IsValid())
}
