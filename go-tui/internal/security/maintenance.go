// maintenance.go - NIST 800-53 MA-4 (Nonlocal Maintenance) and MA-5 (Maintenance Personnel)
//
// Provides maintenance mode controls, personnel authorization, session tracking,
// and audit logging for all maintenance activities per DoD IL5 compliance.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// Maintenance session types
const (
	MaintenanceTypeRoutine   = "routine"    // Regular scheduled maintenance
	MaintenanceTypeEmergency = "emergency"  // Emergency maintenance
	MaintenanceTypeDiagnostic = "diagnostic" // Diagnostic/troubleshooting
	MaintenanceTypeUpdate    = "update"     // Software/config updates
)

// Maintenance action types
const (
	ActionConfigChange   = "config_change"   // Configuration modification
	ActionDataAccess     = "data_access"     // Data file access
	ActionSystemModify   = "system_modify"   // System file modification
	ActionLogAccess      = "log_access"      // Log file access
	ActionUserManagement = "user_management" // User/personnel management
	ActionKeyManagement  = "key_management"  // Cryptographic key operations
)

// MA-4: Maximum maintenance session duration (4 hours)
const MaxMaintenanceSessionDuration = 4 * time.Hour

// MA-5: Sensitive actions requiring supervisor approval
var sensitiveActions = map[string]bool{
	ActionKeyManagement:  true,
	ActionSystemModify:   true,
	ActionUserManagement: true,
}

// =============================================================================
// MAINTENANCE TYPES
// =============================================================================

// MaintenanceSession represents an active or historical maintenance session.
type MaintenanceSession struct {
	ID          string                `json:"id"`            // Session ID (MAINT-YYYYMMDD-XXXX)
	Type        string                `json:"type"`          // routine, emergency, diagnostic, update
	OperatorID  string                `json:"operator_id"`   // ID of maintenance personnel
	StartTime   time.Time             `json:"start_time"`    // Session start timestamp
	EndTime     *time.Time            `json:"end_time"`      // Session end timestamp (nil if active)
	Reason      string                `json:"reason"`        // Reason for maintenance
	Actions     []MaintenanceAction   `json:"actions"`       // All actions performed
	AutoExpired bool                  `json:"auto_expired"`  // True if session auto-expired
	ApprovedBy  string                `json:"approved_by"`   // Supervisor who approved (if required)
}

// MaintenanceAction represents a single action performed during maintenance.
type MaintenanceAction struct {
	Timestamp    time.Time `json:"timestamp"`     // When action occurred
	Action       string    `json:"action"`        // Action type
	Description  string    `json:"description"`   // Detailed description
	Target       string    `json:"target"`        // Target file/resource
	Success      bool      `json:"success"`       // Whether action succeeded
	Error        string    `json:"error"`         // Error message if failed
	ApprovedBy   string    `json:"approved_by"`   // Supervisor approval (if required)
}

// MaintenancePersonnel represents an authorized maintenance person.
type MaintenancePersonnel struct {
	ID           string    `json:"id"`            // Personnel ID
	Name         string    `json:"name"`          // Full name
	Role         string    `json:"role"`          // Role (technician, supervisor, admin)
	AddedAt      time.Time `json:"added_at"`      // When authorized
	AddedBy      string    `json:"added_by"`      // Who authorized them
	LastActivity *time.Time `json:"last_activity"` // Last maintenance activity
}

// MA-5: Personnel roles
const (
	MaintenanceRoleTechnician = "technician" // Can perform routine maintenance
	MaintenanceRoleSupervisor = "supervisor" // Can approve sensitive actions
	MaintenanceRoleAdmin      = "admin"      // Full maintenance privileges
)

// =============================================================================
// MAINTENANCE MANAGER
// =============================================================================

// MaintenanceManager manages maintenance mode and personnel per NIST 800-53 MA-4/MA-5.
type MaintenanceManager struct {
	sessionsFile  string                  // Path to sessions.json
	personnelFile string                  // Path to personnel.json
	currentSession *MaintenanceSession    // Active maintenance session (nil if none)
	sessions      []MaintenanceSession    // Historical sessions
	personnel     []MaintenancePersonnel  // Authorized personnel
	mu            sync.RWMutex
	auditLogger   *AuditLogger            // For logging maintenance actions
}

// NewMaintenanceManager creates a new maintenance manager.
func NewMaintenanceManager() (*MaintenanceManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	rigrunDir := filepath.Join(home, ".rigrun")
	sessionsFile := filepath.Join(rigrunDir, "maintenance_sessions.json")
	personnelFile := filepath.Join(rigrunDir, "maintenance_personnel.json")

	m := &MaintenanceManager{
		sessionsFile:  sessionsFile,
		personnelFile: personnelFile,
		sessions:      make([]MaintenanceSession, 0),
		personnel:     make([]MaintenancePersonnel, 0),
		auditLogger:   GlobalAuditLogger(),
	}

	// Load existing data
	if err := m.loadSessions(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load maintenance sessions: %w", err)
	}
	if err := m.loadPersonnel(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load maintenance personnel: %w", err)
	}

	// Check for active session and auto-expire if needed
	m.checkAndExpireSession()

	return m, nil
}

// =============================================================================
// MAINTENANCE MODE MANAGEMENT (MA-4)
// =============================================================================

// StartMaintenanceMode enters maintenance mode with the specified operator and reason.
// Returns the new maintenance session.
func (m *MaintenanceManager) StartMaintenanceMode(operatorID, maintenanceType, reason string) (*MaintenanceSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already in maintenance mode
	if m.currentSession != nil {
		return nil, fmt.Errorf("maintenance mode already active (session: %s)", m.currentSession.ID)
	}

	// Validate operator is authorized
	if !m.isAuthorizedLocked(operatorID) {
		// Log unauthorized attempt
		m.auditLogger.LogEvent("MAINTENANCE", "UNAUTHORIZED_START_ATTEMPT", map[string]string{
			"operator_id": operatorID,
			"reason":      reason,
		})
		return nil, fmt.Errorf("operator '%s' is not authorized for maintenance", operatorID)
	}

	// Validate maintenance type
	if !isValidMaintenanceType(maintenanceType) {
		return nil, fmt.Errorf("invalid maintenance type '%s'", maintenanceType)
	}

	// Create new session
	session := &MaintenanceSession{
		ID:         generateMaintenanceSessionID(),
		Type:       maintenanceType,
		OperatorID: operatorID,
		StartTime:  time.Now(),
		Reason:     reason,
		Actions:    make([]MaintenanceAction, 0),
	}

	m.currentSession = session

	// Log maintenance start
	m.auditLogger.LogEvent(session.ID, "MAINTENANCE_START", map[string]string{
		"operator_id": operatorID,
		"type":        maintenanceType,
		"reason":      reason,
	})

	// Update personnel last activity
	m.updatePersonnelActivityLocked(operatorID)

	return session, nil
}

// EndMaintenanceMode exits maintenance mode.
func (m *MaintenanceManager) EndMaintenanceMode(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession == nil {
		return fmt.Errorf("no active maintenance session")
	}

	if m.currentSession.ID != sessionID {
		return fmt.Errorf("session ID mismatch: expected %s, got %s", m.currentSession.ID, sessionID)
	}

	// End session
	now := time.Now()
	m.currentSession.EndTime = &now

	// Add to history
	m.sessions = append(m.sessions, *m.currentSession)

	// Log maintenance end
	m.auditLogger.LogEvent(sessionID, "MAINTENANCE_END", map[string]string{
		"operator_id": m.currentSession.OperatorID,
		"duration":    fmt.Sprintf("%.2f", time.Since(m.currentSession.StartTime).Minutes()),
		"actions":     fmt.Sprintf("%d", len(m.currentSession.Actions)),
	})

	// Clear current session
	m.currentSession = nil

	// Save to disk
	return m.saveSessionsLocked()
}

// IsMaintenanceMode returns true if currently in maintenance mode.
func (m *MaintenanceManager) IsMaintenanceMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSession != nil
}

// GetCurrentSession returns the current maintenance session (nil if none).
func (m *MaintenanceManager) GetCurrentSession() *MaintenanceSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentSession == nil {
		return nil
	}
	// Return a copy
	session := *m.currentSession
	return &session
}

// =============================================================================
// MAINTENANCE ACTION LOGGING (MA-4)
// =============================================================================

// LogMaintenanceAction logs an action performed during maintenance.
func (m *MaintenanceManager) LogMaintenanceAction(actionType, description, target string, success bool, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession == nil {
		return fmt.Errorf("no active maintenance session")
	}

	// Create action record
	action := MaintenanceAction{
		Timestamp:   time.Now(),
		Action:      actionType,
		Description: description,
		Target:      target,
		Success:     success,
		Error:       errorMsg,
	}

	// Add to current session
	m.currentSession.Actions = append(m.currentSession.Actions, action)

	// Log to audit
	metadata := map[string]string{
		"session_id":  m.currentSession.ID,
		"operator_id": m.currentSession.OperatorID,
		"action":      actionType,
		"target":      target,
		"success":     fmt.Sprintf("%t", success),
	}
	if errorMsg != "" {
		metadata["error"] = errorMsg
	}

	m.auditLogger.LogEvent(m.currentSession.ID, "MAINTENANCE_ACTION", metadata)

	return nil
}

// =============================================================================
// SUPERVISOR APPROVAL (MA-5)
// =============================================================================

// RequireApproval checks if an action requires supervisor approval and validates it.
// Returns error if approval is required but not provided.
func (m *MaintenanceManager) RequireApproval(actionType, supervisorID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentSession == nil {
		return fmt.Errorf("no active maintenance session")
	}

	// Check if this action requires approval
	if !sensitiveActions[actionType] {
		return nil // No approval needed
	}

	// Validate supervisor ID
	if supervisorID == "" {
		return fmt.Errorf("action '%s' requires supervisor approval", actionType)
	}

	// Verify supervisor is authorized and has appropriate role
	personnel := m.findPersonnelLocked(supervisorID)
	if personnel == nil {
		return fmt.Errorf("supervisor '%s' not found in authorized personnel", supervisorID)
	}

	if personnel.Role != MaintenanceRoleSupervisor && personnel.Role != MaintenanceRoleAdmin {
		return fmt.Errorf("'%s' is not authorized to approve sensitive actions (role: %s)", supervisorID, personnel.Role)
	}

	// Record approval in current action
	if len(m.currentSession.Actions) > 0 {
		m.currentSession.Actions[len(m.currentSession.Actions)-1].ApprovedBy = supervisorID
	}

	// Log approval
	m.auditLogger.LogEvent(m.currentSession.ID, "MAINTENANCE_APPROVAL", map[string]string{
		"supervisor_id": supervisorID,
		"action":        actionType,
	})

	return nil
}

// =============================================================================
// PERSONNEL MANAGEMENT (MA-5)
// =============================================================================

// ValidateMaintenancePersonnel checks if a person is authorized for maintenance.
func (m *MaintenanceManager) ValidateMaintenancePersonnel(operatorID string) (bool, *MaintenancePersonnel) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	personnel := m.findPersonnelLocked(operatorID)
	if personnel == nil {
		return false, nil
	}

	return true, personnel
}

// AddPersonnel adds an authorized maintenance person.
func (m *MaintenanceManager) AddPersonnel(id, name, role, addedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate role
	if !isValidRole(role) {
		return fmt.Errorf("invalid role '%s', must be: technician, supervisor, or admin", role)
	}

	// Check if already exists
	if m.findPersonnelLocked(id) != nil {
		return fmt.Errorf("personnel '%s' already authorized", id)
	}

	// Create personnel record
	personnel := MaintenancePersonnel{
		ID:      id,
		Name:    name,
		Role:    role,
		AddedAt: time.Now(),
		AddedBy: addedBy,
	}

	m.personnel = append(m.personnel, personnel)

	// Log addition
	m.auditLogger.LogEvent("MAINTENANCE", "PERSONNEL_ADDED", map[string]string{
		"personnel_id": id,
		"name":         name,
		"role":         role,
		"added_by":     addedBy,
	})

	return m.savePersonnelLocked()
}

// RemovePersonnel removes authorization for a maintenance person.
func (m *MaintenanceManager) RemovePersonnel(id, removedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove
	found := false
	for i, p := range m.personnel {
		if p.ID == id {
			m.personnel = append(m.personnel[:i], m.personnel[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("personnel '%s' not found", id)
	}

	// Log removal
	m.auditLogger.LogEvent("MAINTENANCE", "PERSONNEL_REMOVED", map[string]string{
		"personnel_id": id,
		"removed_by":   removedBy,
	})

	return m.savePersonnelLocked()
}

// GetAuthorizedPersonnel returns all authorized maintenance personnel.
func (m *MaintenanceManager) GetAuthorizedPersonnel() []MaintenancePersonnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	result := make([]MaintenancePersonnel, len(m.personnel))
	copy(result, m.personnel)
	return result
}

// =============================================================================
// MAINTENANCE HISTORY
// =============================================================================

// GetMaintenanceHistory returns all maintenance sessions, optionally filtered.
func (m *MaintenanceManager) GetMaintenanceHistory(limit int) []MaintenanceSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all sessions (newest first)
	result := make([]MaintenanceSession, len(m.sessions))
	copy(result, m.sessions)

	// PERFORMANCE: O(n log n) standard library sort
	// Sort by start time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.After(result[j].StartTime)
	})

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetSessionByID returns a specific session by ID.
func (m *MaintenanceManager) GetSessionByID(sessionID string) (*MaintenanceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check current session first
	if m.currentSession != nil && m.currentSession.ID == sessionID {
		session := *m.currentSession
		return &session, nil
	}

	// Search history
	for _, s := range m.sessions {
		if s.ID == sessionID {
			session := s
			return &session, nil
		}
	}

	return nil, fmt.Errorf("session '%s' not found", sessionID)
}

// =============================================================================
// SESSION AUTO-EXPIRATION (MA-4)
// =============================================================================

// checkAndExpireSession checks if the current session has exceeded max duration.
func (m *MaintenanceManager) checkAndExpireSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession == nil {
		return
	}

	// Check if session has exceeded max duration
	elapsed := time.Since(m.currentSession.StartTime)
	if elapsed > MaxMaintenanceSessionDuration {
		// Auto-expire the session
		now := time.Now()
		m.currentSession.EndTime = &now
		m.currentSession.AutoExpired = true

		// Add to history
		m.sessions = append(m.sessions, *m.currentSession)

		// Log auto-expiration
		m.auditLogger.LogEvent(m.currentSession.ID, "MAINTENANCE_AUTO_EXPIRED", map[string]string{
			"operator_id": m.currentSession.OperatorID,
			"duration":    fmt.Sprintf("%.2f", elapsed.Minutes()),
		})

		// Clear current session
		m.currentSession = nil

		// Save to disk
		m.saveSessionsLocked()
	}
}

// CheckSessionExpiration checks if current session should be expired and does so.
func (m *MaintenanceManager) CheckSessionExpiration() bool {
	m.checkAndExpireSession()
	return !m.IsMaintenanceMode()
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// loadSessions loads maintenance sessions from disk.
func (m *MaintenanceManager) loadSessions() error {
	data, err := os.ReadFile(m.sessionsFile)
	if err != nil {
		return err
	}

	var loaded struct {
		CurrentSession *MaintenanceSession  `json:"current_session"`
		Sessions       []MaintenanceSession `json:"sessions"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to parse sessions file: %w", err)
	}

	m.currentSession = loaded.CurrentSession
	m.sessions = loaded.Sessions

	return nil
}

// saveSessionsLocked saves maintenance sessions to disk (caller must hold lock).
func (m *MaintenanceManager) saveSessionsLocked() error {
	data := struct {
		CurrentSession *MaintenanceSession  `json:"current_session"`
		Sessions       []MaintenanceSession `json:"sessions"`
	}{
		CurrentSession: m.currentSession,
		Sessions:       m.sessions,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(m.sessionsFile, jsonData, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write sessions file: %w", err)
	}

	return nil
}

// loadPersonnel loads authorized personnel from disk.
func (m *MaintenanceManager) loadPersonnel() error {
	data, err := os.ReadFile(m.personnelFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &m.personnel); err != nil {
		return fmt.Errorf("failed to parse personnel file: %w", err)
	}

	return nil
}

// savePersonnelLocked saves authorized personnel to disk (caller must hold lock).
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (m *MaintenanceManager) savePersonnelLocked() error {
	jsonData, err := json.MarshalIndent(m.personnel, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal personnel: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(m.personnelFile, jsonData, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write personnel file: %w", err)
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isAuthorizedLocked checks if operator is authorized (caller must hold read lock).
func (m *MaintenanceManager) isAuthorizedLocked(operatorID string) bool {
	return m.findPersonnelLocked(operatorID) != nil
}

// findPersonnelLocked finds personnel by ID (caller must hold read lock).
func (m *MaintenanceManager) findPersonnelLocked(id string) *MaintenancePersonnel {
	for i := range m.personnel {
		if m.personnel[i].ID == id {
			return &m.personnel[i]
		}
	}
	return nil
}

// updatePersonnelActivityLocked updates last activity time (caller must hold lock).
func (m *MaintenanceManager) updatePersonnelActivityLocked(operatorID string) {
	personnel := m.findPersonnelLocked(operatorID)
	if personnel != nil {
		now := time.Now()
		personnel.LastActivity = &now
		m.savePersonnelLocked()
	}
}

// generateMaintenanceSessionID generates a unique maintenance session ID.
func generateMaintenanceSessionID() string {
	return fmt.Sprintf("MAINT-%s-%04d",
		time.Now().Format("20060102"),
		time.Now().Unix()%10000)
}

// isValidMaintenanceType checks if maintenance type is valid.
func isValidMaintenanceType(t string) bool {
	return t == MaintenanceTypeRoutine ||
		t == MaintenanceTypeEmergency ||
		t == MaintenanceTypeDiagnostic ||
		t == MaintenanceTypeUpdate
}

// isValidRole checks if personnel role is valid.
func isValidRole(role string) bool {
	return role == MaintenanceRoleTechnician ||
		role == MaintenanceRoleSupervisor ||
		role == MaintenanceRoleAdmin
}

// =============================================================================
// GLOBAL MAINTENANCE MANAGER
// =============================================================================

var (
	globalMaintenanceManager     *MaintenanceManager
	globalMaintenanceManagerOnce sync.Once
	globalMaintenanceManagerMu   sync.Mutex
)

// GlobalMaintenanceManager returns the global maintenance manager instance.
func GlobalMaintenanceManager() *MaintenanceManager {
	globalMaintenanceManagerOnce.Do(func() {
		var err error
		globalMaintenanceManager, err = NewMaintenanceManager()
		if err != nil {
			// If we can't create the manager, create a disabled one
			globalMaintenanceManager = &MaintenanceManager{
				sessions:  make([]MaintenanceSession, 0),
				personnel: make([]MaintenancePersonnel, 0),
			}
		}
	})
	return globalMaintenanceManager
}

// SetGlobalMaintenanceManager sets the global maintenance manager instance.
func SetGlobalMaintenanceManager(manager *MaintenanceManager) {
	globalMaintenanceManagerMu.Lock()
	defer globalMaintenanceManagerMu.Unlock()
	globalMaintenanceManager = manager
}
