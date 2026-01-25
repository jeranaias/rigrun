// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// incident_cmd.go - CLI commands for NIST 800-53 IR-6 Incident Management.
//
// CLI: Comprehensive help and examples for all commands
//
// Provides incident reporting, tracking, and management capabilities for
// DoD IL5 compliance. Supports incident lifecycle from reporting through
// resolution and export.
//
// Command: incident [subcommand]
// Short:   Incident management (IL5 IR-6)
// Aliases: ir
//
// Subcommands:
//   list (default)      List all incidents
//   report              Report a new incident
//   show <id>           Show incident details
//   update <id>         Update incident status
//   resolve <id>        Mark incident as resolved
//   export              Export incident report
//
// Examples:
//   rigrun incident                       List incidents (default)
//   rigrun incident list                  List all incidents
//   rigrun incident list --status open    Filter by status
//   rigrun incident report                Report new incident
//   rigrun incident show INC-001          Show incident details
//   rigrun incident update INC-001        Update incident
//   rigrun incident resolve INC-001       Mark as resolved
//   rigrun incident export --format json  Export report
//   rigrun incident export --format csv   Export as CSV
//
// Incident Severities:
//   critical            Security breach, data exposure
//   high                Significant security event
//   medium              Moderate security concern
//   low                 Minor security observation
//
// Flags:
//   --status STATUS     Filter by status
//   --severity LEVEL    Filter by severity
//   --format FORMAT     Export format
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// INCIDENT COMMAND STYLES
// =============================================================================

var (
	// Incident title style
	incidentTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Incident section style
	incidentSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	// Incident label style
	incidentLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(16)

	// Incident value style
	incidentValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	// Severity color styles
	severityLowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true) // Green

	severityMediumStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Bold(true) // Yellow

	severityHighStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true) // Orange

	severityCriticalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true) // Red

	// Status color styles
	statusOpenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	statusInvestigatingStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("220")) // Yellow

	statusResolvedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	statusClosedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")) // Gray

	// Incident separator style
	incidentSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// INCIDENT ARGUMENTS
// =============================================================================

// IncidentArgs holds parsed incident command arguments.
type IncidentArgs struct {
	Subcommand  string
	IncidentID  string
	Severity    string
	Category    string
	Description string
	Status      string
	Notes       string
	Resolution  string
	Format      string
	JSON        bool
	Limit       int
}

// parseIncidentArgs parses incident command specific arguments.
func parseIncidentArgs(args *Args) IncidentArgs {
	incidentArgs := IncidentArgs{
		Format: "json",
		Limit:  50,
	}

	if len(args.Raw) > 0 {
		incidentArgs.Subcommand = args.Raw[0]
	}

	remaining := args.Raw[1:]
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--severity", "-s":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Severity = strings.ToLower(remaining[i])
			}
		case "--category", "-c":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Category = strings.ToLower(remaining[i])
			}
		case "--description", "-d":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Description = remaining[i]
			}
		case "--status":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Status = strings.ToLower(remaining[i])
			}
		case "--notes", "-n":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Notes = remaining[i]
			}
		case "--resolution", "-r":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Resolution = remaining[i]
			}
		case "--format", "-f":
			if i+1 < len(remaining) {
				i++
				incidentArgs.Format = strings.ToLower(remaining[i])
			}
		case "--limit", "-l":
			if i+1 < len(remaining) {
				i++
				if n, err := strconv.Atoi(remaining[i]); err == nil && n > 0 {
					incidentArgs.Limit = n
				}
			}
		case "--json":
			incidentArgs.JSON = true
		default:
			// Check for = syntax
			if strings.HasPrefix(arg, "--severity=") {
				incidentArgs.Severity = strings.ToLower(strings.TrimPrefix(arg, "--severity="))
			} else if strings.HasPrefix(arg, "--category=") {
				incidentArgs.Category = strings.ToLower(strings.TrimPrefix(arg, "--category="))
			} else if strings.HasPrefix(arg, "--description=") {
				incidentArgs.Description = strings.TrimPrefix(arg, "--description=")
			} else if strings.HasPrefix(arg, "--status=") {
				incidentArgs.Status = strings.ToLower(strings.TrimPrefix(arg, "--status="))
			} else if strings.HasPrefix(arg, "--notes=") {
				incidentArgs.Notes = strings.TrimPrefix(arg, "--notes=")
			} else if strings.HasPrefix(arg, "--resolution=") {
				incidentArgs.Resolution = strings.TrimPrefix(arg, "--resolution=")
			} else if strings.HasPrefix(arg, "--format=") {
				incidentArgs.Format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
			} else if strings.HasPrefix(arg, "--limit=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit=")); err == nil && n > 0 {
					incidentArgs.Limit = n
				}
			} else if !strings.HasPrefix(arg, "-") {
				// First non-flag argument after subcommand is the incident ID
				if incidentArgs.IncidentID == "" {
					incidentArgs.IncidentID = arg
				}
			}
		}
	}

	// Inherit global JSON flag
	if args.JSON {
		incidentArgs.JSON = true
	}

	return incidentArgs
}

// =============================================================================
// HANDLE INCIDENT
// =============================================================================

// HandleIncident handles the "incident" command with various subcommands.
// Subcommands:
//   - incident report --severity <level> --category <cat> --description "<desc>"
//   - incident list [--status <status>] [--severity <level>]
//   - incident show <id>
//   - incident update <id> --status <status> [--notes "<notes>"]
//   - incident resolve <id> --resolution "<resolution>"
//   - incident close <id>
//   - incident export --format json|csv
func HandleIncident(args Args) error {
	incidentArgs := parseIncidentArgs(&args)

	switch incidentArgs.Subcommand {
	case "", "list":
		return handleIncidentList(incidentArgs)
	case "report":
		return handleIncidentReport(incidentArgs)
	case "show":
		return handleIncidentShow(incidentArgs)
	case "update":
		return handleIncidentUpdate(incidentArgs)
	case "resolve":
		return handleIncidentResolve(incidentArgs)
	case "close":
		return handleIncidentClose(incidentArgs)
	case "export":
		return handleIncidentExport(incidentArgs)
	case "stats":
		return handleIncidentStats(incidentArgs)
	default:
		return fmt.Errorf("unknown incident subcommand: %s\n\nUsage:\n"+
			"  rigrun incident report --severity <level> --category <cat> --description \"<desc>\"\n"+
			"  rigrun incident list [--status <status>] [--severity <level>]\n"+
			"  rigrun incident show <id>\n"+
			"  rigrun incident update <id> --status <status> [--notes \"<notes>\"]\n"+
			"  rigrun incident resolve <id> --resolution \"<resolution>\"\n"+
			"  rigrun incident close <id>\n"+
			"  rigrun incident export --format json|csv\n"+
			"  rigrun incident stats", incidentArgs.Subcommand)
	}
}

// =============================================================================
// INCIDENT REPORT
// =============================================================================

// handleIncidentReport creates a new incident.
func handleIncidentReport(incidentArgs IncidentArgs) error {
	// Validate required fields
	if incidentArgs.Severity == "" {
		return fmt.Errorf("--severity is required (low, medium, high, critical)")
	}
	if incidentArgs.Category == "" {
		return fmt.Errorf("--category is required (security, availability, integrity, spillage)")
	}
	if incidentArgs.Description == "" {
		return fmt.Errorf("--description is required")
	}

	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	incident, err := manager.Report(incidentArgs.Severity, incidentArgs.Category, incidentArgs.Description)
	if err != nil {
		return err
	}

	if incidentArgs.JSON {
		return outputIncidentJSON(incident)
	}

	// Display created incident
	fmt.Println()
	fmt.Println(incidentTitleStyle.Render("Incident Created"))
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("ID:"), incidentValueStyle.Render(incident.ID))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Severity:"), formatSeverity(incident.Severity))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Category:"), incidentValueStyle.Render(incident.Category))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Status:"), formatStatus(incident.Status))
	fmt.Println()
	fmt.Printf("  %s\n", incidentValueStyle.Render(incident.Description))
	fmt.Println()

	return nil
}

// =============================================================================
// INCIDENT LIST
// =============================================================================

// handleIncidentList lists incidents with optional filtering.
func handleIncidentList(incidentArgs IncidentArgs) error {
	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	filter := security.IncidentFilter{
		Status:   incidentArgs.Status,
		Severity: incidentArgs.Severity,
		Category: incidentArgs.Category,
		Limit:    incidentArgs.Limit,
	}

	incidents, err := manager.List(filter)
	if err != nil {
		return err
	}

	if incidentArgs.JSON {
		return outputIncidentsJSON(incidents)
	}

	if len(incidents) == 0 {
		fmt.Println()
		fmt.Println("No incidents found matching the specified criteria.")
		fmt.Println()
		return nil
	}

	// Display incidents
	fmt.Println()
	fmt.Println(incidentTitleStyle.Render("Incidents"))
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("=", 80)))
	fmt.Println()

	// Table header
	fmt.Printf("%-20s %-10s %-12s %-15s %-20s\n",
		"ID", "Severity", "Status", "Category", "Created")
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("-", 80)))

	for _, inc := range incidents {
		fmt.Printf("%-20s %-10s %-12s %-15s %-20s\n",
			inc.ID,
			formatSeverityShort(inc.Severity),
			formatStatusShort(inc.Status),
			inc.Category,
			inc.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	fmt.Println()
	fmt.Printf("Total: %d incident(s)\n", len(incidents))
	fmt.Println()

	return nil
}

// =============================================================================
// INCIDENT SHOW
// =============================================================================

// handleIncidentShow displays details of a specific incident.
func handleIncidentShow(incidentArgs IncidentArgs) error {
	if incidentArgs.IncidentID == "" {
		return fmt.Errorf("incident ID required\nUsage: rigrun incident show <id>")
	}

	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	incident, err := manager.Get(incidentArgs.IncidentID)
	if err != nil {
		return err
	}

	if incidentArgs.JSON {
		return outputIncidentJSON(incident)
	}

	// Display full incident details
	fmt.Println()
	fmt.Println(incidentTitleStyle.Render("Incident Details"))
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("ID:"), incidentValueStyle.Render(incident.ID))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Severity:"), formatSeverity(incident.Severity))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Category:"), incidentValueStyle.Render(incident.Category))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Status:"), formatStatus(incident.Status))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Reported By:"), incidentValueStyle.Render(incident.ReportedBy))
	fmt.Printf("  %s%s\n", incidentLabelStyle.Render("Created:"), incidentValueStyle.Render(incident.CreatedAt.Format(time.RFC1123)))
	fmt.Println()

	fmt.Println(incidentSectionStyle.Render("Description:"))
	fmt.Printf("  %s\n", incident.Description)
	fmt.Println()

	if incident.Resolution != "" {
		fmt.Println(incidentSectionStyle.Render("Resolution:"))
		fmt.Printf("  %s\n", incident.Resolution)
		if incident.ResolvedAt != nil {
			fmt.Printf("  Resolved by %s at %s\n", incident.ResolvedBy, incident.ResolvedAt.Format(time.RFC1123))
		}
		fmt.Println()
	}

	// Audit trail
	if len(incident.AuditTrail) > 0 {
		fmt.Println(incidentSectionStyle.Render("Audit Trail:"))
		for _, event := range incident.AuditTrail {
			fmt.Printf("  %s  %-15s  %s\n",
				event.Timestamp.Format("2006-01-02 15:04"),
				event.Action,
				event.Details,
			)
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// INCIDENT UPDATE
// =============================================================================

// handleIncidentUpdate updates an incident's status.
func handleIncidentUpdate(incidentArgs IncidentArgs) error {
	if incidentArgs.IncidentID == "" {
		return fmt.Errorf("incident ID required\nUsage: rigrun incident update <id> --status <status>")
	}
	if incidentArgs.Status == "" {
		return fmt.Errorf("--status is required (open, investigating, resolved, closed)")
	}

	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	if err := manager.UpdateStatus(incidentArgs.IncidentID, incidentArgs.Status, incidentArgs.Notes); err != nil {
		return err
	}

	if incidentArgs.JSON {
		output := map[string]interface{}{
			"success":     true,
			"incident_id": incidentArgs.IncidentID,
			"new_status":  incidentArgs.Status,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("Incident %s updated to status: %s\n", incidentArgs.IncidentID, formatStatus(incidentArgs.Status))
	fmt.Println()

	return nil
}

// =============================================================================
// INCIDENT RESOLVE
// =============================================================================

// handleIncidentResolve resolves an incident.
func handleIncidentResolve(incidentArgs IncidentArgs) error {
	if incidentArgs.IncidentID == "" {
		return fmt.Errorf("incident ID required\nUsage: rigrun incident resolve <id> --resolution \"<resolution>\"")
	}
	if incidentArgs.Resolution == "" {
		return fmt.Errorf("--resolution is required")
	}

	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	if err := manager.Resolve(incidentArgs.IncidentID, incidentArgs.Resolution); err != nil {
		return err
	}

	if incidentArgs.JSON {
		output := map[string]interface{}{
			"success":     true,
			"incident_id": incidentArgs.IncidentID,
			"status":      "resolved",
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("Incident %s resolved.\n", incidentArgs.IncidentID)
	fmt.Println()

	return nil
}

// =============================================================================
// INCIDENT CLOSE
// =============================================================================

// handleIncidentClose closes a resolved incident.
func handleIncidentClose(incidentArgs IncidentArgs) error {
	if incidentArgs.IncidentID == "" {
		return fmt.Errorf("incident ID required\nUsage: rigrun incident close <id>")
	}

	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	if err := manager.Close(incidentArgs.IncidentID); err != nil {
		return err
	}

	if incidentArgs.JSON {
		output := map[string]interface{}{
			"success":     true,
			"incident_id": incidentArgs.IncidentID,
			"status":      "closed",
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("Incident %s closed.\n", incidentArgs.IncidentID)
	fmt.Println()

	return nil
}

// =============================================================================
// INCIDENT EXPORT
// =============================================================================

// handleIncidentExport exports incidents in the specified format.
func handleIncidentExport(incidentArgs IncidentArgs) error {
	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	data, err := manager.Export(incidentArgs.Format)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// =============================================================================
// INCIDENT STATS
// =============================================================================

// handleIncidentStats displays incident statistics.
func handleIncidentStats(incidentArgs IncidentArgs) error {
	manager := security.GlobalIncidentManager()
	if manager == nil {
		return fmt.Errorf("incident manager not available")
	}

	counts := manager.Count()

	if incidentArgs.JSON {
		data, _ := json.MarshalIndent(counts, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(incidentTitleStyle.Render("Incident Statistics"))
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("=", 40)))
	fmt.Println()

	total := 0
	for _, count := range counts {
		total += count
	}

	fmt.Printf("  %s%d\n", incidentLabelStyle.Render("Open:"), counts[security.StatusOpen])
	fmt.Printf("  %s%d\n", incidentLabelStyle.Render("Investigating:"), counts[security.StatusInvestigating])
	fmt.Printf("  %s%d\n", incidentLabelStyle.Render("Resolved:"), counts[security.StatusResolved])
	fmt.Printf("  %s%d\n", incidentLabelStyle.Render("Closed:"), counts[security.StatusClosed])
	fmt.Println(incidentSeparatorStyle.Render(strings.Repeat("-", 30)))
	fmt.Printf("  %s%d\n", incidentLabelStyle.Render("Total:"), total)
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// outputIncidentJSON outputs a single incident as JSON.
func outputIncidentJSON(incident *security.Incident) error {
	data, err := json.MarshalIndent(incident, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// outputIncidentsJSON outputs a list of incidents as JSON.
func outputIncidentsJSON(incidents []security.Incident) error {
	output := map[string]interface{}{
		"count":     len(incidents),
		"incidents": incidents,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// formatSeverity formats severity with color.
func formatSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "low":
		return severityLowStyle.Render("LOW")
	case "medium":
		return severityMediumStyle.Render("MEDIUM")
	case "high":
		return severityHighStyle.Render("HIGH")
	case "critical":
		return severityCriticalStyle.Render("CRITICAL")
	default:
		return incidentValueStyle.Render(severity)
	}
}

// formatSeverityShort formats severity for table display.
func formatSeverityShort(severity string) string {
	return formatSeverity(severity)
}

// formatStatus formats status with color.
func formatStatus(status string) string {
	switch strings.ToLower(status) {
	case "open":
		return statusOpenStyle.Render("OPEN")
	case "investigating":
		return statusInvestigatingStyle.Render("INVESTIGATING")
	case "resolved":
		return statusResolvedStyle.Render("RESOLVED")
	case "closed":
		return statusClosedStyle.Render("CLOSED")
	default:
		return incidentValueStyle.Render(status)
	}
}

// formatStatusShort formats status for table display.
func formatStatusShort(status string) string {
	return formatStatus(status)
}


// Prevent unused import error for os package
var _ = os.Stderr
