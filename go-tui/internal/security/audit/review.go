// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit provides security audit logging and protection.
//
// This file implements NIST 800-53 AU-6: Audit Review, Analysis, and Reporting.
package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// DefaultAnalysisWindow is the default time window for analysis.
	DefaultAnalysisWindow = 24 * time.Hour
)

// =============================================================================
// AUDIT REVIEW CONFIGURATION
// =============================================================================

// ReviewConfig contains configuration for audit review.
type ReviewConfig struct {
	// Analysis window
	AnalysisWindow time.Duration `json:"analysis_window" toml:"analysis_window"`

	// Alert thresholds
	FailedAuthThreshold  int `json:"failed_auth_threshold" toml:"failed_auth_threshold"`
	ErrorRateThreshold   int `json:"error_rate_threshold" toml:"error_rate_threshold"`
	UnusualHoursStart    int `json:"unusual_hours_start" toml:"unusual_hours_start"` // 0-23
	UnusualHoursEnd      int `json:"unusual_hours_end" toml:"unusual_hours_end"`     // 0-23
	HighCostThreshold    float64 `json:"high_cost_threshold" toml:"high_cost_threshold"`
	HighTokenThreshold   int `json:"high_token_threshold" toml:"high_token_threshold"`

	// Report settings
	IncludeQueries bool `json:"include_queries" toml:"include_queries"`
	MaxReportEvents int `json:"max_report_events" toml:"max_report_events"`
}

// DefaultReviewConfig returns the default review configuration.
func DefaultReviewConfig() *ReviewConfig {
	return &ReviewConfig{
		AnalysisWindow:       DefaultAnalysisWindow,
		FailedAuthThreshold:  3,
		ErrorRateThreshold:   10,
		UnusualHoursStart:    22, // 10 PM
		UnusualHoursEnd:      6,  // 6 AM
		HighCostThreshold:    10.0, // $0.10
		HighTokenThreshold:   10000,
		IncludeQueries:       false, // Don't include by default for privacy
		MaxReportEvents:      1000,
	}
}

// =============================================================================
// AUDIT REVIEW RESULT
// =============================================================================

// ReviewResult contains the results of an audit review.
type ReviewResult struct {
	Timestamp   time.Time `json:"timestamp"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	// Summary statistics
	TotalEvents        int     `json:"total_events"`
	QueryCount         int     `json:"query_count"`
	ErrorCount         int     `json:"error_count"`
	SuccessRate        float64 `json:"success_rate"`
	TotalTokens        int     `json:"total_tokens"`
	TotalCost          float64 `json:"total_cost"`
	UniqueSessionCount int     `json:"unique_session_count"`

	// Anomalies detected
	Anomalies []Anomaly `json:"anomalies,omitempty"`

	// Event breakdown by type
	EventsByType map[string]int `json:"events_by_type"`

	// Top events (optional)
	TopEvents []Event `json:"top_events,omitempty"`

	// Security indicators
	SecurityIndicators SecurityIndicators `json:"security_indicators"`
}

// Anomaly represents a detected anomaly in the audit log.
type Anomaly struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"` // low, medium, high, critical
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

// SecurityIndicators contains security-relevant metrics.
type SecurityIndicators struct {
	FailedAuthCount        int       `json:"failed_auth_count"`
	LockedAccountCount     int       `json:"locked_account_count"`
	UnusualHoursActivity   int       `json:"unusual_hours_activity"`
	HighPrivilegeOps       int       `json:"high_privilege_ops"`
	DataExfiltrationRisk   int       `json:"data_exfiltration_risk"`
	SessionHijackIndicators int      `json:"session_hijack_indicators"`
	LastSecurityEvent      time.Time `json:"last_security_event,omitempty"`
}

// =============================================================================
// AUDIT REVIEWER
// =============================================================================

// Reviewer performs audit log review and analysis.
// Implements NIST 800-53 AU-6 (Audit Review, Analysis, and Reporting).
type Reviewer struct {
	auditLogPath string
	config       *ReviewConfig
	mu           sync.RWMutex
}

// NewReviewer creates a new audit reviewer.
func NewReviewer(auditLogPath string, config *ReviewConfig) *Reviewer {
	if config == nil {
		config = DefaultReviewConfig()
	}

	if auditLogPath == "" {
		auditLogPath = DefaultPath()
	}

	return &Reviewer{
		auditLogPath: auditLogPath,
		config:       config,
	}
}

// =============================================================================
// REVIEW METHODS
// =============================================================================

// Review performs a review of the audit log within the configured time window.
func (r *Reviewer) Review() (*ReviewResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Calculate time window
	windowEnd := time.Now()
	windowStart := windowEnd.Add(-r.config.AnalysisWindow)

	return r.ReviewTimeRange(windowStart, windowEnd)
}

// ReviewTimeRange performs a review of the audit log within a specific time range.
func (r *Reviewer) ReviewTimeRange(start, end time.Time) (*ReviewResult, error) {
	// Parse audit log
	events, err := r.parseAuditLog(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to parse audit log: %w", err)
	}

	result := &ReviewResult{
		Timestamp:    time.Now(),
		WindowStart:  start,
		WindowEnd:    end,
		TotalEvents:  len(events),
		EventsByType: make(map[string]int),
		Anomalies:    make([]Anomaly, 0),
	}

	// Analyze events
	r.analyzeEvents(events, result)

	// Detect anomalies
	r.detectAnomalies(events, result)

	// Add top events if configured
	if r.config.IncludeQueries && r.config.MaxReportEvents > 0 {
		result.TopEvents = r.getTopEvents(events, r.config.MaxReportEvents)
	}

	return result, nil
}

// =============================================================================
// EVENT ANALYSIS
// =============================================================================

// analyzeEvents performs statistical analysis on the events.
func (r *Reviewer) analyzeEvents(events []Event, result *ReviewResult) {
	sessionSet := make(map[string]bool)
	successCount := 0

	for _, event := range events {
		// Count by event type
		result.EventsByType[event.EventType]++

		// Track sessions
		if event.SessionID != "" {
			sessionSet[event.SessionID] = true
		}

		// Count queries
		if event.EventType == "QUERY" {
			result.QueryCount++
		}

		// Count successes and errors
		if event.Success {
			successCount++
		} else {
			result.ErrorCount++
		}

		// Sum tokens and cost
		result.TotalTokens += event.Tokens
		result.TotalCost += event.Cost
	}

	result.UniqueSessionCount = len(sessionSet)

	// Calculate success rate
	if result.TotalEvents > 0 {
		result.SuccessRate = float64(successCount) / float64(result.TotalEvents) * 100
	}
}

// detectAnomalies identifies anomalous patterns in the events.
func (r *Reviewer) detectAnomalies(events []Event, result *ReviewResult) {
	// Track various metrics
	failedAuthCount := 0
	lockedCount := 0
	unusualHoursCount := 0
	highPrivilegeCount := 0

	sessionErrors := make(map[string]int)
	var lastSecurityEvent time.Time

	for _, event := range events {
		// Failed authentication detection
		if strings.Contains(event.EventType, "AUTH") && !event.Success {
			failedAuthCount++
			sessionErrors[event.SessionID]++

			if failedAuthCount >= r.config.FailedAuthThreshold {
				result.Anomalies = append(result.Anomalies, Anomaly{
					Type:        "FAILED_AUTH_THRESHOLD",
					Severity:    "high",
					Description: fmt.Sprintf("Failed authentication attempts exceeded threshold: %d", failedAuthCount),
					Timestamp:   event.Timestamp,
					SessionID:   event.SessionID,
				})
			}
		}

		// Lockout detection
		if strings.Contains(event.EventType, "LOCKOUT") {
			lockedCount++
			lastSecurityEvent = event.Timestamp
		}

		// Unusual hours detection
		hour := event.Timestamp.Hour()
		if r.isUnusualHour(hour) {
			unusualHoursCount++
		}

		// High privilege operation detection
		if r.isHighPrivilegeOp(event.EventType) {
			highPrivilegeCount++
			lastSecurityEvent = event.Timestamp
		}

		// High cost query detection
		if event.Cost > r.config.HighCostThreshold {
			result.Anomalies = append(result.Anomalies, Anomaly{
				Type:        "HIGH_COST_QUERY",
				Severity:    "medium",
				Description: fmt.Sprintf("High cost query detected: $%.2f", event.Cost),
				Timestamp:   event.Timestamp,
				SessionID:   event.SessionID,
				Details: map[string]string{
					"cost":   fmt.Sprintf("%.2f", event.Cost),
					"tokens": fmt.Sprintf("%d", event.Tokens),
				},
			})
		}

		// High token usage detection
		if event.Tokens > r.config.HighTokenThreshold {
			result.Anomalies = append(result.Anomalies, Anomaly{
				Type:        "HIGH_TOKEN_USAGE",
				Severity:    "low",
				Description: fmt.Sprintf("High token usage detected: %d tokens", event.Tokens),
				Timestamp:   event.Timestamp,
				SessionID:   event.SessionID,
			})
		}
	}

	// Detect sessions with high error rates
	for sessionID, errorCount := range sessionErrors {
		if errorCount >= r.config.ErrorRateThreshold {
			result.Anomalies = append(result.Anomalies, Anomaly{
				Type:        "HIGH_SESSION_ERRORS",
				Severity:    "medium",
				Description: fmt.Sprintf("Session has high error count: %d errors", errorCount),
				SessionID:   sessionID,
			})
		}
	}

	// Unusual hours activity
	if unusualHoursCount > 10 { // Arbitrary threshold
		result.Anomalies = append(result.Anomalies, Anomaly{
			Type:        "UNUSUAL_HOURS_ACTIVITY",
			Severity:    "medium",
			Description: fmt.Sprintf("Significant activity during unusual hours: %d events", unusualHoursCount),
		})
	}

	// Set security indicators
	result.SecurityIndicators = SecurityIndicators{
		FailedAuthCount:      failedAuthCount,
		LockedAccountCount:   lockedCount,
		UnusualHoursActivity: unusualHoursCount,
		HighPrivilegeOps:     highPrivilegeCount,
		LastSecurityEvent:    lastSecurityEvent,
	}
}

// isUnusualHour checks if the hour is within the unusual hours window.
func (r *Reviewer) isUnusualHour(hour int) bool {
	start := r.config.UnusualHoursStart
	end := r.config.UnusualHoursEnd

	// Handle wrap-around (e.g., 22:00 to 06:00)
	if start > end {
		return hour >= start || hour < end
	}
	return hour >= start && hour < end
}

// isHighPrivilegeOp checks if an event type is a high-privilege operation.
func (r *Reviewer) isHighPrivilegeOp(eventType string) bool {
	highPrivOps := []string{
		"ROLE_ASSIGNED",
		"ROLE_REVOKED",
		"CONFIG_CHANGE",
		"ENCRYPTION_KEY_ROTATE",
		"AUDIT_CLEAR",
		"USER_CREATE",
		"USER_DELETE",
		"SYSTEM_MANAGE",
	}

	for _, op := range highPrivOps {
		if eventType == op {
			return true
		}
	}
	return false
}

// getTopEvents returns the top N events by cost/tokens.
func (r *Reviewer) getTopEvents(events []Event, n int) []Event {
	if len(events) == 0 {
		return nil
	}

	// Sort by cost (descending)
	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Cost > sorted[j].Cost
	})

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// =============================================================================
// LOG PARSING
// =============================================================================

// parseAuditLog parses the audit log file within the given time range.
func (r *Reviewer) parseAuditLog(start, end time.Time) ([]Event, error) {
	file, err := os.Open(r.auditLogPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try parsing as JSON first
		event, err := r.parseEventLine(line)
		if err != nil {
			// Skip unparseable lines
			continue
		}

		// Filter by time range
		if event.Timestamp.Before(start) || event.Timestamp.After(end) {
			continue
		}

		events = append(events, event)
	}

	return events, scanner.Err()
}

// parseEventLine parses a single log line into an Event.
func (r *Reviewer) parseEventLine(line string) (Event, error) {
	var event Event

	// Try JSON format first
	if strings.HasPrefix(line, "{") {
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			return event, nil
		}
	}

	// Parse pipe-delimited format: timestamp | event_type | session_id | tier | query | tokens | cost | status
	parts := strings.Split(line, "|")
	if len(parts) >= 4 {
		// Parse timestamp
		timestamp, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(parts[0]))
		if err != nil {
			return event, fmt.Errorf("invalid timestamp: %w", err)
		}

		event.Timestamp = timestamp
		event.EventType = strings.TrimSpace(parts[1])
		event.SessionID = strings.TrimSpace(parts[2])

		if len(parts) > 3 {
			event.Tier = strings.TrimSpace(parts[3])
		}
		if len(parts) > 4 {
			event.Query = strings.Trim(strings.TrimSpace(parts[4]), "\"")
		}
		if len(parts) > 5 {
			fmt.Sscanf(strings.TrimSpace(parts[5]), "%d", &event.Tokens)
		}
		if len(parts) > 6 {
			fmt.Sscanf(strings.TrimSpace(parts[6]), "%f", &event.Cost)
		}
		if len(parts) > 7 {
			status := strings.TrimSpace(parts[7])
			event.Success = strings.HasPrefix(status, "SUCCESS")
			if !event.Success && strings.HasPrefix(status, "ERROR:") {
				event.Error = strings.TrimPrefix(status, "ERROR: ")
			}
		}

		return event, nil
	}

	return event, fmt.Errorf("unrecognized format")
}

// =============================================================================
// REPORTING
// =============================================================================

// GenerateReport generates a text report from the review result.
func (r *Reviewer) GenerateReport(result *ReviewResult) string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("                          AUDIT REVIEW REPORT                                   \n")
	sb.WriteString("                      NIST 800-53 AU-6 Compliance                               \n")
	sb.WriteString("================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("Report Generated: %s\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Analysis Window: %s to %s\n\n",
		result.WindowStart.Format(time.RFC3339),
		result.WindowEnd.Format(time.RFC3339)))

	// Summary
	sb.WriteString("SUMMARY\n")
	sb.WriteString("-------\n")
	sb.WriteString(fmt.Sprintf("Total Events:      %d\n", result.TotalEvents))
	sb.WriteString(fmt.Sprintf("Query Count:       %d\n", result.QueryCount))
	sb.WriteString(fmt.Sprintf("Error Count:       %d\n", result.ErrorCount))
	sb.WriteString(fmt.Sprintf("Success Rate:      %.1f%%\n", result.SuccessRate))
	sb.WriteString(fmt.Sprintf("Total Tokens:      %d\n", result.TotalTokens))
	sb.WriteString(fmt.Sprintf("Total Cost:        $%.4f\n", result.TotalCost))
	sb.WriteString(fmt.Sprintf("Unique Sessions:   %d\n\n", result.UniqueSessionCount))

	// Security Indicators
	sb.WriteString("SECURITY INDICATORS\n")
	sb.WriteString("-------------------\n")
	sb.WriteString(fmt.Sprintf("Failed Auth Attempts:    %d\n", result.SecurityIndicators.FailedAuthCount))
	sb.WriteString(fmt.Sprintf("Locked Accounts:         %d\n", result.SecurityIndicators.LockedAccountCount))
	sb.WriteString(fmt.Sprintf("Unusual Hours Activity:  %d\n", result.SecurityIndicators.UnusualHoursActivity))
	sb.WriteString(fmt.Sprintf("High Privilege Ops:      %d\n", result.SecurityIndicators.HighPrivilegeOps))
	if !result.SecurityIndicators.LastSecurityEvent.IsZero() {
		sb.WriteString(fmt.Sprintf("Last Security Event:     %s\n",
			result.SecurityIndicators.LastSecurityEvent.Format(time.RFC3339)))
	}
	sb.WriteString("\n")

	// Anomalies
	if len(result.Anomalies) > 0 {
		sb.WriteString("ANOMALIES DETECTED\n")
		sb.WriteString("------------------\n")
		for _, anomaly := range result.Anomalies {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
				strings.ToUpper(anomaly.Severity),
				anomaly.Type,
				anomaly.Description))
			if anomaly.SessionID != "" {
				sb.WriteString(fmt.Sprintf("         Session: %s\n", anomaly.SessionID))
			}
			if !anomaly.Timestamp.IsZero() {
				sb.WriteString(fmt.Sprintf("         Time: %s\n", anomaly.Timestamp.Format(time.RFC3339)))
			}
		}
		sb.WriteString("\n")
	}

	// Events by Type
	sb.WriteString("EVENTS BY TYPE\n")
	sb.WriteString("--------------\n")
	for eventType, count := range result.EventsByType {
		sb.WriteString(fmt.Sprintf("%-25s %d\n", eventType, count))
	}
	sb.WriteString("\n")

	sb.WriteString("================================================================================\n")
	sb.WriteString("                              END OF REPORT                                     \n")
	sb.WriteString("================================================================================\n")

	return sb.String()
}

// GenerateJSONReport generates a JSON report from the review result.
func (r *Reviewer) GenerateJSONReport(result *ReviewResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportReport exports the review report to a file.
func (r *Reviewer) ExportReport(result *ReviewResult, outputPath string, format string) error {
	var content string
	var err error

	switch format {
	case "json":
		content, err = r.GenerateJSONReport(result)
		if err != nil {
			return err
		}
	case "text", "txt":
		content = r.GenerateReport(result)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Write report
	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// GetConfig returns the current review configuration.
func (r *Reviewer) GetConfig() *ReviewConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy
	copy := *r.config
	return &copy
}

// SetConfig updates the review configuration.
func (r *Reviewer) SetConfig(config *ReviewConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalReviewer     *Reviewer
	globalReviewerOnce sync.Once
	globalReviewerMu   sync.RWMutex
)

// GlobalReviewer returns the global audit reviewer instance.
func GlobalReviewer() *Reviewer {
	globalReviewerOnce.Do(func() {
		globalReviewer = NewReviewer("", nil)
	})
	return globalReviewer
}

// SetGlobalReviewer sets the global audit reviewer instance.
func SetGlobalReviewer(reviewer *Reviewer) {
	globalReviewerMu.Lock()
	defer globalReviewerMu.Unlock()
	globalReviewer = reviewer
}

// Review performs a review using the global reviewer.
func Review() (*ReviewResult, error) {
	return GlobalReviewer().Review()
}
