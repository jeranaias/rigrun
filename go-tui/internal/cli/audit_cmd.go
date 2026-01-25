// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// audit_cmd.go - Audit log management CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements AU-9 (Audit Protection), AU-11 (Audit Record Retention),
// and AU-6 (Audit Review) controls for DoD IL5 compliance.
//
// Command: audit [subcommand]
// Short:   Manage audit logs (IL5 AU-5, AU-6, AU-9, AU-11)
// Aliases: (none)
//
// AU-5 Subcommands (Audit Failure Response):
//   show (default)      Display recent audit log entries
//   export              Export logs for SIEM integration
//   clear               Clear audit logs (requires --confirm)
//   stats               Show audit log statistics
//   capacity            Show storage capacity (AU-5)
//   verify              Verify audit log integrity
//   alert               Show alert configuration
//
// AU-6 Subcommands (Audit Review, Analysis, Reporting):
//   analyze             Analyze logs for anomalies
//   report              Generate compliance report
//   alerts              Show triggered alerts
//   search <query>      Search audit logs
//   export-siem         Export for SIEM (multiple formats)
//
// AU-9 Subcommands (Protection of Audit Information):
//   verify-integrity    Verify integrity chain
//   protect             Apply protection to logs
//   archive [days]      Archive logs older than N days
//
// Examples:
//   rigrun audit                            Show recent entries (default)
//   rigrun audit show                       Show recent entries
//   rigrun audit show --lines 100           Show last 100 entries
//   rigrun audit show --since 24h           Entries from last 24 hours
//   rigrun audit show --since 7d            Entries from last 7 days
//   rigrun audit show --type QUERY          Filter by event type
//   rigrun audit export --format json       Export as JSON
//   rigrun audit export --format csv        Export as CSV
//   rigrun audit export --format syslog     Export as syslog (RFC 5424)
//   rigrun audit export --output audit.json Export to file
//   rigrun audit clear --confirm            Clear logs (requires confirm)
//   rigrun audit stats                      Show statistics
//   rigrun audit stats --json               Stats in JSON format
//   rigrun audit capacity                   Check storage capacity
//   rigrun audit verify                     Verify log integrity
//   rigrun audit analyze                    Run anomaly analysis
//   rigrun audit analyze --since 48h        Analyze last 48 hours
//   rigrun audit report                     Generate compliance report
//   rigrun audit report --format csv        Report as CSV
//   rigrun audit search "ERROR"             Search for ERROR entries
//   rigrun audit protect                    Apply log protection
//   rigrun audit archive 90                 Archive logs older than 90 days
//
// Flags:
//   --lines N, -n N     Number of entries to show (default: 50)
//   --since DATE, -s    Filter by date (YYYY-MM-DD or relative: 1h, 24h, 7d)
//   --type TYPE, -t     Filter by event type
//   --format FORMAT     Export format: json, csv, syslog, cef
//   --output FILE, -o   Export to file (default: stdout)
//   --confirm, -y       Confirm destructive operations
//   --json              Output in JSON format
//
// Event Types:
//   QUERY, SESSION_START, SESSION_END, SESSION_TIMEOUT,
//   STARTUP, SHUTDOWN, BANNER_ACK, ERROR
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// AUDIT COMMAND STYLES
// =============================================================================

var (
	// Audit title style
	auditTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	// Audit section style
	auditSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	// Audit label style
	auditLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(16)

	// Audit value styles
	auditValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	auditGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	auditYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	auditRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	auditDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	// Audit success style
	auditSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	// Audit error style
	auditErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Separator style
	auditSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// Event type color map
	eventTypeColors = map[string]lipgloss.Style{
		"QUERY":           lipgloss.NewStyle().Foreground(lipgloss.Color("39")),  // Cyan
		"SESSION_START":   lipgloss.NewStyle().Foreground(lipgloss.Color("82")),  // Green
		"SESSION_END":     lipgloss.NewStyle().Foreground(lipgloss.Color("208")), // Orange
		"SESSION_TIMEOUT": lipgloss.NewStyle().Foreground(lipgloss.Color("196")), // Red
		"STARTUP":         lipgloss.NewStyle().Foreground(lipgloss.Color("82")),  // Green
		"SHUTDOWN":        lipgloss.NewStyle().Foreground(lipgloss.Color("208")), // Orange
		"BANNER_ACK":      lipgloss.NewStyle().Foreground(lipgloss.Color("220")), // Yellow
		"ERROR":           lipgloss.NewStyle().Foreground(lipgloss.Color("196")), // Red
	}

	// PERFORMANCE: Pre-compiled regex (compiled once at startup)
	// Relative time pattern: matches "1h", "24h", "7d", "30m", etc.
	relativeTimeRegex = regexp.MustCompile(`^(\d+)([hdms])$`)
)

// =============================================================================
// AUDIT ARGUMENTS
// =============================================================================

// AuditArgs holds parsed audit command arguments.
type AuditArgs struct {
	Subcommand string
	Lines      int
	Since      string
	EventType  string
	Format     string
	Output     string
	Confirm    bool
	JSON       bool
	Raw        []string
}

// parseAuditArgs parses audit command specific arguments from raw args.
func parseAuditArgs(args *Args, remaining []string) AuditArgs {
	auditArgs := AuditArgs{
		Lines:  50, // Default
		Format: "json",
		Raw:    remaining,
	}

	if len(remaining) > 0 {
		auditArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--lines", "-n":
			if i+1 < len(remaining) {
				i++
				if n, err := strconv.Atoi(remaining[i]); err == nil && n > 0 {
					auditArgs.Lines = n
				}
			}
		case "--since", "-s":
			if i+1 < len(remaining) {
				i++
				auditArgs.Since = remaining[i]
			}
		case "--type", "-t":
			if i+1 < len(remaining) {
				i++
				auditArgs.EventType = strings.ToUpper(remaining[i])
			}
		case "--format", "-f":
			if i+1 < len(remaining) {
				i++
				auditArgs.Format = strings.ToLower(remaining[i])
			}
		case "--output", "-o":
			if i+1 < len(remaining) {
				i++
				auditArgs.Output = remaining[i]
			}
		case "--confirm", "-y":
			auditArgs.Confirm = true
		case "--json":
			auditArgs.JSON = true
		default:
			// Check for --lines=N format
			if strings.HasPrefix(arg, "--lines=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--lines=")); err == nil && n > 0 {
					auditArgs.Lines = n
				}
			} else if strings.HasPrefix(arg, "--since=") {
				auditArgs.Since = strings.TrimPrefix(arg, "--since=")
			} else if strings.HasPrefix(arg, "--type=") {
				auditArgs.EventType = strings.ToUpper(strings.TrimPrefix(arg, "--type="))
			} else if strings.HasPrefix(arg, "--format=") {
				auditArgs.Format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
			} else if strings.HasPrefix(arg, "--output=") {
				auditArgs.Output = strings.TrimPrefix(arg, "--output=")
			}
		}
	}

	return auditArgs
}

// =============================================================================
// HANDLE AUDIT
// =============================================================================

// HandleAudit handles the "audit" command with various subcommands.
// Subcommands:
//   - audit or audit show: Display recent audit log entries
//   - audit show --lines N: Show last N entries
//   - audit show --since "2024-01-01": Filter by date
//   - audit show --type QUERY|SESSION_START|etc: Filter by event type
//   - audit export --format json|csv|syslog: Export for SIEM integration
//   - audit export --output /path/to/file: Export to file
//   - audit clear --confirm: Clear audit logs (requires confirmation)
//   - audit stats: Show audit log statistics
//   - audit capacity: Show storage capacity (AU-5)
//   - audit verify: Verify audit log integrity (AU-5/SI-7)
//   - audit alert: Show/configure alert thresholds (AU-5)
//   - audit analyze: Analyze logs for anomalies (AU-6)
//   - audit report: Generate compliance report (AU-6)
//   - audit alerts: Show triggered alerts (AU-6)
//   - audit search: Search audit logs (AU-6)
//   - audit export-siem: Export for SIEM (AU-6)
//   - audit verify-integrity: Verify integrity chain (AU-9)
//   - audit protect: Apply protection to logs (AU-9)
//   - audit archive: Archive old logs (AU-9)
func HandleAudit(args Args) error {
	auditArgs := parseAuditArgs(&args, args.Raw)

	switch auditArgs.Subcommand {
	case "", "show":
		return handleAuditShow(auditArgs)
	case "export":
		return handleAuditExport(auditArgs)
	case "clear":
		return handleAuditClear(auditArgs)
	case "stats":
		return handleAuditStats(auditArgs)
	// AU-5: Audit Failure Response controls
	case "capacity":
		return handleAuditCapacity(auditArgs)
	case "verify":
		return handleAuditVerify(auditArgs)
	case "alert":
		return handleAuditAlert(auditArgs)
	// AU-6: Audit Review, Analysis, and Reporting
	case "analyze":
		return handleAuditAnalyze(auditArgs)
	case "report":
		return handleAuditReport(auditArgs)
	case "alerts":
		return handleAuditAlerts(auditArgs)
	case "search":
		return handleAuditSearch(auditArgs)
	case "export-siem":
		return handleAuditExportSIEM(auditArgs)
	// AU-9: Protection of Audit Information
	case "verify-integrity", "integrity":
		return handleAuditVerifyIntegrity(auditArgs)
	case "protect":
		return handleAuditProtect(auditArgs)
	case "archive":
		return handleAuditArchive(auditArgs)
	default:
		return fmt.Errorf("unknown audit subcommand: %s\n\nUsage:\n"+
			"  AU-5 Commands (Audit Failure Response):\n"+
			"    rigrun audit show [--lines N] [--since DATE] [--type TYPE]\n"+
			"    rigrun audit export [--format json|csv|syslog] [--output FILE]\n"+
			"    rigrun audit clear --confirm\n"+
			"    rigrun audit stats\n"+
			"    rigrun audit capacity    Show storage capacity\n"+
			"    rigrun audit verify      Verify audit log integrity\n"+
			"    rigrun audit alert       Show alert configuration\n\n"+
			"  AU-6 Commands (Audit Review, Analysis, and Reporting):\n"+
			"    rigrun audit analyze [--since TIMERANGE]    Analyze logs for anomalies\n"+
			"    rigrun audit report [--format text|json|csv] [--output FILE]\n"+
			"    rigrun audit alerts                         Show triggered alerts\n"+
			"    rigrun audit search <query>                 Search audit logs\n"+
			"    rigrun audit export-siem [--format json|csv|syslog|cef]\n\n"+
			"  AU-9 Commands (Protection of Audit Information):\n"+
			"    rigrun audit verify-integrity               Verify integrity chain\n"+
			"    rigrun audit protect                        Apply protection to logs\n"+
			"    rigrun audit archive [days]                 Archive logs older than N days", auditArgs.Subcommand)
	}
}

// =============================================================================
// AUDIT LOG READING
// =============================================================================

// AuditEntry represents a parsed audit log entry.
type AuditEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	EventType string            `json:"event_type"`
	SessionID string            `json:"session_id"`
	Tier      string            `json:"tier,omitempty"`
	Query     string            `json:"query,omitempty"`
	Tokens    int               `json:"tokens,omitempty"`
	Cost      float64           `json:"cost_cents,omitempty"`
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"`
	RawLine   string            `json:"-"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// getAuditLogPath returns the audit log path from config or default.
func getAuditLogPath() string {
	cfg := config.Global()

	// Check for custom audit log path in config
	path := cfg.Security.AuditLogPath
	if path == "" {
		// Use the default path from the security package
		path = security.DefaultAuditPath()
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ""
	}

	// AU-9: Verify permissions before reading
	info, err := os.Stat(path)
	if err == nil {
		mode := info.Mode()
		// Warn if permissions are too open (not 0600 or 0700)
		if mode.Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "%s Audit log permissions are too open (%o). Consider running: chmod 600 %s\n",
				auditYellowStyle.Render("[WARN]"), mode.Perm(), path)
		}
	}

	return path
}

// parseAuditLine parses a single audit log line into an AuditEntry.
// Format: timestamp | event_type | session_id | tier | query | tokens | cost | status
func parseAuditLine(line string) (*AuditEntry, error) {
	// Skip empty lines
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	parts := strings.Split(line, " | ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid audit log format")
	}

	entry := &AuditEntry{
		RawLine: line,
	}

	// Parse timestamp (format: 2006-01-02 15:04:05)
	if len(parts) > 0 {
		t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %w", err)
		}
		entry.Timestamp = t
	}

	// Parse event type
	if len(parts) > 1 {
		entry.EventType = strings.TrimSpace(parts[1])
	}

	// Parse session ID
	if len(parts) > 2 {
		entry.SessionID = strings.TrimSpace(parts[2])
	}

	// Parse tier (optional)
	if len(parts) > 3 {
		entry.Tier = strings.TrimSpace(parts[3])
	}

	// Parse query (optional, may be quoted)
	if len(parts) > 4 {
		query := strings.TrimSpace(parts[4])
		// Remove surrounding quotes if present
		if len(query) >= 2 && query[0] == '"' && query[len(query)-1] == '"' {
			query = query[1 : len(query)-1]
		}
		entry.Query = query
	}

	// Parse tokens (optional)
	if len(parts) > 5 {
		tokens := strings.TrimSpace(parts[5])
		if tokens != "" {
			if n, err := strconv.Atoi(tokens); err == nil {
				entry.Tokens = n
			}
		}
	}

	// Parse cost (optional)
	if len(parts) > 6 {
		cost := strings.TrimSpace(parts[6])
		if cost != "" {
			if f, err := strconv.ParseFloat(cost, 64); err == nil {
				entry.Cost = f
			}
		}
	}

	// Parse status (optional)
	if len(parts) > 7 {
		status := strings.TrimSpace(parts[7])
		if strings.HasPrefix(status, "ERROR:") {
			entry.Status = "ERROR"
			entry.Error = strings.TrimPrefix(status, "ERROR:")
			entry.Error = strings.TrimSpace(entry.Error)
		} else {
			entry.Status = status
		}
	} else {
		entry.Status = "SUCCESS"
	}

	return entry, nil
}

// readAuditEntries reads audit entries from the log file with optional filtering.
func readAuditEntries(path string, limit int, since time.Time, eventType string) ([]AuditEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	// Read all entries first (we'll optimize later with tail-like reading)
	var allEntries []AuditEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		entry, err := parseAuditLine(line)
		if err != nil || entry == nil {
			continue
		}

		// Apply filters
		if !since.IsZero() && entry.Timestamp.Before(since) {
			continue
		}

		if eventType != "" && entry.EventType != eventType {
			continue
		}

		allEntries = append(allEntries, *entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading audit log: %w", err)
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(allEntries) > limit {
		allEntries = allEntries[:limit]
	}

	return allEntries, nil
}

// =============================================================================
// AUDIT SHOW
// =============================================================================

// handleAuditShow displays recent audit log entries.
func handleAuditShow(auditArgs AuditArgs) error {
	path := getAuditLogPath()
	if path == "" {
		if auditArgs.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "no audit log found",
				"message": "Audit logging may not be enabled. Enable with: rigrun config set audit_enabled true",
			})
		}
		fmt.Println()
		fmt.Println(auditYellowStyle.Render("No audit log found."))
		fmt.Println("Audit logging may not be enabled. Enable with:")
		fmt.Println("  rigrun config set audit_enabled true")
		fmt.Println()
		return nil
	}

	// Parse since filter
	var since time.Time
	if auditArgs.Since != "" {
		// Try various date formats
		formats := []string{
			"2006-01-02",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"01/02/2006",
			time.RFC3339,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, auditArgs.Since); err == nil {
				since = t
				break
			}
		}
		if since.IsZero() {
			// Try relative time (e.g., "1h", "24h", "7d")
			if d, err := parseRelativeTime(auditArgs.Since); err == nil {
				since = time.Now().Add(-d)
			} else {
				return fmt.Errorf("invalid date format: %s\nSupported formats: YYYY-MM-DD, YYYY-MM-DD HH:MM:SS, or relative (1h, 24h, 7d)", auditArgs.Since)
			}
		}
	}

	// Read entries
	entries, err := readAuditEntries(path, auditArgs.Lines, since, auditArgs.EventType)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if auditArgs.JSON {
			return outputJSON(map[string]interface{}{
				"entries": []interface{}{},
				"message": "No audit entries found matching the specified criteria",
			})
		}
		fmt.Println()
		fmt.Println(auditDimStyle.Render("No audit entries found matching the specified criteria."))
		fmt.Println()
		return nil
	}

	// Output in JSON format if requested
	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"entries": entries,
			"count":   len(entries),
			"path":    path,
		})
	}

	// Display entries
	separator := strings.Repeat("=", 80)
	fmt.Println()
	fmt.Println(auditTitleStyle.Render("Audit Log"))
	fmt.Println(auditSeparatorStyle.Render(separator))
	fmt.Println()

	// Show filter info
	filterInfo := []string{}
	if auditArgs.Lines != 50 {
		filterInfo = append(filterInfo, fmt.Sprintf("last %d entries", auditArgs.Lines))
	}
	if auditArgs.EventType != "" {
		filterInfo = append(filterInfo, fmt.Sprintf("type=%s", auditArgs.EventType))
	}
	if !since.IsZero() {
		filterInfo = append(filterInfo, fmt.Sprintf("since=%s", since.Format("2006-01-02 15:04:05")))
	}
	if len(filterInfo) > 0 {
		fmt.Printf("Filters: %s\n", auditDimStyle.Render(strings.Join(filterInfo, ", ")))
		fmt.Println()
	}

	// Reverse entries to show oldest first (chronological order)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		formatAuditEntry(entry)
	}

	fmt.Println()
	fmt.Printf("Showing %d entries from: %s\n", len(entries), auditDimStyle.Render(path))
	fmt.Println()

	return nil
}

// formatAuditEntry formats and prints a single audit entry.
func formatAuditEntry(entry AuditEntry) {
	// Get color for event type
	typeStyle := auditValueStyle
	if style, ok := eventTypeColors[entry.EventType]; ok {
		typeStyle = style
	}

	// Format timestamp
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	// Build the entry line
	fmt.Printf("%s  %s  %s",
		auditDimStyle.Render(timestamp),
		typeStyle.Render(fmt.Sprintf("%-16s", entry.EventType)),
		auditDimStyle.Render(entry.SessionID[:8]+"..."))

	// Add tier if present
	if entry.Tier != "" {
		fmt.Printf("  [%s]", entry.Tier)
	}

	// Add status
	if entry.Status == "SUCCESS" {
		fmt.Printf("  %s", auditGreenStyle.Render("OK"))
	} else if entry.Status == "ERROR" {
		fmt.Printf("  %s", auditRedStyle.Render("ERR"))
	} else if entry.Status == "FAILURE" {
		fmt.Printf("  %s", auditYellowStyle.Render("FAIL"))
	}

	// Add query preview if present
	// UNICODE: Rune-aware truncation preserves multi-byte characters
	if entry.Query != "" {
		query := util.TruncateRunes(entry.Query, 50)
		fmt.Printf("  \"%s\"", auditDimStyle.Render(query))
	}

	fmt.Println()

	// Show error details if present
	if entry.Error != "" {
		fmt.Printf("           %s %s\n", auditRedStyle.Render("Error:"), entry.Error)
	}
}

// parseRelativeTime parses relative time strings like "1h", "24h", "7d".
// Uses pre-compiled relativeTimeRegex.
func parseRelativeTime(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	// Match patterns like "1h", "24h", "7d", "30m" using pre-compiled regex
	matches := relativeTimeRegex.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid relative time format")
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "s":
		return time.Duration(value) * time.Second, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}
}

// =============================================================================
// AUDIT EXPORT
// =============================================================================

// handleAuditExport exports audit logs in various formats for SIEM integration.
func handleAuditExport(auditArgs AuditArgs) error {
	path := getAuditLogPath()
	if path == "" {
		return fmt.Errorf("no audit log found. Enable audit logging with: rigrun config set audit_enabled true")
	}

	// Read all entries (no limit for export)
	entries, err := readAuditEntries(path, 0, time.Time{}, "")
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("no audit entries to export")
	}

	// Determine output destination
	var output io.Writer = os.Stdout
	if auditArgs.Output != "" {
		// Validate output path to prevent path traversal attacks
		validatedPath, err := ValidateOutputPath(auditArgs.Output)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}

		// Ensure directory exists
		dir := filepath.Dir(validatedPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		// AU-9: Create file with secure permissions
		file, err := os.OpenFile(validatedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
		// Update auditArgs.Output to use validated path for later display
		auditArgs.Output = validatedPath
	}

	// Export in the requested format
	var exportErr error
	switch auditArgs.Format {
	case "json":
		exportErr = exportJSON(entries, output)
	case "csv":
		exportErr = exportCSV(entries, output)
	case "syslog":
		exportErr = exportSyslog(entries, output)
	default:
		return fmt.Errorf("unsupported export format: %s\nSupported formats: json, csv, syslog", auditArgs.Format)
	}

	if exportErr != nil {
		return exportErr
	}

	if auditArgs.Output != "" {
		fmt.Printf("%s Exported %d audit entries to: %s\n",
			auditSuccessStyle.Render("[OK]"),
			len(entries),
			auditArgs.Output)
	}

	return nil
}

// exportJSON exports entries as JSON.
func exportJSON(entries []AuditEntry, output io.Writer) error {
	// Reverse to chronological order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	data := map[string]interface{}{
		"export_time": time.Now().Format(time.RFC3339),
		"entry_count": len(entries),
		"format":      "rigrun-audit-v1",
		"entries":     entries,
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// exportCSV exports entries as CSV.
func exportCSV(entries []AuditEntry, output io.Writer) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{"timestamp", "event_type", "session_id", "tier", "query", "tokens", "cost_cents", "status", "error"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write entries in chronological order
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.EventType,
			entry.SessionID,
			entry.Tier,
			entry.Query,
			strconv.Itoa(entry.Tokens),
			fmt.Sprintf("%.2f", entry.Cost),
			entry.Status,
			entry.Error,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// exportSyslog exports entries in syslog-compatible format (RFC 5424).
func exportSyslog(entries []AuditEntry, output io.Writer) error {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	// Write entries in chronological order
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		// RFC 5424 format:
		// <priority>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
		// Priority: facility (local0=16) * 8 + severity (info=6) = 134
		priority := 134

		// Format structured data for SIEM
		structuredData := fmt.Sprintf("[rigrun@0 event_type=\"%s\" session_id=\"%s\" tier=\"%s\" status=\"%s\" tokens=\"%d\" cost=\"%.2f\"]",
			entry.EventType,
			entry.SessionID,
			entry.Tier,
			entry.Status,
			entry.Tokens,
			entry.Cost)

		// Build message
		msg := entry.Query
		if entry.Error != "" {
			msg = fmt.Sprintf("ERROR: %s", entry.Error)
		}
		if msg == "" {
			msg = entry.EventType
		}

		// Write syslog line
		syslogLine := fmt.Sprintf("<%d>1 %s %s rigrun - %s %s %s\n",
			priority,
			entry.Timestamp.Format(time.RFC3339),
			hostname,
			entry.EventType,
			structuredData,
			msg)

		if _, err := output.Write([]byte(syslogLine)); err != nil {
			return err
		}
	}

	return nil
}

// =============================================================================
// AUDIT CLEAR
// =============================================================================

// handleAuditClear clears the audit log with confirmation.
// AU-11: Audit Record Retention - requires explicit confirmation
func handleAuditClear(auditArgs AuditArgs) error {
	path := getAuditLogPath()
	if path == "" {
		fmt.Println()
		fmt.Println(auditDimStyle.Render("No audit log found to clear."))
		fmt.Println()
		return nil
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat audit log: %w", err)
	}

	// Read entry count
	entries, _ := readAuditEntries(path, 0, time.Time{}, "")
	entryCount := len(entries)

	// AU-11: Require explicit confirmation
	if !auditArgs.Confirm {
		fmt.Println()
		fmt.Println(auditYellowStyle.Render("WARNING: Audit Log Deletion"))
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println()
		fmt.Printf("  Path:    %s\n", path)
		fmt.Printf("  Entries: %d\n", entryCount)
		fmt.Printf("  Size:    %s\n", formatBytes(info.Size()))
		fmt.Println()
		fmt.Println(auditRedStyle.Render("This action cannot be undone."))
		fmt.Println("Per AU-11 (Audit Record Retention), audit logs should be preserved.")
		fmt.Println()
		fmt.Println("Consider exporting before clearing:")
		fmt.Println("  rigrun audit export --format json --output audit_backup.json")
		fmt.Println()
		fmt.Println("To proceed, run:")
		fmt.Println("  rigrun audit clear --confirm")
		fmt.Println()
		return nil
	}

	// Double-check with interactive prompt if not in JSON mode
	if !auditArgs.JSON {
		fmt.Printf("Are you sure you want to delete %d audit entries? [y/N]: ", entryCount)
		input := promptInput("")
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Log the clear action before clearing (for audit trail)
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		logger.LogEvent("CLI", "AUDIT_CLEAR", map[string]string{
			"entries_cleared": strconv.Itoa(entryCount),
			"file_size":       strconv.FormatInt(info.Size(), 10),
		})
		logger.Sync()
	}

	// Close the global logger before clearing
	if logger != nil {
		logger.Close()
	}

	// Clear the log file (truncate to zero)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to clear audit log: %w", err)
	}
	file.Close()

	// Reinitialize the logger
	security.InitGlobalAuditLogger("", true)

	fmt.Println()
	fmt.Printf("%s Cleared %d audit entries\n", auditSuccessStyle.Render("[OK]"), entryCount)
	fmt.Println()

	return nil
}

// =============================================================================
// AUDIT STATS
// =============================================================================

// AuditStats holds audit log statistics.
type AuditStats struct {
	TotalEntries    int            `json:"total_entries"`
	FileSize        int64          `json:"file_size_bytes"`
	FileSizeHuman   string         `json:"file_size_human"`
	OldestEntry     time.Time      `json:"oldest_entry,omitempty"`
	NewestEntry     time.Time      `json:"newest_entry,omitempty"`
	EventTypeCounts map[string]int `json:"event_type_counts"`
	SuccessCount    int            `json:"success_count"`
	ErrorCount      int            `json:"error_count"`
	TotalTokens     int            `json:"total_tokens"`
	TotalCost       float64        `json:"total_cost_cents"`
	SessionCount    int            `json:"unique_sessions"`
	Path            string         `json:"path"`
}

// handleAuditStats displays audit log statistics.
func handleAuditStats(auditArgs AuditArgs) error {
	path := getAuditLogPath()
	if path == "" {
		if auditArgs.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "no audit log found",
				"message": "Audit logging may not be enabled",
			})
		}
		fmt.Println()
		fmt.Println(auditYellowStyle.Render("No audit log found."))
		fmt.Println("Audit logging may not be enabled. Enable with:")
		fmt.Println("  rigrun config set audit_enabled true")
		fmt.Println()
		return nil
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat audit log: %w", err)
	}

	// Read all entries
	entries, err := readAuditEntries(path, 0, time.Time{}, "")
	if err != nil {
		return err
	}

	// Calculate statistics
	stats := AuditStats{
		TotalEntries:    len(entries),
		FileSize:        info.Size(),
		FileSizeHuman:   formatBytes(info.Size()),
		EventTypeCounts: make(map[string]int),
		Path:            path,
	}

	sessionSet := make(map[string]bool)

	for _, entry := range entries {
		// Track event types
		stats.EventTypeCounts[entry.EventType]++

		// Track success/error
		if entry.Status == "SUCCESS" {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}

		// Track tokens and cost
		stats.TotalTokens += entry.Tokens
		stats.TotalCost += entry.Cost

		// Track unique sessions
		sessionSet[entry.SessionID] = true

		// Track oldest/newest
		if stats.OldestEntry.IsZero() || entry.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.Timestamp
		}
		if stats.NewestEntry.IsZero() || entry.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = entry.Timestamp
		}
	}

	stats.SessionCount = len(sessionSet)

	// Output JSON if requested
	if auditArgs.JSON {
		return outputJSON(stats)
	}

	// Display statistics
	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(auditTitleStyle.Render("Audit Log Statistics"))
	fmt.Println(auditSeparatorStyle.Render(separator))
	fmt.Println()

	// File info
	fmt.Println(auditSectionStyle.Render("File Information"))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Path:"), auditDimStyle.Render(path))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Size:"), auditValueStyle.Render(stats.FileSizeHuman))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Entries:"), auditValueStyle.Render(strconv.Itoa(stats.TotalEntries)))
	fmt.Println()

	// Time range
	if !stats.OldestEntry.IsZero() {
		fmt.Println(auditSectionStyle.Render("Time Range"))
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Oldest:"), auditValueStyle.Render(stats.OldestEntry.Format("2006-01-02 15:04:05")))
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Newest:"), auditValueStyle.Render(stats.NewestEntry.Format("2006-01-02 15:04:05")))
		duration := stats.NewestEntry.Sub(stats.OldestEntry)
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Duration:"), auditValueStyle.Render(formatDuration(duration)))
		fmt.Println()
	}

	// Event type breakdown
	fmt.Println(auditSectionStyle.Render("Event Types"))
	// Sort event types for consistent output
	var eventTypes []string
	for et := range stats.EventTypeCounts {
		eventTypes = append(eventTypes, et)
	}
	sort.Strings(eventTypes)

	for _, et := range eventTypes {
		count := stats.EventTypeCounts[et]
		pct := float64(count) / float64(stats.TotalEntries) * 100
		typeStyle := auditValueStyle
		if style, ok := eventTypeColors[et]; ok {
			typeStyle = style
		}
		fmt.Printf("  %s%s  %s\n",
			auditLabelStyle.Render(et+":"),
			typeStyle.Render(fmt.Sprintf("%d", count)),
			auditDimStyle.Render(fmt.Sprintf("(%.1f%%)", pct)))
	}
	fmt.Println()

	// Status summary
	fmt.Println(auditSectionStyle.Render("Status Summary"))
	successPct := float64(stats.SuccessCount) / float64(stats.TotalEntries) * 100
	fmt.Printf("  %s%s  %s\n",
		auditLabelStyle.Render("Success:"),
		auditGreenStyle.Render(fmt.Sprintf("%d", stats.SuccessCount)),
		auditDimStyle.Render(fmt.Sprintf("(%.1f%%)", successPct)))
	errorPct := float64(stats.ErrorCount) / float64(stats.TotalEntries) * 100
	fmt.Printf("  %s%s  %s\n",
		auditLabelStyle.Render("Errors:"),
		auditRedStyle.Render(fmt.Sprintf("%d", stats.ErrorCount)),
		auditDimStyle.Render(fmt.Sprintf("(%.1f%%)", errorPct)))
	fmt.Println()

	// Usage summary
	fmt.Println(auditSectionStyle.Render("Usage Summary"))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Sessions:"), auditValueStyle.Render(strconv.Itoa(stats.SessionCount)))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Total Tokens:"), auditValueStyle.Render(strconv.Itoa(stats.TotalTokens)))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Total Cost:"), auditValueStyle.Render(fmt.Sprintf("%.2f cents", stats.TotalCost)))
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// =============================================================================
// AU-5: AUDIT CAPACITY COMMAND
// =============================================================================

// AuditCapacityOutput is the JSON output format for audit capacity.
type AuditCapacityOutput struct {
	UsedMB       int64   `json:"used_mb"`
	TotalMB      int64   `json:"total_mb"`
	UsedPercent  float64 `json:"used_percent"`
	WarningMB    int64   `json:"warning_threshold_mb"`
	CriticalMB   int64   `json:"critical_threshold_mb"`
	Status       string  `json:"status"` // "ok", "warning", "critical"
	AuditLogPath string  `json:"audit_log_path"`
	AuditLogSize int64   `json:"audit_log_size_bytes"`
}

// handleAuditCapacity shows storage capacity for audit logs (AU-5).
func handleAuditCapacity(auditArgs AuditArgs) error {
	logger := security.GlobalAuditLogger()
	if logger == nil || !logger.IsEnabled() {
		if auditArgs.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "audit logging not enabled",
				"message": "Enable with: rigrun config set security.audit_enabled true",
			})
		}
		fmt.Println()
		fmt.Println(auditYellowStyle.Render("Audit logging is not enabled."))
		fmt.Println("Enable with: rigrun config set security.audit_enabled true")
		fmt.Println()
		return nil
	}

	// Get capacity
	usedMB, totalMB, err := logger.CheckCapacity()
	if err != nil {
		return fmt.Errorf("failed to check capacity: %w", err)
	}

	// Calculate percentage and status
	usedPercent := float64(usedMB) * 100 / float64(totalMB)
	warningMB := totalMB * 80 / 100
	criticalMB := totalMB * 90 / 100

	status := "ok"
	if usedMB >= criticalMB {
		status = "critical"
	} else if usedMB >= warningMB {
		status = "warning"
	}

	// Get audit log size
	var auditLogSize int64
	auditPath := logger.Path()
	if info, err := os.Stat(auditPath); err == nil {
		auditLogSize = info.Size()
	}

	output := AuditCapacityOutput{
		UsedMB:       usedMB,
		TotalMB:      totalMB,
		UsedPercent:  usedPercent,
		WarningMB:    warningMB,
		CriticalMB:   criticalMB,
		Status:       status,
		AuditLogPath: auditPath,
		AuditLogSize: auditLogSize,
	}

	if auditArgs.JSON {
		return outputJSON(output)
	}

	// Display capacity information
	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-5 Audit Capacity Status"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	// Status indicator
	var statusText string
	switch status {
	case "ok":
		statusText = auditGreenStyle.Render("[OK]")
	case "warning":
		statusText = auditYellowStyle.Render("[WARNING]")
	case "critical":
		statusText = auditRedStyle.Render("[CRITICAL]")
	}
	fmt.Printf("Status: %s\n", statusText)
	fmt.Println()

	// Capacity details
	fmt.Println(auditSectionStyle.Render("Storage Capacity"))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Used:"), auditValueStyle.Render(fmt.Sprintf("%d MB (%.1f%%)", usedMB, usedPercent)))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Total:"), auditValueStyle.Render(fmt.Sprintf("%d MB", totalMB)))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Available:"), auditValueStyle.Render(fmt.Sprintf("%d MB", totalMB-usedMB)))
	fmt.Println()

	// Thresholds
	fmt.Println(auditSectionStyle.Render("Thresholds"))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Warning (80%):"), auditYellowStyle.Render(fmt.Sprintf("%d MB", warningMB)))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Critical (90%):"), auditRedStyle.Render(fmt.Sprintf("%d MB", criticalMB)))
	fmt.Println()

	// Audit log file
	fmt.Println(auditSectionStyle.Render("Audit Log"))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Path:"), auditDimStyle.Render(auditPath))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Size:"), auditValueStyle.Render(formatBytes(auditLogSize)))
	fmt.Println()

	// Recommendations
	if status == "warning" || status == "critical" {
		fmt.Println(auditSectionStyle.Render("Recommendations"))
		fmt.Println("  1. Export audit logs: rigrun audit export --output backup.json")
		fmt.Println("  2. Archive old logs: rigrun audit clear --confirm")
		fmt.Println("  3. Increase storage or reduce log retention")
		fmt.Println()
	}

	return nil
}

// =============================================================================
// AU-5: AUDIT VERIFY COMMAND (Integrity)
// =============================================================================

// handleAuditVerify verifies audit log integrity (AU-5/SI-7 crossover).
func handleAuditVerify(auditArgs AuditArgs) error {
	logger := security.GlobalAuditLogger()
	if logger == nil {
		if auditArgs.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "audit logger not initialized",
				"message": "Audit logging may not be enabled",
			})
		}
		fmt.Println()
		fmt.Println(auditYellowStyle.Render("Audit logger not initialized."))
		fmt.Println()
		return nil
	}

	// Perform integrity verification
	err := logger.VerifyIntegrity()

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"valid":   err == nil,
			"path":    logger.Path(),
			"error":   errorToString(err),
			"checked": time.Now().Format(time.RFC3339),
		})
	}

	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-5 Audit Log Integrity"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	if err == nil {
		fmt.Printf("  %s Audit log integrity verified\n", auditGreenStyle.Render("[PASS]"))
	} else {
		fmt.Printf("  %s Audit log integrity check failed\n", auditRedStyle.Render("[FAIL]"))
		fmt.Printf("  %s %s\n", auditRedStyle.Render("Error:"), err.Error())
	}
	fmt.Println()

	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Path:"), auditDimStyle.Render(logger.Path()))
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Checked:"), auditValueStyle.Render(time.Now().Format("2006-01-02 15:04:05")))
	fmt.Println()

	// Log the verification
	if logger.IsEnabled() {
		status := "PASS"
		if err != nil {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "audit",
			"status": status,
		})
	}

	return nil
}

// errorToString safely converts an error to a string for JSON output.
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// =============================================================================
// AU-5: AUDIT ALERT CONFIGURE COMMAND
// =============================================================================

// handleAuditAlert configures audit alerting thresholds (AU-5).
func handleAuditAlert(auditArgs AuditArgs) error {
	cfg := config.Global()

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"halt_on_failure":      cfg.Security.HaltOnAuditFailure,
			"capacity_warning_mb":  cfg.Security.AuditCapacityWarningMB,
			"capacity_critical_mb": cfg.Security.AuditCapacityCriticalMB,
		})
	}

	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-5 Audit Alert Configuration"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	// Current settings
	fmt.Println(auditSectionStyle.Render("Current Settings"))

	haltStatus := auditRedStyle.Render("Disabled")
	if cfg.Security.HaltOnAuditFailure {
		haltStatus = auditGreenStyle.Render("Enabled")
	}
	fmt.Printf("  %s%s\n", auditLabelStyle.Render("Halt on Failure:"), haltStatus)

	warningMB := cfg.Security.AuditCapacityWarningMB
	if warningMB == 0 {
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Warning Threshold:"), auditDimStyle.Render("Auto (80%)"))
	} else {
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Warning Threshold:"), auditValueStyle.Render(fmt.Sprintf("%d MB", warningMB)))
	}

	criticalMB := cfg.Security.AuditCapacityCriticalMB
	if criticalMB == 0 {
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Critical Threshold:"), auditDimStyle.Render("Auto (90%)"))
	} else {
		fmt.Printf("  %s%s\n", auditLabelStyle.Render("Critical Threshold:"), auditValueStyle.Render(fmt.Sprintf("%d MB", criticalMB)))
	}
	fmt.Println()

	// Configuration commands
	fmt.Println(auditSectionStyle.Render("Configure with:"))
	fmt.Println("  rigrun config set security.halt_on_audit_failure true")
	fmt.Println("  rigrun config set security.audit_capacity_warning_mb 8192")
	fmt.Println("  rigrun config set security.audit_capacity_critical_mb 9216")
	fmt.Println()

	return nil
}

// =============================================================================
// AU-6: AUDIT REVIEW, ANALYSIS, AND REPORTING COMMANDS
// =============================================================================

// handleAuditAnalyze analyzes audit logs for anomalies (AU-6).
func handleAuditAnalyze(auditArgs AuditArgs) error {
	reviewer := security.GlobalAuditReviewer()

	// Parse time range
	timeRange := 24 * time.Hour // Default: last 24 hours
	if auditArgs.Since != "" {
		// Try parsing as duration (e.g., "48h", "7d")
		if d, err := parseRelativeTime(auditArgs.Since); err == nil {
			timeRange = d
		}
	}

	// Run analysis
	result, err := reviewer.AnalyzeLogs(timeRange)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	if auditArgs.JSON {
		return outputJSON(result)
	}

	// Display results
	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-6 Audit Log Analysis"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	fmt.Printf("Period:       %s to %s\n", result.StartTime.Format(time.RFC3339), result.EndTime.Format(time.RFC3339))
	fmt.Printf("Total Events: %d\n", result.TotalEvents)
	fmt.Printf("Alerts:       %d\n", len(result.TriggeredAlerts))
	fmt.Println()

	if len(result.TriggeredAlerts) > 0 {
		fmt.Println(auditSectionStyle.Render("Triggered Alerts"))
		for _, alert := range result.TriggeredAlerts {
			severityStyle := auditValueStyle
			switch alert.Severity {
			case security.SeverityCritical:
				severityStyle = auditRedStyle
			case security.SeverityHigh:
				severityStyle = auditRedStyle
			case security.SeverityMedium:
				severityStyle = auditYellowStyle
			}
			fmt.Printf("  [%s] %s: %s\n", severityStyle.Render(strings.ToUpper(alert.Severity)), alert.EventType, alert.Message)
		}
		fmt.Println()
	}

	if len(result.Findings) > 0 {
		fmt.Println(auditSectionStyle.Render("Key Findings"))
		for i, finding := range result.Findings {
			fmt.Printf("  %d. %s\n", i+1, finding)
		}
		fmt.Println()
	}

	if len(result.TriggeredAlerts) == 0 && len(result.Findings) == 0 {
		fmt.Println(auditGreenStyle.Render("No anomalies detected"))
		fmt.Println()
	}

	return nil
}

// handleAuditReport generates a compliance report (AU-6).
func handleAuditReport(auditArgs AuditArgs) error {
	reviewer := security.GlobalAuditReviewer()

	// Default format and time range
	format := "text"
	if auditArgs.Format != "" {
		format = auditArgs.Format
	}

	timeRange := 7 * 24 * time.Hour // Default: last 7 days
	if auditArgs.Since != "" {
		if d, err := parseRelativeTime(auditArgs.Since); err == nil {
			timeRange = d
		}
	}

	// Generate report
	report, err := reviewer.GenerateReport(format, timeRange)
	if err != nil {
		return fmt.Errorf("report generation failed: %w", err)
	}

	// Output to file or stdout
	if auditArgs.Output != "" {
		// Validate output path to prevent path traversal attacks
		validatedPath, err := ValidateOutputPath(auditArgs.Output)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
		if err := os.WriteFile(validatedPath, report, 0600); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("%s Report saved to: %s\n", auditSuccessStyle.Render("[OK]"), validatedPath)
	} else {
		fmt.Println(string(report))
	}

	return nil
}

// handleAuditAlerts shows triggered alerts (AU-6).
func handleAuditAlerts(auditArgs AuditArgs) error {
	reviewer := security.GlobalAuditReviewer()

	// Get all unacknowledged alerts
	alerts := reviewer.GetAlerts(false)

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"count":  len(alerts),
			"alerts": alerts,
		})
	}

	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-6 Audit Alerts"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	if len(alerts) == 0 {
		fmt.Println(auditGreenStyle.Render("No active alerts"))
		fmt.Println()
		return nil
	}

	for _, alert := range alerts {
		severityStyle := auditValueStyle
		switch alert.Severity {
		case security.SeverityCritical:
			severityStyle = auditRedStyle
		case security.SeverityHigh:
			severityStyle = auditRedStyle
		case security.SeverityMedium:
			severityStyle = auditYellowStyle
		}

		fmt.Printf("[%s] %s\n", severityStyle.Render(strings.ToUpper(alert.Severity)), alert.ID)
		fmt.Printf("  Type:      %s\n", alert.EventType)
		fmt.Printf("  Timestamp: %s\n", alert.Timestamp.Format(time.RFC3339))
		fmt.Printf("  Message:   %s\n", alert.Message)
		if alert.EventCount > 0 {
			fmt.Printf("  Events:    %d\n", alert.EventCount)
		}
		fmt.Println()
	}

	return nil
}

// handleAuditSearch searches audit logs (AU-6).
func handleAuditSearch(auditArgs AuditArgs) error {
	reviewer := security.GlobalAuditReviewer()

	// Extract query from remaining args
	query := strings.Join(auditArgs.Raw, " ")
	if query == "" {
		return fmt.Errorf("search query required")
	}

	results, err := reviewer.SearchLogs(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"query":   query,
			"count":   len(results),
			"results": results,
		})
	}

	fmt.Println()
	fmt.Println(auditTitleStyle.Render(fmt.Sprintf("AU-6 Audit Search: %s", query)))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	if len(results) == 0 {
		fmt.Println(auditDimStyle.Render("No matching entries found"))
		fmt.Println()
		return nil
	}

	fmt.Printf("Found %d matching entries:\n\n", len(results))

	// Show up to 50 results
	limit := len(results)
	if limit > 50 {
		limit = 50
	}

	for i := 0; i < limit; i++ {
		entry := results[i]
		fmt.Printf("%s  %s  %s\n",
			auditDimStyle.Render(entry.Timestamp.Format("2006-01-02 15:04:05")),
			auditValueStyle.Render(fmt.Sprintf("%-16s", entry.EventType)),
			auditDimStyle.Render(entry.SessionID[:min(8, len(entry.SessionID))]+"..."))
	}

	if len(results) > 50 {
		fmt.Printf("\n... and %d more results (use --json for full output)\n", len(results)-50)
	}
	fmt.Println()

	return nil
}

// handleAuditExportSIEM exports logs for SIEM integration (AU-6).
func handleAuditExportSIEM(auditArgs AuditArgs) error {
	reviewer := security.GlobalAuditReviewer()

	format := "json"
	if auditArgs.Format != "" {
		format = auditArgs.Format
	}

	timeRange := 7 * 24 * time.Hour // Default: last 7 days
	if auditArgs.Since != "" {
		if d, err := parseRelativeTime(auditArgs.Since); err == nil {
			timeRange = d
		}
	}

	// Export logs
	data, err := reviewer.ExportLogs(format, timeRange)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Output to file or stdout
	if auditArgs.Output != "" {
		// Validate output path to prevent path traversal attacks
		validatedPath, err := ValidateOutputPath(auditArgs.Output)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
		if err := os.WriteFile(validatedPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write export: %w", err)
		}
		fmt.Printf("%s Exported to: %s\n", auditSuccessStyle.Render("[OK]"), validatedPath)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

// =============================================================================
// AU-9: AUDIT PROTECTION COMMANDS
// =============================================================================

// handleAuditVerifyIntegrity verifies audit log integrity chain (AU-9).
func handleAuditVerifyIntegrity(auditArgs AuditArgs) error {
	protector := security.GlobalAuditProtector()

	valid, issues, err := protector.VerifyLogIntegrity()
	if err != nil {
		return fmt.Errorf("integrity verification failed: %w", err)
	}

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"valid":  valid,
			"issues": issues,
		})
	}

	fmt.Println()
	fmt.Println(auditTitleStyle.Render("AU-9 Audit Log Integrity"))
	fmt.Println(auditSeparatorStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	if valid {
		fmt.Printf("  %s Audit log integrity chain is intact\n", auditGreenStyle.Render("[PASS]"))
	} else {
		fmt.Printf("  %s Audit log integrity chain is BROKEN\n", auditRedStyle.Render("[FAIL]"))
		fmt.Println()
		fmt.Println(auditSectionStyle.Render("Issues Detected:"))
		for _, issue := range issues {
			fmt.Printf("  - %s\n", auditRedStyle.Render(issue))
		}
	}
	fmt.Println()

	return nil
}

// handleAuditProtect applies protection to audit logs (AU-9).
func handleAuditProtect(auditArgs AuditArgs) error {
	protector := security.GlobalAuditProtector()

	if err := protector.ProtectLogs(); err != nil {
		return fmt.Errorf("failed to protect logs: %w", err)
	}

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"protected": true,
			"message":   "Audit logs protected with restrictive permissions",
		})
	}

	fmt.Println()
	fmt.Printf("%s Audit logs protected\n", auditSuccessStyle.Render("[OK]"))
	fmt.Println("  - Set restrictive permissions (0600)")
	fmt.Println("  - Applied security attributes")
	fmt.Println()

	return nil
}

// handleAuditArchive archives old audit logs (AU-9).
func handleAuditArchive(auditArgs AuditArgs) error {
	protector := security.GlobalAuditProtector()

	// Parse retention days
	retentionDays := security.DefaultRetentionDays
	if len(auditArgs.Raw) > 0 {
		if days, err := strconv.Atoi(auditArgs.Raw[0]); err == nil && days > 0 {
			retentionDays = days
		}
	}

	if err := protector.ArchiveLogs(retentionDays); err != nil {
		return fmt.Errorf("archival failed: %w", err)
	}

	if auditArgs.JSON {
		return outputJSON(map[string]interface{}{
			"archived":  true,
			"retention": retentionDays,
		})
	}

	fmt.Println()
	fmt.Printf("%s Audit logs archived\n", auditSuccessStyle.Render("[OK]"))
	fmt.Printf("  Retention period: %d days\n", retentionDays)
	fmt.Println()

	return nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
