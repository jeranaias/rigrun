// incident.go - NIST 800-53 IR-6: Incident Reporting implementation.
//
// Provides incident reporting capability, tracking, and management for
// DoD IL5 compliance. Supports incident lifecycle management, severity
// classification, and export for external reporting.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// Incident severity levels per NIST 800-53 IR-6
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Incident categories
const (
	CategorySecurity     = "security"
	CategoryAvailability = "availability"
	CategoryIntegrity    = "integrity"
	CategorySpillage     = "spillage"
)

// Incident statuses
const (
	StatusOpen          = "open"
	StatusInvestigating = "investigating"
	StatusResolved      = "resolved"
	StatusClosed        = "closed"
)

// =============================================================================
// INCIDENT TYPES
// =============================================================================

// Incident represents a security incident per NIST 800-53 IR-6.
type Incident struct {
	ID          string          `json:"id"`           // INC-YYYYMMDD-XXXX format
	CreatedAt   time.Time       `json:"created_at"`   // When incident was reported
	ReportedBy  string          `json:"reported_by"`  // Username who reported
	Severity    string          `json:"severity"`     // low, medium, high, critical
	Category    string          `json:"category"`     // security, availability, integrity, spillage
	Description string          `json:"description"`  // Detailed description
	Status      string          `json:"status"`       // open, investigating, resolved, closed
	Resolution  string          `json:"resolution"`   // How it was resolved
	ResolvedAt  *time.Time      `json:"resolved_at"`  // When resolved
	ResolvedBy  string          `json:"resolved_by"`  // Who resolved it
	AuditTrail  []IncidentEvent `json:"audit_trail"`  // History of all changes
}

// IncidentEvent represents a change to an incident.
type IncidentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`  // CREATED, STATUS_CHANGE, NOTE_ADDED, RESOLVED, CLOSED
	User      string    `json:"user"`    // Who performed the action
	Details   string    `json:"details"` // Additional information
}

// IncidentFilter defines filtering criteria for listing incidents.
type IncidentFilter struct {
	Status     string    `json:"status,omitempty"`
	Severity   string    `json:"severity,omitempty"`
	Category   string    `json:"category,omitempty"`
	Since      time.Time `json:"since,omitempty"`
	Until      time.Time `json:"until,omitempty"`
	ReportedBy string    `json:"reported_by,omitempty"`
	Limit      int       `json:"limit,omitempty"`
}

// =============================================================================
// INCIDENT MANAGER
// =============================================================================

// IncidentManager manages incident reporting and tracking per NIST 800-53 IR-6.
type IncidentManager struct {
	incidentFile string     // Path to incidents.json
	incidents    []Incident // Cached incidents
	mu           sync.RWMutex
	webhookURL   string // Optional webhook for external reporting
}

// NewIncidentManager creates a new incident manager.
func NewIncidentManager() (*IncidentManager, error) {
	path, err := DefaultIncidentPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine incident file path: %w", err)
	}

	m := &IncidentManager{
		incidentFile: path,
		incidents:    make([]Incident, 0),
	}

	// Load existing incidents
	if err := m.load(); err != nil {
		// If file doesn't exist, that's okay - start fresh
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load incidents: %w", err)
		}
	}

	return m, nil
}

// DefaultIncidentPath returns the default incident file path (~/.rigrun/incidents.json).
func DefaultIncidentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rigrun", "incidents.json"), nil
}

// =============================================================================
// INCIDENT MANAGEMENT
// =============================================================================

// Report creates a new incident and returns it.
// This is the primary method for NIST 800-53 IR-6 incident reporting.
func (m *IncidentManager) Report(severity, category, description string) (*Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate inputs
	if !isValidSeverity(severity) {
		return nil, fmt.Errorf("invalid severity '%s', must be one of: low, medium, high, critical", severity)
	}
	if !isValidCategory(category) {
		return nil, fmt.Errorf("invalid category '%s', must be one of: security, availability, integrity, spillage", category)
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}

	// Get current user
	user := getCurrentUser()

	// Generate incident ID
	id := m.generateID()

	now := time.Now()
	incident := Incident{
		ID:          id,
		CreatedAt:   now,
		ReportedBy:  user,
		Severity:    strings.ToLower(severity),
		Category:    strings.ToLower(category),
		Description: description,
		Status:      StatusOpen,
		AuditTrail: []IncidentEvent{
			{
				Timestamp: now,
				Action:    "CREATED",
				User:      user,
				Details:   fmt.Sprintf("Incident created with severity=%s, category=%s", severity, category),
			},
		},
	}

	m.incidents = append(m.incidents, incident)

	// Save to file
	if err := m.save(); err != nil {
		return nil, fmt.Errorf("failed to save incident: %w", err)
	}

	// Log to audit
	AuditLogEvent("", "INCIDENT_REPORTED", map[string]string{
		"incident_id": id,
		"severity":    severity,
		"category":    category,
	})

	// Send to webhook if configured
	if m.webhookURL != "" {
		go m.sendWebhook(&incident, "created")
	}

	return &incident, nil
}

// UpdateStatus updates the status of an incident.
func (m *IncidentManager) UpdateStatus(incidentID, status, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate status
	if !isValidStatus(status) {
		return fmt.Errorf("invalid status '%s', must be one of: open, investigating, resolved, closed", status)
	}

	// Find incident
	idx := m.findIncidentIndex(incidentID)
	if idx < 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	user := getCurrentUser()
	now := time.Now()

	// Update status
	oldStatus := m.incidents[idx].Status
	m.incidents[idx].Status = strings.ToLower(status)

	// Add audit event
	event := IncidentEvent{
		Timestamp: now,
		Action:    "STATUS_CHANGE",
		User:      user,
		Details:   fmt.Sprintf("Status changed from %s to %s", oldStatus, status),
	}
	if notes != "" {
		event.Details += ": " + notes
	}
	m.incidents[idx].AuditTrail = append(m.incidents[idx].AuditTrail, event)

	// Save
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save incident update: %w", err)
	}

	// Log to audit
	AuditLogEvent("", "INCIDENT_UPDATED", map[string]string{
		"incident_id": incidentID,
		"old_status":  oldStatus,
		"new_status":  status,
	})

	return nil
}

// AddNote adds a note to an incident's audit trail.
func (m *IncidentManager) AddNote(incidentID, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if note == "" {
		return fmt.Errorf("note cannot be empty")
	}

	// Find incident
	idx := m.findIncidentIndex(incidentID)
	if idx < 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	user := getCurrentUser()
	now := time.Now()

	// Add note event
	event := IncidentEvent{
		Timestamp: now,
		Action:    "NOTE_ADDED",
		User:      user,
		Details:   note,
	}
	m.incidents[idx].AuditTrail = append(m.incidents[idx].AuditTrail, event)

	// Save
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	return nil
}

// Resolve marks an incident as resolved with a resolution description.
func (m *IncidentManager) Resolve(incidentID, resolution string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if resolution == "" {
		return fmt.Errorf("resolution description is required")
	}

	// Find incident
	idx := m.findIncidentIndex(incidentID)
	if idx < 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	// Check if already resolved or closed
	if m.incidents[idx].Status == StatusResolved || m.incidents[idx].Status == StatusClosed {
		return fmt.Errorf("incident is already %s", m.incidents[idx].Status)
	}

	user := getCurrentUser()
	now := time.Now()

	// Update incident
	m.incidents[idx].Status = StatusResolved
	m.incidents[idx].Resolution = resolution
	m.incidents[idx].ResolvedAt = &now
	m.incidents[idx].ResolvedBy = user

	// Add audit event
	event := IncidentEvent{
		Timestamp: now,
		Action:    "RESOLVED",
		User:      user,
		Details:   "Resolution: " + resolution,
	}
	m.incidents[idx].AuditTrail = append(m.incidents[idx].AuditTrail, event)

	// Save
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save resolution: %w", err)
	}

	// Log to audit
	AuditLogEvent("", "INCIDENT_RESOLVED", map[string]string{
		"incident_id": incidentID,
		"resolved_by": user,
	})

	// Send to webhook
	if m.webhookURL != "" {
		go m.sendWebhook(&m.incidents[idx], "resolved")
	}

	return nil
}

// Close marks a resolved incident as closed.
func (m *IncidentManager) Close(incidentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find incident
	idx := m.findIncidentIndex(incidentID)
	if idx < 0 {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	// Must be resolved first
	if m.incidents[idx].Status != StatusResolved {
		return fmt.Errorf("incident must be resolved before closing (current status: %s)", m.incidents[idx].Status)
	}

	user := getCurrentUser()
	now := time.Now()

	// Update status
	m.incidents[idx].Status = StatusClosed

	// Add audit event
	event := IncidentEvent{
		Timestamp: now,
		Action:    "CLOSED",
		User:      user,
		Details:   "Incident closed after resolution verification",
	}
	m.incidents[idx].AuditTrail = append(m.incidents[idx].AuditTrail, event)

	// Save
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save closure: %w", err)
	}

	// Log to audit
	AuditLogEvent("", "INCIDENT_CLOSED", map[string]string{
		"incident_id": incidentID,
	})

	return nil
}

// Get retrieves a single incident by ID.
func (m *IncidentManager) Get(incidentID string) (*Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx := m.findIncidentIndex(incidentID)
	if idx < 0 {
		return nil, fmt.Errorf("incident not found: %s", incidentID)
	}

	// Return a copy
	incident := m.incidents[idx]
	return &incident, nil
}

// List returns incidents matching the filter criteria.
func (m *IncidentManager) List(filter IncidentFilter) ([]Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Incident

	for _, inc := range m.incidents {
		// Apply filters
		if filter.Status != "" && !strings.EqualFold(inc.Status, filter.Status) {
			continue
		}
		if filter.Severity != "" && !strings.EqualFold(inc.Severity, filter.Severity) {
			continue
		}
		if filter.Category != "" && !strings.EqualFold(inc.Category, filter.Category) {
			continue
		}
		if filter.ReportedBy != "" && !strings.EqualFold(inc.ReportedBy, filter.ReportedBy) {
			continue
		}
		if !filter.Since.IsZero() && inc.CreatedAt.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && inc.CreatedAt.After(filter.Until) {
			continue
		}

		results = append(results, inc)
	}

	// Sort by creation time descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Count returns the count of incidents by status.
func (m *IncidentManager) Count() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := map[string]int{
		StatusOpen:          0,
		StatusInvestigating: 0,
		StatusResolved:      0,
		StatusClosed:        0,
	}

	for _, inc := range m.incidents {
		counts[inc.Status]++
	}

	return counts
}

// =============================================================================
// EXPORT
// =============================================================================

// Export exports incidents in the specified format (json or csv).
func (m *IncidentManager) Export(format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	format = strings.ToLower(format)
	switch format {
	case "json":
		return m.exportJSON()
	case "csv":
		return m.exportCSV()
	default:
		return nil, fmt.Errorf("unsupported format '%s', must be json or csv", format)
	}
}

// exportJSON exports incidents as JSON.
func (m *IncidentManager) exportJSON() ([]byte, error) {
	data := map[string]interface{}{
		"export_time": time.Now().Format(time.RFC3339),
		"format":      "rigrun-incidents-v1",
		"count":       len(m.incidents),
		"incidents":   m.incidents,
	}

	return json.MarshalIndent(data, "", "  ")
}

// exportCSV exports incidents as CSV.
func (m *IncidentManager) exportCSV() ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Header
	header := []string{"ID", "Created", "ReportedBy", "Severity", "Category", "Status", "Description", "Resolution", "ResolvedAt", "ResolvedBy"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Data
	for _, inc := range m.incidents {
		resolvedAt := ""
		if inc.ResolvedAt != nil {
			resolvedAt = inc.ResolvedAt.Format(time.RFC3339)
		}

		record := []string{
			inc.ID,
			inc.CreatedAt.Format(time.RFC3339),
			inc.ReportedBy,
			inc.Severity,
			inc.Category,
			inc.Status,
			inc.Description,
			inc.Resolution,
			resolvedAt,
			inc.ResolvedBy,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), nil
}

// =============================================================================
// WEBHOOK
// =============================================================================

// SetWebhook sets the webhook URL for external incident reporting.
func (m *IncidentManager) SetWebhook(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhookURL = url
}

// sendWebhook sends incident data to the configured webhook.
func (m *IncidentManager) sendWebhook(incident *Incident, event string) {
	// This is a placeholder - actual implementation would use http.Post
	// For now, just log the attempt
	AuditLogEvent("", "INCIDENT_WEBHOOK", map[string]string{
		"incident_id": incident.ID,
		"event":       event,
		"webhook":     m.webhookURL,
	})
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// load reads incidents from the file.
func (m *IncidentManager) load() error {
	data, err := os.ReadFile(m.incidentFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.incidents)
}

// save writes incidents to the file.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (m *IncidentManager) save() error {
	data, err := json.MarshalIndent(m.incidents, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal incidents: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(m.incidentFile, data, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write incidents: %w", err)
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateID generates a unique incident ID in format INC-YYYYMMDD-XXXX.
func (m *IncidentManager) generateID() string {
	now := time.Now()
	dateStr := now.Format("20060102")

	// Count incidents from today
	count := 0
	for _, inc := range m.incidents {
		if strings.HasPrefix(inc.ID, "INC-"+dateStr) {
			count++
		}
	}

	return fmt.Sprintf("INC-%s-%04d", dateStr, count+1)
}

// findIncidentIndex finds the index of an incident by ID.
func (m *IncidentManager) findIncidentIndex(id string) int {
	for i, inc := range m.incidents {
		if strings.EqualFold(inc.ID, id) {
			return i
		}
	}
	return -1
}

// getCurrentUser returns the current OS username.
func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}

// isValidSeverity checks if a severity is valid.
func isValidSeverity(severity string) bool {
	s := strings.ToLower(severity)
	return s == SeverityLow || s == SeverityMedium || s == SeverityHigh || s == SeverityCritical
}

// isValidCategory checks if a category is valid.
func isValidCategory(category string) bool {
	c := strings.ToLower(category)
	return c == CategorySecurity || c == CategoryAvailability || c == CategoryIntegrity || c == CategorySpillage
}

// isValidStatus checks if a status is valid.
func isValidStatus(status string) bool {
	s := strings.ToLower(status)
	return s == StatusOpen || s == StatusInvestigating || s == StatusResolved || s == StatusClosed
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalIncidentManager     *IncidentManager
	globalIncidentManagerOnce sync.Once
	globalIncidentManagerMu   sync.Mutex
)

// GlobalIncidentManager returns the global incident manager instance.
func GlobalIncidentManager() *IncidentManager {
	globalIncidentManagerOnce.Do(func() {
		var err error
		globalIncidentManager, err = NewIncidentManager()
		if err != nil {
			// If we can't create the manager, create a disabled one
			globalIncidentManager = &IncidentManager{
				incidents: make([]Incident, 0),
			}
		}
	})
	return globalIncidentManager
}

// SetGlobalIncidentManager sets the global incident manager instance.
func SetGlobalIncidentManager(manager *IncidentManager) {
	globalIncidentManagerMu.Lock()
	defer globalIncidentManagerMu.Unlock()
	globalIncidentManager = manager
}
