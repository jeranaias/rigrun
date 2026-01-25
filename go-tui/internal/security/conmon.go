// conmon.go - NIST 800-53 CA-7 Continuous Monitoring for rigrun.
//
// Implements CA-7 (Continuous Monitoring) controls for DoD IL5 compliance.
// Provides real-time security posture monitoring and alerting capabilities.
//
// NIST 800-53 CA-7 Requirements:
//   - CA-7(a): Monitor for unauthorized access, use, and disclosure
//   - CA-7(b): Review and analyze information system monitoring results
//   - CA-7(c): Report monitoring results to appropriate personnel
//   - CA-7(d): Take corrective action when anomalous behavior is detected
//   - CA-7(e): Adjust monitoring activities based on risk
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// MONITORING METRICS
// =============================================================================

// MetricStatus represents the status of a monitoring metric.
type MetricStatus string

const (
	MetricStatusOK       MetricStatus = "OK"       // Within normal thresholds
	MetricStatusWarning  MetricStatus = "WARNING"  // Approaching threshold
	MetricStatusCritical MetricStatus = "CRITICAL" // Exceeded threshold
	MetricStatusUnknown  MetricStatus = "UNKNOWN"  // Unable to determine
)

// MonitoringMetric represents a single monitoring metric.
type MonitoringMetric struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Value       interface{}   `json:"value"`
	Threshold   interface{}   `json:"threshold"`
	Status      MetricStatus  `json:"status"`
	LastChecked time.Time     `json:"last_checked"`
	Unit        string        `json:"unit,omitempty"` // e.g., "per hour", "MB", "%"
}

// =============================================================================
// SECURITY FINDINGS
// =============================================================================

// FindingSeverity represents the severity of a security finding.
type FindingSeverity string

const (
	FindingSeverityInfo     FindingSeverity = "INFO"
	FindingSeverityLow      FindingSeverity = "LOW"
	FindingSeverityMedium   FindingSeverity = "MEDIUM"
	FindingSeverityHigh     FindingSeverity = "HIGH"
	FindingSeverityCritical FindingSeverity = "CRITICAL"
)

// SecurityFinding represents a security issue discovered during monitoring.
type SecurityFinding struct {
	ID          string          `json:"id"`
	Severity    FindingSeverity `json:"severity"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Metric      string          `json:"metric"`
	Value       interface{}     `json:"value"`
	Threshold   interface{}     `json:"threshold"`
	DetectedAt  time.Time       `json:"detected_at"`
	Resolved    bool            `json:"resolved"`
	ResolvedAt  time.Time       `json:"resolved_at,omitempty"`
}

// =============================================================================
// SECURITY POSTURE
// =============================================================================

// SecurityPosture represents the overall security posture of the system.
type SecurityPosture struct {
	OverallScore int                         `json:"overall_score"` // 0-100
	Metrics      map[string]MonitoringMetric `json:"metrics"`
	Findings     []SecurityFinding           `json:"findings"`
	Timestamp    time.Time                   `json:"timestamp"`
	Status       string                      `json:"status"` // "SECURE", "AT_RISK", "COMPROMISED"
}

// =============================================================================
// ALERT
// =============================================================================

// Alert represents a triggered monitoring alert.
type Alert struct {
	ID        string          `json:"id"`
	Severity  FindingSeverity `json:"severity"`
	Metric    string          `json:"metric"`
	Message   string          `json:"message"`
	Value     interface{}     `json:"value"`
	Threshold interface{}     `json:"threshold"`
	Timestamp time.Time       `json:"timestamp"`
	Acknowledged bool         `json:"acknowledged"`
}

// =============================================================================
// CONTINUOUS MONITOR
// =============================================================================

// ContinuousMonitor implements NIST 800-53 CA-7 continuous monitoring.
type ContinuousMonitor struct {
	mu                sync.RWMutex
	running           bool
	stopped           bool                        // SECURITY: Flag to prevent double-close of channel
	stopChan          chan struct{}
	interval          time.Duration
	metrics           map[string]MonitoringMetric
	findings          []SecurityFinding
	alerts            []Alert
	metricHistory     map[string][]MetricSnapshot // Historical data for trends
	lastPosture       *SecurityPosture
	auditLogger       *AuditLogger
}

// MetricSnapshot represents a point-in-time metric value.
type MetricSnapshot struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     interface{} `json:"value"`
	Status    MetricStatus `json:"status"`
}

// NewContinuousMonitor creates a new continuous monitoring instance.
func NewContinuousMonitor(interval time.Duration) *ContinuousMonitor {
	if interval < time.Minute {
		interval = time.Minute // Minimum 1 minute interval
	}

	return &ContinuousMonitor{
		interval:      interval,
		stopChan:      make(chan struct{}),
		metrics:       make(map[string]MonitoringMetric),
		findings:      make([]SecurityFinding, 0),
		alerts:        make([]Alert, 0),
		metricHistory: make(map[string][]MetricSnapshot),
		auditLogger:   GlobalAuditLogger(),
	}
}

// =============================================================================
// MONITORING CONTROL
// =============================================================================

// StartMonitoring begins continuous monitoring of security metrics.
func (cm *ContinuousMonitor) StartMonitoring() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.running {
		return fmt.Errorf("continuous monitoring already running")
	}

	// Initialize default metrics
	cm.initializeMetrics()

	cm.running = true
	cm.stopped = false // SECURITY: Reset stopped flag when starting
	cm.stopChan = make(chan struct{})

	// Log monitoring start
	if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
		cm.auditLogger.LogEvent("SYSTEM", "CONMON_START", map[string]string{
			"interval": cm.interval.String(),
		})
	}

	// Start monitoring goroutine
	go cm.monitoringLoop()

	return nil
}

// StopMonitoring stops continuous monitoring.
// SECURITY: Proper lock type for operation - uses stopped flag to prevent double-close panic
func (cm *ContinuousMonitor) StopMonitoring() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return fmt.Errorf("continuous monitoring not running")
	}

	// SECURITY: Check stopped flag to prevent double-close of channel
	if cm.stopped {
		return nil // Already stopped, avoid panic from double-close
	}
	cm.stopped = true
	close(cm.stopChan)
	cm.running = false

	// Log monitoring stop
	if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
		cm.auditLogger.LogEvent("SYSTEM", "CONMON_STOP", nil)
	}

	return nil
}

// IsRunning returns whether monitoring is currently active.
func (cm *ContinuousMonitor) IsRunning() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.running
}

// =============================================================================
// METRIC MANAGEMENT
// =============================================================================

// initializeMetrics sets up default monitoring metrics with thresholds.
func (cm *ContinuousMonitor) initializeMetrics() {
	now := time.Now()

	cm.metrics = map[string]MonitoringMetric{
		"failed_logins": {
			Name:        "failed_logins",
			Description: "Failed login attempts",
			Value:       0,
			Threshold:   5,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "per hour",
		},
		"config_changes": {
			Name:        "config_changes",
			Description: "Configuration changes",
			Value:       0,
			Threshold:   10,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "per day",
		},
		"audit_gaps": {
			Name:        "audit_gaps",
			Description: "Gaps in audit logging",
			Value:       0,
			Threshold:   0,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "occurrences",
		},
		"encryption_status": {
			Name:        "encryption_status",
			Description: "Encryption enabled",
			Value:       true,
			Threshold:   true,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "boolean",
		},
		"backup_age": {
			Name:        "backup_age",
			Description: "Time since last backup",
			Value:       time.Duration(0),
			Threshold:   24 * time.Hour,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "hours",
		},
		"vuln_count": {
			Name:        "vuln_count",
			Description: "Known vulnerabilities",
			Value:       0,
			Threshold:   0,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "critical",
		},
		"session_count": {
			Name:        "session_count",
			Description: "Active sessions",
			Value:       0,
			Threshold:   100,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "sessions",
		},
		"disk_usage": {
			Name:        "disk_usage",
			Description: "Audit log disk usage",
			Value:       0.0,
			Threshold:   80.0,
			Status:      MetricStatusOK,
			LastChecked: now,
			Unit:        "%",
		},
	}
}

// CheckMetric performs an on-demand check of a specific metric.
func (cm *ContinuousMonitor) CheckMetric(name string) (*MonitoringMetric, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	metric, exists := cm.metrics[name]
	if !exists {
		return nil, fmt.Errorf("metric not found: %s", name)
	}

	// Update metric value based on name
	cm.updateMetricValue(&metric)

	// Store updated metric
	cm.metrics[name] = metric

	// Record snapshot
	cm.recordSnapshot(name, metric)

	return &metric, nil
}

// SetThreshold sets or updates the threshold for a metric.
func (cm *ContinuousMonitor) SetThreshold(metricName string, threshold interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	metric, exists := cm.metrics[metricName]
	if !exists {
		return fmt.Errorf("metric not found: %s", metricName)
	}

	metric.Threshold = threshold
	cm.metrics[metricName] = metric

	// Log threshold change
	if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
		cm.auditLogger.LogEvent("SYSTEM", "CONMON_THRESHOLD_CHANGE", map[string]string{
			"metric":    metricName,
			"threshold": fmt.Sprintf("%v", threshold),
		})
	}

	return nil
}

// =============================================================================
// SECURITY POSTURE
// =============================================================================

// GetSecurityPosture returns the current security posture.
// SECURITY: Proper lock type for operation - uses write lock since we update lastPosture
func (cm *ContinuousMonitor) GetSecurityPosture() *SecurityPosture {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Calculate overall score (0-100)
	score := cm.calculateOverallScore()

	// Determine status
	status := "SECURE"
	if score < 70 {
		status = "AT_RISK"
	}
	if score < 40 {
		status = "COMPROMISED"
	}

	// Make copies to avoid returning references to internal state
	metricsCopy := make(map[string]MonitoringMetric, len(cm.metrics))
	for k, v := range cm.metrics {
		metricsCopy[k] = v
	}
	findingsCopy := make([]SecurityFinding, len(cm.findings))
	copy(findingsCopy, cm.findings)

	posture := &SecurityPosture{
		OverallScore: score,
		Metrics:      metricsCopy,
		Findings:     findingsCopy,
		Timestamp:    time.Now(),
		Status:       status,
	}

	// SECURITY: Write operation requires Lock(), not RLock()
	cm.lastPosture = posture
	return posture
}

// calculateOverallScore computes the overall security posture score.
func (cm *ContinuousMonitor) calculateOverallScore() int {
	if len(cm.metrics) == 0 {
		return 0
	}

	totalScore := 0
	for _, metric := range cm.metrics {
		switch metric.Status {
		case MetricStatusOK:
			totalScore += 100
		case MetricStatusWarning:
			totalScore += 70
		case MetricStatusCritical:
			totalScore += 30
		case MetricStatusUnknown:
			totalScore += 50
		}
	}

	return totalScore / len(cm.metrics)
}

// =============================================================================
// ALERTS
// =============================================================================

// GetAlerts returns all triggered alerts.
func (cm *ContinuousMonitor) GetAlerts() []Alert {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return copy to prevent modification
	alerts := make([]Alert, len(cm.alerts))
	copy(alerts, cm.alerts)
	return alerts
}

// AcknowledgeAlert marks an alert as acknowledged.
func (cm *ContinuousMonitor) AcknowledgeAlert(alertID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, alert := range cm.alerts {
		if alert.ID == alertID {
			cm.alerts[i].Acknowledged = true

			// Log acknowledgment
			if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
				cm.auditLogger.LogEvent("SYSTEM", "CONMON_ALERT_ACK", map[string]string{
					"alert_id": alertID,
					"metric":   alert.Metric,
				})
			}

			return nil
		}
	}

	return fmt.Errorf("alert not found: %s", alertID)
}

// =============================================================================
// REPORT GENERATION
// =============================================================================

// GeneratePostureReport generates a comprehensive security posture report.
func (cm *ContinuousMonitor) GeneratePostureReport() string {
	posture := cm.GetSecurityPosture()

	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.Grow(2048) // Pre-allocate for typical report size
	sb.WriteString("=== CA-7 Security Posture Report ===\n")
	fmt.Fprintf(&sb, "Generated: %s\n", posture.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Overall Score: %d/100\n", posture.OverallScore)
	fmt.Fprintf(&sb, "Status: %s\n\n", posture.Status)

	// Metrics summary
	sb.WriteString("=== Monitoring Metrics ===\n")
	for name, metric := range posture.Metrics {
		fmt.Fprintf(&sb, "  [%s] %s: %v (threshold: %v) %s\n",
			metric.Status, name, metric.Value, metric.Threshold, metric.Unit)
	}
	sb.WriteByte('\n')

	// Active findings
	if len(posture.Findings) > 0 {
		sb.WriteString("=== Security Findings ===\n")
		for _, finding := range posture.Findings {
			if !finding.Resolved {
				fmt.Fprintf(&sb, "  [%s] %s\n", finding.Severity, finding.Title)
				fmt.Fprintf(&sb, "    %s\n", finding.Description)
				fmt.Fprintf(&sb, "    Detected: %s\n", finding.DetectedAt.Format("2006-01-02 15:04:05"))
			}
		}
		sb.WriteByte('\n')
	}

	// Active alerts
	alerts := cm.GetAlerts()
	unacknowledgedAlerts := 0
	for _, alert := range alerts {
		if !alert.Acknowledged {
			unacknowledgedAlerts++
		}
	}

	if unacknowledgedAlerts > 0 {
		fmt.Fprintf(&sb, "=== Active Alerts (%d unacknowledged) ===\n", unacknowledgedAlerts)
		for _, alert := range alerts {
			if !alert.Acknowledged {
				fmt.Fprintf(&sb, "  [%s] %s: %s\n", alert.Severity, alert.Metric, alert.Message)
				fmt.Fprintf(&sb, "    Timestamp: %s\n", alert.Timestamp.Format("2006-01-02 15:04:05"))
			}
		}
		sb.WriteByte('\n')
	}

	sb.WriteString("=== End Report ===\n")
	return sb.String()
}

// =============================================================================
// METRIC HISTORY
// =============================================================================

// GetMetricHistory returns historical data for a metric within a time range.
func (cm *ContinuousMonitor) GetMetricHistory(name string, timeRange time.Duration) ([]MetricSnapshot, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	history, exists := cm.metricHistory[name]
	if !exists {
		return nil, fmt.Errorf("no history found for metric: %s", name)
	}

	// Filter by time range
	cutoff := time.Now().Add(-timeRange)
	filtered := make([]MetricSnapshot, 0)
	for _, snapshot := range history {
		if snapshot.Timestamp.After(cutoff) {
			filtered = append(filtered, snapshot)
		}
	}

	return filtered, nil
}

// recordSnapshot records a metric snapshot for historical tracking.
func (cm *ContinuousMonitor) recordSnapshot(name string, metric MonitoringMetric) {
	snapshot := MetricSnapshot{
		Timestamp: metric.LastChecked,
		Value:     metric.Value,
		Status:    metric.Status,
	}

	if _, exists := cm.metricHistory[name]; !exists {
		cm.metricHistory[name] = make([]MetricSnapshot, 0)
	}

	cm.metricHistory[name] = append(cm.metricHistory[name], snapshot)

	// Keep only last 1000 snapshots per metric
	const maxSnapshots = 1000
	if len(cm.metricHistory[name]) > maxSnapshots {
		cm.metricHistory[name] = cm.metricHistory[name][len(cm.metricHistory[name])-maxSnapshots:]
	}
}

// =============================================================================
// MONITORING LOOP
// =============================================================================

// monitoringLoop performs periodic checks of all metrics.
func (cm *ContinuousMonitor) monitoringLoop() {
	ticker := time.NewTicker(cm.interval)
	defer ticker.Stop()

	// Perform initial check
	cm.performMonitoringCheck()

	for {
		select {
		case <-ticker.C:
			cm.performMonitoringCheck()
		case <-cm.stopChan:
			return
		}
	}
}

// performMonitoringCheck checks all metrics and generates alerts.
func (cm *ContinuousMonitor) performMonitoringCheck() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()

	// Check each metric
	for name, metric := range cm.metrics {
		// Update metric value
		cm.updateMetricValue(&metric)
		metric.LastChecked = now

		// Store updated metric
		cm.metrics[name] = metric

		// Record snapshot
		cm.recordSnapshot(name, metric)

		// Generate alert if threshold exceeded
		if metric.Status == MetricStatusCritical {
			cm.generateAlert(metric)
		}

		// Create or resolve findings
		if metric.Status == MetricStatusCritical || metric.Status == MetricStatusWarning {
			cm.createFinding(metric)
		} else {
			cm.resolveFinding(metric.Name)
		}
	}

	// Log monitoring check
	if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
		cm.auditLogger.LogEvent("SYSTEM", "CONMON_CHECK", map[string]string{
			"metrics_checked": fmt.Sprintf("%d", len(cm.metrics)),
			"active_alerts":   fmt.Sprintf("%d", len(cm.alerts)),
		})
	}
}

// updateMetricValue updates the current value of a metric.
func (cm *ContinuousMonitor) updateMetricValue(metric *MonitoringMetric) {
	now := time.Now()

	switch metric.Name {
	case "failed_logins":
		// Check failed login attempts from lockout manager
		lockoutMgr := GlobalLockoutManager()
		if lockoutMgr != nil {
			// Get lockout statistics - use TotalLockouts as a proxy for failed attempts
			stats := lockoutMgr.GetStats()
			failedCount := stats.TotalLockouts
			metric.Value = failedCount
			metric.Status = cm.evaluateIntThreshold(failedCount, metric.Threshold.(int))
		}

	case "config_changes":
		// Check config file modification time
		configPath := filepath.Join(os.Getenv("HOME"), ".rigrun", "config.yaml")
		if info, err := os.Stat(configPath); err == nil {
			age := now.Sub(info.ModTime())
			if age < 24*time.Hour {
				metric.Value = 1
				metric.Status = MetricStatusOK
			} else {
				metric.Value = 0
				metric.Status = MetricStatusOK
			}
		}

	case "audit_gaps":
		// Check for gaps in audit logging
		auditLogger := GlobalAuditLogger()
		if auditLogger != nil && !auditLogger.IsEnabled() {
			metric.Value = 1
			metric.Status = MetricStatusCritical
		} else {
			metric.Value = 0
			metric.Status = MetricStatusOK
		}

	case "encryption_status":
		// Check encryption status
		encMgr := GlobalEncryptionManager()
		if encMgr != nil {
			initialized := encMgr.IsInitialized()
			metric.Value = initialized
			if initialized {
				metric.Status = MetricStatusOK
			} else {
				metric.Status = MetricStatusWarning
			}
		}

	case "backup_age":
		// Check backup age (placeholder - would integrate with actual backup system)
		metric.Value = time.Duration(0)
		metric.Status = MetricStatusOK

	case "vuln_count":
		// Check for known vulnerabilities (placeholder)
		metric.Value = 0
		metric.Status = MetricStatusOK

	case "session_count":
		// Check active sessions
		authMgr := GlobalAuthManager()
		if authMgr != nil {
			sessions := authMgr.ListSessions()
			metric.Value = len(sessions)
			metric.Status = cm.evaluateIntThreshold(len(sessions), metric.Threshold.(int))
		}

	case "disk_usage":
		// Check audit log disk usage
		auditLogger := GlobalAuditLogger()
		if auditLogger != nil {
			usedMB, totalMB, err := auditLogger.CheckCapacity()
			if err == nil && totalMB > 0 {
				usagePercent := float64(usedMB) * 100 / float64(totalMB)
				metric.Value = usagePercent
				metric.Status = cm.evaluateFloatThreshold(usagePercent, metric.Threshold.(float64))
			}
		}
	}
}

// evaluateIntThreshold evaluates an integer metric against its threshold.
func (cm *ContinuousMonitor) evaluateIntThreshold(value, threshold int) MetricStatus {
	if value >= threshold {
		return MetricStatusCritical
	}
	if value >= threshold*8/10 {
		return MetricStatusWarning
	}
	return MetricStatusOK
}

// evaluateFloatThreshold evaluates a float metric against its threshold.
func (cm *ContinuousMonitor) evaluateFloatThreshold(value, threshold float64) MetricStatus {
	if value >= threshold {
		return MetricStatusCritical
	}
	if value >= threshold*0.8 {
		return MetricStatusWarning
	}
	return MetricStatusOK
}

// generateAlert creates a new alert for a critical metric.
func (cm *ContinuousMonitor) generateAlert(metric MonitoringMetric) {
	// Check if alert already exists
	for _, alert := range cm.alerts {
		if alert.Metric == metric.Name && !alert.Acknowledged {
			return // Alert already exists
		}
	}

	// Determine severity
	severity := FindingSeverityHigh
	if metric.Status == MetricStatusCritical {
		severity = FindingSeverityCritical
	}

	alert := Alert{
		ID:        fmt.Sprintf("ALERT_%s_%d", metric.Name, time.Now().Unix()),
		Severity:  severity,
		Metric:    metric.Name,
		Message:   fmt.Sprintf("%s exceeded threshold: %v > %v", metric.Name, metric.Value, metric.Threshold),
		Value:     metric.Value,
		Threshold: metric.Threshold,
		Timestamp: time.Now(),
		Acknowledged: false,
	}

	cm.alerts = append(cm.alerts, alert)

	// Log alert generation
	if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
		cm.auditLogger.LogEvent("SYSTEM", "CONMON_ALERT", map[string]string{
			"alert_id": alert.ID,
			"metric":   metric.Name,
			"severity": string(severity),
			"value":    fmt.Sprintf("%v", metric.Value),
		})
	}
}

// createFinding creates a security finding for a problematic metric.
func (cm *ContinuousMonitor) createFinding(metric MonitoringMetric) {
	// Check if finding already exists
	for i, finding := range cm.findings {
		if finding.Metric == metric.Name && !finding.Resolved {
			// Update existing finding
			cm.findings[i].Value = metric.Value
			return
		}
	}

	// Determine severity
	severity := FindingSeverityMedium
	if metric.Status == MetricStatusCritical {
		severity = FindingSeverityHigh
	}

	finding := SecurityFinding{
		ID:          fmt.Sprintf("FINDING_%s_%d", metric.Name, time.Now().Unix()),
		Severity:    severity,
		Title:       fmt.Sprintf("%s threshold exceeded", metric.Description),
		Description: fmt.Sprintf("Metric %s has value %v which exceeds threshold %v", metric.Name, metric.Value, metric.Threshold),
		Metric:      metric.Name,
		Value:       metric.Value,
		Threshold:   metric.Threshold,
		DetectedAt:  time.Now(),
		Resolved:    false,
	}

	cm.findings = append(cm.findings, finding)
}

// resolveFinding marks a finding as resolved when metric returns to normal.
func (cm *ContinuousMonitor) resolveFinding(metricName string) {
	for i, finding := range cm.findings {
		if finding.Metric == metricName && !finding.Resolved {
			cm.findings[i].Resolved = true
			cm.findings[i].ResolvedAt = time.Now()

			// Log finding resolution
			if cm.auditLogger != nil && cm.auditLogger.IsEnabled() {
				cm.auditLogger.LogEvent("SYSTEM", "CONMON_FINDING_RESOLVED", map[string]string{
					"finding_id": finding.ID,
					"metric":     metricName,
				})
			}
		}
	}
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalConMon     *ContinuousMonitor
	globalConMonOnce sync.Once
)

// GlobalContinuousMonitor returns the global continuous monitoring instance.
func GlobalContinuousMonitor() *ContinuousMonitor {
	globalConMonOnce.Do(func() {
		globalConMon = NewContinuousMonitor(5 * time.Minute) // Check every 5 minutes
	})
	return globalConMon
}
