// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 PS-6: Access Agreements.
//
// # DoD STIG Requirements
//
//   - PS-6: Personnel security - access agreements for system access
//   - PS-6(2): Ensure agreements are signed before granting access
//   - PS-6(3): Post-employment requirements and restrictions
//   - AU-3: Access agreement events must be logged for audit compliance
package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// PS-6 CONSTANTS
// =============================================================================

const (
	// AgreementTypeNDA represents non-disclosure agreement.
	AgreementTypeNDA AgreementType = "nda"

	// AgreementTypeAUP represents acceptable use policy.
	AgreementTypeAUP AgreementType = "aup"

	// AgreementTypeSecurity represents security awareness acknowledgment.
	AgreementTypeSecurity AgreementType = "security"

	// AgreementTypePII represents PII handling agreement.
	AgreementTypePII AgreementType = "pii"

	// DefaultAgreementExpiration is the default expiration period (1 year).
	DefaultAgreementExpiration = 365 * 24 * time.Hour

	// AgreementsFileName is the default storage file name.
	AgreementsFileName = "agreements.json"
)

// =============================================================================
// AGREEMENT TYPES
// =============================================================================

// AgreementType represents the type of agreement.
type AgreementType string

// String returns the string representation of the agreement type.
func (t AgreementType) String() string {
	return string(t)
}

// =============================================================================
// AGREEMENT STRUCT
// =============================================================================

// Agreement represents a single access agreement that users must sign.
type Agreement struct {
	// ID is the unique identifier for this agreement.
	ID string `json:"id"`

	// Type is the type of agreement (NDA, AUP, Security, PII).
	Type AgreementType `json:"type"`

	// Version is the agreement version (e.g., "1.0", "2.0").
	Version string `json:"version"`

	// Content is the full text of the agreement.
	Content string `json:"content"`

	// RequiredFor specifies which user types must sign this agreement.
	// Empty means required for all users.
	RequiredFor []string `json:"required_for,omitempty"`

	// CreatedAt is when this agreement was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAfter is the duration after which signatures expire.
	// Zero means no expiration.
	ExpiresAfter time.Duration `json:"expires_after,omitempty"`

	// Active indicates if this agreement is currently active.
	Active bool `json:"active"`
}

// IsExpired checks if a signature would be expired after the given duration.
func (a *Agreement) IsExpired(signedAt time.Time) bool {
	if a.ExpiresAfter == 0 {
		return false
	}
	return time.Since(signedAt) > a.ExpiresAfter
}

// RequiredForUserType checks if this agreement is required for a user type.
func (a *Agreement) RequiredForUserType(userType string) bool {
	// If RequiredFor is empty, it's required for all users
	if len(a.RequiredFor) == 0 {
		return a.Active
	}

	// Check if user type is in the list
	for _, t := range a.RequiredFor {
		if t == userType {
			return a.Active
		}
	}
	return false
}

// =============================================================================
// USER AGREEMENT STRUCT
// =============================================================================

// UserAgreement represents a user's signature on an agreement.
type UserAgreement struct {
	// UserID is the user who signed the agreement.
	UserID string `json:"user_id"`

	// AgreementID is the agreement that was signed.
	AgreementID string `json:"agreement_id"`

	// Version is the version of the agreement that was signed.
	Version string `json:"version"`

	// SignedAt is when the agreement was signed.
	SignedAt time.Time `json:"signed_at"`

	// ExpiresAt is when this signature expires (if applicable).
	// Zero value means no expiration.
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// IPAddress is the IP address from which the agreement was signed.
	IPAddress string `json:"ip_address,omitempty"`

	// UserAgent is the user agent string from which the agreement was signed.
	UserAgent string `json:"user_agent,omitempty"`

	// Revoked indicates if this signature has been revoked.
	Revoked bool `json:"revoked"`

	// RevokedAt is when the signature was revoked.
	RevokedAt time.Time `json:"revoked_at,omitempty"`

	// RevokedReason is the reason for revocation.
	RevokedReason string `json:"revoked_reason,omitempty"`
}

// IsValid checks if this user agreement is currently valid.
func (ua *UserAgreement) IsValid() bool {
	if ua.Revoked {
		return false
	}
	if !ua.ExpiresAt.IsZero() && time.Now().After(ua.ExpiresAt) {
		return false
	}
	return true
}

// TimeRemaining returns the duration until expiration.
// Returns 0 if already expired or no expiration.
func (ua *UserAgreement) TimeRemaining() time.Duration {
	if ua.ExpiresAt.IsZero() {
		return 0
	}
	remaining := time.Until(ua.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// =============================================================================
// AGREEMENT MANAGER
// =============================================================================

// AgreementManager manages access agreements per PS-6.
type AgreementManager struct {
	// agreements maps agreement IDs to agreements.
	agreements map[string]*Agreement

	// userAgreements maps user IDs to their signed agreements.
	// Inner map is agreement ID -> UserAgreement.
	userAgreements map[string]map[string]*UserAgreement

	// storagePath is the path to the agreements storage file.
	storagePath string

	// auditLogger is the audit logger for PS-6 events.
	auditLogger *AuditLogger

	// mu protects concurrent access.
	mu sync.RWMutex
}

// AgreementManagerOption is a functional option for configuring AgreementManager.
type AgreementManagerOption func(*AgreementManager)

// WithAgreementAuditLogger sets the audit logger for PS-6 events.
func WithAgreementAuditLogger(logger *AuditLogger) AgreementManagerOption {
	return func(m *AgreementManager) {
		m.auditLogger = logger
	}
}

// WithAgreementStoragePath sets the storage path for agreements.
func WithAgreementStoragePath(path string) AgreementManagerOption {
	return func(m *AgreementManager) {
		m.storagePath = path
	}
}

// NewAgreementManager creates a new AgreementManager with the given options.
func NewAgreementManager(opts ...AgreementManagerOption) *AgreementManager {
	am := &AgreementManager{
		agreements:     make(map[string]*Agreement),
		userAgreements: make(map[string]map[string]*UserAgreement),
		storagePath:    defaultAgreementsPath(),
	}

	// Apply options
	for _, opt := range opts {
		opt(am)
	}

	// Default audit logger if not provided
	if am.auditLogger == nil {
		am.auditLogger = GlobalAuditLogger()
	}

	// Initialize with default agreements
	am.initializeDefaultAgreements()

	// Load from storage
	_ = am.Load()

	return am
}

// =============================================================================
// AGREEMENT OPERATIONS
// =============================================================================

// GetRequiredAgreements returns all agreements required for a user type.
func (m *AgreementManager) GetRequiredAgreements(userType string) []*Agreement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var required []*Agreement
	for _, agreement := range m.agreements {
		if agreement.RequiredForUserType(userType) {
			required = append(required, agreement)
		}
	}

	return required
}

// HasSignedAgreement checks if a user has signed a specific agreement.
func (m *AgreementManager) HasSignedAgreement(userID, agreementID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userAgrs, exists := m.userAgreements[userID]
	if !exists {
		return false
	}

	ua, exists := userAgrs[agreementID]
	if !exists {
		return false
	}

	return ua.IsValid()
}

// SignAgreement records a user's signature on an agreement.
func (m *AgreementManager) SignAgreement(userID, agreementID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify agreement exists
	agreement, exists := m.agreements[agreementID]
	if !exists {
		return fmt.Errorf("agreement not found: %s", agreementID)
	}

	if !agreement.Active {
		return fmt.Errorf("agreement is not active: %s", agreementID)
	}

	// Create user agreement entry
	now := time.Now()
	expiresAt := time.Time{}
	if agreement.ExpiresAfter > 0 {
		expiresAt = now.Add(agreement.ExpiresAfter)
	}

	ua := &UserAgreement{
		UserID:      userID,
		AgreementID: agreementID,
		Version:     agreement.Version,
		SignedAt:    now,
		ExpiresAt:   expiresAt,
	}

	// Store user agreement
	if m.userAgreements[userID] == nil {
		m.userAgreements[userID] = make(map[string]*UserAgreement)
	}
	m.userAgreements[userID][agreementID] = ua

	// Log the signing event
	m.logEvent("AGREEMENT_SIGNED", userID, map[string]string{
		"agreement_id":   agreementID,
		"agreement_type": string(agreement.Type),
		"version":        agreement.Version,
		"expires_at":     expiresAt.Format(time.RFC3339),
	})

	// Persist to storage
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save agreement: %w", err)
	}

	return nil
}

// GetAgreementContent returns the full text of an agreement.
func (m *AgreementManager) GetAgreementContent(agreementID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agreement, exists := m.agreements[agreementID]
	if !exists {
		return "", fmt.Errorf("agreement not found: %s", agreementID)
	}

	return agreement.Content, nil
}

// CheckAgreementsValid checks if all required agreements are signed and valid for a user.
func (m *AgreementManager) CheckAgreementsValid(userID string) (bool, []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var missing []string

	// Check all active agreements
	for _, agreement := range m.agreements {
		if !agreement.Active {
			continue
		}

		// Check if user has signed this agreement
		userAgrs, exists := m.userAgreements[userID]
		if !exists {
			missing = append(missing, agreement.ID)
			continue
		}

		ua, exists := userAgrs[agreement.ID]
		if !exists {
			missing = append(missing, agreement.ID)
			continue
		}

		// Check if signature is still valid
		if !ua.IsValid() {
			missing = append(missing, agreement.ID)
		}
	}

	return len(missing) == 0, missing
}

// GetExpiringAgreements returns user agreements expiring within the specified days.
func (m *AgreementManager) GetExpiringAgreements(days int) []*UserAgreement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threshold := time.Duration(days) * 24 * time.Hour
	var expiring []*UserAgreement

	for _, userAgrs := range m.userAgreements {
		for _, ua := range userAgrs {
			if ua.Revoked || ua.ExpiresAt.IsZero() {
				continue
			}

			remaining := ua.TimeRemaining()
			if remaining > 0 && remaining <= threshold {
				expiring = append(expiring, ua)
			}
		}
	}

	return expiring
}

// RevokeAgreement revokes a user's agreement signature.
func (m *AgreementManager) RevokeAgreement(userID, agreementID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userAgrs, exists := m.userAgreements[userID]
	if !exists {
		return fmt.Errorf("no agreements found for user: %s", userID)
	}

	ua, exists := userAgrs[agreementID]
	if !exists {
		return fmt.Errorf("agreement not signed by user: %s", agreementID)
	}

	// Revoke the agreement
	ua.Revoked = true
	ua.RevokedAt = time.Now()
	ua.RevokedReason = reason

	// Log the revocation event
	m.logEvent("AGREEMENT_REVOKED", userID, map[string]string{
		"agreement_id": agreementID,
		"reason":       reason,
	})

	// Persist to storage
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save revocation: %w", err)
	}

	return nil
}

// =============================================================================
// LISTING AND STATISTICS
// =============================================================================

// GetAgreement returns a specific agreement by ID.
func (m *AgreementManager) GetAgreement(agreementID string) (*Agreement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agreement, exists := m.agreements[agreementID]
	if !exists {
		return nil, fmt.Errorf("agreement not found: %s", agreementID)
	}

	return agreement, nil
}

// ListAgreements returns all agreements.
func (m *AgreementManager) ListAgreements() []*Agreement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agreements := make([]*Agreement, 0, len(m.agreements))
	for _, agreement := range m.agreements {
		agreements = append(agreements, agreement)
	}

	return agreements
}

// GetUserAgreements returns all agreements signed by a user.
func (m *AgreementManager) GetUserAgreements(userID string) []*UserAgreement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userAgrs, exists := m.userAgreements[userID]
	if !exists {
		return nil
	}

	agreements := make([]*UserAgreement, 0, len(userAgrs))
	for _, ua := range userAgrs {
		agreements = append(agreements, ua)
	}

	return agreements
}

// GetAgreementStats returns statistics about agreements.
func (m *AgreementManager) GetAgreementStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalUsers := len(m.userAgreements)
	totalAgreements := len(m.agreements)
	totalSigned := 0
	totalExpired := 0
	totalRevoked := 0

	for _, userAgrs := range m.userAgreements {
		for _, ua := range userAgrs {
			totalSigned++
			if ua.Revoked {
				totalRevoked++
			} else if !ua.IsValid() {
				totalExpired++
			}
		}
	}

	return map[string]interface{}{
		"total_agreements":  totalAgreements,
		"total_users":       totalUsers,
		"total_signed":      totalSigned,
		"total_expired":     totalExpired,
		"total_revoked":     totalRevoked,
		"compliance_status": "PS-6",
	}
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// agreementStorage represents the persisted data structure.
type agreementStorage struct {
	Agreements     map[string]*Agreement                      `json:"agreements"`
	UserAgreements map[string]map[string]*UserAgreement       `json:"user_agreements"`
	Version        string                                     `json:"version"`
}

// save persists agreements to storage.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (m *AgreementManager) save() error {
	storage := agreementStorage{
		Agreements:     m.agreements,
		UserAgreements: m.userAgreements,
		Version:        "1.0",
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agreements: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(m.storagePath, data, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write agreements file: %w", err)
	}

	return nil
}

// Load loads agreements from storage.
func (m *AgreementManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(m.storagePath); errors.Is(err, os.ErrNotExist) {
		// File doesn't exist - use defaults
		return nil
	}

	// Read file
	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return fmt.Errorf("failed to read agreements file: %w", err)
	}

	// Parse JSON
	var storage agreementStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to parse agreements file: %w", err)
	}

	// Load agreements (merge with defaults)
	for id, agreement := range storage.Agreements {
		m.agreements[id] = agreement
	}

	// Load user agreements
	m.userAgreements = storage.UserAgreements

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs an agreement-related event to the audit log.
func (m *AgreementManager) logEvent(eventType, userID string, metadata map[string]string) {
	if m.auditLogger == nil || !m.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: userID,
		Success:   true,
		Metadata:  metadata,
	}

	if err := m.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log agreement event %s: %v\n", eventType, err)
	}
}

// defaultAgreementsPath returns the default agreements storage path.
func defaultAgreementsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rigrun", AgreementsFileName)
}

// initializeDefaultAgreements sets up the default agreements.
func (m *AgreementManager) initializeDefaultAgreements() {
	// NDA Agreement
	m.agreements["nda"] = &Agreement{
		ID:           "nda",
		Type:         AgreementTypeNDA,
		Version:      "1.0",
		Content:      defaultNDAContent,
		RequiredFor:  []string{},
		CreatedAt:    time.Now(),
		ExpiresAfter: DefaultAgreementExpiration,
		Active:       true,
	}

	// AUP Agreement
	m.agreements["aup"] = &Agreement{
		ID:           "aup",
		Type:         AgreementTypeAUP,
		Version:      "1.0",
		Content:      defaultAUPContent,
		RequiredFor:  []string{},
		CreatedAt:    time.Now(),
		ExpiresAfter: DefaultAgreementExpiration,
		Active:       true,
	}

	// Security Awareness Agreement
	m.agreements["security"] = &Agreement{
		ID:           "security",
		Type:         AgreementTypeSecurity,
		Version:      "1.0",
		Content:      defaultSecurityContent,
		RequiredFor:  []string{},
		CreatedAt:    time.Now(),
		ExpiresAfter: DefaultAgreementExpiration,
		Active:       true,
	}

	// PII Handling Agreement
	m.agreements["pii"] = &Agreement{
		ID:           "pii",
		Type:         AgreementTypePII,
		Version:      "1.0",
		Content:      defaultPIIContent,
		RequiredFor:  []string{},
		CreatedAt:    time.Now(),
		ExpiresAfter: DefaultAgreementExpiration,
		Active:       true,
	}
}

// =============================================================================
// DEFAULT AGREEMENT CONTENT
// =============================================================================

const defaultNDAContent = `NON-DISCLOSURE AGREEMENT (NDA)

By accepting this agreement, you acknowledge that:

1. You will protect sensitive information from unauthorized disclosure
2. You will not share access credentials with unauthorized individuals
3. You will report any suspected security incidents immediately
4. You understand that violation may result in legal action

This agreement is required for NIST 800-53 PS-6 compliance.
`

const defaultAUPContent = `ACCEPTABLE USE POLICY (AUP)

By accepting this agreement, you acknowledge that:

1. You will use system resources only for authorized purposes
2. You will not attempt to circumvent security controls
3. You will not use the system for personal gain or malicious purposes
4. You will comply with all applicable laws and regulations

This agreement is required for NIST 800-53 PS-6 compliance.
`

const defaultSecurityContent = `SECURITY AWARENESS ACKNOWLEDGMENT

By accepting this agreement, you acknowledge that:

1. You have completed required security awareness training
2. You understand your responsibilities for protecting sensitive information
3. You will follow security policies and procedures
4. You will report security incidents and vulnerabilities

This agreement is required for NIST 800-53 PS-6 compliance.
`

const defaultPIIContent = `PII HANDLING AGREEMENT

By accepting this agreement, you acknowledge that:

1. You will protect Personally Identifiable Information (PII) from unauthorized access
2. You will only access PII when necessary for authorized purposes
3. You will not share PII with unauthorized parties
4. You will report any PII breaches immediately

This agreement is required for NIST 800-53 PS-6 compliance.
`

// =============================================================================
// GLOBAL AGREEMENT MANAGER
// =============================================================================

var (
	globalAgreementManager     *AgreementManager
	globalAgreementManagerOnce sync.Once
	globalAgreementManagerMu   sync.Mutex
)

// GlobalAgreementManager returns the global agreement manager instance.
func GlobalAgreementManager() *AgreementManager {
	globalAgreementManagerOnce.Do(func() {
		globalAgreementManager = NewAgreementManager()
	})
	return globalAgreementManager
}

// SetGlobalAgreementManager sets the global agreement manager instance.
func SetGlobalAgreementManager(manager *AgreementManager) {
	globalAgreementManagerMu.Lock()
	defer globalAgreementManagerMu.Unlock()
	globalAgreementManager = manager
}

// InitGlobalAgreementManager initializes the global agreement manager with options.
func InitGlobalAgreementManager(opts ...AgreementManagerOption) {
	globalAgreementManagerMu.Lock()
	defer globalAgreementManagerMu.Unlock()
	globalAgreementManager = NewAgreementManager(opts...)
}
