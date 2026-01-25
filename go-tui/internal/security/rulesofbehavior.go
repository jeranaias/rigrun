// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file implements NIST 800-53 PL-4: Rules of Behavior.
//
// # DoD STIG Requirements
//
//   - PL-4: Rules of behavior for information system usage
//   - PL-4(1): Social media and networking restrictions
//   - AU-3: Rules acknowledgment events must be logged for audit compliance
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
// PL-4 CONSTANTS
// =============================================================================

const (
	// RulesCategoryGeneral represents general usage rules.
	RulesCategoryGeneral RuleCategory = "general"

	// RulesCategorySecurity represents security-specific rules.
	RulesCategorySecurity RuleCategory = "security"

	// RulesCategoryData represents data handling rules.
	RulesCategoryData RuleCategory = "data"

	// RulesCategoryNetwork represents network usage rules.
	RulesCategoryNetwork RuleCategory = "network"

	// RulesCategoryIncident represents incident response rules.
	RulesCategoryIncident RuleCategory = "incident"

	// RulesFileName is the default storage file name.
	RulesFileName = "rules.json"
)

// =============================================================================
// RULE TYPES
// =============================================================================

// RuleCategory represents the category of a rule.
type RuleCategory string

// String returns the string representation of the rule category.
func (c RuleCategory) String() string {
	return string(c)
}

// =============================================================================
// RULE STRUCT
// =============================================================================

// Rule represents a single rule of behavior.
type Rule struct {
	// ID is the unique identifier for this rule.
	ID string `json:"id"`

	// Category is the category of this rule.
	Category RuleCategory `json:"category"`

	// Description is the full text of the rule.
	Description string `json:"description"`

	// Required indicates if acknowledgment is mandatory.
	Required bool `json:"required"`

	// Order is the display order (lower numbers first).
	Order int `json:"order"`

	// CreatedAt is when this rule was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this rule was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// Active indicates if this rule is currently active.
	Active bool `json:"active"`
}

// =============================================================================
// RULES ACKNOWLEDGMENT STRUCT
// =============================================================================

// RulesAcknowledgment represents a user's acknowledgment of rules.
type RulesAcknowledgment struct {
	// UserID is the user who acknowledged the rules.
	UserID string `json:"user_id"`

	// AcknowledgedAt is when the rules were acknowledged.
	AcknowledgedAt time.Time `json:"acknowledged_at"`

	// RulesVersion is a hash/checksum of the rules that were acknowledged.
	RulesVersion string `json:"rules_version"`

	// IPAddress is the IP address from which rules were acknowledged.
	IPAddress string `json:"ip_address,omitempty"`

	// UserAgent is the user agent string from which rules were acknowledged.
	UserAgent string `json:"user_agent,omitempty"`
}

// =============================================================================
// RULES MANAGER
// =============================================================================

// RulesManager manages rules of behavior per PL-4.
type RulesManager struct {
	// rules maps rule IDs to rules.
	rules map[string]*Rule

	// acknowledgments maps user IDs to their acknowledgments.
	acknowledgments map[string]*RulesAcknowledgment

	// storagePath is the path to the rules storage file.
	storagePath string

	// auditLogger is the audit logger for PL-4 events.
	auditLogger *AuditLogger

	// currentVersion is the version hash of current rules.
	currentVersion string

	// mu protects concurrent access.
	mu sync.RWMutex
}

// RulesManagerOption is a functional option for configuring RulesManager.
type RulesManagerOption func(*RulesManager)

// WithRulesAuditLogger sets the audit logger for PL-4 events.
func WithRulesAuditLogger(logger *AuditLogger) RulesManagerOption {
	return func(m *RulesManager) {
		m.auditLogger = logger
	}
}

// WithRulesStoragePath sets the storage path for rules.
func WithRulesStoragePath(path string) RulesManagerOption {
	return func(m *RulesManager) {
		m.storagePath = path
	}
}

// NewRulesManager creates a new RulesManager with the given options.
func NewRulesManager(opts ...RulesManagerOption) *RulesManager {
	rm := &RulesManager{
		rules:           make(map[string]*Rule),
		acknowledgments: make(map[string]*RulesAcknowledgment),
		storagePath:     defaultRulesPath(),
	}

	// Apply options
	for _, opt := range opts {
		opt(rm)
	}

	// Default audit logger if not provided
	if rm.auditLogger == nil {
		rm.auditLogger = GlobalAuditLogger()
	}

	// Initialize with default rules
	rm.initializeDefaultRules()

	// Calculate current rules version
	rm.updateRulesVersion()

	// Load from storage
	_ = rm.Load()

	return rm
}

// =============================================================================
// RULES OPERATIONS
// =============================================================================

// GetRulesOfBehavior returns all active rules.
func (m *RulesManager) GetRulesOfBehavior() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.Active {
			rules = append(rules, rule)
		}
	}

	// PERFORMANCE: O(n log n) standard library sort
	// Sort by order
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Order < rules[j].Order
	})

	return rules
}

// AcknowledgeRules records a user's acknowledgment of the rules.
func (m *RulesManager) AcknowledgeRules(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create acknowledgment record
	ack := &RulesAcknowledgment{
		UserID:         userID,
		AcknowledgedAt: time.Now(),
		RulesVersion:   m.currentVersion,
	}

	// Store acknowledgment
	m.acknowledgments[userID] = ack

	// Log the acknowledgment event
	m.logEvent("RULES_ACKNOWLEDGED", userID, map[string]string{
		"rules_version": m.currentVersion,
	})

	// Persist to storage
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save acknowledgment: %w", err)
	}

	return nil
}

// HasAcknowledgedRules checks if a user has acknowledged the current rules.
func (m *RulesManager) HasAcknowledgedRules(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ack, exists := m.acknowledgments[userID]
	if !exists {
		return false
	}

	// Check if acknowledgment is for the current version
	return ack.RulesVersion == m.currentVersion
}

// GetRulesByCategory returns rules filtered by category.
func (m *RulesManager) GetRulesByCategory(category RuleCategory) []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0)
	for _, rule := range m.rules {
		if rule.Active && rule.Category == category {
			rules = append(rules, rule)
		}
	}

	// PERFORMANCE: O(n log n) standard library sort
	// Sort by order
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Order < rules[j].Order
	})

	return rules
}

// UpdateRules updates the rules and invalidates existing acknowledgments.
// This requires users to re-acknowledge the updated rules.
func (m *RulesManager) UpdateRules(rules []*Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update rules
	for _, rule := range rules {
		now := time.Now()
		if rule.CreatedAt.IsZero() {
			rule.CreatedAt = now
		}
		rule.UpdatedAt = now
		m.rules[rule.ID] = rule
	}

	// Update rules version (this invalidates all existing acknowledgments)
	m.updateRulesVersion()

	// Log the update event
	m.logEvent("RULES_UPDATED", "system", map[string]string{
		"new_version":   m.currentVersion,
		"rules_updated": fmt.Sprintf("%d", len(rules)),
	})

	// Persist to storage
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save updated rules: %w", err)
	}

	return nil
}

// =============================================================================
// RULE MANAGEMENT
// =============================================================================

// AddRule adds a new rule.
func (m *RulesManager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate rule
	if rule.ID == "" {
		return errors.New("rule ID cannot be empty")
	}
	if rule.Description == "" {
		return errors.New("rule description cannot be empty")
	}

	// Set timestamps
	now := time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now

	// Add rule
	m.rules[rule.ID] = rule

	// Update version
	m.updateRulesVersion()

	// Persist to storage
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save rule: %w", err)
	}

	return nil
}

// GetRule returns a specific rule by ID.
func (m *RulesManager) GetRule(ruleID string) (*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", ruleID)
	}

	return rule, nil
}

// =============================================================================
// STATISTICS
// =============================================================================

// GetRulesStats returns statistics about rules and acknowledgments.
func (m *RulesManager) GetRulesStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRules := 0
	activeRules := 0
	requiredRules := 0

	for _, rule := range m.rules {
		totalRules++
		if rule.Active {
			activeRules++
			if rule.Required {
				requiredRules++
			}
		}
	}

	totalAcknowledgments := len(m.acknowledgments)
	currentAcknowledgments := 0

	for _, ack := range m.acknowledgments {
		if ack.RulesVersion == m.currentVersion {
			currentAcknowledgments++
		}
	}

	return map[string]interface{}{
		"total_rules":             totalRules,
		"active_rules":            activeRules,
		"required_rules":          requiredRules,
		"total_acknowledgments":   totalAcknowledgments,
		"current_acknowledgments": currentAcknowledgments,
		"rules_version":           m.currentVersion,
		"compliance_status":       "PL-4",
	}
}

// GetUserAcknowledgment returns the acknowledgment record for a user.
func (m *RulesManager) GetUserAcknowledgment(userID string) (*RulesAcknowledgment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ack, exists := m.acknowledgments[userID]
	if !exists {
		return nil, fmt.Errorf("no acknowledgment found for user: %s", userID)
	}

	return ack, nil
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// rulesStorage represents the persisted data structure.
type rulesStorage struct {
	Rules           map[string]*Rule                    `json:"rules"`
	Acknowledgments map[string]*RulesAcknowledgment     `json:"acknowledgments"`
	CurrentVersion  string                              `json:"current_version"`
	Version         string                              `json:"version"`
}

// save persists rules to storage.
func (m *RulesManager) save() error {
	storage := rulesStorage{
		Rules:           m.rules,
		Acknowledgments: m.acknowledgments,
		CurrentVersion:  m.currentVersion,
		Version:         "1.0",
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	if err := util.AtomicWriteFileWithDir(m.storagePath, data, 0600, 0700); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

// Load loads rules from storage.
func (m *RulesManager) Load() error {
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
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Parse JSON
	var storage rulesStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to parse rules file: %w", err)
	}

	// Load rules (merge with defaults)
	for id, rule := range storage.Rules {
		m.rules[id] = rule
	}

	// Load acknowledgments
	m.acknowledgments = storage.Acknowledgments

	// Update version (recalculate in case rules changed)
	m.updateRulesVersion()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// logEvent logs a rules-related event to the audit log.
func (m *RulesManager) logEvent(eventType, userID string, metadata map[string]string) {
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
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log rules event %s: %v\n", eventType, err)
	}
}

// updateRulesVersion calculates a version hash for the current rules.
func (m *RulesManager) updateRulesVersion() {
	// Simple version calculation based on rule count and timestamps
	// In production, this should be a proper hash of all rule content
	hash := fmt.Sprintf("v%d-%d", len(m.rules), time.Now().Unix())
	m.currentVersion = hash
}

// defaultRulesPath returns the default rules storage path.
func defaultRulesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rigrun", RulesFileName)
}

// initializeDefaultRules sets up the default rules of behavior.
func (m *RulesManager) initializeDefaultRules() {
	now := time.Now()

	// General Rules
	m.rules["general-1"] = &Rule{
		ID:          "general-1",
		Category:    RulesCategoryGeneral,
		Description: "Use system resources only for authorized purposes",
		Required:    true,
		Order:       1,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["general-2"] = &Rule{
		ID:          "general-2",
		Category:    RulesCategoryGeneral,
		Description: "Comply with all applicable laws, regulations, and policies",
		Required:    true,
		Order:       2,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	// Security Rules
	m.rules["security-1"] = &Rule{
		ID:          "security-1",
		Category:    RulesCategorySecurity,
		Description: "Protect access credentials and authentication tokens",
		Required:    true,
		Order:       10,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["security-2"] = &Rule{
		ID:          "security-2",
		Category:    RulesCategorySecurity,
		Description: "Do not share passwords or API keys with others",
		Required:    true,
		Order:       11,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["security-3"] = &Rule{
		ID:          "security-3",
		Category:    RulesCategorySecurity,
		Description: "Do not attempt to bypass security controls",
		Required:    true,
		Order:       12,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["security-4"] = &Rule{
		ID:          "security-4",
		Category:    RulesCategorySecurity,
		Description: "Lock your workstation when unattended",
		Required:    true,
		Order:       13,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	// Data Handling Rules
	m.rules["data-1"] = &Rule{
		ID:          "data-1",
		Category:    RulesCategoryData,
		Description: "Protect sensitive and classified information from unauthorized disclosure",
		Required:    true,
		Order:       20,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["data-2"] = &Rule{
		ID:          "data-2",
		Category:    RulesCategoryData,
		Description: "Handle PII in accordance with privacy regulations",
		Required:    true,
		Order:       21,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["data-3"] = &Rule{
		ID:          "data-3",
		Category:    RulesCategoryData,
		Description: "Do not store sensitive data in unauthorized locations",
		Required:    true,
		Order:       22,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	// Network Rules
	m.rules["network-1"] = &Rule{
		ID:          "network-1",
		Category:    RulesCategoryNetwork,
		Description: "Use authorized network connections only",
		Required:    true,
		Order:       30,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["network-2"] = &Rule{
		ID:          "network-2",
		Category:    RulesCategoryNetwork,
		Description: "Do not install unauthorized software or services",
		Required:    true,
		Order:       31,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	// Incident Response Rules
	m.rules["incident-1"] = &Rule{
		ID:          "incident-1",
		Category:    RulesCategoryIncident,
		Description: "Report security incidents and suspicious activities immediately",
		Required:    true,
		Order:       40,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}

	m.rules["incident-2"] = &Rule{
		ID:          "incident-2",
		Category:    RulesCategoryIncident,
		Description: "Report lost or stolen credentials immediately",
		Required:    true,
		Order:       41,
		CreatedAt:   now,
		UpdatedAt:   now,
		Active:      true,
	}
}

// =============================================================================
// GLOBAL RULES MANAGER
// =============================================================================

var (
	globalRulesManager     *RulesManager
	globalRulesManagerOnce sync.Once
	globalRulesManagerMu   sync.Mutex
)

// GlobalRulesManager returns the global rules manager instance.
func GlobalRulesManager() *RulesManager {
	globalRulesManagerOnce.Do(func() {
		globalRulesManager = NewRulesManager()
	})
	return globalRulesManager
}

// SetGlobalRulesManager sets the global rules manager instance.
func SetGlobalRulesManager(manager *RulesManager) {
	globalRulesManagerMu.Lock()
	defer globalRulesManagerMu.Unlock()
	globalRulesManager = manager
}

// InitGlobalRulesManager initializes the global rules manager with options.
func InitGlobalRulesManager(opts ...RulesManagerOption) {
	globalRulesManagerMu.Lock()
	defer globalRulesManagerMu.Unlock()
	globalRulesManager = NewRulesManager(opts...)
}
