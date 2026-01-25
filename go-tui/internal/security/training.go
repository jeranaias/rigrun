// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides DoD STIG-compliant security features.
//
// This file implements NIST 800-53 AT-2 (Security Awareness Training) and
// AT-3 (Role-Based Training) for DoD IL5 compliance.
//
// # DoD STIG Requirements
//
//   - AT-2: Security Awareness Training - All users must complete annual security awareness training
//   - AT-3: Role-Based Training - Users with specific roles must complete specialized training
//   - AT-4: Training Records - System must maintain records of training completion
//
// # Training Requirements
//
//   - Training expires after 1 year and must be renewed
//   - Training completion is recorded with timestamps and scores
//   - Training status is auditable for compliance reporting
package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// AT-2/AT-3 CONSTANTS
// =============================================================================

const (
	// TrainingExpirationPeriod is the time after which training expires (1 year).
	TrainingExpirationPeriod = 365 * 24 * time.Hour

	// TrainingWarningPeriod is the time before expiration to warn users (30 days).
	TrainingWarningPeriod = 30 * 24 * time.Hour

	// DefaultPassingScore is the minimum score to pass training (80%).
	DefaultPassingScore = 80.0
)

// CourseType represents the type of training course.
type CourseType string

const (
	// CourseTypeAwareness is for AT-2 security awareness training.
	CourseTypeAwareness CourseType = "awareness"

	// CourseTypeRoleBased is for AT-3 role-based security training.
	CourseTypeRoleBased CourseType = "role_based"

	// CourseTypeTechnical is for technical/operational training.
	CourseTypeTechnical CourseType = "technical"

	// CourseTypeCompliance is for compliance/policy training.
	CourseTypeCompliance CourseType = "compliance"
)

// UserRole represents a user's role for role-based training requirements.
type UserRole string

const (
	// RoleUser is the standard user role.
	RoleUser UserRole = "user"

	// RoleDataHandler is for users who handle sensitive data.
	RoleDataHandler UserRole = "data_handler"

	// RoleAdmin is for system administrators.
	RoleAdmin UserRole = "admin"

	// RoleOperator is for system operators.
	RoleOperator UserRole = "operator"

	// RoleCryptoUser is for users handling cryptographic materials.
	RoleCryptoUser UserRole = "crypto_user"
)

// =============================================================================
// COURSE DEFINITION
// =============================================================================

// Course represents a training course.
type Course struct {
	// ID is the unique course identifier.
	ID string `json:"id"`

	// Name is the human-readable course name.
	Name string `json:"name"`

	// Type is the course type (awareness, role_based, etc.).
	Type CourseType `json:"type"`

	// Duration is the estimated course duration.
	Duration time.Duration `json:"duration"`

	// RequiredFor lists the roles that must complete this course.
	RequiredFor []UserRole `json:"required_for"`

	// PassingScore is the minimum score required to pass (0-100).
	PassingScore float64 `json:"passing_score"`

	// Description is a detailed course description.
	Description string `json:"description"`

	// ExpirationPeriod is how long the training is valid (default: 1 year).
	ExpirationPeriod time.Duration `json:"expiration_period"`
}

// IsRequiredFor checks if the course is required for a given role.
func (c *Course) IsRequiredFor(role UserRole) bool {
	for _, r := range c.RequiredFor {
		if r == role {
			return true
		}
	}
	return false
}

// =============================================================================
// TRAINING RECORD
// =============================================================================

// TrainingRecord represents a user's training completion record.
type TrainingRecord struct {
	// UserID is the user who completed the training.
	UserID string `json:"user_id"`

	// CourseID is the course that was completed.
	CourseID string `json:"course_id"`

	// CompletedAt is when the training was completed.
	CompletedAt time.Time `json:"completed_at"`

	// ExpiresAt is when the training expires and must be renewed.
	ExpiresAt time.Time `json:"expires_at"`

	// Score is the user's score (0-100).
	Score float64 `json:"score"`

	// Passed indicates if the user passed the training.
	Passed bool `json:"passed"`

	// CertificateID is an optional certificate identifier.
	CertificateID string `json:"certificate_id,omitempty"`

	// Metadata stores additional training information.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IsExpired checks if the training record has expired.
func (r *TrainingRecord) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// IsExpiringSoon checks if the training expires within the warning period.
func (r *TrainingRecord) IsExpiringSoon(warningPeriod time.Duration) bool {
	return time.Until(r.ExpiresAt) <= warningPeriod && !r.IsExpired()
}

// TimeUntilExpiration returns the duration until expiration.
func (r *TrainingRecord) TimeUntilExpiration() time.Duration {
	remaining := time.Until(r.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// =============================================================================
// TRAINING STATUS
// =============================================================================

// TrainingStatus represents a user's overall training status.
type TrainingStatus struct {
	// UserID is the user identifier.
	UserID string `json:"user_id"`

	// Role is the user's role.
	Role UserRole `json:"role"`

	// CompletedCourses is the number of completed courses.
	CompletedCourses int `json:"completed_courses"`

	// RequiredCourses is the number of required courses.
	RequiredCourses int `json:"required_courses"`

	// PendingCourses is the number of required but incomplete courses.
	PendingCourses int `json:"pending_courses"`

	// ExpiringCourses is the number of courses expiring soon.
	ExpiringCourses int `json:"expiring_courses"`

	// ExpiredCourses is the number of expired courses.
	ExpiredCourses int `json:"expired_courses"`

	// IsCurrent indicates if all required training is current.
	IsCurrent bool `json:"is_current"`

	// LastCompleted is the timestamp of the most recent training completion.
	LastCompleted *time.Time `json:"last_completed,omitempty"`
}

// =============================================================================
// TRAINING MANAGER
// =============================================================================

// TrainingManager manages security training records per AT-2 and AT-3.
type TrainingManager struct {
	// courses maps course IDs to course definitions.
	courses map[string]*Course

	// records stores training records by user ID, then course ID.
	records map[string]map[string]*TrainingRecord

	// auditLogger is the audit logger for training events.
	auditLogger *AuditLogger

	// dataPath is the path to the training data file.
	dataPath string

	// mu protects concurrent access.
	mu sync.RWMutex
}

// TrainingManagerOption is a functional option for configuring TrainingManager.
type TrainingManagerOption func(*TrainingManager)

// WithTrainingAuditLogger sets the audit logger for training events.
func WithTrainingAuditLogger(logger *AuditLogger) TrainingManagerOption {
	return func(t *TrainingManager) {
		t.auditLogger = logger
	}
}

// WithTrainingDataPath sets the path for training data persistence.
func WithTrainingDataPath(path string) TrainingManagerOption {
	return func(t *TrainingManager) {
		t.dataPath = path
	}
}

// NewTrainingManager creates a new TrainingManager with default courses.
func NewTrainingManager(opts ...TrainingManagerOption) *TrainingManager {
	tm := &TrainingManager{
		courses: make(map[string]*Course),
		records: make(map[string]map[string]*TrainingRecord),
		dataPath: DefaultTrainingDataPath(),
	}

	// Apply options
	for _, opt := range opts {
		opt(tm)
	}

	// Default audit logger if not provided
	if tm.auditLogger == nil {
		tm.auditLogger = GlobalAuditLogger()
	}

	// Load default courses
	tm.loadDefaultCourses()

	// Load persisted training records
	_ = tm.load()

	return tm
}

// loadDefaultCourses populates the default training courses per AT-2/AT-3.
func (t *TrainingManager) loadDefaultCourses() {
	// AT-2: Annual Security Awareness Training (all users)
	t.courses["security-awareness"] = &Course{
		ID:               "security-awareness",
		Name:             "Annual Security Awareness Training",
		Type:             CourseTypeAwareness,
		Duration:         2 * time.Hour,
		RequiredFor:      []UserRole{RoleUser, RoleDataHandler, RoleAdmin, RoleOperator, RoleCryptoUser},
		PassingScore:     DefaultPassingScore,
		Description:      "DoD IL5 compliant annual security awareness training covering threats, best practices, and incident reporting (AT-2)",
		ExpirationPeriod: TrainingExpirationPeriod,
	}

	// AT-3: PII Handling Training (data handlers)
	t.courses["pii-handling"] = &Course{
		ID:               "pii-handling",
		Name:             "PII Handling Procedures",
		Type:             CourseTypeRoleBased,
		Duration:         1 * time.Hour,
		RequiredFor:      []UserRole{RoleDataHandler},
		PassingScore:     DefaultPassingScore,
		Description:      "Personally Identifiable Information (PII) handling, privacy controls, and data protection procedures (AT-3)",
		ExpirationPeriod: TrainingExpirationPeriod,
	}

	// AT-3: Administrator Security Training (admins)
	t.courses["admin-security"] = &Course{
		ID:               "admin-security",
		Name:             "Administrator Security Training",
		Type:             CourseTypeRoleBased,
		Duration:         3 * time.Hour,
		RequiredFor:      []UserRole{RoleAdmin},
		PassingScore:     DefaultPassingScore,
		Description:      "Advanced security training for system administrators covering privileged access, audit logging, and security controls (AT-3)",
		ExpirationPeriod: TrainingExpirationPeriod,
	}

	// AT-3: Incident Response Training (operators)
	t.courses["incident-response"] = &Course{
		ID:               "incident-response",
		Name:             "Incident Response Procedures",
		Type:             CourseTypeRoleBased,
		Duration:         2 * time.Hour,
		RequiredFor:      []UserRole{RoleOperator},
		PassingScore:     DefaultPassingScore,
		Description:      "Incident detection, reporting, response procedures, and escalation paths per IR-6 (AT-3)",
		ExpirationPeriod: TrainingExpirationPeriod,
	}

	// AT-3: Cryptographic Material Handling (crypto users)
	t.courses["crypto-handling"] = &Course{
		ID:               "crypto-handling",
		Name:             "Cryptographic Material Handling",
		Type:             CourseTypeRoleBased,
		Duration:         2 * time.Hour,
		RequiredFor:      []UserRole{RoleCryptoUser},
		PassingScore:     DefaultPassingScore,
		Description:      "Cryptographic key management, certificate handling, and secure crypto operations per SC-12/SC-13 (AT-3)",
		ExpirationPeriod: TrainingExpirationPeriod,
	}
}

// =============================================================================
// TRAINING OPERATIONS
// =============================================================================

// RecordCompletion records a training completion for a user.
func (t *TrainingManager) RecordCompletion(userID, courseID string, score float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate course exists
	course, exists := t.courses[courseID]
	if !exists {
		return fmt.Errorf("course not found: %s", courseID)
	}

	// Validate score
	if score < 0 || score > 100 {
		return errors.New("score must be between 0 and 100")
	}

	// Check if user passed
	passed := score >= course.PassingScore

	// Create training record
	now := time.Now()
	record := &TrainingRecord{
		UserID:        userID,
		CourseID:      courseID,
		CompletedAt:   now,
		ExpiresAt:     now.Add(course.ExpirationPeriod),
		Score:         score,
		Passed:        passed,
		CertificateID: generateCertificateID(userID, courseID, now),
		Metadata:      make(map[string]string),
	}

	// Store record
	if t.records[userID] == nil {
		t.records[userID] = make(map[string]*TrainingRecord)
	}
	t.records[userID][courseID] = record

	// Persist to disk
	_ = t.save()

	// Log the event
	t.logEvent("TRAINING_COMPLETED", userID, map[string]string{
		"course_id": courseID,
		"score":     fmt.Sprintf("%.1f", score),
		"passed":    fmt.Sprintf("%v", passed),
	})

	if !passed {
		return fmt.Errorf("training failed: score %.1f below passing score %.1f", score, course.PassingScore)
	}

	return nil
}

// GetRequiredTraining returns the courses required for a specific role.
func (t *TrainingManager) GetRequiredTraining(role UserRole) []*Course {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var required []*Course
	for _, course := range t.courses {
		if course.IsRequiredFor(role) {
			required = append(required, course)
		}
	}

	// Sort by course ID for consistent ordering
	sort.Slice(required, func(i, j int) bool {
		return required[i].ID < required[j].ID
	})

	return required
}

// IsTrainingCurrent checks if a user's training is current for their role.
func (t *TrainingManager) IsTrainingCurrent(userID string, role UserRole) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	requiredCourses := t.GetRequiredTraining(role)
	userRecords, exists := t.records[userID]
	if !exists {
		return len(requiredCourses) == 0
	}

	for _, course := range requiredCourses {
		record, completed := userRecords[course.ID]
		if !completed || !record.Passed || record.IsExpired() {
			return false
		}
	}

	return true
}

// GetTrainingStatus returns a user's overall training status.
func (t *TrainingManager) GetTrainingStatus(userID string, role UserRole) TrainingStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	status := TrainingStatus{
		UserID: userID,
		Role:   role,
	}

	requiredCourses := t.GetRequiredTraining(role)
	status.RequiredCourses = len(requiredCourses)

	userRecords, exists := t.records[userID]
	if !exists {
		status.PendingCourses = status.RequiredCourses
		status.IsCurrent = status.RequiredCourses == 0
		return status
	}

	for _, course := range requiredCourses {
		record, completed := userRecords[course.ID]

		if !completed || !record.Passed {
			status.PendingCourses++
		} else if record.IsExpired() {
			status.ExpiredCourses++
			status.PendingCourses++
		} else if record.IsExpiringSoon(TrainingWarningPeriod) {
			status.ExpiringCourses++
			status.CompletedCourses++
		} else {
			status.CompletedCourses++
		}

		if completed && record.Passed {
			if status.LastCompleted == nil || record.CompletedAt.After(*status.LastCompleted) {
				status.LastCompleted = &record.CompletedAt
			}
		}
	}

	status.IsCurrent = status.PendingCourses == 0 && status.ExpiredCourses == 0

	return status
}

// GetTrainingHistory returns all training records for a user.
func (t *TrainingManager) GetTrainingHistory(userID string) []*TrainingRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	userRecords, exists := t.records[userID]
	if !exists {
		return []*TrainingRecord{}
	}

	var history []*TrainingRecord
	for _, record := range userRecords {
		// Return a copy
		recordCopy := *record
		history = append(history, &recordCopy)
	}

	// Sort by completion date (most recent first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].CompletedAt.After(history[j].CompletedAt)
	})

	return history
}

// GetExpiringTraining returns training records expiring within the specified number of days.
func (t *TrainingManager) GetExpiringTraining(days int) map[string][]*TrainingRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	expiringPeriod := time.Duration(days) * 24 * time.Hour
	expiring := make(map[string][]*TrainingRecord)

	for userID, userRecords := range t.records {
		for _, record := range userRecords {
			if record.Passed && record.IsExpiringSoon(expiringPeriod) {
				expiring[userID] = append(expiring[userID], record)
			}
		}
	}

	return expiring
}

// GetCourse returns a course by ID.
func (t *TrainingManager) GetCourse(courseID string) (*Course, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	course, exists := t.courses[courseID]
	if !exists {
		return nil, fmt.Errorf("course not found: %s", courseID)
	}

	return course, nil
}

// ListCourses returns all available courses.
func (t *TrainingManager) ListCourses() []*Course {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var courses []*Course
	for _, course := range t.courses {
		courses = append(courses, course)
	}

	// Sort by course ID
	sort.Slice(courses, func(i, j int) bool {
		return courses[i].ID < courses[j].ID
	})

	return courses
}

// =============================================================================
// COMPLIANCE REPORTING
// =============================================================================

// TrainingReport represents a training compliance report.
type TrainingReport struct {
	GeneratedAt     time.Time                       `json:"generated_at"`
	TotalUsers      int                             `json:"total_users"`
	CompliantUsers  int                             `json:"compliant_users"`
	NonCompliant    int                             `json:"non_compliant"`
	ExpiringTraining int                            `json:"expiring_training"`
	ExpiredTraining  int                            `json:"expired_training"`
	UserStatuses    map[string]TrainingStatus       `json:"user_statuses"`
	ExpiringRecords map[string][]*TrainingRecord    `json:"expiring_records"`
}

// GenerateTrainingReport generates a compliance report for all users.
func (t *TrainingManager) GenerateTrainingReport(role UserRole) TrainingReport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report := TrainingReport{
		GeneratedAt:     time.Now(),
		UserStatuses:    make(map[string]TrainingStatus),
		ExpiringRecords: make(map[string][]*TrainingRecord),
	}

	// Calculate statistics for all users
	for userID := range t.records {
		status := t.GetTrainingStatus(userID, role)
		report.UserStatuses[userID] = status
		report.TotalUsers++

		if status.IsCurrent {
			report.CompliantUsers++
		} else {
			report.NonCompliant++
		}

		report.ExpiringTraining += status.ExpiringCourses
		report.ExpiredTraining += status.ExpiredCourses
	}

	// Get expiring training (30 days)
	report.ExpiringRecords = t.GetExpiringTraining(30)

	return report
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// trainingData represents the persisted training data structure.
type trainingData struct {
	Records map[string]map[string]*TrainingRecord `json:"records"`
	Version string                                 `json:"version"`
}

// save persists training records to disk.
func (t *TrainingManager) save() error {
	data := trainingData{
		Records: t.records,
		Version: "1.0",
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal training data: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(t.dataPath, jsonData, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write training data: %w", err)
	}

	return nil
}

// load loads training records from disk.
func (t *TrainingManager) load() error {
	// Check if file exists
	if _, err := os.Stat(t.dataPath); os.IsNotExist(err) {
		return nil // No data to load yet
	}

	// Read file
	jsonData, err := os.ReadFile(t.dataPath)
	if err != nil {
		return fmt.Errorf("failed to read training data: %w", err)
	}

	// Parse JSON
	var data trainingData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal training data: %w", err)
	}

	// Load records
	t.records = data.Records
	if t.records == nil {
		t.records = make(map[string]map[string]*TrainingRecord)
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a training-related event to the audit log.
func (t *TrainingManager) logEvent(eventType, userID string, metadata map[string]string) {
	if t.auditLogger == nil || !t.auditLogger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: userID,
		Success:   true,
		Metadata:  metadata,
	}

	if err := t.auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log training event %s: %v\n", eventType, err)
	}
}

// generateCertificateID generates a unique certificate ID.
func generateCertificateID(userID, courseID string, completedAt time.Time) string {
	timestamp := completedAt.Format("20060102")
	return fmt.Sprintf("CERT-%s-%s-%s", courseID, userID[:min(8, len(userID))], timestamp)
}

// DefaultTrainingDataPath returns the default training data file path.
func DefaultTrainingDataPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rigrun", "training.json")
}

// =============================================================================
// GLOBAL TRAINING MANAGER
// =============================================================================

var (
	globalTrainingManager     *TrainingManager
	globalTrainingManagerOnce sync.Once
	globalTrainingManagerMu   sync.Mutex
)

// GlobalTrainingManager returns the global training manager instance.
func GlobalTrainingManager() *TrainingManager {
	globalTrainingManagerOnce.Do(func() {
		globalTrainingManager = NewTrainingManager()
	})
	return globalTrainingManager
}

// SetGlobalTrainingManager sets the global training manager instance.
func SetGlobalTrainingManager(manager *TrainingManager) {
	globalTrainingManagerMu.Lock()
	defer globalTrainingManagerMu.Unlock()
	globalTrainingManager = manager
}

// InitGlobalTrainingManager initializes the global training manager with options.
func InitGlobalTrainingManager(opts ...TrainingManagerOption) {
	globalTrainingManagerMu.Lock()
	defer globalTrainingManagerMu.Unlock()
	globalTrainingManager = NewTrainingManager(opts...)
}
