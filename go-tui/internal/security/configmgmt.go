// configmgmt.go - NIST 800-53 CM-5 and CM-6 implementation for configuration management.
//
// Implements:
// - CM-5: Access Restrictions for Change (dual approval for sensitive changes)
// - CM-6: Configuration Settings (security baseline enforcement)
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SECURITY BASELINE (CM-6)
// =============================================================================

// SecurityBaseline defines the required security configuration settings.
// This implements NIST 800-53 CM-6 (Configuration Settings).
var SecurityBaseline = map[string]BaselineSetting{
	"session_timeout": {
		Name:        "session_timeout",
		Value:       "120",
		Unit:        "seconds",
		Description: "Maximum session idle timeout",
		Control:     "AC-12",
		Rationale:   "DoD STIG requires 15-30 minute timeout for IL5 systems",
	},
	"consent_required": {
		Name:        "consent_required",
		Value:       "true",
		Unit:        "boolean",
		Description: "Require consent banner acknowledgment",
		Control:     "AC-8",
		Rationale:   "DoD IL5 requires system use notification",
	},
	"encryption_enabled": {
		Name:        "encryption_enabled",
		Value:       "true",
		Unit:        "boolean",
		Description: "Enable data-at-rest encryption",
		Control:     "SC-28",
		Rationale:   "FIPS 140-2 validated encryption for IL5",
	},
	"audit_enabled": {
		Name:        "audit_enabled",
		Value:       "true",
		Unit:        "boolean",
		Description: "Enable audit logging",
		Control:     "AU-2, AU-3, AU-12",
		Rationale:   "Comprehensive audit trail required for IL5",
	},
	"max_login_attempts": {
		Name:        "max_login_attempts",
		Value:       "3",
		Unit:        "attempts",
		Description: "Maximum failed login attempts before lockout",
		Control:     "AC-7",
		Rationale:   "Prevent brute force attacks",
	},
	"lockout_duration": {
		Name:        "lockout_duration",
		Value:       "900",
		Unit:        "seconds",
		Description: "Account lockout duration (15 minutes)",
		Control:     "AC-7",
		Rationale:   "Balance security with availability",
	},
	"tls_min_version": {
		Name:        "tls_min_version",
		Value:       "1.2",
		Unit:        "version",
		Description: "Minimum TLS version for connections",
		Control:     "SC-8, SC-13",
		Rationale:   "TLS 1.2+ required for FIPS compliance",
	},
	"allowed_ciphers": {
		Name:        "allowed_ciphers",
		Value:       "AES-256-GCM",
		Unit:        "cipher suite",
		Description: "Allowed encryption cipher suites",
		Control:     "SC-13",
		Rationale:   "FIPS 140-2 validated ciphers only",
	},
}

// BaselineSetting represents a security baseline configuration item.
type BaselineSetting struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	Control     string `json:"control"`      // NIST 800-53 control ID
	Rationale   string `json:"rationale"`    // Compliance justification
}

// =============================================================================
// CHANGE REQUEST (CM-5)
// =============================================================================

// ChangeRequestStatus represents the status of a configuration change request.
type ChangeRequestStatus string

const (
	StatusPending  ChangeRequestStatus = "pending"
	StatusApproved ChangeRequestStatus = "approved"
	StatusRejected ChangeRequestStatus = "rejected"
	StatusApplied  ChangeRequestStatus = "applied"
)

// ChangeRequest represents a configuration change request.
// Implements NIST 800-53 CM-5 (Access Restrictions for Change).
type ChangeRequest struct {
	ID          string              `json:"id"`
	Setting     string              `json:"setting"`
	OldValue    string              `json:"old_value"`
	NewValue    string              `json:"new_value"`
	RequestedBy string              `json:"requested_by"`
	RequestedAt time.Time           `json:"requested_at"`
	ApprovedBy  string              `json:"approved_by,omitempty"`
	ApprovedAt  time.Time           `json:"approved_at,omitempty"`
	RejectedBy  string              `json:"rejected_by,omitempty"`
	RejectedAt  time.Time           `json:"rejected_at,omitempty"`
	Reason      string              `json:"reason,omitempty"`      // Rejection reason
	Status      ChangeRequestStatus `json:"status"`
	IsSensitive bool                `json:"is_sensitive"`          // Requires dual approval
}

// =============================================================================
// CONFIGURATION MANAGER
// =============================================================================

// ConfigManager manages configuration changes with security controls.
// Implements CM-5 (Access Restrictions for Change) and CM-6 (Configuration Settings).
type ConfigManager struct {
	mu                sync.RWMutex
	pendingChanges    map[string]*ChangeRequest // ID -> ChangeRequest
	changeHistory     []*ChangeRequest
	storePath         string
	sensitiveSettings map[string]bool           // Settings requiring dual approval
}

// NewConfigManager creates a new configuration manager.
func NewConfigManager() (*ConfigManager, error) {
	// Determine storage path
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	storePath := filepath.Join(home, ".rigrun", "config_changes.json")

	// Ensure directory exists
	dir := filepath.Dir(storePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config management directory: %w", err)
	}

	cm := &ConfigManager{
		pendingChanges: make(map[string]*ChangeRequest),
		changeHistory:  make([]*ChangeRequest, 0),
		storePath:      storePath,
		sensitiveSettings: map[string]bool{
			"encryption_enabled":   true,
			"audit_enabled":        true,
			"consent_required":     true,
			"tls_min_version":      true,
			"allowed_ciphers":      true,
			"max_login_attempts":   true,
		},
	}

	// Load existing changes
	if err := cm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load change history: %w", err)
	}

	return cm, nil
}

// RequestChange creates a new configuration change request.
// Implements CM-5: All changes must be documented and approved.
func (cm *ConfigManager) RequestChange(setting, oldValue, newValue, requestedBy string) (*ChangeRequest, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Generate unique ID
	id := fmt.Sprintf("CR-%d", time.Now().Unix())

	// Determine if this is a sensitive setting
	isSensitive := cm.sensitiveSettings[setting]

	cr := &ChangeRequest{
		ID:          id,
		Setting:     setting,
		OldValue:    oldValue,
		NewValue:    newValue,
		RequestedBy: requestedBy,
		RequestedAt: time.Now(),
		Status:      StatusPending,
		IsSensitive: isSensitive,
	}

	// Store pending change
	cm.pendingChanges[id] = cr
	cm.changeHistory = append(cm.changeHistory, cr)

	// Persist to disk
	if err := cm.save(); err != nil {
		return nil, fmt.Errorf("failed to save change request: %w", err)
	}

	// Log the change request
	AuditLogEvent("CONFIG_MGMT", "CHANGE_REQUESTED", map[string]string{
		"request_id":   id,
		"setting":      setting,
		"old_value":    oldValue,
		"new_value":    newValue,
		"requested_by": requestedBy,
		"sensitive":    fmt.Sprintf("%t", isSensitive),
	})

	return cr, nil
}

// ApproveChange approves a pending configuration change.
// Implements CM-5: Dual approval for sensitive settings.
func (cm *ConfigManager) ApproveChange(requestID, approverID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cr, exists := cm.pendingChanges[requestID]
	if !exists {
		return fmt.Errorf("change request not found: %s", requestID)
	}

	if cr.Status != StatusPending {
		return fmt.Errorf("change request is not pending (status: %s)", cr.Status)
	}

	// CM-5: Dual approval check for sensitive settings
	if cr.IsSensitive && cr.RequestedBy == approverID {
		return fmt.Errorf("sensitive settings require dual approval - approver must be different from requester")
	}

	// Approve the change
	cr.ApprovedBy = approverID
	cr.ApprovedAt = time.Now()
	cr.Status = StatusApproved

	// Remove from pending
	delete(cm.pendingChanges, requestID)

	// Persist to disk
	if err := cm.save(); err != nil {
		return fmt.Errorf("failed to save approval: %w", err)
	}

	// Log the approval
	AuditLogEvent("CONFIG_MGMT", "CHANGE_APPROVED", map[string]string{
		"request_id":  requestID,
		"setting":     cr.Setting,
		"approved_by": approverID,
		"sensitive":   fmt.Sprintf("%t", cr.IsSensitive),
	})

	return nil
}

// RejectChange rejects a pending configuration change.
func (cm *ConfigManager) RejectChange(requestID, rejecterID, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cr, exists := cm.pendingChanges[requestID]
	if !exists {
		return fmt.Errorf("change request not found: %s", requestID)
	}

	if cr.Status != StatusPending {
		return fmt.Errorf("change request is not pending (status: %s)", cr.Status)
	}

	// Reject the change
	cr.RejectedBy = rejecterID
	cr.RejectedAt = time.Now()
	cr.Reason = reason
	cr.Status = StatusRejected

	// Remove from pending
	delete(cm.pendingChanges, requestID)

	// Persist to disk
	if err := cm.save(); err != nil {
		return fmt.Errorf("failed to save rejection: %w", err)
	}

	// Log the rejection
	AuditLogEvent("CONFIG_MGMT", "CHANGE_REJECTED", map[string]string{
		"request_id":   requestID,
		"setting":      cr.Setting,
		"rejected_by":  rejecterID,
		"reason":       reason,
	})

	return nil
}

// ListPendingChanges returns all pending configuration change requests.
func (cm *ConfigManager) ListPendingChanges() []*ChangeRequest {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	pending := make([]*ChangeRequest, 0, len(cm.pendingChanges))
	for _, cr := range cm.pendingChanges {
		pending = append(pending, cr)
	}

	return pending
}

// GetChangeHistory returns the complete change history.
func (cm *ConfigManager) GetChangeHistory() []*ChangeRequest {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modification
	history := make([]*ChangeRequest, len(cm.changeHistory))
	copy(history, cm.changeHistory)

	return history
}

// ValidateSetting validates a configuration setting against the security baseline.
// Implements CM-6: Configuration Settings.
func (cm *ConfigManager) ValidateSetting(name, value string) error {
	baseline, exists := SecurityBaseline[name]
	if !exists {
		return fmt.Errorf("setting '%s' is not in the security baseline", name)
	}

	// Type-specific validation
	switch baseline.Unit {
	case "boolean":
		if value != "true" && value != "false" {
			return fmt.Errorf("setting '%s' must be 'true' or 'false', got '%s'", name, value)
		}
	case "seconds":
		var seconds int
		if _, err := fmt.Sscanf(value, "%d", &seconds); err != nil || seconds < 0 {
			return fmt.Errorf("setting '%s' must be a non-negative integer (seconds), got '%s'", name, value)
		}
	case "attempts":
		var attempts int
		if _, err := fmt.Sscanf(value, "%d", &attempts); err != nil || attempts <= 0 {
			return fmt.Errorf("setting '%s' must be a positive integer, got '%s'", name, value)
		}
	case "version":
		// Validate TLS version format (e.g., "1.2", "1.3")
		if !strings.HasPrefix(value, "1.") || len(value) < 3 {
			return fmt.Errorf("setting '%s' must be a valid TLS version (e.g., '1.2'), got '%s'", name, value)
		}
	case "cipher suite":
		// Validate cipher suite (basic check)
		if value == "" {
			return fmt.Errorf("setting '%s' cannot be empty", name)
		}
	}

	return nil
}

// ComplianceCheck verifies current configuration against the security baseline.
// Returns a map of setting name to compliance status (true = compliant).
func (cm *ConfigManager) ComplianceCheck(currentConfig map[string]string) map[string]ComplianceStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	results := make(map[string]ComplianceStatus)

	for settingName, baseline := range SecurityBaseline {
		currentValue, exists := currentConfig[settingName]

		status := ComplianceStatus{
			Setting:         settingName,
			BaselineValue:   baseline.Value,
			CurrentValue:    currentValue,
			Control:         baseline.Control,
			Description:     baseline.Description,
			Compliant:       exists && currentValue == baseline.Value,
			Exists:          exists,
		}

		if !exists {
			status.Issue = "Setting not configured"
		} else if currentValue != baseline.Value {
			status.Issue = fmt.Sprintf("Value mismatch: expected '%s', got '%s'", baseline.Value, currentValue)
		}

		results[settingName] = status
	}

	return results
}

// ComplianceStatus represents the compliance status of a configuration setting.
type ComplianceStatus struct {
	Setting       string `json:"setting"`
	BaselineValue string `json:"baseline_value"`
	CurrentValue  string `json:"current_value"`
	Control       string `json:"control"`
	Description   string `json:"description"`
	Compliant     bool   `json:"compliant"`
	Exists        bool   `json:"exists"`
	Issue         string `json:"issue,omitempty"`
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// save persists the change history to disk.
func (cm *ConfigManager) save() error {
	data := struct {
		Pending []*ChangeRequest `json:"pending"`
		History []*ChangeRequest `json:"history"`
	}{
		Pending: make([]*ChangeRequest, 0, len(cm.pendingChanges)),
		History: cm.changeHistory,
	}

	// Collect pending changes
	for _, cr := range cm.pendingChanges {
		data.Pending = append(data.Pending, cr)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change data: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFile(cm.storePath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write change data: %w", err)
	}

	return nil
}

// load reads the change history from disk.
func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.storePath)
	if err != nil {
		return err
	}

	var loaded struct {
		Pending []*ChangeRequest `json:"pending"`
		History []*ChangeRequest `json:"history"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to unmarshal change data: %w", err)
	}

	// Restore pending changes
	cm.pendingChanges = make(map[string]*ChangeRequest)
	for _, cr := range loaded.Pending {
		cm.pendingChanges[cr.ID] = cr
	}

	// Restore history
	cm.changeHistory = loaded.History

	return nil
}

// =============================================================================
// GLOBAL CONFIGURATION MANAGER
// =============================================================================

var (
	globalConfigMgr     *ConfigManager
	globalConfigMgrOnce sync.Once
	globalConfigMgrMu   sync.Mutex
)

// GlobalConfigManager returns the global configuration manager instance.
func GlobalConfigManager() *ConfigManager {
	globalConfigMgrOnce.Do(func() {
		mgr, err := NewConfigManager()
		if err != nil {
			// Log error but return a disabled manager
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize config manager: %v\n", err)
			globalConfigMgr = &ConfigManager{
				pendingChanges:    make(map[string]*ChangeRequest),
				changeHistory:     make([]*ChangeRequest, 0),
				sensitiveSettings: make(map[string]bool),
			}
		} else {
			globalConfigMgr = mgr
		}
	})
	return globalConfigMgr
}
