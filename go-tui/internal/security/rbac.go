// Package security provides RBAC (Role-Based Access Control) for NIST 800-53 AC-5 and AC-6 compliance.
//
// Implements:
//   - AC-5: Separation of Duties
//   - AC-6: Least Privilege
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// =============================================================================
// NIST 800-53 AU-9: RBAC INTEGRITY PROTECTION CONSTANTS
// =============================================================================

const (
	// RBACHMACKeyEnvVar is the environment variable for the RBAC HMAC key.
	RBACHMACKeyEnvVar = "RIGRUN_RBAC_HMAC_KEY"

	// rbacHMACKeySize is the size of the HMAC key in bytes (256 bits).
	rbacHMACKeySize = 32
)

// =============================================================================
// NIST 800-53 AC-5 & AC-6 CONSTANTS
// =============================================================================

// RBACRole represents a user role in the RBAC system.
type RBACRole string

const (
	// RBACRoleAdmin has full access to all operations, user management, and audit logs.
	RBACRoleAdmin RBACRole = "admin"

	// RBACRoleOperator can run queries, manage configuration, and view results.
	RBACRoleOperator RBACRole = "operator"

	// RBACRoleAuditor has read-only access to logs, can export reports, read-only config.
	RBACRoleAuditor RBACRole = "auditor"

	// RBACRoleUser has read-only access and can view own history only.
	RBACRoleUser RBACRole = "user"
)

// Permission represents a specific permission in the system.
type Permission string

const (
	// Query permissions
	PermRunQuery       Permission = "query:run"
	PermViewResults    Permission = "query:view_results"
	PermViewHistory    Permission = "query:view_history"
	PermViewOwnHistory Permission = "query:view_own_history"

	// Configuration permissions
	PermConfigRead   Permission = "config:read"
	PermConfigWrite  Permission = "config:write"
	PermConfigManage Permission = "config:manage"

	// User management permissions
	PermUserCreate Permission = "user:create"
	PermUserDelete Permission = "user:delete"
	PermUserModify Permission = "user:modify"
	PermUserView   Permission = "user:view"

	// Audit permissions
	PermAuditView   Permission = "audit:view"
	PermAuditExport Permission = "audit:export"
	PermAuditClear  Permission = "audit:clear"
	PermAuditManage Permission = "audit:manage"

	// System permissions
	PermSystemManage Permission = "system:manage"
	PermToolExecute  Permission = "tool:execute"
	PermToolApprove  Permission = "tool:approve"

	// Encryption permissions
	PermEncryptManage Permission = "encrypt:manage"
	PermEncryptRotate Permission = "encrypt:rotate"

	// Session permissions
	PermSessionManage Permission = "session:manage"
	PermSessionView   Permission = "session:view"
	PermSessionDelete Permission = "session:delete"
)

// =============================================================================
// ROLE PERMISSIONS MATRIX
// =============================================================================

// rolePermissions defines the permissions matrix for each role (AC-5, AC-6).
var rolePermissions = map[RBACRole][]Permission{
	RBACRoleAdmin: {
		// All permissions
		PermRunQuery,
		PermViewResults,
		PermViewHistory,
		PermViewOwnHistory,
		PermConfigRead,
		PermConfigWrite,
		PermConfigManage,
		PermUserCreate,
		PermUserDelete,
		PermUserModify,
		PermUserView,
		PermAuditView,
		PermAuditExport,
		PermAuditClear,
		PermAuditManage,
		PermSystemManage,
		PermToolExecute,
		PermToolApprove,
		PermEncryptManage,
		PermEncryptRotate,
		PermSessionManage,
		PermSessionView,
		PermSessionDelete,
	},
	RBACRoleOperator: {
		// Query and configuration management
		PermRunQuery,
		PermViewResults,
		PermViewHistory,
		PermViewOwnHistory,
		PermConfigRead,
		PermConfigWrite,
		PermToolExecute,
		PermSessionView,
		PermSessionManage,
	},
	RBACRoleAuditor: {
		// Read-only access to logs, reports, and configuration
		PermViewHistory,
		PermViewResults,
		PermConfigRead,
		PermAuditView,
		PermAuditExport,
		PermSessionView,
	},
	RBACRoleUser: {
		// Read-only access, view own history only
		PermViewOwnHistory,
		PermViewResults,
		PermConfigRead,
	},
}

// =============================================================================
// RBAC MANAGER
// =============================================================================

// RBACUserRole represents a user and their assigned role.
type RBACUserRole struct {
	UserID     string    `json:"user_id"`
	Role       RBACRole  `json:"role"`
	AssignedBy string    `json:"assigned_by"`
	AssignedAt time.Time `json:"assigned_at"`
}

// RBACManager manages role-based access control for the system.
// Implements NIST 800-53 AC-5 (Separation of Duties) and AC-6 (Least Privilege).
// NIST 800-53 AU-9: Includes integrity protection via HMAC signatures.
type RBACManager struct {
	// userRoles maps user IDs to their roles
	userRoles map[string]RBACUserRole

	// auditLogger logs all role changes for compliance
	auditLogger *AuditLogger

	// storagePath is where role assignments are persisted
	storagePath string

	// hmacKey is the key used for HMAC signature verification (AU-9)
	hmacKey []byte

	// mu protects concurrent access
	mu sync.RWMutex

	// denyAll is set to true when initialization fails, causing all permission checks to fail
	// SECURITY FIX: Fail-safe defaults - deny all access on initialization errors
	denyAll bool

	// SECURITY: Rate limiting prevents DoS via permission check flooding
	// Global rate limiter for all permission checks
	checkLimiter *rate.Limiter

	// Per-user rate limiters to prevent individual user abuse
	userLimiters map[string]*rate.Limiter
	limiterMu    sync.RWMutex

	// lastAccess tracks when each user limiter was last used for cleanup
	lastAccess   map[string]time.Time
	lastAccessMu sync.RWMutex
}

// RBACManagerOption is a functional option for configuring RBACManager.
type RBACManagerOption func(*RBACManager)

// WithRBACAuditLogger sets the audit logger for RBAC events.
func WithRBACAuditLogger(logger *AuditLogger) RBACManagerOption {
	return func(r *RBACManager) {
		r.auditLogger = logger
	}
}

// WithRBACStoragePath sets the storage path for role assignments.
func WithRBACStoragePath(path string) RBACManagerOption {
	return func(r *RBACManager) {
		r.storagePath = path
	}
}

// NewRBACManager creates a new RBAC manager.
// NIST 800-53 AU-9: Initializes HMAC key for integrity protection.
// Key priority: environment variable > key file > generate new key.
// SECURITY: Rate limiting prevents DoS via permission check flooding.
func NewRBACManager(opts ...RBACManagerOption) (*RBACManager, error) {
	rm := &RBACManager{
		userRoles:   make(map[string]RBACUserRole),
		auditLogger: GlobalAuditLogger(),
		storagePath: DefaultRBACStoragePath(),
		// SECURITY: Rate limiting prevents DoS via permission check flooding
		// Allow 100 checks per second burst, 10 sustained for global limiter
		checkLimiter: rate.NewLimiter(rate.Limit(10), 100),
		userLimiters: make(map[string]*rate.Limiter),
		lastAccess:   make(map[string]time.Time),
	}

	// Apply options
	for _, opt := range opts {
		opt(rm)
	}

	// AU-9: Initialize HMAC key for integrity protection
	hmacKey, err := rm.loadOrGenerateRBACHMACKey()
	if err != nil {
		return nil, fmt.Errorf("AU-9: failed to initialize HMAC key: %w", err)
	}
	rm.hmacKey = hmacKey

	// Load existing role assignments
	if err := rm.load(); err != nil {
		// If file doesn't exist, that's OK - we'll create it on first save
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load RBAC data: %w", err)
		}
	}

	// Start background cleanup of stale user limiters
	go rm.startLimiterCleanup()

	return rm, nil
}

// =============================================================================
// RATE LIMITING (SECURITY: DoS Prevention)
// =============================================================================

// getUserLimiter returns the rate limiter for a specific user, creating one if needed.
// SECURITY: Rate limiting prevents DoS via permission check flooding.
func (r *RBACManager) getUserLimiter(userID string) *rate.Limiter {
	r.limiterMu.RLock()
	limiter, ok := r.userLimiters[userID]
	r.limiterMu.RUnlock()

	if ok {
		// Update last access time
		r.lastAccessMu.Lock()
		r.lastAccess[userID] = time.Now()
		r.lastAccessMu.Unlock()
		return limiter
	}

	r.limiterMu.Lock()
	defer r.limiterMu.Unlock()

	// Double-check after acquiring write lock
	if limiter, ok = r.userLimiters[userID]; ok {
		return limiter
	}

	// Create new limiter: 20 checks/sec per user with burst of 50
	limiter = rate.NewLimiter(rate.Limit(20), 50)
	r.userLimiters[userID] = limiter

	// Track last access time for cleanup
	r.lastAccessMu.Lock()
	r.lastAccess[userID] = time.Now()
	r.lastAccessMu.Unlock()

	return limiter
}

// startLimiterCleanup runs periodic cleanup of stale user limiters.
// SECURITY: Prevents memory exhaustion from unlimited user limiter creation.
func (r *RBACManager) startLimiterCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.cleanupStaleLimiters()
	}
}

// cleanupStaleLimiters removes user limiters that haven't been used recently.
// SECURITY: Run periodically to prevent memory exhaustion.
func (r *RBACManager) cleanupStaleLimiters() {
	cutoff := time.Now().Add(-30 * time.Minute)

	r.lastAccessMu.RLock()
	staleUsers := make([]string, 0)
	for userID, lastAccess := range r.lastAccess {
		if lastAccess.Before(cutoff) {
			staleUsers = append(staleUsers, userID)
		}
	}
	r.lastAccessMu.RUnlock()

	if len(staleUsers) == 0 {
		return
	}

	r.limiterMu.Lock()
	r.lastAccessMu.Lock()
	for _, userID := range staleUsers {
		delete(r.userLimiters, userID)
		delete(r.lastAccess, userID)
	}
	r.lastAccessMu.Unlock()
	r.limiterMu.Unlock()
}

// logRateLimitExceeded logs when a rate limit is exceeded.
// ERROR HANDLING: Errors must not be silently ignored

func (r *RBACManager) logRateLimitExceeded(userID string, limitType string) {
	if r.auditLogger == nil || !r.auditLogger.IsEnabled() {
		return
	}

	metadata := map[string]string{
		"user_id":    userID,
		"limit_type": limitType,
		"event":      "rate_limit_exceeded",
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "RATE_LIMIT_EXCEEDED",
		SessionID: userID,
		Success:   false,
		Metadata:  metadata,
	}

	if err := r.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log rate limit event: %v\n", err)
	}
}

// =============================================================================
// PERMISSION CHECKING (AC-6: Least Privilege)
// =============================================================================

// CheckPermission checks if a user has a specific permission.
// Returns true if the user has the permission, false otherwise.
// SECURITY: Rate limiting prevents DoS via permission check flooding.
func (r *RBACManager) CheckPermission(userID string, permission Permission) bool {
	// SECURITY: Apply rate limiting before permission check
	// Global rate limit - protects against distributed attacks
	if r.checkLimiter != nil && !r.checkLimiter.Allow() {
		r.logRateLimitExceeded(userID, "global")
		r.logPermissionCheck(userID, permission, false)
		return false
	}

	// Per-user rate limit - protects against single-user abuse
	if r.userLimiters != nil {
		userLimiter := r.getUserLimiter(userID)
		if !userLimiter.Allow() {
			r.logRateLimitExceeded(userID, "per_user")
			r.logPermissionCheck(userID, permission, false)
			return false
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// SECURITY FIX: Fail-safe defaults - if initialization failed, deny all access
	if r.denyAll {
		r.logPermissionCheck(userID, permission, false)
		return false
	}

	userRole, exists := r.userRoles[userID]
	if !exists {
		// No role assigned - default to least privilege (no permissions)
		r.logPermissionCheck(userID, permission, false)
		return false
	}

	// Check if role has the permission
	permissions, ok := rolePermissions[userRole.Role]
	if !ok {
		r.logPermissionCheck(userID, permission, false)
		return false
	}

	hasPermission := false
	for _, perm := range permissions {
		if perm == permission {
			hasPermission = true
			break
		}
	}

	// SECURITY FIX: Log all permission checks for security audit trail
	r.logPermissionCheck(userID, permission, hasPermission)

	return hasPermission
}

// CheckPermissions checks if a user has all the specified permissions.
// Returns true only if the user has ALL permissions.
func (r *RBACManager) CheckPermissions(userID string, permissions ...Permission) bool {
	for _, perm := range permissions {
		if !r.CheckPermission(userID, perm) {
			return false
		}
	}
	return true
}

// CheckAnyPermission checks if a user has ANY of the specified permissions.
// Returns true if the user has at least one of the permissions.
func (r *RBACManager) CheckAnyPermission(userID string, permissions ...Permission) bool {
	for _, perm := range permissions {
		if r.CheckPermission(userID, perm) {
			return true
		}
	}
	return false
}

// CheckPermissionWithContext checks if a user has a specific permission with context support.
// This version waits for rate limit tokens (up to context deadline) instead of immediately failing.
// Returns an error if rate limited, context cancelled, or permission denied.
// SECURITY: Rate limiting prevents DoS via permission check flooding.
func (r *RBACManager) CheckPermissionWithContext(ctx context.Context, userID string, permission Permission) (bool, error) {
	// SECURITY: Apply rate limiting with context-aware waiting
	// Global rate limit - protects against distributed attacks
	if r.checkLimiter != nil {
		if err := r.checkLimiter.Wait(ctx); err != nil {
			r.logRateLimitExceeded(userID, "global")
			return false, fmt.Errorf("rate limit exceeded: too many permission checks: %w", err)
		}
	}

	// Per-user rate limit - protects against single-user abuse
	if r.userLimiters != nil {
		userLimiter := r.getUserLimiter(userID)
		if err := userLimiter.Wait(ctx); err != nil {
			r.logRateLimitExceeded(userID, "per_user")
			return false, fmt.Errorf("rate limit exceeded for user %s: %w", userID, err)
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// SECURITY FIX: Fail-safe defaults - if initialization failed, deny all access
	if r.denyAll {
		r.logPermissionCheck(userID, permission, false)
		return false, errors.New("RBAC system in deny-all mode due to initialization failure")
	}

	userRole, exists := r.userRoles[userID]
	if !exists {
		// No role assigned - default to least privilege (no permissions)
		r.logPermissionCheck(userID, permission, false)
		return false, nil
	}

	// Check if role has the permission
	permissions, ok := rolePermissions[userRole.Role]
	if !ok {
		r.logPermissionCheck(userID, permission, false)
		return false, nil
	}

	hasPermission := false
	for _, perm := range permissions {
		if perm == permission {
			hasPermission = true
			break
		}
	}

	// SECURITY FIX: Log all permission checks for security audit trail
	r.logPermissionCheck(userID, permission, hasPermission)

	return hasPermission, nil
}

// =============================================================================
// ROLE MANAGEMENT (AC-5: Separation of Duties)
// =============================================================================

// AssignRole assigns a role to a user.
// The assignedBy parameter should be the admin user ID performing the assignment.
// Returns an error if the assignedBy user is not an admin.
func (r *RBACManager) AssignRole(userID string, role RBACRole, assignedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate role
	if !r.isValidRole(role) {
		return fmt.Errorf("invalid role: %s", role)
	}

	// Check if assignedBy has admin permissions (AC-5: Separation of Duties)
	// Only admins can assign roles
	// SECURITY FIX: All authorization checks happen atomically under the same lock
	// to prevent TOCTOU (Time-of-Check-Time-of-Use) vulnerabilities
	if assignedBy != "" {
		assignedByRole, exists := r.userRoles[assignedBy]
		if !exists || assignedByRole.Role != RBACRoleAdmin {
			r.logRoleChange("ROLE_ASSIGN_DENIED", userID, "", string(role), assignedBy, "not_admin")
			return errors.New("AC-5: only administrators can assign roles")
		}
	}

	// Get previous role for audit logging
	previousRole := ""
	if existingRole, exists := r.userRoles[userID]; exists {
		previousRole = string(existingRole.Role)
	}

	// Assign the role
	r.userRoles[userID] = RBACUserRole{
		UserID:     userID,
		Role:       role,
		AssignedBy: assignedBy,
		AssignedAt: time.Now(),
	}

	// Save to disk
	if err := r.saveLocked(); err != nil {
		return fmt.Errorf("failed to save role assignment: %w", err)
	}

	// Audit log the role change
	r.logRoleChange("ROLE_ASSIGNED", userID, previousRole, string(role), assignedBy, "success")

	return nil
}

// RevokeRole removes a user's role assignment.
// The revokedBy parameter should be the admin user ID performing the revocation.
func (r *RBACManager) RevokeRole(userID string, revokedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if revokedBy has admin permissions (AC-5)
	// SECURITY FIX: All authorization checks happen atomically under the same lock
	// to prevent TOCTOU (Time-of-Check-Time-of-Use) vulnerabilities
	if revokedBy != "" {
		revokedByRole, exists := r.userRoles[revokedBy]
		if !exists || revokedByRole.Role != RBACRoleAdmin {
			r.logRoleChange("ROLE_REVOKE_DENIED", userID, "", "", revokedBy, "not_admin")
			return errors.New("AC-5: only administrators can revoke roles")
		}
	}

	// Get previous role for audit
	previousRole := ""
	if existingRole, exists := r.userRoles[userID]; exists {
		previousRole = string(existingRole.Role)
	} else {
		return fmt.Errorf("user has no role assigned: %s", userID)
	}

	// Revoke the role
	delete(r.userRoles, userID)

	// Save to disk
	if err := r.saveLocked(); err != nil {
		return fmt.Errorf("failed to save role revocation: %w", err)
	}

	// Audit log the revocation
	r.logRoleChange("ROLE_REVOKED", userID, previousRole, "", revokedBy, "success")

	return nil
}

// =============================================================================
// ROLE QUERIES
// =============================================================================

// GetUserRole returns the role assigned to a user.
// Returns empty string if no role is assigned.
func (r *RBACManager) GetUserRole(userID string) RBACRole {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if userRole, exists := r.userRoles[userID]; exists {
		return userRole.Role
	}
	return ""
}

// GetUserRoleDetails returns detailed information about a user's role.
func (r *RBACManager) GetUserRoleDetails(userID string) (RBACUserRole, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userRole, exists := r.userRoles[userID]
	return userRole, exists
}

// GetRolePermissions returns all permissions for a given role.
func (r *RBACManager) GetRolePermissions(role RBACRole) []Permission {
	permissions, ok := rolePermissions[role]
	if !ok {
		return []Permission{}
	}
	// Return a copy to prevent modification
	result := make([]Permission, len(permissions))
	copy(result, permissions)
	return result
}

// GetUserPermissions returns all permissions for a user based on their role.
func (r *RBACManager) GetUserPermissions(userID string) []Permission {
	role := r.GetUserRole(userID)
	if role == "" {
		return []Permission{}
	}
	return r.GetRolePermissions(role)
}

// ListRoles returns all available roles in the system.
func (r *RBACManager) ListRoles() []RBACRole {
	return []RBACRole{RBACRoleAdmin, RBACRoleOperator, RBACRoleAuditor, RBACRoleUser}
}

// ListUsers returns all users and their role assignments.
func (r *RBACManager) ListUsers() []RBACUserRole {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]RBACUserRole, 0, len(r.userRoles))
	for _, userRole := range r.userRoles {
		users = append(users, userRole)
	}
	return users
}

// =============================================================================
// ROLE DESCRIPTIONS
// =============================================================================

// RoleDescription provides a human-readable description of a role.
type RoleDescription struct {
	Role        RBACRole `json:"role"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// GetRoleDescription returns a detailed description of a role.
func (r *RBACManager) GetRoleDescription(role RBACRole) RoleDescription {
	descriptions := map[RBACRole]RoleDescription{
		RBACRoleAdmin: {
			Role:        RBACRoleAdmin,
			Name:        "Administrator",
			Description: "Full access to all operations, user management, and audit logs",
			Permissions: []string{
				"Manage users and roles",
				"Run queries and manage configuration",
				"View and manage audit logs",
				"Execute and approve tools",
				"Manage encryption and sessions",
			},
		},
		RBACRoleOperator: {
			Role:        RBACRoleOperator,
			Name:        "Operator",
			Description: "Run queries, manage configuration, and view results",
			Permissions: []string{
				"Run queries",
				"Manage configuration",
				"Execute tools",
				"View results and history",
				"Manage sessions",
			},
		},
		RBACRoleAuditor: {
			Role:        RBACRoleAuditor,
			Name:        "Auditor",
			Description: "Read-only access to logs, reports, and configuration",
			Permissions: []string{
				"View audit logs",
				"Export reports",
				"Read configuration",
				"View query history",
				"View session information",
			},
		},
		RBACRoleUser: {
			Role:        RBACRoleUser,
			Name:        "User",
			Description: "Read-only access, view own history only",
			Permissions: []string{
				"View own query history",
				"View results",
				"Read configuration",
			},
		},
	}

	desc, ok := descriptions[role]
	if !ok {
		return RoleDescription{Role: role, Name: string(role), Description: "Unknown role"}
	}
	return desc
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// rbacStorage represents the on-disk storage format.
// NIST 800-53 AU-9: Includes HMAC signature for integrity protection.
type rbacStorage struct {
	Version   int                     `json:"version"`
	UpdatedAt time.Time               `json:"updated_at"`
	Roles     map[string]RBACUserRole `json:"roles"`
	Signature string                  `json:"signature"` // AU-9: HMAC-SHA256 signature
}

// save persists role assignments to disk.
func (r *RBACManager) save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked()
}

// saveLocked persists role assignments (caller must hold write lock).
// NIST 800-53 AU-9: Signs data with HMAC before saving.
func (r *RBACManager) saveLocked() error {
	// Ensure directory exists
	dir := filepath.Dir(r.storagePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create RBAC storage directory: %w", err)
	}

	// Create storage structure (without signature first)
	storage := rbacStorage{
		Version:   1,
		UpdatedAt: time.Now(),
		Roles:     r.userRoles,
		Signature: "", // Clear for signing
	}

	// Marshal without signature for computing HMAC
	dataForSigning, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal RBAC data for signing: %w", err)
	}

	// AU-9: Compute HMAC signature
	storage.Signature = r.computeRBACHMAC(dataForSigning)

	// Marshal to JSON with signature
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal RBAC data: %w", err)
	}

	// Write to file atomically using temp file
	tmpPath := r.storagePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write RBAC data: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, r.storagePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename RBAC file: %w", err)
	}

	return nil
}

// load reads role assignments from disk.
// NIST 800-53 AU-9: Verifies HMAC signature before loading.
// SECURITY: Rejects tampered data - does NOT fall back to defaults.
func (r *RBACManager) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Read file
	data, err := os.ReadFile(r.storagePath)
	if err != nil {
		return err
	}

	// Unmarshal
	var storage rbacStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to unmarshal RBAC data: %w", err)
	}

	// AU-9: Verify HMAC signature
	savedSignature := storage.Signature
	storage.Signature = "" // Clear signature for verification

	// Re-marshal for signature verification
	dataForVerify, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("AU-9: failed to marshal RBAC data for verification: %w", err)
	}

	// Compute expected signature
	expectedSignature := r.computeRBACHMAC(dataForVerify)

	// Verify signature using constant-time comparison
	if !hmac.Equal([]byte(savedSignature), []byte(expectedSignature)) {
		// AU-9: CRITICAL - Tampering detected, DO NOT load data
		r.logIntegrityViolation("RBAC_INTEGRITY_VIOLATION", "HMAC signature mismatch - possible tampering")
		return errors.New("AU-9: RBAC integrity check failed - signature mismatch (possible tampering)")
	}

	// Signature verified - safe to load roles
	r.userRoles = storage.Roles
	if r.userRoles == nil {
		r.userRoles = make(map[string]RBACUserRole)
	}

	return nil
}

// =============================================================================
// AUDIT LOGGING
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logRoleChange logs a role change event for audit compliance.
func (r *RBACManager) logRoleChange(eventType, userID, previousRole, newRole, changedBy, status string) {
	if r.auditLogger == nil || !r.auditLogger.IsEnabled() {
		return
	}

	metadata := map[string]string{
		"user_id":       userID,
		"previous_role": previousRole,
		"new_role":      newRole,
		"changed_by":    changedBy,
		"status":        status,
		"nist_control":  "AC-5/AC-6",
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: changedBy,
		Success:   status == "success",
		Metadata:  metadata,
	}

	if err := r.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log role change event %s: %v\n", eventType, err)
	}
}

// logPermissionCheck logs a permission check event for audit compliance.
// SECURITY FIX: All permission checks are now logged for security monitoring.
func (r *RBACManager) logPermissionCheck(userID string, permission Permission, granted bool) {
	if r.auditLogger == nil || !r.auditLogger.IsEnabled() {
		return
	}

	eventType := "PERMISSION_CHECK_GRANTED"
	if !granted {
		eventType = "PERMISSION_CHECK_DENIED"
	}

	metadata := map[string]string{
		"user_id":      userID,
		"permission":   string(permission),
		"granted":      fmt.Sprintf("%t", granted),
		"nist_control": "AC-6",
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: userID,
		Success:   granted,
		Metadata:  metadata,
	}

	if err := r.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log permission check event %s: %v\n", eventType, err)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isValidRole checks if a role is valid.
func (r *RBACManager) isValidRole(role RBACRole) bool {
	validRoles := []RBACRole{RBACRoleAdmin, RBACRoleOperator, RBACRoleAuditor, RBACRoleUser}
	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

// DefaultRBACStoragePath returns the default path for RBAC storage.
func DefaultRBACStoragePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rigrun", "rbac.json")
}

// =============================================================================
// NIST 800-53 AU-9: HMAC INTEGRITY PROTECTION
// =============================================================================

// loadOrGenerateRBACHMACKey loads or generates the HMAC key for RBAC integrity.
// Key priority: 1) Environment variable, 2) Key file, 3) Generate new key
// SECURITY: If key cannot be loaded or generated, returns error (fail-secure).
// NOTE: Caller is responsible for zeroing the returned key when no longer needed
// via ZeroBytes(). The key is stored in RBACManager.hmacKey and zeroed in Close().
func (r *RBACManager) loadOrGenerateRBACHMACKey() ([]byte, error) {
	// Priority 1: Environment variable
	if envKey := os.Getenv(RBACHMACKeyEnvVar); envKey != "" {
		key, err := hex.DecodeString(envKey)
		if err != nil {
			return nil, fmt.Errorf("AU-9: invalid HMAC key in %s: must be hex-encoded: %w", RBACHMACKeyEnvVar, err)
		}
		if len(key) != rbacHMACKeySize {
			// SECURITY: Zero invalid key material to prevent memory disclosure
			ZeroBytes(key)
			return nil, fmt.Errorf("AU-9: HMAC key in %s must be %d bytes (got %d)", RBACHMACKeyEnvVar, rbacHMACKeySize, len(key))
		}
		return key, nil
	}

	// Priority 2: Key file
	keyPath := r.rbacHMACKeyPath()
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) == rbacHMACKeySize {
			return key, nil
		}
		// Key file exists but has wrong size - fail secure
		// SECURITY: Zero invalid key material to prevent memory disclosure
		ZeroBytes(key)
		return nil, fmt.Errorf("AU-9: RBAC HMAC key file has invalid size: expected %d, got %d", rbacHMACKeySize, len(key))
	}

	if !os.IsNotExist(err) {
		// File exists but couldn't be read - fail secure
		return nil, fmt.Errorf("AU-9: failed to read RBAC HMAC key file: %w", err)
	}

	// Priority 3: Generate new key (only if file doesn't exist)
	key = make([]byte, rbacHMACKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("AU-9: failed to generate RBAC HMAC key: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		// SECURITY: Zero key material on failure to prevent memory disclosure
		ZeroBytes(key)
		return nil, fmt.Errorf("AU-9: failed to create RBAC key directory: %w", err)
	}

	// Save key with restrictive permissions
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		// SECURITY: Zero key material on failure to prevent memory disclosure
		ZeroBytes(key)
		return nil, fmt.Errorf("AU-9: failed to save RBAC HMAC key: %w", err)
	}

	return key, nil
}

// Close cleans up RBAC manager resources and zeros sensitive key material.
// SECURITY: Zero key material to prevent memory disclosure
func (r *RBACManager) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hmacKey != nil {
		ZeroBytes(r.hmacKey)
		r.hmacKey = nil
	}
}

// rbacHMACKeyPath returns the path to the RBAC HMAC key file.
func (r *RBACManager) rbacHMACKeyPath() string {
	dir := filepath.Dir(r.storagePath)
	return filepath.Join(dir, ".rbac_hmac_key")
}

// computeRBACHMAC computes the HMAC-SHA256 of the given data.
func (r *RBACManager) computeRBACHMAC(data []byte) string {
	mac := hmac.New(sha256.New, r.hmacKey)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// logIntegrityViolation logs an AU-9 integrity violation event.
func (r *RBACManager) logIntegrityViolation(eventType, details string) {
	if r.auditLogger == nil || !r.auditLogger.IsEnabled() {
		return
	}

	metadata := map[string]string{
		"details":      details,
		"storage_path": r.storagePath,
		"nist_control": "AU-9",
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: "system",
		Success:   false,
		Metadata:  metadata,
	}

	if err := r.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log integrity violation event %s: %v\n", eventType, err)
	}
}

// VerifyRBACIntegrity verifies the integrity of the stored RBAC data.
// Returns nil if integrity check passes, error if tampering detected.
func (r *RBACManager) VerifyRBACIntegrity() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read file
	data, err := os.ReadFile(r.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file to verify
		}
		return fmt.Errorf("AU-9: failed to read RBAC file for integrity check: %w", err)
	}

	// Unmarshal
	var storage rbacStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("AU-9: RBAC file corrupted - invalid JSON: %w", err)
	}

	// Verify signature
	savedSignature := storage.Signature
	if savedSignature == "" {
		return errors.New("AU-9: RBAC file missing integrity signature")
	}

	storage.Signature = "" // Clear for verification
	dataForVerify, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("AU-9: failed to marshal RBAC data for verification: %w", err)
	}

	expectedSignature := r.computeRBACHMAC(dataForVerify)
	if !hmac.Equal([]byte(savedSignature), []byte(expectedSignature)) {
		return errors.New("AU-9: RBAC integrity check failed - signature mismatch (possible tampering)")
	}

	return nil
}

// =============================================================================
// GLOBAL RBAC MANAGER
// =============================================================================

var (
	globalRBACManager     *RBACManager
	globalRBACManagerOnce sync.Once
	globalRBACManagerMu   sync.Mutex
)

// GlobalRBACManager returns the global RBAC manager instance.
func GlobalRBACManager() *RBACManager {
	globalRBACManagerOnce.Do(func() {
		var err error
		globalRBACManager, err = NewRBACManager()
		if err != nil {
			// SECURITY FIX: If we can't create the manager, create one that denies all access
			// This prevents a scenario where an empty userRoles map bypasses authorization
			globalRBACManager = &RBACManager{
				userRoles: make(map[string]RBACUserRole),
				denyAll:   true, // Fail-safe: deny all permissions on initialization error
				// SECURITY: Rate limiting prevents DoS via permission check flooding
				checkLimiter: rate.NewLimiter(rate.Limit(10), 100),
				userLimiters: make(map[string]*rate.Limiter),
				lastAccess:   make(map[string]time.Time),
			}
		}
	})
	return globalRBACManager
}

// SetGlobalRBACManager sets the global RBAC manager instance.
func SetGlobalRBACManager(manager *RBACManager) {
	globalRBACManagerMu.Lock()
	defer globalRBACManagerMu.Unlock()
	globalRBACManager = manager
}

// InitGlobalRBACManager initializes the global RBAC manager with options.
func InitGlobalRBACManager(opts ...RBACManagerOption) error {
	globalRBACManagerMu.Lock()
	defer globalRBACManagerMu.Unlock()

	manager, err := NewRBACManager(opts...)
	if err != nil {
		return err
	}
	globalRBACManager = manager
	return nil
}
