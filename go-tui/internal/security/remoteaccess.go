// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 AC-17: Remote Access.
//
// # DoD STIG Requirements
//
//   - AC-17: Authorizes remote access before allowing such connections
//   - AC-17(1): Monitors and controls remote access methods
//   - AC-17(2): Protects confidentiality and integrity of remote access sessions
//   - AC-17(3): Requires managed access control points
//   - AU-3: Remote access events must be logged for audit compliance
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// =============================================================================
// AC-17 CONSTANTS
// =============================================================================

const (
	// RemoteSessionTimeout is the default timeout for remote sessions (15 minutes).
	RemoteSessionTimeout = 15 * time.Minute

	// RemoteSessionCleanupInterval is how often to clean up expired sessions.
	RemoteSessionCleanupInterval = 5 * time.Minute

	// RemoteSessionIDPrefix is the prefix for remote session IDs.
	RemoteSessionIDPrefix = "remote_"
)

// =============================================================================
// REMOTE SESSION
// =============================================================================

// RemoteSession represents a remote access session per AC-17.
type RemoteSession struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// UserID is the authenticated user identifier.
	UserID string `json:"user_id"`

	// RemoteAddr is the remote IP address.
	RemoteAddr string `json:"remote_addr"`

	// StartTime is when the session was established.
	StartTime time.Time `json:"start_time"`

	// LastActivity is the last activity timestamp.
	LastActivity time.Time `json:"last_activity"`

	// ExpiresAt is when the session expires.
	ExpiresAt time.Time `json:"expires_at"`

	// Protocol is the access protocol (e.g., "https", "ssh", "rdp").
	Protocol string `json:"protocol"`

	// Metadata stores additional session information.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Active indicates if the session is currently active.
	Active bool `json:"active"`
}

// IsExpired returns true if the session has expired.
func (s *RemoteSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive returns true if the session is active and not expired.
func (s *RemoteSession) IsActive() bool {
	return s.Active && !s.IsExpired()
}

// TimeRemaining returns the duration until the session expires.
func (s *RemoteSession) TimeRemaining() time.Duration {
	remaining := time.Until(s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Refresh updates the LastActivity timestamp and extends expiration.
func (s *RemoteSession) Refresh(timeout time.Duration) {
	s.LastActivity = time.Now()
	s.ExpiresAt = s.LastActivity.Add(timeout)
}

// =============================================================================
// REMOTE ACCESS MANAGER
// =============================================================================

// RemoteAccessManager manages remote access sessions per AC-17.
type RemoteAccessManager struct {
	// sessions maps session IDs to remote sessions.
	sessions map[string]*RemoteSession

	// allowedIPs stores allowed IP ranges (CIDR notation).
	allowedIPs []string

	// allowedTimes stores time-based access control schedule.
	allowedTimes *AccessSchedule

	// sessionTimeout is the default session timeout.
	sessionTimeout time.Duration

	// auditLogger is the audit logger for AC-17 events.
	auditLogger *AuditLogger

	// accessPolicy is the current access policy.
	accessPolicy *AccessPolicy

	// mu protects concurrent access.
	mu sync.RWMutex

	// stopCleanup signals the cleanup goroutine to stop.
	stopCleanup chan struct{}
}

// RemoteAccessManagerOption is a functional option for configuring RemoteAccessManager.
type RemoteAccessManagerOption func(*RemoteAccessManager)

// WithRemoteAccessAuditLogger sets the audit logger for AC-17 events.
func WithRemoteAccessAuditLogger(logger *AuditLogger) RemoteAccessManagerOption {
	return func(ram *RemoteAccessManager) {
		ram.auditLogger = logger
	}
}

// WithSessionTimeout sets the default session timeout.
func WithSessionTimeout(timeout time.Duration) RemoteAccessManagerOption {
	return func(ram *RemoteAccessManager) {
		if timeout > 0 {
			ram.sessionTimeout = timeout
		}
	}
}

// WithAllowedIPs sets the allowed IP ranges for remote access.
func WithAllowedIPs(cidrs []string) RemoteAccessManagerOption {
	return func(ram *RemoteAccessManager) {
		ram.allowedIPs = cidrs
	}
}

// NewRemoteAccessManager creates a new RemoteAccessManager with the given options.
func NewRemoteAccessManager(opts ...RemoteAccessManagerOption) *RemoteAccessManager {
	ram := &RemoteAccessManager{
		sessions:       make(map[string]*RemoteSession),
		allowedIPs:     make([]string, 0),
		sessionTimeout: RemoteSessionTimeout,
		accessPolicy:   NewAccessPolicy(),
		stopCleanup:    make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(ram)
	}

	// Default audit logger if not provided
	if ram.auditLogger == nil {
		ram.auditLogger = GlobalAuditLogger()
	}

	// Start cleanup goroutine
	go ram.cleanupLoop()

	return ram
}

// =============================================================================
// REMOTE ACCESS VALIDATION
// =============================================================================

// ValidateRemoteAccess validates a remote access request.
// Returns true if access is allowed, false otherwise.
func (ram *RemoteAccessManager) ValidateRemoteAccess(request *AccessRequest) error {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	// Check IP allowlist
	if len(ram.allowedIPs) > 0 {
		if !ram.isIPAllowed(request.RemoteAddr) {
			ram.logEvent("REMOTE_ACCESS_DENIED", "", false, map[string]string{
				"remote_addr": request.RemoteAddr,
				"reason":      "IP not in allowlist",
			})
			return errors.New("AC-17: IP address not in allowlist")
		}
	}

	// Check time-based access control
	if ram.allowedTimes != nil {
		if !ram.allowedTimes.IsAllowed(time.Now()) {
			ram.logEvent("REMOTE_ACCESS_DENIED", "", false, map[string]string{
				"remote_addr": request.RemoteAddr,
				"reason":      "outside allowed time window",
			})
			return errors.New("AC-17: access outside allowed time window")
		}
	}

	// Check access policy
	if !ram.accessPolicy.IsAllowed(request) {
		ram.logEvent("REMOTE_ACCESS_DENIED", "", false, map[string]string{
			"remote_addr": request.RemoteAddr,
			"protocol":    request.Protocol,
			"reason":      "policy violation",
		})
		return errors.New("AC-17: access denied by policy")
	}

	return nil
}

// CreateSession creates a new remote access session.
func (ram *RemoteAccessManager) CreateSession(userID, remoteAddr, protocol string) (*RemoteSession, error) {
	// Validate the request
	request := &AccessRequest{
		UserID:     userID,
		RemoteAddr: remoteAddr,
		Protocol:   protocol,
		Timestamp:  time.Now(),
	}

	if err := ram.ValidateRemoteAccess(request); err != nil {
		return nil, err
	}

	ram.mu.Lock()
	defer ram.mu.Unlock()

	// Create session
	now := time.Now()
	session := &RemoteSession{
		ID:           generateRemoteSessionID(),
		UserID:       userID,
		RemoteAddr:   remoteAddr,
		StartTime:    now,
		LastActivity: now,
		ExpiresAt:    now.Add(ram.sessionTimeout),
		Protocol:     protocol,
		Active:       true,
		Metadata:     make(map[string]string),
	}

	// Store session
	ram.sessions[session.ID] = session

	// Log the session creation
	ram.logEvent("REMOTE_SESSION_START", session.ID, true, map[string]string{
		"user_id":     userID,
		"remote_addr": remoteAddr,
		"protocol":    protocol,
	})

	return session, nil
}

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================

// GetSession retrieves a session by its ID.
func (ram *RemoteAccessManager) GetSession(sessionID string) *RemoteSession {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	session, exists := ram.sessions[sessionID]
	if !exists {
		return nil
	}

	// Check if expired
	if session.IsExpired() {
		return nil
	}

	return session
}

// GetActiveSessions returns a list of all active remote sessions.
func (ram *RemoteAccessManager) GetActiveSessions() []*RemoteSession {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	sessions := make([]*RemoteSession, 0)
	for _, session := range ram.sessions {
		if session.IsActive() {
			// Return a copy
			sessionCopy := *session
			sessions = append(sessions, &sessionCopy)
		}
	}

	return sessions
}

// RefreshSession refreshes a session's activity timestamp.
func (ram *RemoteAccessManager) RefreshSession(sessionID string) error {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	session, exists := ram.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	if session.IsExpired() {
		return errors.New("session expired")
	}

	session.Refresh(ram.sessionTimeout)

	ram.logEvent("REMOTE_SESSION_REFRESH", sessionID, true, map[string]string{
		"user_id":     session.UserID,
		"remote_addr": session.RemoteAddr,
	})

	return nil
}

// TerminateSession terminates a remote access session.
func (ram *RemoteAccessManager) TerminateSession(sessionID string) error {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	session, exists := ram.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.Active = false

	ram.logEvent("REMOTE_SESSION_TERMINATE", sessionID, true, map[string]string{
		"user_id":     session.UserID,
		"remote_addr": session.RemoteAddr,
	})

	// Remove from map
	delete(ram.sessions, sessionID)

	return nil
}

// =============================================================================
// IP ALLOWLIST MANAGEMENT
// =============================================================================

// SetAllowedIPs sets the allowed IP ranges for remote access (CIDR notation).
func (ram *RemoteAccessManager) SetAllowedIPs(cidrs []string) error {
	// Validate CIDRs
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
	}

	ram.mu.Lock()
	defer ram.mu.Unlock()

	ram.allowedIPs = make([]string, len(cidrs))
	copy(ram.allowedIPs, cidrs)

	ram.logEvent("ALLOWLIST_UPDATED", "", true, map[string]string{
		"count": fmt.Sprintf("%d", len(cidrs)),
	})

	return nil
}

// GetAllowedIPs returns the current allowed IP ranges.
func (ram *RemoteAccessManager) GetAllowedIPs() []string {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	result := make([]string, len(ram.allowedIPs))
	copy(result, ram.allowedIPs)
	return result
}

// AddAllowedIP adds an IP range to the allowlist.
func (ram *RemoteAccessManager) AddAllowedIP(cidr string) error {
	// Validate CIDR
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	ram.mu.Lock()
	defer ram.mu.Unlock()

	// Check if already exists
	for _, existing := range ram.allowedIPs {
		if existing == cidr {
			return nil // Already exists
		}
	}

	ram.allowedIPs = append(ram.allowedIPs, cidr)

	ram.logEvent("ALLOWLIST_IP_ADDED", "", true, map[string]string{
		"cidr": cidr,
	})

	return nil
}

// RemoveAllowedIP removes an IP range from the allowlist.
func (ram *RemoteAccessManager) RemoveAllowedIP(cidr string) {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	// Find and remove
	for i, existing := range ram.allowedIPs {
		if existing == cidr {
			ram.allowedIPs = append(ram.allowedIPs[:i], ram.allowedIPs[i+1:]...)
			ram.logEvent("ALLOWLIST_IP_REMOVED", "", true, map[string]string{
				"cidr": cidr,
			})
			return
		}
	}
}

// isIPAllowed checks if an IP address is in the allowlist (caller must hold lock).
func (ram *RemoteAccessManager) isIPAllowed(ipAddr string) bool {
	// Parse IP
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		// Try extracting IP from "IP:port" format
		host, _, err := net.SplitHostPort(ipAddr)
		if err == nil {
			ip = net.ParseIP(host)
		}
	}

	if ip == nil {
		return false
	}

	// Check against each CIDR
	for _, cidr := range ram.allowedIPs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// =============================================================================
// TIME-BASED ACCESS CONTROL
// =============================================================================

// SetAllowedTimes sets the time-based access control schedule.
func (ram *RemoteAccessManager) SetAllowedTimes(schedule *AccessSchedule) {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	ram.allowedTimes = schedule

	ram.logEvent("ACCESS_SCHEDULE_UPDATED", "", true, nil)
}

// GetAllowedTimes returns the current access schedule.
func (ram *RemoteAccessManager) GetAllowedTimes() *AccessSchedule {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	return ram.allowedTimes
}

// =============================================================================
// ACCESS POLICY
// =============================================================================

// GetAccessPolicy returns the current access policy.
func (ram *RemoteAccessManager) GetAccessPolicy() *AccessPolicy {
	ram.mu.RLock()
	defer ram.mu.RUnlock()

	return ram.accessPolicy
}

// SetAccessPolicy sets the access policy.
func (ram *RemoteAccessManager) SetAccessPolicy(policy *AccessPolicy) {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	ram.accessPolicy = policy

	ram.logEvent("ACCESS_POLICY_UPDATED", "", true, nil)
}

// =============================================================================
// AUDIT LOGGING
// =============================================================================

// AuditRemoteAccess logs a remote access event to the audit log.
func (ram *RemoteAccessManager) AuditRemoteAccess(session *RemoteSession, action string) {
	if session == nil {
		return
	}

	metadata := map[string]string{
		"user_id":     session.UserID,
		"remote_addr": session.RemoteAddr,
		"protocol":    session.Protocol,
		"action":      action,
	}

	ram.logEvent("REMOTE_ACCESS_AUDIT", session.ID, true, metadata)
}

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a remote access event to the audit log.
func (ram *RemoteAccessManager) logEvent(eventType, sessionID string, success bool, metadata map[string]string) {
	if ram.auditLogger == nil || !ram.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		Success:   success,
		Metadata:  metadata,
	}

	if err := ram.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log remote access event %s: %v\n", eventType, err)
	}
}

// =============================================================================
// CLEANUP
// =============================================================================

// cleanupLoop periodically cleans up expired sessions.
func (ram *RemoteAccessManager) cleanupLoop() {
	ticker := time.NewTicker(RemoteSessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ram.cleanup()
		case <-ram.stopCleanup:
			return
		}
	}
}

// cleanup removes expired sessions.
func (ram *RemoteAccessManager) cleanup() {
	ram.mu.Lock()
	defer ram.mu.Unlock()

	cleaned := 0
	for sessionID, session := range ram.sessions {
		if session.IsExpired() {
			ram.logEvent("REMOTE_SESSION_EXPIRED", sessionID, true, map[string]string{
				"user_id":     session.UserID,
				"remote_addr": session.RemoteAddr,
			})
			delete(ram.sessions, sessionID)
			cleaned++
		}
	}

	if cleaned > 0 {
		ram.logEvent("SESSION_CLEANUP", "", true, map[string]string{
			"cleaned": fmt.Sprintf("%d", cleaned),
		})
	}
}

// Stop stops the cleanup goroutine.
func (ram *RemoteAccessManager) Stop() {
	// BUGFIX: Add mutex to prevent double-close panic
	ram.mu.Lock()
	defer ram.mu.Unlock()

	select {
	case <-ram.stopCleanup:
		// Already closed
		return
	default:
		close(ram.stopCleanup)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateRemoteSessionID generates a cryptographically secure session ID.
func generateRemoteSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%s%016x", RemoteSessionIDPrefix, time.Now().UnixNano())
	}
	return RemoteSessionIDPrefix + hex.EncodeToString(bytes)
}

// =============================================================================
// ACCESS REQUEST
// =============================================================================

// AccessRequest represents a remote access request.
type AccessRequest struct {
	UserID     string
	RemoteAddr string
	Protocol   string
	Timestamp  time.Time
	Metadata   map[string]string
}

// =============================================================================
// ACCESS SCHEDULE
// =============================================================================

// AccessSchedule defines time-based access control windows.
type AccessSchedule struct {
	// AllowedDays are the days of the week when access is allowed (0 = Sunday, 6 = Saturday).
	AllowedDays []time.Weekday `json:"allowed_days"`

	// StartHour is the start hour for access (0-23).
	StartHour int `json:"start_hour"`

	// EndHour is the end hour for access (0-23).
	EndHour int `json:"end_hour"`

	// Timezone is the timezone for the schedule.
	Timezone string `json:"timezone"`
}

// NewAccessSchedule creates a new access schedule.
func NewAccessSchedule(allowedDays []time.Weekday, startHour, endHour int) *AccessSchedule {
	return &AccessSchedule{
		AllowedDays: allowedDays,
		StartHour:   startHour,
		EndHour:     endHour,
		Timezone:    "UTC",
	}
}

// IsAllowed checks if access is allowed at the given time.
func (s *AccessSchedule) IsAllowed(t time.Time) bool {
	if s == nil {
		return true // No schedule means always allowed
	}

	// Check day of week
	dayAllowed := false
	for _, day := range s.AllowedDays {
		if t.Weekday() == day {
			dayAllowed = true
			break
		}
	}
	if !dayAllowed {
		return false
	}

	// Check hour
	hour := t.Hour()
	if s.StartHour <= s.EndHour {
		// Normal range (e.g., 9-17)
		return hour >= s.StartHour && hour < s.EndHour
	} else {
		// Wrap-around range (e.g., 22-06)
		return hour >= s.StartHour || hour < s.EndHour
	}
}

// =============================================================================
// ACCESS POLICY
// =============================================================================

// AccessPolicy defines the remote access policy.
type AccessPolicy struct {
	// AllowedProtocols are the allowed access protocols.
	AllowedProtocols []string `json:"allowed_protocols"`

	// RequireMFA indicates if MFA is required for remote access.
	RequireMFA bool `json:"require_mfa"`

	// MaxConcurrentSessions is the maximum number of concurrent sessions per user.
	MaxConcurrentSessions int `json:"max_concurrent_sessions"`
}

// NewAccessPolicy creates a new access policy with defaults.
func NewAccessPolicy() *AccessPolicy {
	return &AccessPolicy{
		AllowedProtocols:      []string{"https", "ssh"},
		RequireMFA:            false, // Placeholder for future MFA implementation
		MaxConcurrentSessions: 5,
	}
}

// IsAllowed checks if an access request is allowed by the policy.
func (p *AccessPolicy) IsAllowed(request *AccessRequest) bool {
	if p == nil {
		return true
	}

	// Check protocol
	protocolAllowed := false
	for _, proto := range p.AllowedProtocols {
		if proto == request.Protocol {
			protocolAllowed = true
			break
		}
	}
	if !protocolAllowed {
		return false
	}

	return true
}

// =============================================================================
// GLOBAL REMOTE ACCESS MANAGER
// =============================================================================

var (
	globalRemoteAccessManager     *RemoteAccessManager
	globalRemoteAccessManagerOnce sync.Once
	globalRemoteAccessManagerMu   sync.Mutex
)

// GlobalRemoteAccessManager returns the global remote access manager instance.
func GlobalRemoteAccessManager() *RemoteAccessManager {
	globalRemoteAccessManagerOnce.Do(func() {
		globalRemoteAccessManager = NewRemoteAccessManager()
	})
	return globalRemoteAccessManager
}

// SetGlobalRemoteAccessManager sets the global remote access manager instance.
func SetGlobalRemoteAccessManager(ram *RemoteAccessManager) {
	globalRemoteAccessManagerMu.Lock()
	defer globalRemoteAccessManagerMu.Unlock()
	globalRemoteAccessManager = ram
}

// InitGlobalRemoteAccessManager initializes the global remote access manager with options.
func InitGlobalRemoteAccessManager(opts ...RemoteAccessManagerOption) {
	globalRemoteAccessManagerMu.Lock()
	defer globalRemoteAccessManagerMu.Unlock()
	globalRemoteAccessManager = NewRemoteAccessManager(opts...)
}
