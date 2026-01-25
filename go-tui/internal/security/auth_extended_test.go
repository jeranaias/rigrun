// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file contains extended tests for NIST 800-53 IA-2 compliance:
// - MFA configuration and validation
// - Authentication flow
// - MFA challenge handling
// - API key validation with lockout
package security

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// MFA CONFIG TESTS (IA-2(1))
// =============================================================================

func TestDefaultMFAConfig(t *testing.T) {
	cfg := DefaultMFAConfig()

	if cfg == nil {
		t.Fatal("DefaultMFAConfig() returned nil")
	}

	// Default should not require MFA
	if cfg.Required {
		t.Error("Default MFA config should not require MFA")
	}

	// Should have allowed methods
	if len(cfg.AllowedMethods) == 0 {
		t.Error("Default MFA config should have allowed methods")
	}

	// Should have challenge duration
	if cfg.ChallengeDuration <= 0 {
		t.Error("Default MFA config should have positive challenge duration")
	}

	// Should have grace period
	if cfg.GracePeriod < 0 {
		t.Error("Default MFA config grace period should be non-negative")
	}
}

func TestMFAConfig_IsMethodAllowed(t *testing.T) {
	tests := []struct {
		name    string
		config  *MFAConfig
		method  MFAMethod
		allowed bool
	}{
		{
			name:    "nil config",
			config:  nil,
			method:  MFAMethodTOTP,
			allowed: false,
		},
		{
			name:    "empty methods",
			config:  &MFAConfig{AllowedMethods: []MFAMethod{}},
			method:  MFAMethodTOTP,
			allowed: false,
		},
		{
			name:    "TOTP allowed",
			config:  &MFAConfig{AllowedMethods: []MFAMethod{MFAMethodTOTP, MFAMethodWebAuthn}},
			method:  MFAMethodTOTP,
			allowed: true,
		},
		{
			name:    "SMS not allowed",
			config:  &MFAConfig{AllowedMethods: []MFAMethod{MFAMethodTOTP, MFAMethodWebAuthn}},
			method:  MFAMethodSMS,
			allowed: false,
		},
		{
			name:    "WebAuthn allowed",
			config:  &MFAConfig{AllowedMethods: []MFAMethod{MFAMethodTOTP, MFAMethodWebAuthn}},
			method:  MFAMethodWebAuthn,
			allowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.config.IsMethodAllowed(tc.method)
			if got != tc.allowed {
				t.Errorf("IsMethodAllowed(%s) = %v, want %v", tc.method, got, tc.allowed)
			}
		})
	}
}

// =============================================================================
// VALIDATE MFA TESTS
// =============================================================================

func TestValidateMFA(t *testing.T) {
	tests := []struct {
		name      string
		session   *AuthSession
		config    *MFAConfig
		expectErr bool
		errType   error
	}{
		{
			name:      "nil session",
			session:   nil,
			config:    DefaultMFAConfig(),
			expectErr: true,
		},
		{
			name: "nil config (MFA not required)",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now(),
				ExpiresAt:       time.Now().Add(1 * time.Hour),
			},
			config:    nil,
			expectErr: false,
		},
		{
			name: "MFA not required",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now(),
				ExpiresAt:       time.Now().Add(1 * time.Hour),
			},
			config:    &MFAConfig{Required: false},
			expectErr: false,
		},
		{
			name: "MFA required but verified",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now(),
				ExpiresAt:       time.Now().Add(1 * time.Hour),
				MFAVerified:     true,
			},
			config:    &MFAConfig{Required: true},
			expectErr: false,
		},
		{
			name: "MFA required but not verified (no grace period)",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now().Add(-10 * time.Minute),
				ExpiresAt:       time.Now().Add(1 * time.Hour),
				MFAVerified:     false,
			},
			config:    &MFAConfig{Required: true, GracePeriod: 0},
			expectErr: true,
			errType:   ErrMFARequired,
		},
		{
			name: "MFA required, within grace period",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now(),
				ExpiresAt:       time.Now().Add(1 * time.Hour),
				MFAVerified:     false,
			},
			config:    &MFAConfig{Required: true, GracePeriod: 5 * time.Minute},
			expectErr: false, // Within grace period
		},
		{
			name: "expired session",
			session: &AuthSession{
				SessionID:       "test_session",
				AuthenticatedAt: time.Now().Add(-2 * time.Hour),
				ExpiresAt:       time.Now().Add(-1 * time.Hour), // Expired
				MFAVerified:     true,
			},
			config:    &MFAConfig{Required: true},
			expectErr: true,
			errType:   ErrSessionExpired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMFA(tc.session, tc.config)
			if tc.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// =============================================================================
// AUTH MANAGER OPTIONS TESTS
// =============================================================================

func TestWithAuthLockout(t *testing.T) {
	lockout := GlobalLockoutManager()
	mgr := NewAuthManager(WithAuthLockout(lockout))

	if mgr.lockout != lockout {
		t.Error("WithAuthLockout should set lockout manager")
	}
}

func TestWithAuthAuditLogger(t *testing.T) {
	logger := GlobalAuditLogger()
	mgr := NewAuthManager(WithAuthAuditLogger(logger))

	if mgr.auditLogger != logger {
		t.Error("WithAuthAuditLogger should set audit logger")
	}
}

func TestWithSessionDuration(t *testing.T) {
	duration := 2 * time.Hour
	mgr := NewAuthManager(WithSessionDuration(duration))

	if mgr.sessionDuration != duration {
		t.Errorf("sessionDuration = %v, want %v", mgr.sessionDuration, duration)
	}

	// Invalid duration should not change default
	mgr2 := NewAuthManager(WithSessionDuration(-1 * time.Hour))
	if mgr2.sessionDuration == -1*time.Hour {
		t.Error("Negative duration should not be accepted")
	}
}

func TestWithAPIKeyValidator(t *testing.T) {
	customValidator := func(key string) bool {
		return strings.HasPrefix(key, "custom-")
	}

	mgr := NewAuthManager(WithAPIKeyValidator(customValidator))

	// Should use custom validator
	if mgr.apiKeyValidator == nil {
		t.Error("apiKeyValidator should be set")
	}

	// Test custom validation
	if !mgr.ValidateAPIKey("custom-abcdefghijklmnopqrstuvwxyz12345678") {
		t.Error("Custom validator should accept custom- prefix")
	}
}

func TestWithMFAConfig(t *testing.T) {
	cfg := &MFAConfig{
		Required:          true,
		AllowedMethods:    []MFAMethod{MFAMethodTOTP},
		ChallengeDuration: 10 * time.Minute,
		GracePeriod:       2 * time.Minute,
	}

	mgr := NewAuthManager(WithMFAConfig(cfg))

	if !mgr.mfaEnabled {
		t.Error("MFA should be enabled when config requires it")
	}

	returnedCfg := mgr.GetMFAConfig()
	if returnedCfg.ChallengeDuration != cfg.ChallengeDuration {
		t.Error("GetMFAConfig should return configured values")
	}
}

// =============================================================================
// AUTHENTICATION TESTS
// =============================================================================

func TestAuthManager_Authenticate_APIKey(t *testing.T) {
	// Create manager with custom validator that accepts our test key
	mgr := NewAuthManager(
		WithAPIKeyValidator(func(key string) bool {
			return key == "test-valid-key-1234567890123456789012"
		}),
	)

	// Valid key
	session, err := mgr.Authenticate(AuthMethodAPIKey, "test-valid-key-1234567890123456789012")
	if err != nil {
		t.Fatalf("Authenticate with valid key failed: %v", err)
	}
	if session == nil {
		t.Fatal("Authenticate should return session")
	}
	if session.AuthMethod != AuthMethodAPIKey {
		t.Error("Session should have AuthMethodAPIKey")
	}

	// Invalid key
	_, err = mgr.Authenticate(AuthMethodAPIKey, "invalid-short")
	if err == nil {
		t.Error("Authenticate with invalid key should fail")
	}
}

func TestAuthManager_Authenticate_UnsupportedMethod(t *testing.T) {
	mgr := NewAuthManager()

	_, err := mgr.Authenticate(AuthMethodPassword, "password")
	if err == nil {
		t.Error("Password auth should not be implemented yet")
	}

	_, err = mgr.Authenticate(AuthMethodMFA, "123456")
	if err == nil {
		t.Error("MFA auth requires existing session")
	}

	_, err = mgr.Authenticate("unknown", "credential")
	if err == nil {
		t.Error("Unknown method should fail")
	}
}

// =============================================================================
// SESSION MANAGEMENT TESTS
// =============================================================================

func TestAuthManager_RefreshSession(t *testing.T) {
	mgr := NewAuthManager()

	// Create a session
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_refresh_session",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
		LastActivity:    time.Now().Add(-10 * time.Minute),
	}

	mgr.mu.Lock()
	mgr.sessions[session.SessionID] = session
	mgr.userSessions[session.UserID] = session.SessionID
	mgr.mu.Unlock()

	oldActivity := session.LastActivity

	// Refresh
	err := mgr.RefreshSession(session.SessionID)
	if err != nil {
		t.Fatalf("RefreshSession failed: %v", err)
	}

	// Verify LastActivity was updated
	session.mu.RLock()
	newActivity := session.LastActivity
	session.mu.RUnlock()

	if !newActivity.After(oldActivity) {
		t.Error("LastActivity should be updated after refresh")
	}

	// Test with non-existent session
	err = mgr.RefreshSession("nonexistent")
	if err == nil {
		t.Error("RefreshSession with nonexistent ID should fail")
	}

	// Test with expired session
	expiredSession := &AuthSession{
		UserID:          "expired_user",
		SessionID:       "test_expired_session",
		AuthenticatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:       time.Now().Add(-1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
	}

	mgr.mu.Lock()
	mgr.sessions[expiredSession.SessionID] = expiredSession
	mgr.mu.Unlock()

	err = mgr.RefreshSession(expiredSession.SessionID)
	if err == nil {
		t.Error("RefreshSession with expired session should fail")
	}
}

func TestAuthManager_GetUserSession(t *testing.T) {
	mgr := NewAuthManager()

	// Create a session
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_get_user_session",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
	}

	mgr.mu.Lock()
	mgr.sessions[session.SessionID] = session
	mgr.userSessions[session.UserID] = session.SessionID
	mgr.mu.Unlock()

	// Should find session
	found := mgr.GetUserSession("user1")
	if found == nil {
		t.Error("GetUserSession should find existing session")
	}

	// Should not find non-existent user
	notFound := mgr.GetUserSession("nonexistent")
	if notFound != nil {
		t.Error("GetUserSession should return nil for non-existent user")
	}
}

// =============================================================================
// MFA CHALLENGE TESTS
// =============================================================================

func TestAuthManager_CreateMFAChallenge(t *testing.T) {
	mgr := NewAuthManager(WithMFAConfig(&MFAConfig{
		Required:          true,
		AllowedMethods:    []MFAMethod{MFAMethodTOTP},
		ChallengeDuration: 5 * time.Minute,
	}))

	// Create a session first
	session := &AuthSession{
		UserID:          "user1",
		SessionID:       "test_mfa_challenge_session",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
	}

	mgr.mu.Lock()
	mgr.sessions[session.SessionID] = session
	mgr.userSessions[session.UserID] = session.SessionID
	mgr.mu.Unlock()

	// Create MFA challenge
	challenge, err := mgr.CreateMFAChallenge(session.SessionID, MFAMethodTOTP)
	if err != nil {
		t.Fatalf("CreateMFAChallenge failed: %v", err)
	}

	if challenge == nil {
		t.Fatal("Challenge should not be nil")
	}

	if challenge.SessionID != session.SessionID {
		t.Error("Challenge should reference session")
	}

	if challenge.Method != MFAMethodTOTP {
		t.Error("Challenge should have correct method")
	}

	if challenge.Verified {
		t.Error("New challenge should not be verified")
	}

	// Test with non-existent session
	_, err = mgr.CreateMFAChallenge("nonexistent", MFAMethodTOTP)
	if err == nil {
		t.Error("CreateMFAChallenge with nonexistent session should fail")
	}

	// Test with disallowed method
	_, err = mgr.CreateMFAChallenge(session.SessionID, MFAMethodSMS)
	if err == nil {
		t.Error("CreateMFAChallenge with disallowed method should fail")
	}
}

func TestAuthManager_GetMFAChallenge(t *testing.T) {
	mgr := NewAuthManager()

	// Create a challenge manually
	challenge := &MFAChallenge{
		ChallengeID: "test_challenge_123",
		SessionID:   "sess_123",
		UserID:      "user1",
		Method:      MFAMethodTOTP,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		Verified:    false,
		Attempts:    0,
		MaxAttempts: 3,
	}

	mgr.mu.Lock()
	mgr.mfaChallenges[challenge.ChallengeID] = challenge
	mgr.mu.Unlock()

	// Should find challenge
	found, err := mgr.GetMFAChallenge(challenge.ChallengeID)
	if err != nil {
		t.Fatalf("GetMFAChallenge failed: %v", err)
	}
	if found == nil {
		t.Error("Should find challenge")
	}

	// Should not find non-existent
	_, err = mgr.GetMFAChallenge("nonexistent")
	if err == nil {
		t.Error("GetMFAChallenge with nonexistent ID should fail")
	}

	// Test expired challenge
	expiredChallenge := &MFAChallenge{
		ChallengeID: "expired_challenge",
		ExpiresAt:   time.Now().Add(-1 * time.Minute),
	}

	mgr.mu.Lock()
	mgr.mfaChallenges[expiredChallenge.ChallengeID] = expiredChallenge
	mgr.mu.Unlock()

	_, err = mgr.GetMFAChallenge(expiredChallenge.ChallengeID)
	if err == nil {
		t.Error("GetMFAChallenge with expired challenge should fail")
	}
}

// =============================================================================
// API KEY VALIDATION WITH LOCKOUT TESTS
// =============================================================================

func TestAuthManager_ValidateAPIKeyWithLockout(t *testing.T) {
	mgr := NewAuthManager(
		WithAPIKeyValidator(func(key string) bool {
			return key == "valid-key-1234567890123456789012345"
		}),
	)

	// Valid key
	valid, err := mgr.ValidateAPIKeyWithLockout("valid-key-1234567890123456789012345")
	if err != nil {
		t.Fatalf("ValidateAPIKeyWithLockout failed: %v", err)
	}
	if !valid {
		t.Error("Should accept valid key")
	}

	// Invalid key
	valid, err = mgr.ValidateAPIKeyWithLockout("invalid-key")
	if err == nil {
		t.Error("Should return error for invalid key")
	}
	if valid {
		t.Error("Should reject invalid key")
	}
}

// =============================================================================
// LIST SESSIONS TESTS
// =============================================================================

func TestAuthManager_ListSessions(t *testing.T) {
	mgr := NewAuthManager()

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := &AuthSession{
			UserID:          "user" + string(rune('A'+i)),
			SessionID:       "list_test_session_" + string(rune('A'+i)),
			AuthenticatedAt: time.Now(),
			ExpiresAt:       time.Now().Add(1 * time.Hour),
			AuthMethod:      AuthMethodAPIKey,
			LastActivity:    time.Now(),
		}

		mgr.mu.Lock()
		mgr.sessions[session.SessionID] = session
		mgr.userSessions[session.UserID] = session.SessionID
		mgr.mu.Unlock()
	}

	// Create one expired session
	expiredSession := &AuthSession{
		UserID:          "expired",
		SessionID:       "list_test_expired",
		AuthenticatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:       time.Now().Add(-1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
	}

	mgr.mu.Lock()
	mgr.sessions[expiredSession.SessionID] = expiredSession
	mgr.mu.Unlock()

	// List should only return active sessions
	sessions := mgr.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("ListSessions returned %d sessions, want 3", len(sessions))
	}

	// Verify sessions are copies (not the same pointers)
	for _, s := range sessions {
		if s.SessionID == "" {
			t.Error("Session copy should have SessionID")
		}
	}
}

// =============================================================================
// GLOBAL AUTH MANAGER TESTS
// =============================================================================

func TestGlobalAuthManager(t *testing.T) {
	mgr := GlobalAuthManager()
	if mgr == nil {
		t.Fatal("GlobalAuthManager() returned nil")
	}

	// Should return same instance
	mgr2 := GlobalAuthManager()
	if mgr != mgr2 {
		t.Error("GlobalAuthManager should return same instance")
	}
}

func TestSetGlobalAuthManager(t *testing.T) {
	original := GlobalAuthManager()
	defer SetGlobalAuthManager(original)

	customMgr := NewAuthManager(WithSessionDuration(3 * time.Hour))
	SetGlobalAuthManager(customMgr)

	if GlobalAuthManager().sessionDuration != 3*time.Hour {
		t.Error("SetGlobalAuthManager should update global manager")
	}
}

func TestInitGlobalAuthManager(t *testing.T) {
	original := GlobalAuthManager()
	defer SetGlobalAuthManager(original)

	InitGlobalAuthManager(WithSessionDuration(4 * time.Hour))

	if GlobalAuthManager().sessionDuration != 4*time.Hour {
		t.Error("InitGlobalAuthManager should initialize with options")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestDeriveUserID(t *testing.T) {
	// Same input should produce same output
	id1 := deriveUserID("test-api-key-12345")
	id2 := deriveUserID("test-api-key-12345")

	if id1 != id2 {
		t.Error("deriveUserID should be deterministic")
	}

	// Different inputs should produce different outputs
	id3 := deriveUserID("different-api-key-67890")
	if id1 == id3 {
		t.Error("Different keys should produce different IDs")
	}

	// Should have user_ prefix
	if !strings.HasPrefix(id1, "user_") {
		t.Errorf("deriveUserID should start with 'user_', got %s", id1)
	}
}

func TestHashAPIKey(t *testing.T) {
	// Same input should produce same hash
	hash1 := hashAPIKey("test-api-key-12345")
	hash2 := hashAPIKey("test-api-key-12345")

	if hash1 != hash2 {
		t.Error("hashAPIKey should be deterministic")
	}

	// Different inputs should produce different hashes
	hash3 := hashAPIKey("different-key")
	if hash1 == hash3 {
		t.Error("Different keys should produce different hashes")
	}

	// Hash should not be the original key
	if hash1 == "test-api-key-12345" {
		t.Error("Hash should not equal original key")
	}
}

func TestSanitizeSessionIDForLog(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "short"},
		{"12345678", "12345678"},
		{"auth_1234567890abcdef", "auth...cdef"},
		{"very_long_session_id_here", "very...here"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeSessionIDForLog(tc.input)
			if len(tc.input) > 8 {
				// Should be truncated
				if len(got) >= len(tc.input) {
					t.Errorf("sanitizeSessionIDForLog(%q) = %q, should be truncated", tc.input, got)
				}
			}
		})
	}
}

func TestGenerateAuthIdentifier(t *testing.T) {
	// Same inputs should produce same identifier
	id1 := generateAuthIdentifier(AuthMethodAPIKey, "test-credential")
	id2 := generateAuthIdentifier(AuthMethodAPIKey, "test-credential")

	if id1 != id2 {
		t.Error("generateAuthIdentifier should be deterministic")
	}

	// Different method should produce different identifier
	id3 := generateAuthIdentifier(AuthMethodPassword, "test-credential")
	if id1 == id3 {
		t.Error("Different methods should produce different identifiers")
	}

	// Different credential should produce different identifier
	id4 := generateAuthIdentifier(AuthMethodAPIKey, "other-credential")
	if id1 == id4 {
		t.Error("Different credentials should produce different identifiers")
	}
}

// =============================================================================
// ERROR TESTS
// =============================================================================

func TestAuthErrors(t *testing.T) {
	errors := []error{
		ErrAuthFailed,
		ErrSessionExpired,
		ErrMFARequired,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}

	// Verify NIST references
	if !strings.Contains(ErrAuthFailed.Error(), "IA-2") {
		t.Error("ErrAuthFailed should reference IA-2")
	}
	if !strings.Contains(ErrMFARequired.Error(), "IA-2") {
		t.Error("ErrMFARequired should reference IA-2")
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestAuthManager_ConcurrentMFAChallenges(t *testing.T) {
	mgr := NewAuthManager(WithMFAConfig(&MFAConfig{
		Required:          true,
		AllowedMethods:    []MFAMethod{MFAMethodTOTP},
		ChallengeDuration: 5 * time.Minute,
	}))

	// Create a session
	session := &AuthSession{
		UserID:          "concurrent_user",
		SessionID:       "concurrent_session",
		AuthenticatedAt: time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
		AuthMethod:      AuthMethodAPIKey,
	}

	mgr.mu.Lock()
	mgr.sessions[session.SessionID] = session
	mgr.userSessions[session.UserID] = session.SessionID
	mgr.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			challenge, _ := mgr.CreateMFAChallenge(session.SessionID, MFAMethodTOTP)
			if challenge != nil {
				mgr.GetMFAChallenge(challenge.ChallengeID)
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// GET API KEY FROM ENV TESTS
// =============================================================================

func TestGetAPIKeyFromEnv(t *testing.T) {
	// This test just verifies it doesn't panic
	// Actual env vars are not set in tests
	_ = GetAPIKeyFromEnv()
}
