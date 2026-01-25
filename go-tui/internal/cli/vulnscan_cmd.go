// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// vulnscan_cmd.go - CLI commands for NIST 800-53 RA-5 vulnerability scanning.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 RA-5 (Vulnerability Monitoring and Scanning)
// for DoD IL5 compliance.
//
// Command: vuln [subcommand]
// Short:   Vulnerability scanning (IL5 RA-5)
// Aliases: vulnscan, vs
//
// Subcommands:
//   scan (default)      Run full vulnerability scan
//   list                List all vulnerabilities
//   show <id>           Show vulnerability details
//   export [format]     Export vulnerability report
//   schedule [interval] Set scan schedule
//   status              Show last scan results
//
// Examples:
//   rigrun vuln                           Run scan (default)
//   rigrun vuln scan                      Full vulnerability scan
//   rigrun vuln scan --deps               Scan dependencies only
//   rigrun vuln scan --config             Scan config only
//   rigrun vuln list                      List all vulnerabilities
//   rigrun vuln list --severity critical  Filter by severity
//   rigrun vuln show CVE-2024-1234        Show details
//   rigrun vuln export json               Export as JSON
//   rigrun vuln export csv                Export as CSV
//   rigrun vuln schedule 24h              Daily scans
//   rigrun vuln status                    Last scan summary
//   rigrun vuln status --json             JSON format
//
// Severity Levels:
//   critical            Immediate action required
//   high                Prioritize remediation
//   medium              Standard remediation timeline
//   low                 Scheduled maintenance
//   info                Informational only
//
// Flags:
//   --deps              Scan dependencies only
//   --config            Scan configuration only
//   --severity LEVEL    Filter by severity
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// VULN COMMAND STYLES
// =============================================================================

var (
	vulnTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	vulnSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	vulnLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(16)

	vulnValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	vulnGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	vulnYellowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // Yellow

	vulnRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	vulnOrangeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")) // Orange

	vulnDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	vulnSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	vulnErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	vulnSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// VULN ARGUMENTS
// =============================================================================

// VulnArgs holds parsed vuln command arguments.
type VulnArgs struct {
	Subcommand string
	DepsOnly   bool
	ConfigOnly bool
	BinaryOnly bool
	Severity   string
	Format     string
	Output     string
	Interval   string
	ID         string
	JSON       bool
}

// parseVulnArgs parses vuln command specific arguments.
func parseVulnArgs(args *Args, remaining []string) VulnArgs {
	vulnArgs := VulnArgs{
		JSON:   args.JSON,
		Format: "json", // Default format
	}

	if len(remaining) > 0 {
		vulnArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--deps":
			vulnArgs.DepsOnly = true
		case "--config":
			vulnArgs.ConfigOnly = true
		case "--binary":
			vulnArgs.BinaryOnly = true
		case "--severity", "-s":
			if i+1 < len(remaining) {
				i++
				vulnArgs.Severity = strings.ToUpper(remaining[i])
			}
		case "--format", "-f":
			if i+1 < len(remaining) {
				i++
				vulnArgs.Format = strings.ToLower(remaining[i])
			}
		case "--output", "-o":
			if i+1 < len(remaining) {
				i++
				vulnArgs.Output = remaining[i]
			}
		case "--interval":
			if i+1 < len(remaining) {
				i++
				vulnArgs.Interval = remaining[i]
			}
		case "--json":
			vulnArgs.JSON = true
		default:
			// Check for --severity=value format
			if strings.HasPrefix(arg, "--severity=") {
				vulnArgs.Severity = strings.ToUpper(strings.TrimPrefix(arg, "--severity="))
			} else if strings.HasPrefix(arg, "--format=") {
				vulnArgs.Format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
			} else if strings.HasPrefix(arg, "--output=") {
				vulnArgs.Output = strings.TrimPrefix(arg, "--output=")
			} else if strings.HasPrefix(arg, "--interval=") {
				vulnArgs.Interval = strings.TrimPrefix(arg, "--interval=")
			} else if !strings.HasPrefix(arg, "-") && vulnArgs.ID == "" {
				vulnArgs.ID = arg
			}
		}
	}

	return vulnArgs
}

// =============================================================================
// HANDLE VULN
// =============================================================================

// HandleVuln handles the "vuln" command with various subcommands.
func HandleVuln(args Args) error {
	vulnArgs := parseVulnArgs(&args, args.Raw)

	switch vulnArgs.Subcommand {
	case "", "scan":
		return handleVulnScan(vulnArgs)
	case "list", "ls":
		return handleVulnList(vulnArgs)
	case "show":
		return handleVulnShow(vulnArgs)
	case "export":
		return handleVulnExport(vulnArgs)
	case "schedule":
		return handleVulnSchedule(vulnArgs)
	case "status":
		return handleVulnStatus(vulnArgs)
	default:
		return fmt.Errorf("unknown vuln subcommand: %s\n\nUsage:\n"+
			"  rigrun vuln scan [--deps|--config|--binary]  Run vulnerability scan\n"+
			"  rigrun vuln list [--severity LEVEL]          List vulnerabilities\n"+
			"  rigrun vuln show <id>                        Show vulnerability details\n"+
			"  rigrun vuln export [--format json|csv]       Export report\n"+
			"  rigrun vuln schedule [interval]              Set scan schedule\n"+
			"  rigrun vuln status                           Show scan status", vulnArgs.Subcommand)
	}
}

// =============================================================================
// VULN SCAN
// =============================================================================

// handleVulnScan runs a vulnerability scan.
func handleVulnScan(vulnArgs VulnArgs) error {
	scanner := security.GlobalVulnScanner()

	if vulnArgs.JSON {
		return handleVulnScanJSON(scanner, vulnArgs)
	}

	// Display scan header
	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(vulnTitleStyle.Render("RA-5 Vulnerability Scan"))
	fmt.Println(vulnSeparatorStyle.Render(separator))
	fmt.Println()

	// Determine scan type
	scanType := "Full Scan"
	if vulnArgs.DepsOnly {
		scanType = "Dependencies Only"
	} else if vulnArgs.ConfigOnly {
		scanType = "Configuration Only"
	} else if vulnArgs.BinaryOnly {
		scanType = "Binary Only"
	}

	fmt.Printf("Scan Type: %s\n", vulnValueStyle.Render(scanType))
	fmt.Printf("Started: %s\n", vulnDimStyle.Render(time.Now().Format("2006-01-02 15:04:05")))
	fmt.Println()

	// Run scans
	var allVulns []security.Vulnerability
	var errors []error

	if vulnArgs.DepsOnly || (!vulnArgs.ConfigOnly && !vulnArgs.BinaryOnly) {
		fmt.Print(vulnDimStyle.Render("Scanning dependencies... "))
		vulns, err := scanner.ScanDependencies()
		if err != nil {
			fmt.Println(vulnRedStyle.Render("FAILED"))
			errors = append(errors, err)
		} else {
			fmt.Println(vulnGreenStyle.Render("OK"))
			allVulns = append(allVulns, vulns...)
		}
	}

	if vulnArgs.ConfigOnly || (!vulnArgs.DepsOnly && !vulnArgs.BinaryOnly) {
		fmt.Print(vulnDimStyle.Render("Scanning configuration... "))
		vulns, err := scanner.ScanConfig()
		if err != nil {
			fmt.Println(vulnRedStyle.Render("FAILED"))
			errors = append(errors, err)
		} else {
			fmt.Println(vulnGreenStyle.Render("OK"))
			allVulns = append(allVulns, vulns...)
		}
	}

	if vulnArgs.BinaryOnly || (!vulnArgs.DepsOnly && !vulnArgs.ConfigOnly) {
		fmt.Print(vulnDimStyle.Render("Scanning binaries... "))
		vulns, err := scanner.ScanBinaries()
		if err != nil {
			fmt.Println(vulnRedStyle.Render("FAILED"))
			errors = append(errors, err)
		} else {
			fmt.Println(vulnGreenStyle.Render("OK"))
			allVulns = append(allVulns, vulns...)
		}
	}

	fmt.Println()

	// Display summary
	summary := scanner.GetScanSummary()
	displayScanSummary(summary, len(allVulns))

	// Display vulnerabilities if found
	if len(allVulns) > 0 {
		fmt.Println()
		fmt.Println(vulnSectionStyle.Render("Discovered Vulnerabilities"))
		fmt.Println()

		// Sort by severity
		security.SortBySeverity(allVulns)

		// Display each vulnerability
		for _, vuln := range allVulns {
			displayVulnerability(vuln, false)
		}
	}

	// Display errors if any
	if len(errors) > 0 {
		fmt.Println()
		fmt.Println(vulnSectionStyle.Render("Scan Errors"))
		for _, err := range errors {
			fmt.Printf("  %s %s\n", vulnRedStyle.Render("[ERROR]"), err.Error())
		}
	}

	fmt.Println()

	// Log the scan
	security.AuditLogEvent("CLI", "VULN_SCAN", map[string]string{
		"type":              scanType,
		"vulnerabilities":   fmt.Sprintf("%d", len(allVulns)),
		"critical":          fmt.Sprintf("%d", summary[security.SeverityCritical]),
		"high":              fmt.Sprintf("%d", summary[security.SeverityHigh]),
	})

	return nil
}

// handleVulnScanJSON runs scan and outputs JSON.
func handleVulnScanJSON(scanner *security.VulnScanner, vulnArgs VulnArgs) error {
	var allVulns []security.Vulnerability
	var errors []string

	if vulnArgs.DepsOnly || (!vulnArgs.ConfigOnly && !vulnArgs.BinaryOnly) {
		vulns, err := scanner.ScanDependencies()
		if err != nil {
			errors = append(errors, "dependencies: "+err.Error())
		} else {
			allVulns = append(allVulns, vulns...)
		}
	}

	if vulnArgs.ConfigOnly || (!vulnArgs.DepsOnly && !vulnArgs.BinaryOnly) {
		vulns, err := scanner.ScanConfig()
		if err != nil {
			errors = append(errors, "config: "+err.Error())
		} else {
			allVulns = append(allVulns, vulns...)
		}
	}

	if vulnArgs.BinaryOnly || (!vulnArgs.DepsOnly && !vulnArgs.ConfigOnly) {
		vulns, err := scanner.ScanBinaries()
		if err != nil {
			errors = append(errors, "binary: "+err.Error())
		} else {
			allVulns = append(allVulns, vulns...)
		}
	}

	output := map[string]interface{}{
		"scan_time":        time.Now().Format(time.RFC3339),
		"vulnerabilities":  allVulns,
		"total_count":      len(allVulns),
		"summary":          scanner.GetScanSummary(),
	}

	if len(errors) > 0 {
		output["errors"] = errors
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// =============================================================================
// VULN LIST
// =============================================================================

// handleVulnList lists discovered vulnerabilities.
func handleVulnList(vulnArgs VulnArgs) error {
	scanner := security.GlobalVulnScanner()
	var vulns []security.Vulnerability

	// Filter by severity if specified
	if vulnArgs.Severity != "" {
		vulns = scanner.GetVulnBySeverity(vulnArgs.Severity)
	} else {
		vulns = scanner.GetVulnerabilities()
	}

	if vulnArgs.JSON {
		output := map[string]interface{}{
			"vulnerabilities": vulns,
			"count":           len(vulns),
		}
		if vulnArgs.Severity != "" {
			output["filter"] = vulnArgs.Severity
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	// Display header
	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(vulnTitleStyle.Render("Vulnerability List"))
	fmt.Println(vulnSeparatorStyle.Render(separator))
	fmt.Println()

	if vulnArgs.Severity != "" {
		fmt.Printf("Filter: %s\n", vulnValueStyle.Render("Severity = "+vulnArgs.Severity))
		fmt.Println()
	}

	if len(vulns) == 0 {
		fmt.Println(vulnGreenStyle.Render("No vulnerabilities found."))
		fmt.Println()
		return nil
	}

	// Sort by severity
	security.SortBySeverity(vulns)

	// Display each vulnerability
	for _, vuln := range vulns {
		displayVulnerability(vuln, false)
	}

	fmt.Println()
	fmt.Printf("Total: %d vulnerabilities\n", len(vulns))
	fmt.Println()

	return nil
}

// =============================================================================
// VULN SHOW
// =============================================================================

// handleVulnShow shows detailed information about a vulnerability.
func handleVulnShow(vulnArgs VulnArgs) error {
	if vulnArgs.ID == "" {
		return fmt.Errorf("vulnerability ID required\n\nUsage: rigrun vuln show <id>")
	}

	scanner := security.GlobalVulnScanner()
	vulns := scanner.GetVulnerabilities()

	// Find vulnerability by ID
	var found *security.Vulnerability
	for i := range vulns {
		if vulns[i].ID == vulnArgs.ID {
			found = &vulns[i]
			break
		}
	}

	if found == nil {
		return fmt.Errorf("vulnerability not found: %s", vulnArgs.ID)
	}

	if vulnArgs.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(found)
	}

	// Display detailed vulnerability
	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(vulnTitleStyle.Render("Vulnerability Details"))
	fmt.Println(vulnSeparatorStyle.Render(separator))
	fmt.Println()

	displayVulnerability(*found, true)

	fmt.Println()

	return nil
}

// =============================================================================
// VULN EXPORT
// =============================================================================

// handleVulnExport exports vulnerabilities to a file or stdout.
func handleVulnExport(vulnArgs VulnArgs) error {
	scanner := security.GlobalVulnScanner()

	// Determine output destination
	var output *os.File
	if vulnArgs.Output != "" {
		// Validate output path to prevent path traversal attacks
		validatedPath, err := ValidateOutputPath(vulnArgs.Output)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
		file, err := os.Create(validatedPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Export report
	if err := scanner.ExportReport(vulnArgs.Format, output); err != nil {
		return fmt.Errorf("failed to export report: %w", err)
	}

	if vulnArgs.Output != "" && !vulnArgs.JSON {
		fmt.Printf("%s Report exported to: %s\n", vulnSuccessStyle.Render("[OK]"), vulnArgs.Output)
	}

	return nil
}

// =============================================================================
// VULN SCHEDULE
// =============================================================================

// handleVulnSchedule configures scheduled vulnerability scanning.
func handleVulnSchedule(vulnArgs VulnArgs) error {
	if vulnArgs.Interval == "" {
		return fmt.Errorf("interval required\n\nUsage: rigrun vuln schedule <interval>\nExamples: 1h, 24h, 7d")
	}

	// Parse interval
	duration, err := parseVulnScanDuration(vulnArgs.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	scanner := security.GlobalVulnScanner()
	if err := scanner.ScheduleScan(duration); err != nil {
		return fmt.Errorf("failed to schedule scan: %w", err)
	}

	if vulnArgs.JSON {
		output := map[string]interface{}{
			"scheduled": true,
			"interval":  vulnArgs.Interval,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	fmt.Println()
	fmt.Printf("%s Vulnerability scanning scheduled every %s\n",
		vulnSuccessStyle.Render("[OK]"),
		vulnValueStyle.Render(vulnArgs.Interval))
	fmt.Println()

	return nil
}

// =============================================================================
// VULN STATUS
// =============================================================================

// handleVulnStatus shows the current vulnerability scan status.
func handleVulnStatus(vulnArgs VulnArgs) error {
	scanner := security.GlobalVulnScanner()
	lastScan := scanner.GetLastScanTime()
	summary := scanner.GetScanSummary()
	vulns := scanner.GetVulnerabilities()

	if vulnArgs.JSON {
		output := map[string]interface{}{
			"last_scan": lastScan.Format(time.RFC3339),
			"summary":   summary,
			"total":     len(vulns),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	// Display status
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(vulnTitleStyle.Render("Vulnerability Scan Status"))
	fmt.Println(vulnSeparatorStyle.Render(separator))
	fmt.Println()

	// Last scan time
	if lastScan.IsZero() {
		fmt.Printf("%s%s\n", vulnLabelStyle.Render("Last Scan:"), vulnDimStyle.Render("Never"))
	} else {
		fmt.Printf("%s%s\n", vulnLabelStyle.Render("Last Scan:"), vulnValueStyle.Render(lastScan.Format("2006-01-02 15:04:05")))
		fmt.Printf("%s%s\n", vulnLabelStyle.Render("Time Ago:"), vulnDimStyle.Render(formatDuration(time.Since(lastScan))))
	}

	fmt.Println()

	// Summary
	displayScanSummary(summary, len(vulns))

	fmt.Println()

	return nil
}

// =============================================================================
// DISPLAY HELPERS
// =============================================================================

// displayScanSummary displays vulnerability summary by severity.
func displayScanSummary(summary map[string]int, total int) {
	fmt.Println(vulnSectionStyle.Render("Summary"))

	critical := summary[security.SeverityCritical]
	high := summary[security.SeverityHigh]
	medium := summary[security.SeverityMedium]
	low := summary[security.SeverityLow]
	info := summary["info"]

	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Total:"), vulnValueStyle.Render(fmt.Sprintf("%d", total)))
	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Critical:"), severityStyle(security.SeverityCritical, fmt.Sprintf("%d", critical)))
	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("High:"), severityStyle(security.SeverityHigh, fmt.Sprintf("%d", high)))
	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Medium:"), severityStyle(security.SeverityMedium, fmt.Sprintf("%d", medium)))
	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Low:"), severityStyle(security.SeverityLow, fmt.Sprintf("%d", low)))
	fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Info:"), severityStyle("info", fmt.Sprintf("%d", info)))
}

// displayVulnerability displays a single vulnerability.
func displayVulnerability(vuln security.Vulnerability, detailed bool) {
	// Severity badge
	severityBadge := formatSeverityBadge(vuln.Severity)

	// Header line
	fmt.Printf("%s %s  %s\n",
		severityBadge,
		vulnValueStyle.Render(vuln.ID),
		vulnDimStyle.Render(vuln.Component))

	if detailed {
		// Detailed view
		fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Severity:"), severityStyle(vuln.Severity, string(vuln.Severity)))
		fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Component:"), vulnValueStyle.Render(vuln.Component))
		fmt.Printf("  %s%s\n", vulnLabelStyle.Render("Discovered:"), vulnDimStyle.Render(vuln.Timestamp()))

		if vuln.CVE != "" {
			fmt.Printf("  %s%s\n", vulnLabelStyle.Render("CVE:"), vulnValueStyle.Render(vuln.CVE))
		}
		if vuln.CVSS > 0 {
			fmt.Printf("  %s%s\n", vulnLabelStyle.Render("CVSS Score:"), vulnValueStyle.Render(fmt.Sprintf("%.1f", vuln.CVSS)))
		}

		fmt.Println()
		fmt.Println(vulnSectionStyle.Render("Description"))
		fmt.Printf("  %s\n", wrapText(vuln.Description, 68))
		fmt.Println()
		fmt.Println(vulnSectionStyle.Render("Remediation"))
		fmt.Printf("  %s\n", wrapText(vuln.Remediation, 68))

		if len(vuln.References) > 0 {
			fmt.Println()
			fmt.Println(vulnSectionStyle.Render("References"))
			for _, ref := range vuln.References {
				fmt.Printf("  - %s\n", vulnDimStyle.Render(ref))
			}
		}
	} else {
		// Compact view
		fmt.Printf("    %s\n", vuln.Description)
		if vuln.CVE != "" {
			fmt.Printf("    %s %s\n", vulnDimStyle.Render("CVE:"), vuln.CVE)
		}
	}

	fmt.Println()
}

// formatSeverityBadge returns a styled severity badge.
func formatSeverityBadge(severity string) string {
	switch severity {
	case security.SeverityCritical:
		return vulnRedStyle.Render("[CRIT]")
	case security.SeverityHigh:
		return vulnOrangeStyle.Render("[HIGH]")
	case security.SeverityMedium:
		return vulnYellowStyle.Render("[MED ]")
	case security.SeverityLow:
		return vulnGreenStyle.Render("[LOW ]")
	case "info":
		return vulnDimStyle.Render("[INFO]")
	default:
		return "[UNKN]"
	}
}

// severityStyle returns styled text for severity level.
func severityStyle(severity string, text string) string {
	switch severity {
	case security.SeverityCritical:
		return vulnRedStyle.Render(text)
	case security.SeverityHigh:
		return vulnOrangeStyle.Render(text)
	case security.SeverityMedium:
		return vulnYellowStyle.Render(text)
	case security.SeverityLow:
		return vulnGreenStyle.Render(text)
	default:
		return vulnDimStyle.Render(text)
	}
}

// wrapText wraps text to specified width.
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		if len(currentLine)+len(word)+1 > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				lines = append(lines, word)
			}
		} else {
			if currentLine != "" {
				currentLine += " " + word
			} else {
				currentLine = word
			}
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n  ")
}

// parseVulnScanDuration parses duration strings like "1h", "24h", "7d".
func parseVulnScanDuration(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	// Handle day suffix
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}

	// Use standard time.ParseDuration for h, m, s
	return time.ParseDuration(s)
}
