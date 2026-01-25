// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// consent.go - Consent command implementation for rigrun IL5 compliance (AC-8).
//
// CLI: Comprehensive help and examples for all commands
//
// This implements the "rigrun consent" command for DoD System Use Notification
// compliance per NIST 800-53 AC-8 and IL5 requirements.
//
// Command: consent [subcommand]
// Short:   System use notification consent management (IL5 AC-8)
// Aliases: (none)
//
// Subcommands:
//   show (default)      Show current consent status
//   ack                 Acknowledge system use notification
//   reset               Reset consent (requires re-acknowledgment)
//   history             Show consent history
//
// Examples:
//   rigrun consent                 Show consent status
//   rigrun consent show            Show current consent status
//   rigrun consent show --json     Status in JSON format
//   rigrun consent ack             Acknowledge system use notification
//   rigrun consent reset           Reset consent status
//   rigrun consent history         View consent acknowledgment history
//
// IL5 AC-8 Requirements:
//   - System use notification banner displayed before access
//   - User acknowledgment required and logged
//   - Consent timestamp and user ID recorded for audit
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
	"os/user"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/config"
)

// =============================================================================
// DOD CONSENT BANNER TEXT (AC-8 System Use Notification)
// =============================================================================

// ConsentBannerVersion is the version of the consent banner text.
// Increment this when the banner text changes to require re-acknowledgment.
const ConsentBannerVersion = "1.0.0"

// DoDConsentBanner is the standard DoD System Use Notification text.
// This text is required by NIST 800-53 AC-8 and DoD IL5 requirements.
const DoDConsentBanner = `
================================================================================
                    U.S. GOVERNMENT INFORMATION SYSTEM
================================================================================

You are accessing a U.S. Government (USG) Information System (IS) that is
provided for USG-authorized use only.

By using this IS (which includes any device attached to this IS), you consent
to the following conditions:

- The USG routinely intercepts and monitors communications on this IS for
  purposes including, but not limited to, penetration testing, COMSEC
  monitoring, network operations and defense, personnel misconduct (PM),
  law enforcement (LE), and counterintelligence (CI) investigations.

- At any time, the USG may inspect and seize data stored on this IS.

- Communications using, or data stored on, this IS are not private, are
  subject to routine monitoring, interception, and search, and may be
  disclosed or used for any USG-authorized purpose.

- This IS includes security measures (e.g., authentication and access
  controls) to protect USG interests--not for your personal benefit or
  privacy.

- Notwithstanding the above, using this IS does not constitute consent to
  PM, LE, or CI investigative searching or monitoring of the content of
  privileged communications, or work product, related to personal
  representation or services by attorneys, psychotherapists, or clergy,
  and their assistants. Such communications and work product are private
  and confidential. See User Agreement for details.

================================================================================
                         CONSENT ACKNOWLEDGMENT REQUIRED
================================================================================

By continuing to use this system, you acknowledge that you have read,
understand, and agree to abide by the terms and conditions stated above.

`

// =============================================================================
// CONSENT STYLES
// =============================================================================

var (
	// Banner title style
	consentTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196")). // Red for attention
				Background(lipgloss.Color("226")). // Yellow background
				Padding(0, 2)

	// Banner text style
	consentBannerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")). // White
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("196")). // Red border
				Padding(1, 2)

	// Status styles
	consentStatusOKStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")). // Green
				Bold(true)

	consentStatusWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")). // Yellow
				Bold(true)

	consentStatusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")). // Red
				Bold(true)

	// Label style
	consentLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(16)

	// Value style
	consentValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	// Section separator
	consentSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// CONSENT STATUS STRUCTURE
// =============================================================================

// ConsentStatus represents the current consent state for JSON output.
type ConsentStatus struct {
	Accepted       bool      `json:"accepted"`
	AcceptedAt     time.Time `json:"accepted_at,omitempty"`
	AcceptedBy     string    `json:"accepted_by,omitempty"`
	BannerVersion  string    `json:"banner_version"`
	CurrentVersion string    `json:"current_version"`
	Required       bool      `json:"required"`
	Valid          bool      `json:"valid"`
	Message        string    `json:"message,omitempty"`
}

// =============================================================================
// HANDLE CONSENT COMMAND
// =============================================================================

// HandleConsent handles the "consent" command with various subcommands.
// Subcommands:
//   - consent show: Display the DoD consent banner text
//   - consent status: Show if user has acknowledged consent
//   - consent accept: Record user consent acceptance
//   - consent reset: Reset consent (for testing/re-acknowledgment)
//   - consent require: Re-enable mandatory consent (default behavior)
//   - consent not-required: Disable consent requirement (non-compliant)
func HandleConsent(args Args) error {
	// Check --json flag from global flags or raw args
	jsonOutput := args.JSON
	if !jsonOutput {
		for _, arg := range args.Raw {
			if arg == "--json" {
				jsonOutput = true
				break
			}
		}
	}

	switch args.Subcommand {
	case "", "show":
		return handleConsentShow(jsonOutput)
	case "status":
		return handleConsentStatus(jsonOutput)
	case "accept":
		return handleConsentAccept(jsonOutput)
	case "reset":
		return handleConsentReset(jsonOutput)
	case "require":
		return handleConsentRequire(jsonOutput)
	case "not-required":
		return handleConsentNotRequired(jsonOutput)
	default:
		return fmt.Errorf("unknown consent subcommand: %s\n\nValid subcommands:\n"+
			"  show         - Display the DoD consent banner\n"+
			"  status       - Show current consent status\n"+
			"  accept       - Accept and record consent\n"+
			"  reset        - Reset consent for re-acknowledgment\n"+
			"  require      - Re-enable mandatory consent (default)\n"+
			"  not-required - Disable consent requirement (non-compliant)", args.Subcommand)
	}
}

// =============================================================================
// SUBCOMMAND HANDLERS
// =============================================================================

// handleConsentShow displays the DoD consent banner text.
func handleConsentShow(jsonOutput bool) error {
	if jsonOutput {
		output := map[string]interface{}{
			"banner_version": ConsentBannerVersion,
			"banner_text":    strings.TrimSpace(DoDConsentBanner),
			"compliance": map[string]string{
				"control":     "AC-8",
				"description": "System Use Notification",
				"framework":   "NIST 800-53",
				"impact":      "IL5",
			},
		}
		return consentOutputJSON(output)
	}

	// Display formatted banner
	fmt.Println()
	fmt.Println(consentTitleStyle.Render(" DoD SYSTEM USE NOTIFICATION (AC-8) "))
	fmt.Println()
	fmt.Print(DoDConsentBanner)
	fmt.Println()
	fmt.Printf("Banner Version: %s\n", ConsentBannerVersion)
	fmt.Println()

	return nil
}

// handleConsentStatus shows the current consent status.
func handleConsentStatus(jsonOutput bool) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	status := buildConsentStatus(cfg)

	if jsonOutput {
		return consentOutputJSON(status)
	}

	// Display formatted status
	fmt.Println()
	fmt.Println(consentTitleStyle.Render(" CONSENT STATUS "))
	fmt.Println()

	separator := strings.Repeat("-", 50)
	fmt.Println(consentSeparatorStyle.Render(separator))
	fmt.Println()

	// Accepted status
	acceptedStr := consentStatusErrorStyle.Render("NO")
	if status.Accepted {
		acceptedStr = consentStatusOKStyle.Render("YES")
	}
	fmt.Printf("  %s%s\n", consentLabelStyle.Render("Accepted:"), acceptedStr)

	// Accepted time
	if status.Accepted && !status.AcceptedAt.IsZero() {
		fmt.Printf("  %s%s\n",
			consentLabelStyle.Render("Accepted At:"),
			consentValueStyle.Render(status.AcceptedAt.Format(time.RFC3339)))
	}

	// Accepted by
	if status.Accepted && status.AcceptedBy != "" {
		fmt.Printf("  %s%s\n",
			consentLabelStyle.Render("Accepted By:"),
			consentValueStyle.Render(status.AcceptedBy))
	}

	// Banner version
	fmt.Printf("  %s%s\n",
		consentLabelStyle.Render("Banner Version:"),
		consentValueStyle.Render(status.BannerVersion))

	// Current version
	fmt.Printf("  %s%s\n",
		consentLabelStyle.Render("Current:"),
		consentValueStyle.Render(status.CurrentVersion))

	// Required status
	requiredStr := consentStatusWarnStyle.Render("No (non-DoD mode)")
	if status.Required {
		requiredStr = consentStatusOKStyle.Render("Yes (IL5 compliance)")
	}
	fmt.Printf("  %s%s\n",
		consentLabelStyle.Render("Required:"),
		requiredStr)

	// Validity
	fmt.Println()
	if status.Valid {
		fmt.Printf("  %s\n", consentStatusOKStyle.Render("[OK] Consent is valid and current"))
	} else {
		fmt.Printf("  %s\n", consentStatusErrorStyle.Render("[!] "+status.Message))
		if status.Required {
			fmt.Println()
			fmt.Println("  Run 'rigrun consent accept' to acknowledge the consent banner.")
		}
	}

	fmt.Println()
	fmt.Println(consentSeparatorStyle.Render(separator))
	fmt.Println()

	// Audit log entry for AU-12 compliance
	logConsentEvent("status_check", status.Accepted, "")

	return nil
}

// handleConsentAccept records user consent acceptance.
func handleConsentAccept(jsonOutput bool) error {
	// Get current user
	currentUser, err := user.Current()
	username := "unknown"
	if err == nil {
		username = currentUser.Username
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// Record consent
	now := time.Now()
	cfg.Consent.Accepted = true
	cfg.Consent.AcceptedAt = now
	cfg.Consent.AcceptedBy = username
	cfg.Consent.BannerVersion = ConsentBannerVersion

	// Save config
	if err := config.Save(cfg); err != nil {
		if jsonOutput {
			return consentOutputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to save consent: %w", err)
	}

	// Audit log entry for AU-12 compliance
	logConsentEvent("accept", true, username)

	if jsonOutput {
		return consentOutputJSON(map[string]interface{}{
			"success":        true,
			"accepted":       true,
			"accepted_at":    now.Format(time.RFC3339),
			"accepted_by":    username,
			"banner_version": ConsentBannerVersion,
			"message":        "Consent recorded successfully",
		})
	}

	fmt.Println()
	fmt.Println(consentStatusOKStyle.Render("[OK] Consent Accepted"))
	fmt.Println()
	fmt.Printf("  Timestamp: %s\n", now.Format(time.RFC3339))
	fmt.Printf("  User:      %s\n", username)
	fmt.Printf("  Version:   %s\n", ConsentBannerVersion)
	fmt.Println()
	fmt.Println("  Your consent has been recorded for IL5 compliance (AC-8).")
	fmt.Println("  This event has been logged for audit purposes (AU-12).")
	fmt.Println()

	return nil
}

// handleConsentReset resets the consent status.
func handleConsentReset(jsonOutput bool) error {
	// Get current user for audit
	currentUser, err := user.Current()
	username := "unknown"
	if err == nil {
		username = currentUser.Username
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// Clear consent
	cfg.Consent.Accepted = false
	cfg.Consent.AcceptedAt = time.Time{}
	cfg.Consent.AcceptedBy = ""
	cfg.Consent.BannerVersion = ""

	// Save config
	if err := config.Save(cfg); err != nil {
		if jsonOutput {
			return consentOutputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to reset consent: %w", err)
	}

	// Audit log entry for AU-12 compliance
	logConsentEvent("reset", false, username)

	if jsonOutput {
		return consentOutputJSON(map[string]interface{}{
			"success": true,
			"message": "Consent has been reset",
		})
	}

	fmt.Println()
	fmt.Println(consentStatusWarnStyle.Render("[OK] Consent Reset"))
	fmt.Println()
	fmt.Println("  Consent status has been cleared.")
	fmt.Println("  Run 'rigrun consent accept' to re-acknowledge.")
	fmt.Println()
	fmt.Println("  This event has been logged for audit purposes (AU-12).")
	fmt.Println()

	return nil
}

// handleConsentRequire enables mandatory consent on startup.
func handleConsentRequire(jsonOutput bool) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// Enable consent requirement
	cfg.Consent.Required = true

	// Save config
	if err := config.Save(cfg); err != nil {
		if jsonOutput {
			return consentOutputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to enable consent requirement: %w", err)
	}

	// Audit log
	logConsentEvent("require_enabled", true, "")

	if jsonOutput {
		return consentOutputJSON(map[string]interface{}{
			"success":  true,
			"required": true,
			"message":  "Consent requirement enabled for IL5 compliance",
		})
	}

	fmt.Println()
	fmt.Println(consentStatusOKStyle.Render("[OK] Consent Requirement Enabled"))
	fmt.Println()
	fmt.Println("  DoD consent banner is now required on startup.")
	fmt.Println("  This setting is required for IL5 compliance (AC-8).")
	fmt.Println()
	fmt.Println("  Users must acknowledge the banner before using rigrun.")
	fmt.Println()

	return nil
}

// handleConsentNotRequired disables the consent requirement.
// WARNING: This makes the system non-compliant with IL5 AC-8 requirements.
func handleConsentNotRequired(jsonOutput bool) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// Disable consent requirement
	cfg.Consent.Required = false

	// Save config
	if err := config.Save(cfg); err != nil {
		if jsonOutput {
			return consentOutputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to disable consent requirement: %w", err)
	}

	// Audit log
	logConsentEvent("require_disabled", false, "")

	if jsonOutput {
		return consentOutputJSON(map[string]interface{}{
			"success":  true,
			"required": false,
			"warning":  "System is now NON-COMPLIANT with IL5 AC-8 requirements",
			"message":  "Consent requirement disabled - NOT RECOMMENDED for DoD systems",
		})
	}

	fmt.Println()
	fmt.Println(consentStatusErrorStyle.Render("[WARNING] Consent Requirement Disabled - NON-COMPLIANT"))
	fmt.Println()
	fmt.Println("  DoD consent banner is no longer required on startup.")
	fmt.Println()
	fmt.Println(consentStatusErrorStyle.Render("  *** WARNING: SYSTEM IS NOW NON-COMPLIANT WITH IL5 ***"))
	fmt.Println()
	fmt.Println("  IL5 deployments REQUIRE consent acknowledgment per NIST 800-53 AC-8.")
	fmt.Println("  Only disable consent for non-DoD/non-government use cases.")
	fmt.Println()
	fmt.Println("  To re-enable compliance: rigrun consent require")
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// buildConsentStatus builds a ConsentStatus from the config.
func buildConsentStatus(cfg *config.Config) ConsentStatus {
	status := ConsentStatus{
		Accepted:       cfg.Consent.Accepted,
		AcceptedAt:     cfg.Consent.AcceptedAt,
		AcceptedBy:     cfg.Consent.AcceptedBy,
		BannerVersion:  cfg.Consent.BannerVersion,
		CurrentVersion: ConsentBannerVersion,
		Required:       cfg.Consent.Required,
	}

	// Determine validity
	if !cfg.Consent.Required {
		// Not required, always valid
		status.Valid = true
		status.Message = "Consent not required (non-DoD mode)"
	} else if !cfg.Consent.Accepted {
		// Required but not accepted
		status.Valid = false
		status.Message = "Consent not yet accepted"
	} else if cfg.Consent.BannerVersion != ConsentBannerVersion {
		// Accepted but version mismatch
		status.Valid = false
		status.Message = fmt.Sprintf("Banner version changed (was %s, now %s) - re-acknowledgment required",
			cfg.Consent.BannerVersion, ConsentBannerVersion)
	} else {
		// All good
		status.Valid = true
		status.Message = "Consent is valid and current"
	}

	return status
}

// consentOutputJSON outputs data as formatted JSON.
func consentOutputJSON(data interface{}) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	fmt.Println(string(output))
	return nil
}

// logConsentEvent logs a consent-related event for AU-12 audit compliance.
func logConsentEvent(event string, accepted bool, username string) {
	// Get current user if not provided
	if username == "" {
		if u, err := user.Current(); err == nil {
			username = u.Username
		} else {
			username = "unknown"
		}
	}

	// Build audit entry
	entry := map[string]interface{}{
		"timestamp":      time.Now().Format(time.RFC3339),
		"event_type":     "consent",
		"event_action":   event,
		"user":           username,
		"accepted":       accepted,
		"banner_version": ConsentBannerVersion,
		"control":        "AC-8",
		"audit_control":  "AU-12",
	}

	// Write to audit log file
	writeAuditLog(entry)
}

// writeAuditLog writes an entry to the audit log file.
func writeAuditLog(entry map[string]interface{}) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Ensure audit directory exists
	auditDir := homeDir + "/.rigrun/audit"
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		return
	}

	// Write to daily audit log
	today := time.Now().Format("2006-01-02")
	auditFile := fmt.Sprintf("%s/consent_%s.log", auditDir, today)

	// Append entry as JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.Write([]byte("\n"))
}

// =============================================================================
// CONSENT CHECK FOR STARTUP
// =============================================================================

// CheckConsentRequired checks if consent is required and valid.
// Returns true if the application can proceed, false if consent is needed.
// This function should be called during application startup for IL5 compliance.
func CheckConsentRequired() (bool, string) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// If consent is not required, always allow
	if !cfg.Consent.Required {
		return true, ""
	}

	// Check if consent was accepted
	if !cfg.Consent.Accepted {
		return false, "Consent required but not accepted. Run 'rigrun consent accept'."
	}

	// Check banner version
	if cfg.Consent.BannerVersion != ConsentBannerVersion {
		return false, fmt.Sprintf("Consent banner updated (v%s). Re-acknowledgment required. Run 'rigrun consent accept'.",
			ConsentBannerVersion)
	}

	return true, ""
}

// PromptForConsent displays the consent banner and prompts for acceptance.
// Returns true if the user accepts, false otherwise.
// This is used for interactive consent during TUI startup.
func PromptForConsent() bool {
	fmt.Print(DoDConsentBanner)
	fmt.Println()
	fmt.Print("Do you accept these terms? [y/N]: ")

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		// Record acceptance
		handleConsentAccept(false)
		return true
	}

	return false
}

// RecordConsentAcceptance records consent acceptance from the TUI.
// This is a convenience wrapper for TUI consent flow.
// Returns an error if recording fails.
func RecordConsentAcceptance() error {
	return handleConsentAccept(false)
}
