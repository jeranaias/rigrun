// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// configmgmt_cmd.go - Configuration management CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 CM-5 (Access Restrictions for Change) and
// CM-6 (Configuration Settings) controls for DoD IL5 compliance.
//
// Command: configmgmt [subcommand]
// Short:   Configuration management (IL5 CM-5, CM-6)
// Aliases: cm
//
// Subcommands:
//   status (default)    Show configuration management status
//   baseline            Show/create configuration baseline
//   diff                Compare current config to baseline
//   audit               Show configuration change audit
//   lock                Lock configuration changes
//   unlock              Unlock configuration changes
//
// Examples:
//   rigrun configmgmt                     Show status (default)
//   rigrun configmgmt status              Show CM status
//   rigrun configmgmt status --json       Status in JSON format
//   rigrun configmgmt baseline            Show current baseline
//   rigrun configmgmt baseline create     Create new baseline
//   rigrun configmgmt diff                Compare to baseline
//   rigrun configmgmt audit               View change history
//   rigrun configmgmt lock                Lock changes
//   rigrun configmgmt unlock              Unlock changes
//
// CM-5/CM-6 Features:
//   - Baseline configuration management
//   - Configuration drift detection
//   - Change access restrictions
//   - All changes audited
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// CONFIG MGMT COMMAND STYLES
// =============================================================================

var (
	// Config mgmt title style
	cmTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	// Config mgmt section style
	cmSectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")). // White
			MarginTop(1)

	// Config mgmt label style
	cmLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(20)

	// Config mgmt value styles
	cmValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	cmCompliantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	cmNonCompliantStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	cmCriticalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	cmDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	// Config mgmt success style
	cmSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	// Config mgmt error style
	cmErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Separator style
	cmSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// CONFIG MGMT ARGUMENTS
// =============================================================================

// ConfigMgmtArgs holds parsed config-mgmt command arguments.
type ConfigMgmtArgs struct {
	Subcommand string
	Setting    string
	Value      string
	RequestID  string
	Reason     string
	JSON       bool
}

// parseConfigMgmtArgs parses config-mgmt command specific arguments from raw args.
func parseConfigMgmtArgs(args *Args, remaining []string) ConfigMgmtArgs {
	cmArgs := ConfigMgmtArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		cmArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	if len(remaining) > 0 {
		// First remaining arg could be setting, ID, or other parameter
		switch cmArgs.Subcommand {
		case "request":
			if len(remaining) >= 1 {
				cmArgs.Setting = remaining[0]
			}
			if len(remaining) >= 2 {
				cmArgs.Value = remaining[1]
			}
		case "approve", "reject":
			if len(remaining) >= 1 {
				cmArgs.RequestID = remaining[0]
			}
			if cmArgs.Subcommand == "reject" && len(remaining) >= 2 {
				// Join remaining args as reason
				cmArgs.Reason = strings.Join(remaining[1:], " ")
			}
		}
	}

	return cmArgs
}

// =============================================================================
// HANDLE CONFIG MGMT
// =============================================================================

// HandleConfigMgmt handles the "config-mgmt" command with various subcommands.
// Implements NIST 800-53 CM-5 and CM-6 controls.
// Subcommands:
//   - config-mgmt baseline: Show security baseline settings
//   - config-mgmt check: Compare current config to baseline
//   - config-mgmt request <setting> <value>: Request a configuration change
//   - config-mgmt approve <id>: Approve pending change
//   - config-mgmt reject <id> <reason>: Reject change
//   - config-mgmt pending: List pending changes
//   - config-mgmt history: Show change history
func HandleConfigMgmt(args Args) error {
	cmArgs := parseConfigMgmtArgs(&args, args.Raw)

	switch cmArgs.Subcommand {
	case "", "baseline":
		return handleConfigMgmtBaseline(cmArgs)
	case "check", "compliance":
		return handleConfigMgmtCheck(cmArgs)
	case "request":
		return handleConfigMgmtRequest(cmArgs)
	case "approve":
		return handleConfigMgmtApprove(cmArgs)
	case "reject":
		return handleConfigMgmtReject(cmArgs)
	case "pending":
		return handleConfigMgmtPending(cmArgs)
	case "history":
		return handleConfigMgmtHistory(cmArgs)
	default:
		return fmt.Errorf("unknown config-mgmt subcommand: %s\n\nUsage:\n"+
			"  rigrun config-mgmt baseline              Show security baseline settings (CM-6)\n"+
			"  rigrun config-mgmt check                 Compare current config to baseline\n"+
			"  rigrun config-mgmt request <setting> <value>  Request a configuration change (CM-5)\n"+
			"  rigrun config-mgmt approve <id>          Approve pending change\n"+
			"  rigrun config-mgmt reject <id> <reason>  Reject change\n"+
			"  rigrun config-mgmt pending               List pending changes\n"+
			"  rigrun config-mgmt history               Show change history", cmArgs.Subcommand)
	}
}

// =============================================================================
// CONFIG MGMT BASELINE
// =============================================================================

// handleConfigMgmtBaseline displays the security baseline configuration settings.
// Implements CM-6: Configuration Settings.
func handleConfigMgmtBaseline(args ConfigMgmtArgs) error {
	if args.JSON {
		return outputJSON(map[string]interface{}{
			"baseline": security.SecurityBaseline,
		})
	}

	separator := strings.Repeat("=", 80)
	fmt.Println()
	fmt.Println(cmTitleStyle.Render("CM-6: Security Configuration Baseline"))
	fmt.Println(cmSeparatorStyle.Render(separator))
	fmt.Println()

	// Get baseline settings sorted by name
	var names []string
	for name := range security.SecurityBaseline {
		names = append(names, name)
	}
	sort.Strings(names)

	// Display each setting
	for _, name := range names {
		setting := security.SecurityBaseline[name]
		fmt.Println(cmSectionStyle.Render(setting.Name))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Value:"), cmCompliantStyle.Render(setting.Value+" "+setting.Unit))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Control:"), cmValueStyle.Render(setting.Control))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Description:"), cmDimStyle.Render(setting.Description))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Rationale:"), cmDimStyle.Render(setting.Rationale))
		fmt.Println()
	}

	fmt.Println(cmDimStyle.Render("These baseline settings implement NIST 800-53 security controls."))
	fmt.Println(cmDimStyle.Render("Changes to these settings require approval per CM-5."))
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT COMPLIANCE CHECK
// =============================================================================

// handleConfigMgmtCheck compares current configuration to the security baseline.
// Implements CM-6: Configuration Settings verification.
func handleConfigMgmtCheck(args ConfigMgmtArgs) error {
	// Load current configuration
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Build current config map
	currentConfig := map[string]string{
		"session_timeout":      fmt.Sprintf("%d", cfg.Security.SessionTimeoutSecs),
		"consent_required":     fmt.Sprintf("%t", cfg.Security.ConsentRequired),
		"encryption_enabled":   fmt.Sprintf("%t", cfg.Security.EncryptionEnabled),
		"audit_enabled":        fmt.Sprintf("%t", cfg.Security.AuditEnabled),
		"max_login_attempts":   fmt.Sprintf("%d", cfg.Security.MaxLoginAttempts),
		"lockout_duration":     fmt.Sprintf("%d", cfg.Security.LockoutDurationSecs),
		"tls_min_version":      cfg.Security.TLSMinVersion,
		"allowed_ciphers":      cfg.Security.AllowedCiphers,
	}

	// Perform compliance check
	cm := security.GlobalConfigManager()
	results := cm.ComplianceCheck(currentConfig)

	if args.JSON {
		return outputJSON(results)
	}

	// Count compliant vs non-compliant
	var compliant, nonCompliant int
	for _, status := range results {
		if status.Compliant {
			compliant++
		} else {
			nonCompliant++
		}
	}

	separator := strings.Repeat("=", 80)
	fmt.Println()
	fmt.Println(cmTitleStyle.Render("CM-6: Configuration Compliance Check"))
	fmt.Println(cmSeparatorStyle.Render(separator))
	fmt.Println()

	// Overall status
	if nonCompliant == 0 {
		fmt.Printf("%s Configuration is fully compliant with security baseline\n", cmCompliantStyle.Render("[PASS]"))
	} else {
		fmt.Printf("%s Configuration has %d non-compliant settings\n", cmNonCompliantStyle.Render("[WARN]"), nonCompliant)
	}
	fmt.Println()

	// Sort results by setting name
	var sortedNames []string
	for name := range results {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	// Display results
	fmt.Println(cmSectionStyle.Render("Compliance Status"))
	for _, name := range sortedNames {
		status := results[name]

		statusText := cmCompliantStyle.Render("[OK]")
		if !status.Compliant {
			statusText = cmNonCompliantStyle.Render("[FAIL]")
		}

		fmt.Printf("  %s %s\n", statusText, cmValueStyle.Render(name))
		fmt.Printf("      %s%s\n", cmLabelStyle.Render("Expected:"), cmDimStyle.Render(status.BaselineValue))
		fmt.Printf("      %s%s\n", cmLabelStyle.Render("Current:"), cmDimStyle.Render(status.CurrentValue))

		if !status.Compliant && status.Issue != "" {
			fmt.Printf("      %s%s\n", cmLabelStyle.Render("Issue:"), cmCriticalStyle.Render(status.Issue))
		}
		fmt.Println()
	}

	// Summary
	fmt.Println(cmSeparatorStyle.Render(strings.Repeat("-", 80)))
	fmt.Printf("Compliant: %s | Non-Compliant: %s\n",
		cmCompliantStyle.Render(fmt.Sprintf("%d", compliant)),
		cmNonCompliantStyle.Render(fmt.Sprintf("%d", nonCompliant)))
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT REQUEST CHANGE
// =============================================================================

// handleConfigMgmtRequest creates a new configuration change request.
// Implements CM-5: Access Restrictions for Change.
func handleConfigMgmtRequest(args ConfigMgmtArgs) error {
	if args.Setting == "" {
		return fmt.Errorf("setting name required\nUsage: rigrun config-mgmt request <setting> <value>")
	}
	if args.Value == "" {
		return fmt.Errorf("value required\nUsage: rigrun config-mgmt request %s <value>", args.Setting)
	}

	cm := security.GlobalConfigManager()

	// Validate the setting
	if err := cm.ValidateSetting(args.Setting, args.Value); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Load current configuration to get old value
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get current value
	currentConfig := map[string]string{
		"session_timeout":      fmt.Sprintf("%d", cfg.Security.SessionTimeoutSecs),
		"consent_required":     fmt.Sprintf("%t", cfg.Security.ConsentRequired),
		"encryption_enabled":   fmt.Sprintf("%t", cfg.Security.EncryptionEnabled),
		"audit_enabled":        fmt.Sprintf("%t", cfg.Security.AuditEnabled),
		"max_login_attempts":   fmt.Sprintf("%d", cfg.Security.MaxLoginAttempts),
		"lockout_duration":     fmt.Sprintf("%d", cfg.Security.LockoutDurationSecs),
		"tls_min_version":      cfg.Security.TLSMinVersion,
		"allowed_ciphers":      cfg.Security.AllowedCiphers,
	}

	oldValue := currentConfig[args.Setting]
	if oldValue == "" {
		oldValue = "(not set)"
	}

	// Get current user (simplified - in production would use actual auth)
	requester := os.Getenv("USER")
	if requester == "" {
		requester = "system"
	}

	// Create change request
	cr, err := cm.RequestChange(args.Setting, oldValue, args.Value, requester)
	if err != nil {
		return fmt.Errorf("failed to create change request: %w", err)
	}

	if args.JSON {
		return outputJSON(cr)
	}

	fmt.Println()
	fmt.Printf("%s Change request created: %s\n", cmSuccessStyle.Render("[OK]"), cr.ID)
	fmt.Println()
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Setting:"), cmValueStyle.Render(cr.Setting))
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Old Value:"), cmDimStyle.Render(cr.OldValue))
	fmt.Printf("  %s%s ->%s\n", cmLabelStyle.Render("New Value:"), cmDimStyle.Render(cr.OldValue), cmCompliantStyle.Render(cr.NewValue))
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Requested By:"), cmValueStyle.Render(cr.RequestedBy))
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Status:"), cmValueStyle.Render(string(cr.Status)))

	if cr.IsSensitive {
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Approval:"), cmNonCompliantStyle.Render("Dual approval required (CM-5)"))
		fmt.Println()
		fmt.Println(cmDimStyle.Render("This is a sensitive setting that requires approval from a different user."))
		fmt.Printf("To approve: %s\n", cmDimStyle.Render(fmt.Sprintf("rigrun config-mgmt approve %s", cr.ID)))
	} else {
		fmt.Println()
		fmt.Println(cmDimStyle.Render("This change can be approved by any authorized user."))
		fmt.Printf("To approve: %s\n", cmDimStyle.Render(fmt.Sprintf("rigrun config-mgmt approve %s", cr.ID)))
	}
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT APPROVE CHANGE
// =============================================================================

// handleConfigMgmtApprove approves a pending configuration change.
// Implements CM-5: Dual approval for sensitive settings.
func handleConfigMgmtApprove(args ConfigMgmtArgs) error {
	if args.RequestID == "" {
		return fmt.Errorf("request ID required\nUsage: rigrun config-mgmt approve <id>")
	}

	cm := security.GlobalConfigManager()

	// Get current user (simplified - in production would use actual auth)
	approver := os.Getenv("USER")
	if approver == "" {
		approver = "system"
	}

	// Approve the change
	if err := cm.ApproveChange(args.RequestID, approver); err != nil {
		return fmt.Errorf("failed to approve change: %w", err)
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"status":     "approved",
			"request_id": args.RequestID,
			"approved_by": approver,
		})
	}

	fmt.Println()
	fmt.Printf("%s Change request approved: %s\n", cmSuccessStyle.Render("[OK]"), args.RequestID)
	fmt.Println()
	fmt.Println(cmDimStyle.Render("The configuration change has been approved."))
	fmt.Println(cmDimStyle.Render("Apply the change using: rigrun config set <setting> <value>"))
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT REJECT CHANGE
// =============================================================================

// handleConfigMgmtReject rejects a pending configuration change.
func handleConfigMgmtReject(args ConfigMgmtArgs) error {
	if args.RequestID == "" {
		return fmt.Errorf("request ID required\nUsage: rigrun config-mgmt reject <id> <reason>")
	}
	if args.Reason == "" {
		return fmt.Errorf("rejection reason required\nUsage: rigrun config-mgmt reject %s <reason>", args.RequestID)
	}

	cm := security.GlobalConfigManager()

	// Get current user (simplified - in production would use actual auth)
	rejecter := os.Getenv("USER")
	if rejecter == "" {
		rejecter = "system"
	}

	// Reject the change
	if err := cm.RejectChange(args.RequestID, rejecter, args.Reason); err != nil {
		return fmt.Errorf("failed to reject change: %w", err)
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"status":      "rejected",
			"request_id":  args.RequestID,
			"rejected_by": rejecter,
			"reason":      args.Reason,
		})
	}

	fmt.Println()
	fmt.Printf("%s Change request rejected: %s\n", cmErrorStyle.Render("[REJECTED]"), args.RequestID)
	fmt.Println()
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Rejected By:"), cmValueStyle.Render(rejecter))
	fmt.Printf("  %s%s\n", cmLabelStyle.Render("Reason:"), cmDimStyle.Render(args.Reason))
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT PENDING CHANGES
// =============================================================================

// handleConfigMgmtPending lists all pending configuration change requests.
func handleConfigMgmtPending(args ConfigMgmtArgs) error {
	cm := security.GlobalConfigManager()
	pending := cm.ListPendingChanges()

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"pending": pending,
			"count":   len(pending),
		})
	}

	separator := strings.Repeat("=", 80)
	fmt.Println()
	fmt.Println(cmTitleStyle.Render("CM-5: Pending Configuration Changes"))
	fmt.Println(cmSeparatorStyle.Render(separator))
	fmt.Println()

	if len(pending) == 0 {
		fmt.Println(cmDimStyle.Render("No pending configuration changes."))
		fmt.Println()
		return nil
	}

	// Display each pending change
	for _, cr := range pending {
		fmt.Println(cmSectionStyle.Render(fmt.Sprintf("[%s] %s", cr.ID, cr.Setting)))
		fmt.Printf("  %s%s ->%s\n", cmLabelStyle.Render("Change:"),
			cmDimStyle.Render(cr.OldValue),
			cmCompliantStyle.Render(cr.NewValue))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Requested By:"), cmValueStyle.Render(cr.RequestedBy))
		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Requested At:"), cmDimStyle.Render(cr.RequestedAt.Format("2006-01-02 15:04:05")))

		if cr.IsSensitive {
			fmt.Printf("  %s%s\n", cmLabelStyle.Render("Approval:"), cmNonCompliantStyle.Render("Dual approval required"))
		}

		fmt.Println()
		fmt.Printf("    Approve: %s\n", cmDimStyle.Render(fmt.Sprintf("rigrun config-mgmt approve %s", cr.ID)))
		fmt.Printf("    Reject:  %s\n", cmDimStyle.Render(fmt.Sprintf("rigrun config-mgmt reject %s \"reason\"", cr.ID)))
		fmt.Println()
	}

	fmt.Printf("Total pending: %d\n", len(pending))
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIG MGMT HISTORY
// =============================================================================

// handleConfigMgmtHistory displays the complete configuration change history.
func handleConfigMgmtHistory(args ConfigMgmtArgs) error {
	cm := security.GlobalConfigManager()
	history := cm.GetChangeHistory()

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"history": history,
			"count":   len(history),
		})
	}

	separator := strings.Repeat("=", 80)
	fmt.Println()
	fmt.Println(cmTitleStyle.Render("CM-5: Configuration Change History"))
	fmt.Println(cmSeparatorStyle.Render(separator))
	fmt.Println()

	if len(history) == 0 {
		fmt.Println(cmDimStyle.Render("No configuration change history."))
		fmt.Println()
		return nil
	}

	// Display changes in reverse chronological order (newest first)
	for i := len(history) - 1; i >= 0; i-- {
		cr := history[i]

		// Status indicator
		var statusStyle lipgloss.Style
		var statusText string
		switch cr.Status {
		case security.StatusPending:
			statusStyle = cmNonCompliantStyle
			statusText = "PENDING"
		case security.StatusApproved:
			statusStyle = cmCompliantStyle
			statusText = "APPROVED"
		case security.StatusRejected:
			statusStyle = cmErrorStyle
			statusText = "REJECTED"
		case security.StatusApplied:
			statusStyle = cmSuccessStyle
			statusText = "APPLIED"
		}

		fmt.Printf("[%s] %s %s\n",
			cmDimStyle.Render(cr.RequestedAt.Format("2006-01-02 15:04")),
			statusStyle.Render(statusText),
			cmValueStyle.Render(cr.Setting))

		fmt.Printf("  %s%s ->%s\n", cmLabelStyle.Render("Change:"),
			cmDimStyle.Render(cr.OldValue),
			cmCompliantStyle.Render(cr.NewValue))

		fmt.Printf("  %s%s\n", cmLabelStyle.Render("Requested By:"), cmValueStyle.Render(cr.RequestedBy))

		if cr.Status == security.StatusApproved || cr.Status == security.StatusApplied {
			fmt.Printf("  %s%s (%s)\n", cmLabelStyle.Render("Approved By:"),
				cmValueStyle.Render(cr.ApprovedBy),
				cmDimStyle.Render(cr.ApprovedAt.Format("2006-01-02 15:04")))
		} else if cr.Status == security.StatusRejected {
			fmt.Printf("  %s%s (%s)\n", cmLabelStyle.Render("Rejected By:"),
				cmValueStyle.Render(cr.RejectedBy),
				cmDimStyle.Render(cr.RejectedAt.Format("2006-01-02 15:04")))
			if cr.Reason != "" {
				fmt.Printf("  %s%s\n", cmLabelStyle.Render("Reason:"), cmDimStyle.Render(cr.Reason))
			}
		}

		fmt.Println()
	}

	fmt.Printf("Total changes: %d\n", len(history))
	fmt.Println()

	return nil
}
