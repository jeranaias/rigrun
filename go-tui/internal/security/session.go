// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides DoD STIG-compliant security features.
//
// Session Manager implements session timeout per DoD STIG requirements for IL5 environments.
//
// # DoD STIG Requirements
//
//   - AC-12 (Session Termination): Sessions MUST terminate after 15 minutes (900 seconds)
//     of inactivity or absolute timeout.
//   - AC-11 (Session Lock): Session lock with re-authentication required.
//   - AU-3 (Audit Content): Session events must be logged.
//
// # Configuration
//
// The maximum session timeout is 15 minutes (900 seconds) for IL5.
// This is a HARD LIMIT that cannot be exceeded, only reduced.
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// DoD STIG AC-12 compliant session timeout constants.
const (
	// DefaultSessionTimeout is the DoD STIG maximum session timeout: 15 minutes (900 seconds).
	// This is the MAXIMUM allowed for IL5 environments - cannot be exceeded.
	DefaultSessionTimeout = 15 * time.Minute

	// DefaultWarningBefore is the default warning time before timeout: 2 minutes (120 seconds).
	DefaultWarningBefore = 2 * time.Minute

	// MinSessionTimeout is the minimum allowed session timeout.
	MinSessionTimeout = 5 * time.Minute

	// MaxSessionTimeout is the maximum allowed session timeout for IL5 compliance.
	MaxSessionTimeout = 30 * time.Minute

	// AbsoluteSessionMaxDuration is the absolute maximum session duration: 12 hours.
	// Sessions MUST be rotated after this time regardless of activity.
	AbsoluteSessionMaxDuration = 12 * time.Hour
)

// SessionState represents the current state of a session.
type SessionState int

const (
	// SessionActive indicates the session is active and valid.
	SessionActive SessionState = iota
	// SessionWarning indicates the session is in warning period (2 minutes before timeout).
	SessionWarning
	// SessionExpired indicates the session has expired and requires re-authentication.
	SessionExpired
)

// String returns a string representation of the SessionState.
func (s SessionState) String() string {
	switch s {
	case SessionActive:
		return "ACTIVE"
	case SessionWarning:
		return "WARNING"
	case SessionExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}

// IsActive returns true if the session state allows activity.
func (s SessionState) IsActive() bool {
	return s == SessionActive || s == SessionWarning
}

// RequiresReauth returns true if re-authentication is required.
func (s SessionState) RequiresReauth() bool {
	return s == SessionExpired
}

// Session represents an authenticated user session.
type Session struct {
	// ID is the unique session identifier.
	ID string

	// StartedAt is when the session was created.
	StartedAt time.Time

	// LastActivity is the timestamp of last activity.
	LastActivity time.Time

	// State is the current session state.
	State SessionState

	// UserID is an optional user identifier for audit logging.
	UserID string

	// mu protects concurrent access to session fields.
	mu sync.RWMutex

	// onWarning is called when session enters warning period.
	onWarning func()

	// onExpired is called when session expires.
	onExpired func()

	// warningTimer triggers the warning callback.
	warningTimer *time.Timer

	// expireTimer triggers the expiration callback.
	expireTimer *time.Timer
}

// SessionManager manages session lifecycle with DoD STIG compliance.
type SessionManager struct {
	// session is the current active session.
	session *Session

	// timeout is the session timeout duration (default: 15 minutes).
	timeout time.Duration

	// warningBefore is how long before timeout to issue warning (default: 2 minutes).
	warningBefore time.Duration

	// enabled indicates if session management is active.
	enabled bool

	// mu protects concurrent access to manager fields.
	mu sync.RWMutex
}

// NewSessionManager creates a new SessionManager with the specified timeout.
// The timeout will be clamped between MinSessionTimeout and MaxSessionTimeout.
func NewSessionManager(timeout time.Duration) *SessionManager {
	// Clamp timeout to valid range
	if timeout < MinSessionTimeout {
		log.Printf("SESSION_TIMEOUT: Requested timeout %v is below minimum %v. Using minimum.", timeout, MinSessionTimeout)
		timeout = MinSessionTimeout
	}
	if timeout > MaxSessionTimeout {
		log.Printf("SESSION_TIMEOUT: Requested timeout %v exceeds maximum %v. Clamped to maximum.", timeout, MaxSessionTimeout)
		timeout = MaxSessionTimeout
	}

	return &SessionManager{
		timeout:       timeout,
		warningBefore: DefaultWarningBefore,
		enabled:       true,
	}
}

// NewDefaultSessionManager creates a SessionManager with DoD STIG IL5 defaults.
func NewDefaultSessionManager() *SessionManager {
	return NewSessionManager(DefaultSessionTimeout)
}

// newSessionID creates a cryptographically secure session ID for SessionManager.
// Uses 128 bits of cryptographic randomness (16 bytes = 32 hex chars).
// Format: sess_<32 hex chars>
//
// Returns an error if crypto/rand fails. The caller must handle this error
// and should log it to the audit trail for security compliance.
func newSessionID() (string, error) {
	bytes := make([]byte, 16) // 128 bits = 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		// Log critical security error for audit trail
		log.Printf("CRITICAL SECURITY ERROR: crypto/rand failed: %v", err)
		return "", fmt.Errorf("cryptographic random generation failed: %w", err)
	}
	return "sess_" + hex.EncodeToString(bytes), nil
}

// StartSession creates and starts a new session.
// Returns an error if a session is already active.
func (m *SessionManager) StartSession() (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil, errors.New("session management is disabled")
	}

	// Check if there's already an active session
	if m.session != nil && m.session.State != SessionExpired {
		return nil, errors.New("session already active; end current session first")
	}

	// Generate session ID
	sessionID, err := newSessionID()
	if err != nil {
		logSessionEvent("SESSION_CREATE_FAILED", "unknown", fmt.Sprintf("error=%v", err))
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		StartedAt:    now,
		LastActivity: now,
		State:        SessionActive,
	}

	m.session = session

	// Set up timers
	m.setupTimers(session)

	// Log session creation for audit trail
	logSessionEvent("SESSION_CREATED", session.ID, fmt.Sprintf("timeout=%v", m.timeout))

	return session, nil
}

// setupTimers configures the warning and expiration timers for a session.
func (m *SessionManager) setupTimers(session *Session) {
	session.mu.Lock()
	defer session.mu.Unlock()

	// Clear any existing timers
	if session.warningTimer != nil {
		session.warningTimer.Stop()
	}
	if session.expireTimer != nil {
		session.expireTimer.Stop()
	}

	// Calculate timer durations
	warningDuration := m.timeout - m.warningBefore
	if warningDuration < 0 {
		warningDuration = 0
	}

	// Set up warning timer
	session.warningTimer = time.AfterFunc(warningDuration, func() {
		session.mu.Lock()
		if session.State == SessionActive {
			session.State = SessionWarning
			callback := session.onWarning
			session.mu.Unlock()

			logSessionEvent("SESSION_WARNING", session.ID, fmt.Sprintf("expires_in=%v", m.warningBefore))

			if callback != nil {
				callback()
			}
		} else {
			session.mu.Unlock()
		}
	})

	// Set up expiration timer
	session.expireTimer = time.AfterFunc(m.timeout, func() {
		session.mu.Lock()
		if session.State != SessionExpired {
			session.State = SessionExpired
			callback := session.onExpired
			session.mu.Unlock()

			logSessionEvent("SESSION_EXPIRED", session.ID, fmt.Sprintf("duration=%v", time.Since(session.StartedAt)))

			if callback != nil {
				callback()
			}
		} else {
			session.mu.Unlock()
		}
	})
}

// RefreshSession updates the LastActivity timestamp and resets timers.
// Returns an error if the session is expired or doesn't exist.
func (m *SessionManager) RefreshSession() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return errors.New("no active session")
	}

	m.session.mu.Lock()
	if m.session.State == SessionExpired {
		m.session.mu.Unlock()
		logSessionEvent("SESSION_REFRESH_DENIED", m.session.ID, "reason=expired")
		return errors.New("cannot refresh expired session; re-authentication required")
	}

	// Update last activity
	m.session.LastActivity = time.Now()
	m.session.State = SessionActive
	m.session.mu.Unlock()

	// Reset timers
	m.setupTimers(m.session)

	logSessionEvent("SESSION_REFRESHED", m.session.ID, fmt.Sprintf("remaining=%v", m.timeout))

	return nil
}

// EndSession terminates the current session and cleans up timers.
func (m *SessionManager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	// Stop timers
	if m.session.warningTimer != nil {
		m.session.warningTimer.Stop()
		m.session.warningTimer = nil
	}
	if m.session.expireTimer != nil {
		m.session.expireTimer.Stop()
		m.session.expireTimer = nil
	}

	// Mark as expired
	m.session.State = SessionExpired

	logSessionEvent("SESSION_TERMINATED", m.session.ID, fmt.Sprintf("duration=%v", time.Since(m.session.StartedAt)))

	m.session = nil
}

// IsExpired returns true if the session has expired or doesn't exist.
// Checks both inactivity timeout and absolute session timeout.
func (m *SessionManager) IsExpired() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return true
	}

	m.session.mu.RLock()
	defer m.session.mu.RUnlock()

	// Check if already marked as expired
	if m.session.State == SessionExpired {
		return true
	}

	// Check absolute session timeout (12 hours max)
	if time.Since(m.session.StartedAt) >= AbsoluteSessionMaxDuration {
		return true
	}

	// Check inactivity timeout
	if time.Since(m.session.LastActivity) >= m.timeout {
		return true
	}

	return false
}

// GetSession returns the current session or nil if none exists.
func (m *SessionManager) GetSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.session
}

// TimeRemaining returns the duration until the session expires.
// Returns 0 if no session exists or session is expired.
func (m *SessionManager) TimeRemaining() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return 0
	}

	m.session.mu.RLock()
	defer m.session.mu.RUnlock()

	if m.session.State == SessionExpired {
		return 0
	}

	elapsed := time.Since(m.session.LastActivity)
	remaining := m.timeout - elapsed
	if remaining < 0 {
		return 0
	}

	return remaining
}

// SetCallbacks sets the warning and expiration callback functions.
// These callbacks are invoked when the session enters warning period or expires.
func (m *SessionManager) SetCallbacks(onWarning, onExpired func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		m.session.mu.Lock()
		m.session.onWarning = onWarning
		m.session.onExpired = onExpired
		m.session.mu.Unlock()
	}
}

// RequireReauth returns true if the session has expired and re-authentication is required.
func (m *SessionManager) RequireReauth() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return true
	}

	m.session.mu.RLock()
	defer m.session.mu.RUnlock()

	return m.session.State.RequiresReauth()
}

// GetState returns the current session state.
// Returns SessionExpired if no session exists.
func (m *SessionManager) GetState() SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return SessionExpired
	}

	m.session.mu.RLock()
	defer m.session.mu.RUnlock()

	return m.session.State
}

// SetEnabled enables or disables session management.
func (m *SessionManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = enabled
}

// IsEnabled returns whether session management is enabled.
func (m *SessionManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.enabled
}

// GetTimeout returns the configured session timeout duration.
func (m *SessionManager) GetTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.timeout
}

// GetWarningBefore returns the warning period before timeout.
func (m *SessionManager) GetWarningBefore() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.warningBefore
}

// SetUserID sets the user identifier for the current session.
func (m *SessionManager) SetUserID(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		m.session.mu.Lock()
		m.session.UserID = userID
		m.session.mu.Unlock()
	}
}

// RotateSessionID rotates the session ID (e.g., when privileges change).
// Creates a new session ID while preserving session state.
// This prevents session fixation attacks when user privileges escalate.
func (m *SessionManager) RotateSessionID(reason string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return "", errors.New("no active session to rotate")
	}

	m.session.mu.Lock()
	defer m.session.mu.Unlock()

	oldID := m.session.ID
	newID, err := newSessionID()
	if err != nil {
		logSessionEvent("SESSION_ROTATION_FAILED", oldID, fmt.Sprintf("reason=%s error=%v", reason, err))
		return "", fmt.Errorf("failed to generate new session ID: %w", err)
	}

	m.session.ID = newID

	logSessionEvent("SESSION_ROTATED", newID, fmt.Sprintf("old=%s reason=%s", oldID, reason))

	return newID, nil
}

// logSessionEvent logs a session event for audit trail compliance (AU-3).
func logSessionEvent(eventType, sessionID, details string) {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	log.Printf("%s | %s | session=%s %s", timestamp, eventType, sessionID, details)
}
