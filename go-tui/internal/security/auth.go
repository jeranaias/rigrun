// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 IA-2: Identification and Authentication.
//
// # DoD STIG Requirements
//
//   - IA-2: Uniquely identifies and authenticates organizational users
//   - IA-2(1): Multi-factor authentication for network access (placeholder)
//   - IA-2(8): Network access to privileged accounts requires replay-resistant auth
//   - IA-5: Authenticator management (API key validation)
//   - AU-3: Authentication events must be logged for audit compliance
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pquerna/otp/totp"
)

// =============================================================================
// IA-2 CONSTANTS
// =============================================================================

const (
	// DefaultSessionDuration is the default duration for auth sessions.
	// Maximum 12 hours per DoD STIG requirements.
	DefaultSessionDuration = 8 * time.Hour

	// MaxAuthSessionDuration is the absolute maximum for auth sessions: 12 hours.
	MaxAuthSessionDuration = 12 * time.Hour

	// AuthSessionIDPrefix is the prefix for auth session IDs.
	AuthSessionIDPrefix = "auth_"

	// APIKeyMinLength is the minimum length for a valid API key.
	// SECURITY: Increased to 32 for stronger key entropy (GSEC-2).
	APIKeyMinLength = 32

	// MFAChallengeIDPrefix is the prefix for MFA challenge IDs.
	MFAChallengeIDPrefix = "mfa_"

	// DefaultMFAChallengeDuration is the default duration for MFA challenges.
	DefaultMFAChallengeDuration = 5 * time.Minute
)

// AuthMethod represents the authentication method used.
type AuthMethod string

const (
	// AuthMethodAPIKey indicates API key authentication.
	AuthMethodAPIKey AuthMethod = "api_key"

	// AuthMethodPassword indicates password authentication (placeholder).
	AuthMethodPassword AuthMethod = "password"

	// AuthMethodMFA indicates multi-factor authentication (placeholder for IA-2(1)).
	AuthMethodMFA AuthMethod = "mfa"

	// AuthMethodCertificate indicates CAC/PKI certificate-based authentication (IA-2(12)).
	AuthMethodCertificate AuthMethod = "certificate"

	// AuthMethodNone indicates no authentication.
	AuthMethodNone AuthMethod = "none"
)

// =============================================================================
// MFA CONFIGURATION (IA-2(1))
// =============================================================================

// MFAMethod represents a supported MFA method.
type MFAMethod string

const (
	// MFAMethodTOTP indicates Time-based One-Time Password authentication.
	MFAMethodTOTP MFAMethod = "totp"

	// MFAMethodWebAuthn indicates WebAuthn/FIDO2 authentication (placeholder).
	MFAMethodWebAuthn MFAMethod = "webauthn"

	// MFAMethodSMS indicates SMS-based authentication (not recommended for IL5).
	MFAMethodSMS MFAMethod = "sms"
)

// MFAConfig holds MFA settings for IL5 compliance (IA-2(1)).
// This configuration determines whether MFA is required and which methods are allowed.
type MFAConfig struct {
	// Required indicates whether MFA is mandatory for all sessions.
	Required bool `json:"required"`

	// AllowedMethods specifies which MFA methods are permitted.
	// For IL5 compliance, TOTP and WebAuthn are recommended.
	AllowedMethods []MFAMethod `json:"allowed_methods"`

	// ChallengeDuration is how long an MFA challenge remains valid.
	ChallengeDuration time.Duration `json:"challenge_duration"`

	// GracePeriod allows a brief window for MFA completion after initial auth.
	// Set to 0 to require immediate MFA verification.
	GracePeriod time.Duration `json:"grace_period"`
}

// DefaultMFAConfig returns a default MFA configuration for IL5 compliance.
func DefaultMFAConfig() *MFAConfig {
	return &MFAConfig{
		Required:          false,
		AllowedMethods:    []MFAMethod{MFAMethodTOTP, MFAMethodWebAuthn},
		ChallengeDuration: DefaultMFAChallengeDuration,
		GracePeriod:       1 * time.Minute,
	}
}

// IsMethodAllowed checks if an MFA method is allowed by the configuration.
func (c *MFAConfig) IsMethodAllowed(method MFAMethod) bool {
	if c == nil || len(c.AllowedMethods) == 0 {
		return false
	}
	for _, allowed := range c.AllowedMethods {
		if allowed == method {
			return true
		}
	}
	return false
}

// MFAChallenge represents a pending MFA challenge (IA-2(1)).
// This tracks MFA verification attempts and their status.
type MFAChallenge struct {
	// ChallengeID is the unique identifier for this challenge.
	ChallengeID string `json:"challenge_id"`

	// SessionID is the session this challenge is associated with.
	SessionID string `json:"session_id"`

	// UserID is the user being challenged.
	UserID string `json:"user_id"`

	// Method is the MFA method being used for this challenge.
	Method MFAMethod `json:"method"`

	// CreatedAt is when the challenge was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the challenge expires.
	ExpiresAt time.Time `json:"expires_at"`

	// Verified indicates if the challenge has been successfully verified.
	Verified bool `json:"verified"`

	// Attempts tracks the number of verification attempts.
	Attempts int `json:"attempts"`

	// MaxAttempts is the maximum number of allowed attempts.
	MaxAttempts int `json:"max_attempts"`
}

// =============================================================================
// AUTH SESSION
// =============================================================================

// AuthSession represents an authenticated user session per IA-2.
// SECURITY: Session state protected by mutex.
type AuthSession struct {
	// mu protects concurrent access to session fields.
	mu sync.RWMutex `json:"-"`

	// UserID is the authenticated user identifier.
	UserID string `json:"user_id"`

	// SessionID is the unique session identifier.
	SessionID string `json:"session_id"`

	// AuthenticatedAt is when authentication occurred.
	AuthenticatedAt time.Time `json:"authenticated_at"`

	// ExpiresAt is when the session expires.
	ExpiresAt time.Time `json:"expires_at"`

	// AuthMethod indicates how the user authenticated.
	AuthMethod AuthMethod `json:"auth_method"`

	// MFAVerified indicates if MFA was completed (IA-2(1) placeholder).
	MFAVerified bool `json:"mfa_verified"`

	// APIKeyHash stores a hash of the API key used (for audit, not the key itself).
	APIKeyHash string `json:"api_key_hash,omitempty"`

	// CertificateInfo stores information from a client certificate (IA-2(12)).
	// Only populated when AuthMethod is AuthMethodCertificate.
	CertificateInfo *CertificateInfo `json:"certificate_info,omitempty"`

	// Metadata stores additional session information.
	Metadata map[string]string `json:"metadata,omitempty"`

	// LastActivity tracks the last activity timestamp.
	LastActivity time.Time `json:"last_activity"`
}

// IsExpired returns true if the session has expired.
// Checks both explicit expiration time and absolute session timeout.
// SECURITY: Uses RLock for thread-safe read access.
func (s *AuthSession) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	// Check explicit expiration
	if now.After(s.ExpiresAt) {
		return true
	}

	// Check absolute session timeout (12 hours max)
	if now.Sub(s.AuthenticatedAt) >= MaxAuthSessionDuration {
		return true
	}

	return false
}

// IsValid returns true if the session is valid and not expired.
// SECURITY: Uses RLock for thread-safe read access.
// Note: IsExpired() has its own locking, so we check SessionID separately.
func (s *AuthSession) IsValid() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	sessionID := s.SessionID
	s.mu.RUnlock()

	return sessionID != "" && !s.IsExpired()
}

// TimeRemaining returns the duration until the session expires.
// SECURITY: Uses RLock for thread-safe read access.
func (s *AuthSession) TimeRemaining() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	remaining := time.Until(s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Refresh updates the LastActivity timestamp.
// SECURITY: Uses Lock for thread-safe write access.
func (s *AuthSession) Refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastActivity = time.Now()
}

// =============================================================================
// AUTH MANAGER
// =============================================================================

// AuthManager manages authentication and sessions per IA-2.
type AuthManager struct {
	// lockout is the lockout manager for AC-7 integration.
	lockout *LockoutManager

	// sessions maps session IDs to auth sessions.
	sessions map[string]*AuthSession

	// userSessions maps user IDs to their current session IDs.
	userSessions map[string]string

	// totpSecrets maps user IDs to their TOTP secrets (IA-2(1)).
	totpSecrets map[string]string

	// mfaChallenges maps challenge IDs to active MFA challenges.
	mfaChallenges map[string]*MFAChallenge

	// sessionDuration is the default session duration.
	sessionDuration time.Duration

	// auditLogger is the audit logger for IA-2 events.
	auditLogger *AuditLogger

	// apiKeyValidator is an optional custom API key validator.
	apiKeyValidator func(key string) bool

	// mfaEnabled indicates if MFA is required (IA-2(1) placeholder).
	mfaEnabled bool

	// mfaConfig holds the MFA configuration for IL5 compliance.
	mfaConfig *MFAConfig

	// pkiConfig holds the PKI/CAC certificate authentication configuration (IA-2(12)).
	pkiConfig *PKIConfig

	// mu protects concurrent access.
	mu sync.RWMutex
}

// AuthManagerOption is a functional option for configuring AuthManager.
type AuthManagerOption func(*AuthManager)

// WithAuthLockout sets the lockout manager for AC-7 integration.
func WithAuthLockout(lockout *LockoutManager) AuthManagerOption {
	return func(a *AuthManager) {
		a.lockout = lockout
	}
}

// WithAuthAuditLogger sets the audit logger for IA-2 events.
func WithAuthAuditLogger(logger *AuditLogger) AuthManagerOption {
	return func(a *AuthManager) {
		a.auditLogger = logger
	}
}

// WithSessionDuration sets the default session duration.
func WithSessionDuration(d time.Duration) AuthManagerOption {
	return func(a *AuthManager) {
		if d > 0 {
			a.sessionDuration = d
		}
	}
}

// WithAPIKeyValidator sets a custom API key validator function.
func WithAPIKeyValidator(validator func(key string) bool) AuthManagerOption {
	return func(a *AuthManager) {
		a.apiKeyValidator = validator
	}
}

// WithMFAEnabled enables MFA requirement (IA-2(1) placeholder).
func WithMFAEnabled(enabled bool) AuthManagerOption {
	return func(a *AuthManager) {
		a.mfaEnabled = enabled
		if enabled && a.mfaConfig == nil {
			a.mfaConfig = DefaultMFAConfig()
			a.mfaConfig.Required = true
		}
	}
}

// WithMFAConfig sets the MFA configuration for IL5 compliance.
func WithMFAConfig(config *MFAConfig) AuthManagerOption {
	return func(a *AuthManager) {
		if config != nil {
			a.mfaConfig = config
			a.mfaEnabled = config.Required
		}
	}
}

// WithPKIConfig sets the PKI/CAC certificate authentication configuration (IA-2(12)).
func WithPKIConfig(config *PKIConfig) AuthManagerOption {
	return func(a *AuthManager) {
		a.pkiConfig = config
	}
}

// NewAuthManager creates a new AuthManager with the given options.
func NewAuthManager(opts ...AuthManagerOption) *AuthManager {
	am := &AuthManager{
		sessions:        make(map[string]*AuthSession),
		userSessions:    make(map[string]string),
		totpSecrets:     make(map[string]string),
		mfaChallenges:   make(map[string]*MFAChallenge),
		sessionDuration: DefaultSessionDuration,
		mfaConfig:       DefaultMFAConfig(),
	}

	// Apply options
	for _, opt := range opts {
		opt(am)
	}

	// Default lockout manager if not provided
	if am.lockout == nil {
		am.lockout = GlobalLockoutManager()
	}

	// Default audit logger if not provided
	if am.auditLogger == nil {
		am.auditLogger = GlobalAuditLogger()
	}

	return am
}

// =============================================================================
// AUTHENTICATION
// =============================================================================

// Authenticate performs authentication with the given method and credential.
// Returns an AuthSession on success, or an error on failure.
//
// This implements IA-2 identification and authentication requirements.
// Failed attempts are tracked for AC-7 lockout compliance.
func (a *AuthManager) Authenticate(method AuthMethod, credential string) (*AuthSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Generate identifier for lockout tracking
	identifier := generateAuthIdentifier(method, credential)

	// Check if locked out (AC-7 integration)
	if a.lockout != nil && a.lockout.IsLocked(identifier) {
		a.logEvent("AUTH_LOGIN", identifier, false, map[string]string{
			"method": string(method),
			"error":  "locked_out",
		})
		return nil, ErrLocked
	}

	var session *AuthSession
	var err error

	switch method {
	case AuthMethodAPIKey:
		session, err = a.authenticateAPIKey(credential)
	case AuthMethodPassword:
		// Placeholder for password authentication
		err = errors.New("password authentication not yet implemented")
	case AuthMethodMFA:
		// Placeholder for MFA - requires existing session
		err = errors.New("MFA authentication requires existing session")
	default:
		err = fmt.Errorf("unsupported authentication method: %s", method)
	}

	// Record attempt for lockout tracking
	if a.lockout != nil {
		if recordErr := a.lockout.RecordAttempt(identifier, err == nil); recordErr != nil {
			// If locked, return that error
			if errors.Is(recordErr, ErrLocked) {
				return nil, recordErr
			}
		}
	}

	if err != nil {
		a.logEvent("AUTH_LOGIN", identifier, false, map[string]string{
			"method": string(method),
			"error":  err.Error(),
		})
		return nil, err
	}

	// Check MFA requirement (IA-2(1) placeholder)
	if a.mfaEnabled && method != AuthMethodMFA {
		session.MFAVerified = false
		a.logEvent("AUTH_LOGIN", sanitizeSessionIDForLog(session.SessionID), true, map[string]string{
			"method":      string(method),
			"mfa_pending": "true",
		})
	} else {
		a.logEvent("AUTH_LOGIN", sanitizeSessionIDForLog(session.SessionID), true, map[string]string{
			"method": string(method),
		})
	}

	return session, nil
}

// authenticateAPIKey validates an API key and creates a session.
func (a *AuthManager) authenticateAPIKey(apiKey string) (*AuthSession, error) {
	// Validate API key format
	if len(apiKey) < APIKeyMinLength {
		return nil, errors.New("invalid API key: too short")
	}

	// Validate API key using custom validator or default
	isValid := false
	if a.apiKeyValidator != nil {
		isValid = a.apiKeyValidator(apiKey)
	} else {
		isValid = ValidateAPIKeyFormat(apiKey)
	}

	if !isValid {
		return nil, errors.New("invalid API key: validation failed")
	}

	// Create session
	now := time.Now()
	sessionID, err := generateSessionID()
	if err != nil {
		a.logEvent("AUTH_LOGIN", "unknown", false, map[string]string{
			"method": string(AuthMethodAPIKey),
			"error":  "session_id_generation_failed",
		})
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &AuthSession{
		UserID:          deriveUserID(apiKey),
		SessionID:       sessionID,
		AuthenticatedAt: now,
		ExpiresAt:       now.Add(a.sessionDuration),
		AuthMethod:      AuthMethodAPIKey,
		APIKeyHash:      hashAPIKey(apiKey),
		LastActivity:    now,
		Metadata:        make(map[string]string),
	}

	// Store session
	a.sessions[session.SessionID] = session
	a.userSessions[session.UserID] = session.SessionID

	return session, nil
}

// AuthenticateCertificate authenticates a user using a client certificate (IA-2(12)).
// This implements Personal Identity Verification using CAC/PKI certificates.
//
// The certificate is validated against the PKI configuration, and if valid,
// a session is created with the certificate information attached.
func (a *AuthManager) AuthenticateCertificate(cert *x509.Certificate) (*AuthSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if PKI authentication is enabled
	if a.pkiConfig == nil || !a.pkiConfig.Enabled {
		return nil, errors.New("certificate authentication is not enabled")
	}

	// Validate the certificate
	certInfo, err := ValidateClientCertificate(cert, a.pkiConfig)
	if err != nil {
		a.logEvent("AUTH_LOGIN", "certificate", false, map[string]string{
			"method": string(AuthMethodCertificate),
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("certificate validation failed: %w", err)
	}

	// Create session with certificate information
	now := time.Now()

	// Derive user ID from certificate (use EDIPI if available, otherwise subject)
	userID := certInfo.EDIPI
	if userID == "" {
		userID = "cert_" + certInfo.Fingerprint[:16]
	}

	sessionID, err := generateSessionID()
	if err != nil {
		a.logEvent("AUTH_LOGIN", "certificate", false, map[string]string{
			"method": string(AuthMethodCertificate),
			"error":  "session_id_generation_failed",
		})
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &AuthSession{
		UserID:          userID,
		SessionID:       sessionID,
		AuthenticatedAt: now,
		ExpiresAt:       now.Add(a.sessionDuration),
		AuthMethod:      AuthMethodCertificate,
		CertificateInfo: certInfo,
		LastActivity:    now,
		Metadata:        make(map[string]string),
	}

	// Add certificate metadata
	session.Metadata["cert_subject"] = certInfo.Subject
	session.Metadata["cert_issuer"] = certInfo.Issuer
	session.Metadata["cert_fingerprint"] = certInfo.Fingerprint
	if certInfo.EDIPI != "" {
		session.Metadata["edipi"] = certInfo.EDIPI
	}
	if certInfo.Email != "" {
		session.Metadata["email"] = certInfo.Email
	}

	// Store session
	a.sessions[session.SessionID] = session
	a.userSessions[session.UserID] = session.SessionID

	a.logEvent("AUTH_LOGIN", sanitizeSessionIDForLog(session.SessionID), true, map[string]string{
		"method":      string(AuthMethodCertificate),
		"user_id":     userID,
		"cert_issuer": certInfo.Issuer,
	})

	return session, nil
}

// ValidateAPIKey checks if an API key is valid.
// This is a public method for use in other packages.
func (a *AuthManager) ValidateAPIKey(key string) bool {
	// Check for lockout first
	identifier := generateAuthIdentifier(AuthMethodAPIKey, key)
	if a.lockout != nil && a.lockout.IsLocked(identifier) {
		return false
	}

	// Validate using custom validator or default
	if a.apiKeyValidator != nil {
		return a.apiKeyValidator(key)
	}

	return ValidateAPIKeyFormat(key)
}

// ValidateAPIKeyWithLockout validates an API key and records the attempt for lockout.
// Returns true if valid, false if invalid or locked out.
func (a *AuthManager) ValidateAPIKeyWithLockout(key string) (bool, error) {
	identifier := generateAuthIdentifier(AuthMethodAPIKey, key)

	// Check if locked out
	if a.lockout != nil && a.lockout.IsLocked(identifier) {
		return false, ErrLocked
	}

	// Validate the key
	isValid := a.ValidateAPIKey(key)

	// Record the attempt
	if a.lockout != nil {
		if err := a.lockout.RecordAttempt(identifier, isValid); err != nil {
			return false, err
		}
	}

	if !isValid {
		return false, errors.New("invalid API key")
	}

	return true, nil
}

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================

// GetSession retrieves a session by its ID.
func (a *AuthManager) GetSession(sessionID string) *AuthSession {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return nil
	}

	// Check if expired
	if session.IsExpired() {
		return nil
	}

	return session
}

// GetUserSession retrieves the current session for a user.
func (a *AuthManager) GetUserSession(userID string) *AuthSession {
	a.mu.RLock()
	defer a.mu.RUnlock()

	sessionID, exists := a.userSessions[userID]
	if !exists {
		return nil
	}

	return a.sessions[sessionID]
}

// RefreshSession refreshes a session's activity timestamp.
func (a *AuthManager) RefreshSession(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	if session.IsExpired() {
		return errors.New("session expired")
	}

	session.Refresh()
	return nil
}

// Logout terminates a session.
// SECURITY: Acquires session lock to safely read UserID.
func (a *AuthManager) Logout(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return
	}

	// Safely read UserID under session lock
	session.mu.RLock()
	userID := session.UserID
	session.mu.RUnlock()

	// Log the logout event
	a.logEvent("AUTH_LOGOUT", sanitizeSessionIDForLog(sessionID), true, map[string]string{
		"user_id": userID,
	})

	// Remove session
	delete(a.userSessions, userID)
	delete(a.sessions, sessionID)
}

// LogoutUser terminates all sessions for a user.
func (a *AuthManager) LogoutUser(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	sessionID, exists := a.userSessions[userID]
	if !exists {
		return
	}

	a.logEvent("AUTH_LOGOUT", sanitizeSessionIDForLog(sessionID), true, map[string]string{
		"user_id": userID,
		"reason":  "user_logout",
	})

	delete(a.sessions, sessionID)
	delete(a.userSessions, userID)
}

// RotateSessionID rotates a session ID (e.g., when privileges change).
// Creates a new session ID while preserving session data.
// This prevents session fixation attacks when user privileges escalate.
// SECURITY: Acquires session lock to safely read/write session fields.
func (a *AuthManager) RotateSessionID(oldSessionID string, reason string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[oldSessionID]
	if !exists {
		return "", errors.New("session not found")
	}

	if session.IsExpired() {
		return "", errors.New("cannot rotate expired session")
	}

	// Generate new session ID
	newSessionID, err := generateSessionID()
	if err != nil {
		a.logEvent("AUTH_SESSION_ROTATED", sanitizeSessionIDForLog(oldSessionID), false, map[string]string{
			"reason": reason,
			"error":  "session_id_generation_failed",
		})
		return "", fmt.Errorf("failed to generate new session ID: %w", err)
	}

	// Update session ID and read UserID under session lock
	session.mu.Lock()
	session.SessionID = newSessionID
	userID := session.UserID
	session.mu.Unlock()

	// Move to new key in map
	delete(a.sessions, oldSessionID)
	a.sessions[newSessionID] = session
	a.userSessions[userID] = newSessionID

	// Log the rotation (sanitize both old and new session IDs)
	a.logEvent("AUTH_SESSION_ROTATED", sanitizeSessionIDForLog(newSessionID), true, map[string]string{
		"old_session_id": sanitizeSessionIDForLog(oldSessionID),
		"reason":         reason,
		"user_id":        userID,
	})

	return newSessionID, nil
}

// =============================================================================
// MFA INFRASTRUCTURE (IA-2(1))
// =============================================================================

// ValidateMFA checks if MFA is satisfied for a session per IL5 requirements.
// Returns ErrMFARequired if MFA is required but not completed.
//
// This is the primary enforcement hook for MFA requirements.
// Call this function before granting access to sensitive operations.
func ValidateMFA(session *AuthSession, config *MFAConfig) error {
	if session == nil {
		return errors.New("session cannot be nil")
	}

	// If MFA is not required, allow access
	if config == nil || !config.Required {
		return nil
	}

	// Check if session has expired
	if session.IsExpired() {
		return ErrSessionExpired
	}

	// Safely read MFAVerified flag under session lock
	session.mu.RLock()
	mfaVerified := session.MFAVerified
	authenticatedAt := session.AuthenticatedAt
	session.mu.RUnlock()

	// Check if MFA is verified
	if !mfaVerified {
		// Check if we're still within grace period
		if config.GracePeriod > 0 {
			elapsed := time.Since(authenticatedAt)
			if elapsed < config.GracePeriod {
				// Still within grace period, allow but warn
				return nil
			}
		}

		return ErrMFARequired
	}

	return nil
}

// CreateMFAChallenge creates a new MFA challenge for a session.
// This is called when MFA verification is required.
func (a *AuthManager) CreateMFAChallenge(sessionID string, method MFAMethod) (*MFAChallenge, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Verify session exists
	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	if session.IsExpired() {
		return nil, ErrSessionExpired
	}

	// Check if method is allowed
	if a.mfaConfig != nil && !a.mfaConfig.IsMethodAllowed(method) {
		return nil, fmt.Errorf("MFA method %s not allowed", method)
	}

	// Get user ID from session
	session.mu.RLock()
	userID := session.UserID
	session.mu.RUnlock()

	// Create challenge
	now := time.Now()
	challengeDuration := DefaultMFAChallengeDuration
	if a.mfaConfig != nil && a.mfaConfig.ChallengeDuration > 0 {
		challengeDuration = a.mfaConfig.ChallengeDuration
	}

	challengeID, err := generateChallengeID()
	if err != nil {
		a.logEvent("AUTH_MFA_CHALLENGE", sanitizeSessionIDForLog(sessionID), false, map[string]string{
			"method": string(method),
			"error":  "challenge_id_generation_failed",
		})
		return nil, fmt.Errorf("failed to generate challenge ID: %w", err)
	}

	challenge := &MFAChallenge{
		ChallengeID: challengeID,
		SessionID:   sessionID,
		UserID:      userID,
		Method:      method,
		CreatedAt:   now,
		ExpiresAt:   now.Add(challengeDuration),
		Verified:    false,
		Attempts:    0,
		MaxAttempts: 3,
	}

	// Store challenge
	a.mfaChallenges[challenge.ChallengeID] = challenge

	// Log challenge creation
	a.logEvent("AUTH_MFA_CHALLENGE_CREATED", sanitizeSessionIDForLog(sessionID), true, map[string]string{
		"challenge_id": challenge.ChallengeID[:8] + "...",
		"method":       string(method),
		"user_id":      userID,
	})

	return challenge, nil
}

// GetMFAChallenge retrieves an MFA challenge by ID.
func (a *AuthManager) GetMFAChallenge(challengeID string) (*MFAChallenge, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	challenge, exists := a.mfaChallenges[challengeID]
	if !exists {
		return nil, errors.New("MFA challenge not found")
	}

	// Check if expired
	if time.Now().After(challenge.ExpiresAt) {
		return nil, errors.New("MFA challenge expired")
	}

	return challenge, nil
}

// VerifyMFAChallenge verifies an MFA challenge response.
// This updates the session's MFAVerified flag on success.
func (a *AuthManager) VerifyMFAChallenge(challengeID string, response string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	challenge, exists := a.mfaChallenges[challengeID]
	if !exists {
		return errors.New("MFA challenge not found")
	}

	// Check if expired
	if time.Now().After(challenge.ExpiresAt) {
		delete(a.mfaChallenges, challengeID)
		return errors.New("MFA challenge expired")
	}

	// Check if already verified
	if challenge.Verified {
		return errors.New("MFA challenge already verified")
	}

	// Check max attempts
	if challenge.Attempts >= challenge.MaxAttempts {
		delete(a.mfaChallenges, challengeID)
		return errors.New("maximum MFA verification attempts exceeded")
	}

	// Increment attempts
	challenge.Attempts++

	// Verify based on method
	var valid bool
	var err error

	switch challenge.Method {
	case MFAMethodTOTP:
		// Get TOTP secret for user
		secret, secretErr := a.getTOTPSecret(challenge.UserID)
		if secretErr != nil {
			a.logEvent("AUTH_MFA_VERIFY", challenge.ChallengeID[:8]+"...", false, map[string]string{
				"user_id": challenge.UserID,
				"error":   "totp_secret_not_found",
			})
			return secretErr
		}

		// Verify TOTP code
		valid = totp.Validate(response, secret)

	case MFAMethodWebAuthn:
		// Placeholder for WebAuthn verification
		err = errors.New("WebAuthn verification not yet implemented")

	case MFAMethodSMS:
		// Placeholder for SMS verification (not recommended for IL5)
		err = errors.New("SMS verification not yet implemented")

	default:
		err = fmt.Errorf("unsupported MFA method: %s", challenge.Method)
	}

	if err != nil {
		a.logEvent("AUTH_MFA_VERIFY", challenge.ChallengeID[:8]+"...", false, map[string]string{
			"user_id": challenge.UserID,
			"error":   err.Error(),
		})
		return err
	}

	if !valid {
		a.logEvent("AUTH_MFA_VERIFY", challenge.ChallengeID[:8]+"...", false, map[string]string{
			"user_id":  challenge.UserID,
			"attempts": fmt.Sprintf("%d/%d", challenge.Attempts, challenge.MaxAttempts),
		})
		return errors.New("invalid MFA code")
	}

	// Mark challenge as verified
	challenge.Verified = true

	// Update session MFA status
	session, exists := a.sessions[challenge.SessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.mu.Lock()
	session.MFAVerified = true
	session.mu.Unlock()

	// Log successful verification
	a.logEvent("AUTH_MFA_VERIFY", sanitizeSessionIDForLog(challenge.SessionID), true, map[string]string{
		"user_id": challenge.UserID,
		"method":  string(challenge.Method),
	})

	// Clean up challenge
	delete(a.mfaChallenges, challengeID)

	return nil
}

// VerifyMFA verifies MFA for an existing session using TOTP (IA-2(1)).
// SECURITY FIX (GSEC-1): Implements real TOTP verification instead of accepting any code.
// SECURITY: Acquires session lock to safely read/write session fields.
func (a *AuthManager) VerifyMFA(sessionID string, mfaCode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	if session.IsExpired() {
		return errors.New("session expired")
	}

	if mfaCode == "" {
		return errors.New("MFA code required")
	}

	// Read UserID under session lock
	session.mu.RLock()
	userID := session.UserID
	session.mu.RUnlock()

	// Get user's TOTP secret
	secret, err := a.getTOTPSecret(userID)
	if err != nil {
		a.logEvent("AUTH_MFA_VERIFY", sanitizeSessionIDForLog(sessionID), false, map[string]string{
			"user_id": userID,
			"error":   "totp_secret_not_found",
		})
		return err
	}

	// Verify the TOTP code using the otp library
	valid := totp.Validate(mfaCode, secret)
	if !valid {
		a.logEvent("AUTH_MFA_VERIFY", sanitizeSessionIDForLog(sessionID), false, map[string]string{
			"user_id": userID,
			"error":   "invalid_mfa_code",
		})
		return errors.New("invalid MFA code")
	}

	// Write MFAVerified under session lock
	session.mu.Lock()
	session.MFAVerified = true
	session.mu.Unlock()

	a.logEvent("AUTH_MFA_VERIFY", sanitizeSessionIDForLog(sessionID), true, map[string]string{
		"user_id": userID,
	})

	return nil
}

// getTOTPSecret retrieves the TOTP secret for a user.
// Returns an error if the secret is not found or not configured.
func (a *AuthManager) getTOTPSecret(userID string) (string, error) {
	secret, exists := a.totpSecrets[userID]
	if !exists || secret == "" {
		return "", errors.New("TOTP secret not configured for user")
	}
	return secret, nil
}

// SetTOTPSecret sets the TOTP secret for a user.
// This should be called during user enrollment in MFA.
func (a *AuthManager) SetTOTPSecret(userID string, secret string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if userID == "" {
		return errors.New("user ID required")
	}
	if secret == "" {
		return errors.New("TOTP secret required")
	}

	a.totpSecrets[userID] = secret
	return nil
}

// IsMFARequired returns whether MFA is required.
func (a *AuthManager) IsMFARequired() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mfaEnabled
}

// GetMFAConfig returns the current MFA configuration.
func (a *AuthManager) GetMFAConfig() *MFAConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.mfaConfig == nil {
		return DefaultMFAConfig()
	}
	// Return a copy to prevent external modifications
	return &MFAConfig{
		Required:          a.mfaConfig.Required,
		AllowedMethods:    append([]MFAMethod(nil), a.mfaConfig.AllowedMethods...),
		ChallengeDuration: a.mfaConfig.ChallengeDuration,
		GracePeriod:       a.mfaConfig.GracePeriod,
	}
}

// MFAStatus returns the MFA status message.
func (a *AuthManager) MFAStatus() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.mfaEnabled {
		if a.mfaConfig != nil {
			methods := make([]string, len(a.mfaConfig.AllowedMethods))
			for i, m := range a.mfaConfig.AllowedMethods {
				methods[i] = string(m)
			}
			return fmt.Sprintf("MFA is enabled (methods: %s)", strings.Join(methods, ", "))
		}
		return "MFA is enabled"
	}
	return "MFA is not enabled (placeholder for IA-2(1) compliance)"
}

// =============================================================================
// STATISTICS AND LISTING
// =============================================================================

// AuthStats provides authentication statistics.
type AuthStats struct {
	ActiveSessions      int           `json:"active_sessions"`
	ExpiredSessions     int           `json:"expired_sessions"`
	MFAEnabled          bool          `json:"mfa_enabled"`
	MFAVerifiedSessions int           `json:"mfa_verified_sessions"`
	PendingMFAChallenges int          `json:"pending_mfa_challenges"`
	SessionDuration     time.Duration `json:"session_duration"`
	LockoutEnabled      bool          `json:"lockout_enabled"`
}

// GetStats returns authentication statistics.
func (a *AuthManager) GetStats() AuthStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := AuthStats{
		MFAEnabled:      a.mfaEnabled,
		SessionDuration: a.sessionDuration,
		LockoutEnabled:  a.lockout != nil && a.lockout.IsEnabled(),
	}

	for _, session := range a.sessions {
		if session.IsExpired() {
			stats.ExpiredSessions++
		} else {
			stats.ActiveSessions++
			// Count MFA-verified sessions
			session.mu.RLock()
			if session.MFAVerified {
				stats.MFAVerifiedSessions++
			}
			session.mu.RUnlock()
		}
	}

	// Count pending MFA challenges
	now := time.Now()
	for _, challenge := range a.mfaChallenges {
		if !challenge.Verified && now.Before(challenge.ExpiresAt) {
			stats.PendingMFAChallenges++
		}
	}

	return stats
}

// ListSessions returns a list of active sessions.
// SECURITY: Creates safe copies of sessions under their locks.
func (a *AuthManager) ListSessions() []*AuthSession {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var sessions []*AuthSession
	for _, session := range a.sessions {
		if !session.IsExpired() {
			// Return a safe copy (must copy fields under lock, not the mutex)
			session.mu.RLock()
			sessionCopy := &AuthSession{
				UserID:          session.UserID,
				SessionID:       session.SessionID,
				AuthenticatedAt: session.AuthenticatedAt,
				ExpiresAt:       session.ExpiresAt,
				AuthMethod:      session.AuthMethod,
				MFAVerified:     session.MFAVerified,
				APIKeyHash:      session.APIKeyHash,
				LastActivity:    session.LastActivity,
			}
			if session.Metadata != nil {
				sessionCopy.Metadata = make(map[string]string, len(session.Metadata))
				for k, v := range session.Metadata {
					sessionCopy.Metadata[k] = v
				}
			}
			session.mu.RUnlock()
			sessions = append(sessions, sessionCopy)
		}
	}

	return sessions
}

// =============================================================================
// CLEANUP
// =============================================================================

// Cleanup removes expired sessions and MFA challenges.
// SECURITY: Acquires session lock to safely read UserID.
func (a *AuthManager) Cleanup() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	cleaned := 0

	// Clean up expired sessions
	for sessionID, session := range a.sessions {
		if session.IsExpired() {
			// Read UserID under session lock
			session.mu.RLock()
			userID := session.UserID
			session.mu.RUnlock()

			delete(a.userSessions, userID)
			delete(a.sessions, sessionID)
			cleaned++
		}
	}

	// Clean up expired MFA challenges
	now := time.Now()
	for challengeID, challenge := range a.mfaChallenges {
		if now.After(challenge.ExpiresAt) {
			delete(a.mfaChallenges, challengeID)
			cleaned++
		}
	}

	return cleaned
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs an authentication-related event to the audit log.
func (a *AuthManager) logEvent(eventType, sessionID string, success bool, metadata map[string]string) {
	if a.auditLogger == nil || !a.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		Success:   success,
		Metadata:  metadata,
	}

	if err := a.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log auth event %s: %v\n", eventType, err)
	}
}

// generateSessionID generates a cryptographically secure session ID.
// Uses 128 bits of cryptographic randomness (16 bytes = 32 hex chars).
// Format: auth_<32 hex chars>
//
// Returns an error if crypto/rand fails. The caller must handle this error
// and should log it to the audit trail for security compliance.
func generateSessionID() (string, error) {
	bytes := make([]byte, 16) // 128 bits = 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		// Log critical security error to stderr for audit trail
		fmt.Fprintf(os.Stderr, "CRITICAL SECURITY ERROR: crypto/rand failed: %v\n", err)
		return "", fmt.Errorf("cryptographic random generation failed: %w", err)
	}
	return AuthSessionIDPrefix + hex.EncodeToString(bytes), nil
}

// generateChallengeID generates a cryptographically secure MFA challenge ID.
// Uses 128 bits of cryptographic randomness (16 bytes = 32 hex chars).
// Format: mfa_<32 hex chars>
//
// Returns an error if crypto/rand fails. The caller must handle this error
// and should log it to the audit trail for security compliance.
func generateChallengeID() (string, error) {
	bytes := make([]byte, 16) // 128 bits = 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		// Log critical security error to stderr for audit trail
		fmt.Fprintf(os.Stderr, "CRITICAL SECURITY ERROR: crypto/rand failed: %v\n", err)
		return "", fmt.Errorf("cryptographic random generation failed: %w", err)
	}
	return MFAChallengeIDPrefix + hex.EncodeToString(bytes), nil
}

// generateAuthIdentifier creates an identifier for lockout tracking.
func generateAuthIdentifier(method AuthMethod, credential string) string {
	// Use a hash of the credential to avoid storing sensitive data
	hash := sha256.Sum256([]byte(string(method) + ":" + credential))
	return hex.EncodeToString(hash[:8])
}

// deriveUserID derives a user ID from an API key using SHA-256 hash.
// SECURITY FIX (GSEC-3): Uses hash instead of raw API key substring to prevent
// information leakage about the API key structure.
func deriveUserID(apiKey string) string {
	// Use SHA-256 hash of the API key for secure derivation
	hash := sha256.Sum256([]byte(apiKey))
	return "user_" + hex.EncodeToString(hash[:8])
}

// hashAPIKey creates a one-way hash of an API key for audit logging.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:8])
}

// sanitizeSessionIDForLog truncates a session ID for safe logging.
// SECURITY FIX (GSEC-4): Prevents full session ID exposure in logs while
// maintaining enough information for correlation.
func sanitizeSessionIDForLog(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:4] + "..." + sessionID[len(sessionID)-4:]
}

// ValidateAPIKeyFormat validates the format of an API key.
// This checks for common API key formats (OpenRouter, OpenAI, etc.).
func ValidateAPIKeyFormat(apiKey string) bool {
	apiKey = strings.TrimSpace(apiKey)

	// Check minimum length
	if len(apiKey) < APIKeyMinLength {
		return false
	}

	// OpenRouter keys: sk-or-...
	if strings.HasPrefix(apiKey, "sk-or-") && len(apiKey) > 20 {
		return true
	}

	// OpenAI keys: sk-...
	if strings.HasPrefix(apiKey, "sk-") && len(apiKey) > 20 {
		return true
	}

	// Anthropic keys: sk-ant-...
	if strings.HasPrefix(apiKey, "sk-ant-") && len(apiKey) > 20 {
		return true
	}

	// Generic key format (at least 20 alphanumeric characters)
	if len(apiKey) >= 20 {
		validChars := true
		for _, c := range apiKey {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_') {
				validChars = false
				break
			}
		}
		if validChars {
			return true
		}
	}

	return false
}

// =============================================================================
// ERRORS
// =============================================================================

var (
	// ErrAuthFailed indicates authentication failed.
	ErrAuthFailed = errors.New("authentication failed (IA-2)")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session expired")

	// ErrMFARequired indicates MFA verification is required.
	ErrMFARequired = errors.New("MFA verification required (IA-2(1))")
)

// =============================================================================
// GLOBAL AUTH MANAGER
// =============================================================================

var (
	globalAuthManager     *AuthManager
	globalAuthManagerOnce sync.Once
	globalAuthManagerMu   sync.Mutex
)

// GlobalAuthManager returns the global auth manager instance.
func GlobalAuthManager() *AuthManager {
	globalAuthManagerOnce.Do(func() {
		globalAuthManager = NewAuthManager()
	})
	return globalAuthManager
}

// SetGlobalAuthManager sets the global auth manager instance.
func SetGlobalAuthManager(manager *AuthManager) {
	globalAuthManagerMu.Lock()
	defer globalAuthManagerMu.Unlock()
	globalAuthManager = manager
}

// InitGlobalAuthManager initializes the global auth manager with options.
func InitGlobalAuthManager(opts ...AuthManagerOption) {
	globalAuthManagerMu.Lock()
	defer globalAuthManagerMu.Unlock()
	globalAuthManager = NewAuthManager(opts...)
}

// =============================================================================
// HELPER: GET API KEY FROM ENVIRONMENT
// =============================================================================

// GetAPIKeyFromEnv retrieves the API key from environment variables.
func GetAPIKeyFromEnv() string {
	// Check for OPENROUTER_API_KEY first
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key
	}

	// Check for RIGRUN_OPENROUTER_KEY
	if key := os.Getenv("RIGRUN_OPENROUTER_KEY"); key != "" {
		return key
	}

	// Check for generic OPENAI_API_KEY
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}

	return ""
}
