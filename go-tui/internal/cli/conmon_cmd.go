// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// conmon_cmd.go - Continuous Monitoring CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 CA-7 (Continuous Monitoring) command interface.
//
// Command: conmon [subcommand]
// Short:   Continuous monitoring (IL5 CA-7)
// Aliases: monitor
//
// Subcommands:
//   status (default)    Show monitoring status
//   start               Start continuous monitoring
//   stop                Stop monitoring
//   metrics             Show current metrics
//   alerts              Show active alerts
//   history             Show monitoring history
//
// Examples:
//   rigrun conmon                     Show status (default)
//   rigrun conmon status              Show monitoring status
//   rigrun conmon status --json       Status in JSON format
//   rigrun conmon start               Start monitoring
//   rigrun conmon stop                Stop monitoring
//   rigrun conmon metrics             Show metrics dashboard
//   rigrun conmon alerts              Show active alerts
//   rigrun conmon history             Show historical data
//
// CA-7 Monitoring Includes:
//   - Security control effectiveness
//   - Configuration drift detection
//   - Vulnerability scanning
//   - Compliance status tracking
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// CONMON COMMAND STYLES
// =============================================================================

var (
	conmonTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	conmonSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	conmonLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(18)

	conmonValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	conmonOKStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	conmonWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	conmonCriticalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	conmonDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	conmonSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	conmonErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)
)

// =============================================================================
// CONMON ARGUMENTS
// =============================================================================

// ConmonArgs holds parsed conmon command arguments.
type ConmonArgs struct {
	Subcommand string
	Metric     string
	Threshold  string
	TimeRange  string
	JSON       bool
}

// parseConmonArgs parses conmon command arguments.
func parseConmonArgs(args *Args, remaining []string) ConmonArgs {
	conmonArgs := ConmonArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		conmonArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	// Parse metric name for check/threshold commands
	if len(remaining) > 0 && (conmonArgs.Subcommand == "check" || conmonArgs.Subcommand == "threshold") {
		conmonArgs.Metric = remaining[0]
		remaining = remaining[1:]
	}

	// Parse threshold value for threshold command
	if len(remaining) > 0 && conmonArgs.Subcommand == "threshold" {
		conmonArgs.Threshold = remaining[0]
		remaining = remaining[1:]
	}

	// Parse flags
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--range", "-r":
			if i+1 < len(remaining) {
				i++
				conmonArgs.TimeRange = remaining[i]
			}
		case "--json":
			conmonArgs.JSON = true
		default:
			if strings.HasPrefix(arg, "--range=") {
				conmonArgs.TimeRange = strings.TrimPrefix(arg, "--range=")
			}
		}
	}

	// Default time range
	if conmonArgs.TimeRange == "" {
		conmonArgs.TimeRange = "24h"
	}

	return conmonArgs
}

// =============================================================================
// HANDLE CONMON
// =============================================================================

// HandleConmon handles the "conmon" command with various subcommands.
func HandleConmon(args Args) error {
	conmonArgs := parseConmonArgs(&args, args.Raw)

	switch conmonArgs.Subcommand {
	case "", "status":
		return handleConmonStatus(conmonArgs)
	case "start":
		return handleConmonStart(conmonArgs)
	case "stop":
		return handleConmonStop(conmonArgs)
	case "posture":
		return handleConmonPosture(conmonArgs)
	case "metrics":
		return handleConmonMetrics(conmonArgs)
	case "check":
		return handleConmonCheck(conmonArgs)
	case "alerts":
		return handleConmonAlerts(conmonArgs)
	case "threshold":
		return handleConmonThreshold(conmonArgs)
	case "report":
		return handleConmonReport(conmonArgs)
	case "history":
		return handleConmonHistory(conmonArgs)
	default:
		return fmt.Errorf("unknown conmon subcommand: %s\n\nUsage:\n"+
			"  rigrun conmon start                   Start continuous monitoring\n"+
			"  rigrun conmon stop                    Stop continuous monitoring\n"+
			"  rigrun conmon status                  Show monitoring status\n"+
			"  rigrun conmon posture                 Show security posture summary\n"+
			"  rigrun conmon metrics                 List all metrics and values\n"+
			"  rigrun conmon check <metric>          Check specific metric\n"+
			"  rigrun conmon alerts                  Show active alerts\n"+
			"  rigrun conmon threshold <metric> <value>  Set alert threshold\n"+
			"  rigrun conmon report                  Generate posture report\n"+
			"  rigrun conmon history <metric>        Show metric history\n"+
			"    --range 24h                         Time range (default: 24h)",
			conmonArgs.Subcommand)
	}
}

// =============================================================================
// CONMON START
// =============================================================================

// handleConmonStart starts continuous monitoring.
func handleConmonStart(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()

	if conMon.IsRunning() {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"message": "continuous monitoring already running",
			})
		}
		fmt.Println()
		fmt.Println(conmonWarningStyle.Render("Continuous monitoring is already running."))
		fmt.Println()
		return nil
	}

	if err := conMon.StartMonitoring(); err != nil {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"success": true,
			"message": "continuous monitoring started",
		})
	}

	fmt.Println()
	fmt.Printf("%s Continuous monitoring started\n", conmonSuccessStyle.Render("[OK]"))
	fmt.Println("  Monitoring interval: 5 minutes")
	fmt.Println("  Use 'rigrun conmon status' to view current status")
	fmt.Println()

	return nil
}

// =============================================================================
// CONMON STOP
// =============================================================================

// handleConmonStop stops continuous monitoring.
func handleConmonStop(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()

	if !conMon.IsRunning() {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"message": "continuous monitoring not running",
			})
		}
		fmt.Println()
		fmt.Println(conmonWarningStyle.Render("Continuous monitoring is not running."))
		fmt.Println()
		return nil
	}

	if err := conMon.StopMonitoring(); err != nil {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"success": true,
			"message": "continuous monitoring stopped",
		})
	}

	fmt.Println()
	fmt.Printf("%s Continuous monitoring stopped\n", conmonSuccessStyle.Render("[OK]"))
	fmt.Println()

	return nil
}

// =============================================================================
// CONMON STATUS
// =============================================================================

// handleConmonStatus shows the current monitoring status.
func handleConmonStatus(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()
	isRunning := conMon.IsRunning()

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"running": isRunning,
		})
	}

	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(conmonTitleStyle.Render("CA-7 Continuous Monitoring Status"))
	fmt.Println(conmonDimStyle.Render(separator))
	fmt.Println()

	if isRunning {
		fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Status:"), conmonOKStyle.Render("RUNNING"))
	} else {
		fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Status:"), conmonCriticalStyle.Render("STOPPED"))
		fmt.Println()
		fmt.Println(conmonDimStyle.Render("  Use 'rigrun conmon start' to begin monitoring"))
	}
	fmt.Println()

	return nil
}

// =============================================================================
// CONMON POSTURE
// =============================================================================

// handleConmonPosture shows the security posture summary.
func handleConmonPosture(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()
	posture := conMon.GetSecurityPosture()

	if args.JSON {
		return outputJSONConmon(posture)
	}

	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(conmonTitleStyle.Render("CA-7 Security Posture"))
	fmt.Println(conmonDimStyle.Render(separator))
	fmt.Println()

	// Overall status
	statusStyle := conmonOKStyle
	if posture.Status == "AT_RISK" {
		statusStyle = conmonWarningStyle
	} else if posture.Status == "COMPROMISED" {
		statusStyle = conmonCriticalStyle
	}

	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Status:"), statusStyle.Render(posture.Status))
	fmt.Printf("  %s%d/100\n", conmonLabelStyle.Render("Overall Score:"), posture.OverallScore)
	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Timestamp:"), posture.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Metrics summary
	fmt.Println(conmonSectionStyle.Render("Metrics Summary"))
	okCount := 0
	warningCount := 0
	criticalCount := 0
	for _, metric := range posture.Metrics {
		switch metric.Status {
		case security.MetricStatusOK:
			okCount++
		case security.MetricStatusWarning:
			warningCount++
		case security.MetricStatusCritical:
			criticalCount++
		}
	}
	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("OK:"), conmonOKStyle.Render(fmt.Sprintf("%d", okCount)))
	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Warning:"), conmonWarningStyle.Render(fmt.Sprintf("%d", warningCount)))
	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Critical:"), conmonCriticalStyle.Render(fmt.Sprintf("%d", criticalCount)))
	fmt.Println()

	// Active findings
	unresolvedFindings := 0
	for _, finding := range posture.Findings {
		if !finding.Resolved {
			unresolvedFindings++
		}
	}
	if unresolvedFindings > 0 {
		fmt.Println(conmonSectionStyle.Render(fmt.Sprintf("Active Findings (%d)", unresolvedFindings)))
		for _, finding := range posture.Findings {
			if !finding.Resolved {
				severityStyle := conmonValueStyle
				switch finding.Severity {
				case security.FindingSeverityHigh, security.FindingSeverityCritical:
					severityStyle = conmonCriticalStyle
				case security.FindingSeverityMedium:
					severityStyle = conmonWarningStyle
				}
				fmt.Printf("  %s %s\n", severityStyle.Render("["+string(finding.Severity)+"]"), finding.Title)
			}
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// CONMON METRICS
// =============================================================================

// handleConmonMetrics lists all metrics and their current values.
func handleConmonMetrics(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()
	posture := conMon.GetSecurityPosture()

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"metrics": posture.Metrics,
		})
	}

	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(conmonTitleStyle.Render("CA-7 Monitoring Metrics"))
	fmt.Println(conmonDimStyle.Render(separator))
	fmt.Println()

	// Sort metrics by name for consistent output
	var metricNames []string
	for name := range posture.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Display metrics
	for _, name := range metricNames {
		metric := posture.Metrics[name]

		statusStyle := conmonOKStyle
		switch metric.Status {
		case security.MetricStatusWarning:
			statusStyle = conmonWarningStyle
		case security.MetricStatusCritical:
			statusStyle = conmonCriticalStyle
		case security.MetricStatusUnknown:
			statusStyle = conmonDimStyle
		}

		fmt.Printf("  %s %s\n", statusStyle.Render("["+string(metric.Status)+"]"), metric.Description)
		fmt.Printf("    Name:      %s\n", name)
		fmt.Printf("    Value:     %v %s\n", metric.Value, metric.Unit)
		fmt.Printf("    Threshold: %v %s\n", metric.Threshold, metric.Unit)
		fmt.Printf("    Checked:   %s\n", metric.LastChecked.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	return nil
}

// =============================================================================
// CONMON CHECK
// =============================================================================

// handleConmonCheck performs an on-demand check of a specific metric.
func handleConmonCheck(args ConmonArgs) error {
	if args.Metric == "" {
		return fmt.Errorf("metric name required\nUsage: rigrun conmon check <metric>")
	}

	conMon := security.GlobalContinuousMonitor()
	metric, err := conMon.CheckMetric(args.Metric)
	if err != nil {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"success": true,
			"metric":  metric,
		})
	}

	fmt.Println()
	fmt.Println(conmonTitleStyle.Render("Metric Check: " + metric.Description))
	fmt.Println()

	statusStyle := conmonOKStyle
	switch metric.Status {
	case security.MetricStatusWarning:
		statusStyle = conmonWarningStyle
	case security.MetricStatusCritical:
		statusStyle = conmonCriticalStyle
	}

	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Status:"), statusStyle.Render(string(metric.Status)))
	fmt.Printf("  %s%v %s\n", conmonLabelStyle.Render("Current Value:"), metric.Value, metric.Unit)
	fmt.Printf("  %s%v %s\n", conmonLabelStyle.Render("Threshold:"), metric.Threshold, metric.Unit)
	fmt.Printf("  %s%s\n", conmonLabelStyle.Render("Last Checked:"), metric.LastChecked.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

// =============================================================================
// CONMON ALERTS
// =============================================================================

// handleConmonAlerts shows active alerts.
func handleConmonAlerts(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()
	alerts := conMon.GetAlerts()

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"alerts": alerts,
			"count":  len(alerts),
		})
	}

	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(conmonTitleStyle.Render("CA-7 Active Alerts"))
	fmt.Println(conmonDimStyle.Render(separator))
	fmt.Println()

	if len(alerts) == 0 {
		fmt.Println(conmonDimStyle.Render("  No active alerts."))
		fmt.Println()
		return nil
	}

	// Count unacknowledged alerts
	unackCount := 0
	for _, alert := range alerts {
		if !alert.Acknowledged {
			unackCount++
		}
	}
	fmt.Printf("  Total: %d alerts (%d unacknowledged)\n\n", len(alerts), unackCount)

	// Display alerts
	for _, alert := range alerts {
		severityStyle := conmonValueStyle
		switch alert.Severity {
		case security.FindingSeverityCritical:
			severityStyle = conmonCriticalStyle
		case security.FindingSeverityHigh:
			severityStyle = conmonCriticalStyle
		case security.FindingSeverityMedium:
			severityStyle = conmonWarningStyle
		}

		ackStatus := ""
		if alert.Acknowledged {
			ackStatus = conmonDimStyle.Render(" (acknowledged)")
		}

		fmt.Printf("  %s %s%s\n", severityStyle.Render("["+string(alert.Severity)+"]"), alert.Message, ackStatus)
		fmt.Printf("    ID:        %s\n", alert.ID)
		fmt.Printf("    Metric:    %s\n", alert.Metric)
		fmt.Printf("    Value:     %v (threshold: %v)\n", alert.Value, alert.Threshold)
		fmt.Printf("    Timestamp: %s\n", alert.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	return nil
}

// =============================================================================
// CONMON THRESHOLD
// =============================================================================

// handleConmonThreshold sets or updates a metric threshold.
func handleConmonThreshold(args ConmonArgs) error {
	if args.Metric == "" || args.Threshold == "" {
		return fmt.Errorf("metric name and threshold value required\nUsage: rigrun conmon threshold <metric> <value>")
	}

	conMon := security.GlobalContinuousMonitor()

	// Parse threshold value (try int first, then float, then bool)
	var threshold interface{}
	if intVal, err := strconv.Atoi(args.Threshold); err == nil {
		threshold = intVal
	} else if floatVal, err := strconv.ParseFloat(args.Threshold, 64); err == nil {
		threshold = floatVal
	} else if boolVal, err := strconv.ParseBool(args.Threshold); err == nil {
		threshold = boolVal
	} else {
		return fmt.Errorf("invalid threshold value: %s", args.Threshold)
	}

	if err := conMon.SetThreshold(args.Metric, threshold); err != nil {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"success":   true,
			"metric":    args.Metric,
			"threshold": threshold,
		})
	}

	fmt.Println()
	fmt.Printf("%s Threshold updated\n", conmonSuccessStyle.Render("[OK]"))
	fmt.Printf("  Metric:    %s\n", args.Metric)
	fmt.Printf("  Threshold: %v\n", threshold)
	fmt.Println()

	return nil
}

// =============================================================================
// CONMON REPORT
// =============================================================================

// handleConmonReport generates a security posture report.
func handleConmonReport(args ConmonArgs) error {
	conMon := security.GlobalContinuousMonitor()

	if args.JSON {
		posture := conMon.GetSecurityPosture()
		return outputJSONConmon(posture)
	}

	report := conMon.GeneratePostureReport()
	fmt.Println(report)

	return nil
}

// =============================================================================
// CONMON HISTORY
// =============================================================================

// handleConmonHistory shows historical data for a metric.
func handleConmonHistory(args ConmonArgs) error {
	if args.Metric == "" {
		return fmt.Errorf("metric name required\nUsage: rigrun conmon history <metric> [--range 24h]")
	}

	// Parse time range
	timeRange, err := time.ParseDuration(args.TimeRange)
	if err != nil {
		return fmt.Errorf("invalid time range: %s (use format like '24h', '7d', '30m')", args.TimeRange)
	}

	conMon := security.GlobalContinuousMonitor()
	history, err := conMon.GetMetricHistory(args.Metric, timeRange)
	if err != nil {
		if args.JSON {
			return outputJSONConmon(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSONConmon(map[string]interface{}{
			"metric":   args.Metric,
			"range":    args.TimeRange,
			"history":  history,
			"count":    len(history),
		})
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(conmonTitleStyle.Render(fmt.Sprintf("Metric History: %s", args.Metric)))
	fmt.Println(conmonDimStyle.Render(separator))
	fmt.Printf("  Time Range: %s\n", args.TimeRange)
	fmt.Printf("  Data Points: %d\n", len(history))
	fmt.Println()

	if len(history) == 0 {
		fmt.Println(conmonDimStyle.Render("  No historical data available."))
		fmt.Println()
		return nil
	}

	// Display history (most recent first)
	for i := len(history) - 1; i >= 0; i-- {
		snapshot := history[i]

		statusStyle := conmonOKStyle
		switch snapshot.Status {
		case security.MetricStatusWarning:
			statusStyle = conmonWarningStyle
		case security.MetricStatusCritical:
			statusStyle = conmonCriticalStyle
		}

		fmt.Printf("  %s %s %v\n",
			conmonDimStyle.Render(snapshot.Timestamp.Format("2006-01-02 15:04:05")),
			statusStyle.Render(string(snapshot.Status)),
			snapshot.Value)
	}
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// outputJSONConmon outputs data as JSON.
func outputJSONConmon(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
