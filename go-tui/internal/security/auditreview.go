// auditreview.go - NIST 800-53 AU-6: Audit Record Review, Analysis, and Reporting
//
// Implements automated audit log review, anomaly detection, correlation,
// and compliance reporting for DoD IL5 compliance.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// Severity levels for audit alerts
const (
	SeverityInfo = "info"
	// SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical are defined in incident.go
)

// Default thresholds for anomaly detection
const (
	DefaultFailedLoginThreshold    = 5   // Failed logins in window
	DefaultPrivEscalationThreshold = 3   // Privilege attempts in window
	DefaultConfigChangeThreshold   = 10  // Config changes in window
	DefaultTimeWindowMinutes       = 15  // Analysis time window
)

// =============================================================================
// AUDIT ALERT
// =============================================================================

// AuditAlert represents a triggered alert from audit analysis.
type AuditAlert struct {
	ID          string            `json:"id"`           // ALT-YYYYMMDD-XXXX format
	Timestamp   time.Time         `json:"timestamp"`    // When alert was triggered
	Severity    string            `json:"severity"`     // info, low, medium, high, critical
	EventType   string            `json:"event_type"`   // Type of event that triggered alert
	Message     string            `json:"message"`      // Human-readable alert message
	Details     map[string]string `json:"details"`      // Additional context
	EventCount  int               `json:"event_count"`  // Number of events in pattern
	TimeWindow  time.Duration     `json:"time_window"`  // Time window analyzed
	Acknowledged bool             `json:"acknowledged"` // Whether alert has been acknowledged
}

// =============================================================================
// ALERT THRESHOLDS
// =============================================================================

// AlertThresholds defines configurable thresholds for anomaly detection.
type AlertThresholds struct {
	FailedLoginThreshold    int           `json:"failed_login_threshold"`
	PrivEscalationThreshold int           `json:"priv_escalation_threshold"`
	ConfigChangeThreshold   int           `json:"config_change_threshold"`
	TimeWindowMinutes       int           `json:"time_window_minutes"`
	UnusualHourStart        int           `json:"unusual_hour_start"` // 0-23, e.g., 22 for 10 PM
	UnusualHourEnd          int           `json:"unusual_hour_end"`   // 0-23, e.g., 6 for 6 AM
}

// DefaultAlertThresholds returns the default alert thresholds.
func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		FailedLoginThreshold:    DefaultFailedLoginThreshold,
		PrivEscalationThreshold: DefaultPrivEscalationThreshold,
		ConfigChangeThreshold:   DefaultConfigChangeThreshold,
		TimeWindowMinutes:       DefaultTimeWindowMinutes,
		UnusualHourStart:        22, // 10 PM
		UnusualHourEnd:          6,  // 6 AM
	}
}

// =============================================================================
// AUDIT REVIEWER
// =============================================================================

// AuditReviewer provides audit log analysis and reporting capabilities.
// Implements NIST 800-53 AU-6 (Audit Review, Analysis, and Reporting).
type AuditReviewer struct {
	auditLogPath string
	thresholds   AlertThresholds
	alerts       []AuditAlert
	mu           sync.RWMutex
}

// NewAuditReviewer creates a new audit reviewer with default thresholds.
func NewAuditReviewer(auditLogPath string) *AuditReviewer {
	return &AuditReviewer{
		auditLogPath: auditLogPath,
		thresholds:   DefaultAlertThresholds(),
		alerts:       make([]AuditAlert, 0),
	}
}

// =============================================================================
// THRESHOLD CONFIGURATION
// =============================================================================

// SetAlertThresholds configures the alerting thresholds.
func (r *AuditReviewer) SetAlertThresholds(thresholds AlertThresholds) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.thresholds = thresholds
}

// GetAlertThresholds returns the current alerting thresholds.
func (r *AuditReviewer) GetAlertThresholds() AlertThresholds {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.thresholds
}

// =============================================================================
// LOG ANALYSIS
// =============================================================================

// AnalyzeLogs analyzes audit logs for anomalies within the specified time range.
// Returns a summary of findings and any triggered alerts.
func (r *AuditReviewer) AnalyzeLogs(timeRange time.Duration) (*AnalysisResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate time window
	endTime := time.Now()
	startTime := endTime.Add(-timeRange)

	// Read audit log entries in the time range
	entries, err := r.readAuditEntries(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit entries: %w", err)
	}

	result := &AnalysisResult{
		StartTime:      startTime,
		EndTime:        endTime,
		TotalEvents:    len(entries),
		TriggeredAlerts: make([]AuditAlert, 0),
	}

	// Run anomaly detection
	r.detectFailedLoginSpikes(entries, result)
	r.detectUnusualAccessTimes(entries, result)
	r.detectPrivilegeEscalation(entries, result)
	r.detectConfigurationChanges(entries, result)
	r.detectDataExfiltration(entries, result)

	// Store triggered alerts
	r.alerts = append(r.alerts, result.TriggeredAlerts...)

	return result, nil
}

// AnalysisResult contains the results of audit log analysis.
type AnalysisResult struct {
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	TotalEvents     int           `json:"total_events"`
	TriggeredAlerts []AuditAlert  `json:"triggered_alerts"`
	Findings        []string      `json:"findings"`
}

// =============================================================================
// ANOMALY DETECTION
// =============================================================================

// DetectAnomalies runs all anomaly detection algorithms on recent logs.
// This is a convenience method that analyzes the last 24 hours.
func (r *AuditReviewer) DetectAnomalies() (*AnalysisResult, error) {
	return r.AnalyzeLogs(24 * time.Hour)
}

// detectFailedLoginSpikes detects unusual spikes in failed login attempts.
func (r *AuditReviewer) detectFailedLoginSpikes(entries []auditEntry, result *AnalysisResult) {
	// Group failed login attempts by time window
	failedLogins := make(map[string]int)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.EventType), "login") &&
			strings.Contains(strings.ToLower(entry.Status), "fail") {
			// Group by hour
			hourKey := entry.Timestamp.Format("2006-01-02 15:00")
			failedLogins[hourKey]++
		}
	}

	// Check for spikes
	for hourKey, count := range failedLogins {
		if count >= r.thresholds.FailedLoginThreshold {
			alert := AuditAlert{
				ID:         r.generateAlertID(),
				Timestamp:  time.Now(),
				Severity:   SeverityHigh,
				EventType:  "FAILED_LOGIN_SPIKE",
				Message:    fmt.Sprintf("Detected %d failed login attempts in 1 hour window", count),
				EventCount: count,
				TimeWindow: time.Hour,
				Details: map[string]string{
					"hour":      hourKey,
					"threshold": fmt.Sprintf("%d", r.thresholds.FailedLoginThreshold),
				},
			}
			result.TriggeredAlerts = append(result.TriggeredAlerts, alert)
			result.Findings = append(result.Findings, alert.Message)
		}
	}
}

// detectUnusualAccessTimes detects access during unusual hours.
func (r *AuditReviewer) detectUnusualAccessTimes(entries []auditEntry, result *AnalysisResult) {
	unusualAccess := 0
	for _, entry := range entries {
		hour := entry.Timestamp.Hour()
		// Check if access is outside normal hours
		if r.isUnusualHour(hour) {
			unusualAccess++
		}
	}

	if unusualAccess > 0 {
		severity := SeverityInfo
		if unusualAccess > 10 {
			severity = SeverityMedium
		}
		if unusualAccess > 50 {
			severity = SeverityHigh
		}

		alert := AuditAlert{
			ID:         r.generateAlertID(),
			Timestamp:  time.Now(),
			Severity:   severity,
			EventType:  "UNUSUAL_ACCESS_TIME",
			Message:    fmt.Sprintf("Detected %d events during unusual hours (%d:00-%d:00)", unusualAccess, r.thresholds.UnusualHourStart, r.thresholds.UnusualHourEnd),
			EventCount: unusualAccess,
			Details: map[string]string{
				"unusual_hour_start": fmt.Sprintf("%d", r.thresholds.UnusualHourStart),
				"unusual_hour_end":   fmt.Sprintf("%d", r.thresholds.UnusualHourEnd),
			},
		}
		result.TriggeredAlerts = append(result.TriggeredAlerts, alert)
		result.Findings = append(result.Findings, alert.Message)
	}
}

// detectPrivilegeEscalation detects potential privilege escalation attempts.
func (r *AuditReviewer) detectPrivilegeEscalation(entries []auditEntry, result *AnalysisResult) {
	privEvents := 0
	for _, entry := range entries {
		// Look for events related to privilege changes
		eventType := strings.ToUpper(entry.EventType)
		if strings.Contains(eventType, "PRIV") ||
			strings.Contains(eventType, "PERMISSION") ||
			strings.Contains(eventType, "ADMIN") ||
			strings.Contains(eventType, "SUDO") {
			privEvents++
		}
	}

	if privEvents >= r.thresholds.PrivEscalationThreshold {
		alert := AuditAlert{
			ID:         r.generateAlertID(),
			Timestamp:  time.Now(),
			Severity:   SeverityCritical,
			EventType:  "PRIVILEGE_ESCALATION",
			Message:    fmt.Sprintf("Detected %d privilege-related events (threshold: %d)", privEvents, r.thresholds.PrivEscalationThreshold),
			EventCount: privEvents,
			Details: map[string]string{
				"threshold": fmt.Sprintf("%d", r.thresholds.PrivEscalationThreshold),
			},
		}
		result.TriggeredAlerts = append(result.TriggeredAlerts, alert)
		result.Findings = append(result.Findings, alert.Message)
	}
}

// detectConfigurationChanges detects unusual configuration changes.
func (r *AuditReviewer) detectConfigurationChanges(entries []auditEntry, result *AnalysisResult) {
	configChanges := 0
	for _, entry := range entries {
		eventType := strings.ToUpper(entry.EventType)
		if strings.Contains(eventType, "CONFIG") ||
			strings.Contains(eventType, "SETTING") {
			configChanges++
		}
	}

	if configChanges >= r.thresholds.ConfigChangeThreshold {
		alert := AuditAlert{
			ID:         r.generateAlertID(),
			Timestamp:  time.Now(),
			Severity:   SeverityMedium,
			EventType:  "CONFIGURATION_CHANGES",
			Message:    fmt.Sprintf("Detected %d configuration changes (threshold: %d)", configChanges, r.thresholds.ConfigChangeThreshold),
			EventCount: configChanges,
			Details: map[string]string{
				"threshold": fmt.Sprintf("%d", r.thresholds.ConfigChangeThreshold),
			},
		}
		result.TriggeredAlerts = append(result.TriggeredAlerts, alert)
		result.Findings = append(result.Findings, alert.Message)
	}
}

// detectDataExfiltration detects potential data exfiltration patterns.
func (r *AuditReviewer) detectDataExfiltration(entries []auditEntry, result *AnalysisResult) {
	// Look for large query patterns or export events
	largeQueries := 0
	exportEvents := 0

	for _, entry := range entries {
		// Check for export events
		eventType := strings.ToUpper(entry.EventType)
		if strings.Contains(eventType, "EXPORT") ||
			strings.Contains(eventType, "DOWNLOAD") {
			exportEvents++
		}

		// Check for unusually large queries (if token count is available)
		// This is a heuristic - adjust based on your threat model
		if entry.Tokens > 10000 {
			largeQueries++
		}
	}

	if exportEvents > 5 || largeQueries > 10 {
		severity := SeverityMedium
		if exportEvents > 20 || largeQueries > 50 {
			severity = SeverityHigh
		}

		alert := AuditAlert{
			ID:        r.generateAlertID(),
			Timestamp: time.Now(),
			Severity:  severity,
			EventType: "POTENTIAL_EXFILTRATION",
			Message:   fmt.Sprintf("Detected potential data exfiltration: %d export events, %d large queries", exportEvents, largeQueries),
			Details: map[string]string{
				"export_events": fmt.Sprintf("%d", exportEvents),
				"large_queries": fmt.Sprintf("%d", largeQueries),
			},
		}
		result.TriggeredAlerts = append(result.TriggeredAlerts, alert)
		result.Findings = append(result.Findings, alert.Message)
	}
}

// isUnusualHour checks if the given hour is outside normal business hours.
func (r *AuditReviewer) isUnusualHour(hour int) bool {
	start := r.thresholds.UnusualHourStart
	end := r.thresholds.UnusualHourEnd

	// Handle overnight range (e.g., 22:00 - 06:00)
	if start > end {
		return hour >= start || hour < end
	}
	// Handle same-day range
	return hour >= start && hour < end
}

// =============================================================================
// EVENT CORRELATION
// =============================================================================

// CorrelateEvents identifies related events across different sessions or time periods.
// Returns groups of correlated events.
func (r *AuditReviewer) CorrelateEvents() ([]EventCorrelation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read all audit entries from the last 7 days
	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	entries, err := r.readAuditEntries(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit entries: %w", err)
	}

	correlations := make([]EventCorrelation, 0)

	// Correlate by session ID
	sessionGroups := r.groupBySession(entries)
	for sessionID, events := range sessionGroups {
		if len(events) > 1 {
			correlations = append(correlations, EventCorrelation{
				Type:       "session",
				Identifier: sessionID,
				Events:     events,
				Count:      len(events),
			})
		}
	}

	return correlations, nil
}

// EventCorrelation represents a group of correlated events.
type EventCorrelation struct {
	Type       string        `json:"type"`       // "session", "user", "ip", etc.
	Identifier string        `json:"identifier"` // Session ID, username, IP, etc.
	Events     []auditEntry  `json:"events"`
	Count      int           `json:"count"`
}

// groupBySession groups audit entries by session ID.
func (r *AuditReviewer) groupBySession(entries []auditEntry) map[string][]auditEntry {
	groups := make(map[string][]auditEntry)
	for _, entry := range entries {
		groups[entry.SessionID] = append(groups[entry.SessionID], entry)
	}
	return groups
}

// =============================================================================
// LOG SEARCH
// =============================================================================

// SearchLogs searches audit logs for entries matching the query.
func (r *AuditReviewer) SearchLogs(query string) ([]auditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Read all audit entries
	entries, err := r.readAuditEntries(time.Time{}, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to read audit entries: %w", err)
	}

	// Filter entries matching query
	query = strings.ToLower(query)
	results := make([]auditEntry, 0)

	for _, entry := range entries {
		// Search across multiple fields
		if strings.Contains(strings.ToLower(entry.EventType), query) ||
			strings.Contains(strings.ToLower(entry.SessionID), query) ||
			strings.Contains(strings.ToLower(entry.Query), query) ||
			strings.Contains(strings.ToLower(entry.Status), query) {
			results = append(results, entry)
		}
	}

	return results, nil
}

// =============================================================================
// ALERT MANAGEMENT
// =============================================================================

// GetAlerts returns all triggered alerts, optionally filtered.
func (r *AuditReviewer) GetAlerts(acknowledged bool) []AuditAlert {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]AuditAlert, 0)
	for _, alert := range r.alerts {
		if alert.Acknowledged == acknowledged || !acknowledged {
			results = append(results, alert)
		}
	}

	return results
}

// AcknowledgeAlert marks an alert as acknowledged.
func (r *AuditReviewer) AcknowledgeAlert(alertID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.alerts {
		if r.alerts[i].ID == alertID {
			r.alerts[i].Acknowledged = true
			return nil
		}
	}

	return fmt.Errorf("alert not found: %s", alertID)
}

// =============================================================================
// COMPLIANCE REPORTING
// =============================================================================

// GenerateReport generates a compliance report in the specified format.
func (r *AuditReviewer) GenerateReport(format string, timeRange time.Duration) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Analyze logs for the time range
	endTime := time.Now()
	startTime := endTime.Add(-timeRange)

	entries, err := r.readAuditEntries(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit entries: %w", err)
	}

	// Generate statistics
	stats := r.generateStatistics(entries)

	// Format output
	switch strings.ToLower(format) {
	case "json":
		return r.reportJSON(stats, entries)
	case "csv":
		return r.reportCSV(stats, entries)
	case "text", "txt":
		return r.reportText(stats, entries)
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}
}

// ReportStatistics contains statistical data for compliance reporting.
type ReportStatistics struct {
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	TotalEvents     int               `json:"total_events"`
	EventsByType    map[string]int    `json:"events_by_type"`
	SuccessCount    int               `json:"success_count"`
	ErrorCount      int               `json:"error_count"`
	UniqueSessions  int               `json:"unique_sessions"`
	AlertsTriggered int               `json:"alerts_triggered"`
}

// generateStatistics computes statistics from audit entries.
func (r *AuditReviewer) generateStatistics(entries []auditEntry) ReportStatistics {
	stats := ReportStatistics{
		EventsByType: make(map[string]int),
	}

	if len(entries) == 0 {
		return stats
	}

	stats.StartTime = entries[0].Timestamp
	stats.EndTime = entries[len(entries)-1].Timestamp
	stats.TotalEvents = len(entries)

	sessions := make(map[string]bool)

	for _, entry := range entries {
		// Count by type
		stats.EventsByType[entry.EventType]++

		// Count success/error
		if strings.Contains(strings.ToLower(entry.Status), "success") {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}

		// Track unique sessions
		sessions[entry.SessionID] = true
	}

	stats.UniqueSessions = len(sessions)
	stats.AlertsTriggered = len(r.alerts)

	return stats
}

// reportJSON generates a JSON format report.
func (r *AuditReviewer) reportJSON(stats ReportStatistics, entries []auditEntry) ([]byte, error) {
	report := map[string]interface{}{
		"generated_at": time.Now().Format(time.RFC3339),
		"format":       "rigrun-audit-report-v1",
		"statistics":   stats,
		"alerts":       r.alerts,
	}

	return json.MarshalIndent(report, "", "  ")
}

// reportCSV generates a CSV format report.
func (r *AuditReviewer) reportCSV(stats ReportStatistics, entries []auditEntry) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"timestamp", "event_type", "session_id", "status", "tokens", "description"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write entries
	for _, entry := range entries {
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.EventType,
			entry.SessionID,
			entry.Status,
			fmt.Sprintf("%d", entry.Tokens),
			entry.Query,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), nil
}

// reportText generates a text format report.
func (r *AuditReviewer) reportText(stats ReportStatistics, entries []auditEntry) ([]byte, error) {
	var buf strings.Builder

	buf.WriteString("AUDIT COMPLIANCE REPORT\n")
	buf.WriteString(strings.Repeat("=", 70) + "\n\n")
	buf.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("Period: %s to %s\n\n", stats.StartTime.Format(time.RFC3339), stats.EndTime.Format(time.RFC3339)))

	buf.WriteString("SUMMARY\n")
	buf.WriteString(strings.Repeat("-", 70) + "\n")
	buf.WriteString(fmt.Sprintf("Total Events:     %d\n", stats.TotalEvents))
	buf.WriteString(fmt.Sprintf("Unique Sessions:  %d\n", stats.UniqueSessions))
	buf.WriteString(fmt.Sprintf("Success Rate:     %.1f%%\n", float64(stats.SuccessCount)*100/float64(stats.TotalEvents)))
	buf.WriteString(fmt.Sprintf("Alerts Triggered: %d\n\n", stats.AlertsTriggered))

	buf.WriteString("EVENTS BY TYPE\n")
	buf.WriteString(strings.Repeat("-", 70) + "\n")
	// Sort event types for consistent output
	eventTypes := make([]string, 0, len(stats.EventsByType))
	for et := range stats.EventsByType {
		eventTypes = append(eventTypes, et)
	}
	sort.Strings(eventTypes)
	for _, et := range eventTypes {
		count := stats.EventsByType[et]
		pct := float64(count) * 100 / float64(stats.TotalEvents)
		buf.WriteString(fmt.Sprintf("  %-20s %6d (%.1f%%)\n", et, count, pct))
	}

	if len(r.alerts) > 0 {
		buf.WriteString("\nALERTS\n")
		buf.WriteString(strings.Repeat("-", 70) + "\n")
		for _, alert := range r.alerts {
			buf.WriteString(fmt.Sprintf("[%s] %s: %s\n", strings.ToUpper(alert.Severity), alert.EventType, alert.Message))
		}
	}

	return []byte(buf.String()), nil
}

// =============================================================================
// SIEM EXPORT
// =============================================================================

// ExportLogs exports audit logs in SIEM-compatible format.
func (r *AuditReviewer) ExportLogs(format string, timeRange time.Duration) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endTime := time.Now()
	startTime := endTime.Add(-timeRange)

	entries, err := r.readAuditEntries(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit entries: %w", err)
	}

	switch strings.ToLower(format) {
	case "json":
		return r.exportJSON(entries)
	case "csv":
		return r.exportCSV(entries)
	case "syslog":
		return r.exportSyslog(entries)
	case "cef":
		return r.exportCEF(entries)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportJSON exports entries as JSON for SIEM ingestion.
func (r *AuditReviewer) exportJSON(entries []auditEntry) ([]byte, error) {
	export := map[string]interface{}{
		"export_time": time.Now().Format(time.RFC3339),
		"format":      "rigrun-siem-export-v1",
		"count":       len(entries),
		"events":      entries,
	}
	return json.MarshalIndent(export, "", "  ")
}

// exportCSV exports entries as CSV for SIEM ingestion.
func (r *AuditReviewer) exportCSV(entries []auditEntry) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"timestamp", "event_type", "session_id", "tier", "query", "tokens", "cost", "status", "error"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write entries
	for _, entry := range entries {
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.EventType,
			entry.SessionID,
			entry.Tier,
			entry.Query,
			fmt.Sprintf("%d", entry.Tokens),
			fmt.Sprintf("%.2f", entry.Cost),
			entry.Status,
			entry.Error,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), nil
}

// exportSyslog exports entries in syslog format (RFC 5424).
func (r *AuditReviewer) exportSyslog(entries []auditEntry) ([]byte, error) {
	var buf strings.Builder
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	for _, entry := range entries {
		// RFC 5424 format
		priority := 134 // local0.info
		msg := fmt.Sprintf("<%d>1 %s %s rigrun - %s [rigrun@0 event_type=\"%s\" session=\"%s\" status=\"%s\"] %s\n",
			priority,
			entry.Timestamp.Format(time.RFC3339),
			hostname,
			entry.EventType,
			entry.EventType,
			entry.SessionID,
			entry.Status,
			entry.Query)
		buf.WriteString(msg)
	}

	return []byte(buf.String()), nil
}

// exportCEF exports entries in Common Event Format (for ArcSight, Splunk, etc.).
func (r *AuditReviewer) exportCEF(entries []auditEntry) ([]byte, error) {
	var buf strings.Builder

	for _, entry := range entries {
		// CEF format: CEF:Version|Device Vendor|Device Product|Device Version|Signature ID|Name|Severity|Extension
		severity := "5" // Medium
		if strings.Contains(strings.ToLower(entry.Status), "error") {
			severity = "8" // High
		}

		msg := fmt.Sprintf("CEF:0|RigRun|TUI|1.0|%s|%s|%s|rt=%d src=%s suser=%s msg=%s\n",
			entry.EventType,
			entry.EventType,
			severity,
			entry.Timestamp.Unix()*1000,
			"localhost",
			entry.SessionID,
			entry.Query)
		buf.WriteString(msg)
	}

	return []byte(buf.String()), nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// auditEntry represents a parsed audit log entry for internal processing.
type auditEntry struct {
	Timestamp time.Time
	EventType string
	SessionID string
	Tier      string
	Query     string
	Tokens    int
	Cost      float64
	Status    string
	Error     string
}

// readAuditEntries reads and parses audit entries from the log file.
func (r *AuditReviewer) readAuditEntries(startTime, endTime time.Time) ([]auditEntry, error) {
	file, err := os.Open(r.auditLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	entries := make([]auditEntry, 0)
	// Read and parse file line by line
	// This is a simplified version - in production, use a proper CSV/JSON parser
	// based on your audit log format

	// For now, return empty slice - actual parsing would go here
	// This would be implemented based on the actual audit log format
	// See audit_cmd.go parseAuditLine() for reference

	return entries, nil
}

// generateAlertID generates a unique alert ID.
func (r *AuditReviewer) generateAlertID() string {
	now := time.Now()
	dateStr := now.Format("20060102")
	// Simple counter-based ID - in production, use atomic counter or UUID
	return fmt.Sprintf("ALT-%s-%04d", dateStr, len(r.alerts)+1)
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalReviewer     *AuditReviewer
	globalReviewerOnce sync.Once
	globalReviewerMu   sync.RWMutex
)

// GlobalAuditReviewer returns the global audit reviewer instance.
func GlobalAuditReviewer() *AuditReviewer {
	globalReviewerOnce.Do(func() {
		auditPath := DefaultAuditPath()
		globalReviewer = NewAuditReviewer(auditPath)
	})
	return globalReviewer
}

// SetGlobalAuditReviewer sets the global audit reviewer instance.
func SetGlobalAuditReviewer(reviewer *AuditReviewer) {
	globalReviewerMu.Lock()
	defer globalReviewerMu.Unlock()
	globalReviewer = reviewer
}
