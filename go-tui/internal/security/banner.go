// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security implements security-related functionality including
// the DoD consent banner required for IL5 compliance.
package security

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// DoDConsentBanner is the exact DoD IS consent banner text.
// This is legally required for government systems and MUST NOT be modified.
const DoDConsentBanner = `
╔═══════════════════════════════════════════════════════════════════════════════════════════════╗
║                         U.S. GOVERNMENT INFORMATION SYSTEM                                    ║
╠═══════════════════════════════════════════════════════════════════════════════════════════════╣
║  You are accessing a U.S. Government information system, which includes:                      ║
║  (1) this computer, (2) this computer network, (3) all computers connected to this network,   ║
║  and (4) all devices and storage media attached to this network or to a computer on this      ║
║  network. This information system is provided for U.S. Government-authorized use only.        ║
║                                                                                               ║
║  Unauthorized or improper use of this system may result in disciplinary action, as well as    ║
║  civil and criminal penalties.                                                                ║
║                                                                                               ║
║  By using this information system, you understand and consent to the following:               ║
║  • You have no reasonable expectation of privacy regarding any communications or data         ║
║    transiting or stored on this information system.                                           ║
║  • At any time, the government may monitor, intercept, search and seize any communication     ║
║    or data transiting or stored on this information system.                                   ║
║  • Any communications or data transiting or stored on this information system may be          ║
║    disclosed or used for any U.S. Government-authorized purpose.                              ║
╚═══════════════════════════════════════════════════════════════════════════════════════════════╝
`

// CompactDoDBanner is a shorter version for terminals less than 100 columns wide.
// This maintains the legally required text in a narrower format.
const CompactDoDBanner = `
╔════════════════════════════════════════════════════════════════╗
║           U.S. GOVERNMENT INFORMATION SYSTEM                   ║
╠════════════════════════════════════════════════════════════════╣
║  You are accessing a U.S. Government information system,       ║
║  which includes: (1) this computer, (2) this computer          ║
║  network, (3) all computers connected to this network,         ║
║  and (4) all devices and storage media attached to this        ║
║  network or to a computer on this network. This information    ║
║  system is provided for U.S. Government-authorized use only.   ║
║                                                                ║
║  Unauthorized or improper use of this system may result in     ║
║  disciplinary action, as well as civil and criminal penalties. ║
║                                                                ║
║  By using this information system, you understand and consent  ║
║  to the following:                                             ║
║  • You have no reasonable expectation of privacy regarding     ║
║    any communications or data transiting or stored on this     ║
║    information system.                                         ║
║  • At any time, the government may monitor, intercept, search  ║
║    and seize any communication or data transiting or stored    ║
║    on this information system.                                 ║
║  • Any communications or data transiting or stored on this     ║
║    information system may be disclosed or used for any U.S.    ║
║    Government-authorized purpose.                              ║
╚════════════════════════════════════════════════════════════════╝
`

// AmberGold is the standard amber/gold color for DoD consent banners (#FFB000)
const AmberGold = "#FFB000"

// BannerConfig holds configuration for the consent banner display.
type BannerConfig struct {
	Enabled      bool   // Whether the banner is enabled
	RequireAck   bool   // Require explicit acknowledgment
	AuditLog     bool   // Log acknowledgments to audit file
	CustomBanner string // Optional custom banner text (use with caution)
}

// DefaultBannerConfig returns the default configuration for DoD compliance.
func DefaultBannerConfig() BannerConfig {
	return BannerConfig{
		Enabled:      true,
		RequireAck:   true,
		AuditLog:     true,
		CustomBanner: "",
	}
}

// bannerStyle creates the lipgloss style for the banner in amber/gold.
func bannerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(AmberGold)).
		Bold(true)
}

// promptStyle creates the style for the acknowledgment prompt.
func promptStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(AmberGold)).
		Italic(true)
}

// RenderBanner returns the DoD consent banner with proper amber/gold styling.
func RenderBanner() string {
	style := bannerStyle()
	return style.Render(DoDConsentBanner)
}

// RenderCompactBanner returns the compact DoD consent banner with proper styling.
func RenderCompactBanner() string {
	style := bannerStyle()
	return style.Render(CompactDoDBanner)
}

// getTerminalWidth returns the current terminal width, defaulting to 80 if unavailable.
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80 // Default width
	}
	return width
}

// getAuditLogPath returns the path to the audit log file.
func getAuditLogPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	rigrunDir := filepath.Join(homeDir, ".rigrun")
	if err := os.MkdirAll(rigrunDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .rigrun directory: %w", err)
	}

	return filepath.Join(rigrunDir, "audit.log"), nil
}

// writeAuditLog appends a line to the audit log file.
func writeAuditLog(entry string) error {
	logPath, err := getAuditLogPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write to audit log: %w", err)
	}

	return nil
}

// formatTimestamp returns the current time formatted for audit logging.
func formatTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ShowBanner displays the DoD consent banner in amber/gold color and waits
// for user acknowledgment by pressing Enter.
// Returns an error if the user doesn't acknowledge or if there's an I/O error.
func ShowBanner() error {
	width := getTerminalWidth()

	var banner string
	if width < 100 {
		banner = RenderCompactBanner()
	} else {
		banner = RenderBanner()
	}

	fmt.Println(banner)

	prompt := promptStyle().Render("\nPress Enter to acknowledge and continue...")
	fmt.Println(prompt)

	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read acknowledgment: %w", err)
	}

	return nil
}

// ShowBannerWithAck displays the banner and prompts for explicit acknowledgment.
// Returns (true, nil) if acknowledged, (false, nil) if cancelled, or (false, error) on error.
func ShowBannerWithAck() (bool, error) {
	width := getTerminalWidth()

	var banner string
	if width < 100 {
		banner = RenderCompactBanner()
	} else {
		banner = RenderBanner()
	}

	fmt.Println(banner)

	prompt := promptStyle().Render("\nPress Enter to acknowledge and continue, or Ctrl+C to exit")
	fmt.Println(prompt)

	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	if err != nil {
		// Check if it was an interrupt (Ctrl+C typically causes EOF or interrupt)
		if err.Error() == "EOF" || strings.Contains(err.Error(), "interrupt") {
			return false, nil
		}
		return false, fmt.Errorf("failed to read acknowledgment: %w", err)
	}

	return true, nil
}

// LogBannerAcknowledgment logs a banner acknowledgment to the audit file.
// Format: YYYY-MM-DD HH:MM:SS | BANNER_ACK | session_id
func LogBannerAcknowledgment(sessionID string) error {
	entry := fmt.Sprintf("%s | BANNER_ACK | %s", formatTimestamp(), sessionID)
	return writeAuditLog(entry)
}

// LogBannerSkipped logs that the banner was skipped for compliance audit.
// This should always be called when the banner is not shown.
func LogBannerSkipped(reason string) error {
	entry := fmt.Sprintf("%s | BANNER_SKIPPED | reason=%s", formatTimestamp(), reason)
	return writeAuditLog(entry)
}

// skipBannerFlagSet tracks whether --skip-banner was used (set by CLI parsing)
var skipBannerFlagSet bool

// SetSkipBannerFlag sets the skip banner flag state (called by CLI parsing).
func SetSkipBannerFlag(skip bool) {
	skipBannerFlagSet = skip
}

// IsBannerRequired checks if the banner should be displayed.
// Returns false if --skip-banner was used (but logs it for compliance).
func IsBannerRequired() bool {
	// Check for --skip-banner flag
	if skipBannerFlagSet {
		// Log that banner was skipped for audit trail
		_ = LogBannerSkipped("CLI flag")
		return false
	}

	// Check for CI environment
	if isCIEnvironment() {
		// Log that banner was skipped in CI
		_ = LogBannerSkipped("CI environment")
		return false
	}

	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		_ = LogBannerSkipped("non-interactive environment")
		return false
	}

	return true
}

// isCIEnvironment checks if running in a CI/automated environment.
func isCIEnvironment() bool {
	ciEnvVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_URL",
		"TRAVIS",
		"CIRCLECI",
		"BUILDKITE",
		"TF_BUILD",       // Azure DevOps
		"TEAMCITY_VERSION",
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	return false
}

// CheckBannerCompliance verifies the banner configuration is compliant.
// Returns (isCompliant, issues) where issues is a list of compliance problems.
func CheckBannerCompliance() (bool, []string) {
	var issues []string

	// Check if audit log directory is writable
	logPath, err := getAuditLogPath()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot access audit log path: %v", err))
	} else {
		// Try to write to verify permissions
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Cannot write to audit log: %v", err))
		} else {
			f.Close()
		}
	}

	// Verify banner text contains required elements
	requiredPhrases := []string{
		"U.S. Government",
		"information system",
		"authorized use only",
		"Unauthorized or improper use",
		"disciplinary action",
		"civil and criminal penalties",
		"no reasonable expectation of privacy",
		"may monitor, intercept, search and seize",
		"disclosed or used",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(DoDConsentBanner, phrase) {
			issues = append(issues, fmt.Sprintf("Banner missing required phrase: %q", phrase))
		}
	}

	// Check terminal is available for interactive acknowledgment
	if !term.IsTerminal(int(os.Stdin.Fd())) && !isCIEnvironment() {
		issues = append(issues, "Non-interactive environment detected; banner acknowledgment may not work")
	}

	return len(issues) == 0, issues
}

// HandleConsentBanner is the main entry point for consent banner handling.
// Call this at application startup before any other operations.
// Returns nil if consent was acknowledged or properly skipped with logging.
func HandleConsentBanner(config BannerConfig, sessionID string) error {
	if !config.Enabled {
		if config.AuditLog {
			_ = LogBannerSkipped("banner disabled in config")
		}
		return nil
	}

	if !IsBannerRequired() {
		// Already logged by IsBannerRequired
		return nil
	}

	var acknowledged bool
	var err error

	if config.RequireAck {
		acknowledged, err = ShowBannerWithAck()
	} else {
		err = ShowBanner()
		acknowledged = err == nil
	}

	if err != nil {
		return fmt.Errorf("banner display error: %w", err)
	}

	if !acknowledged {
		return fmt.Errorf("banner acknowledgment required")
	}

	if config.AuditLog && sessionID != "" {
		if logErr := LogBannerAcknowledgment(sessionID); logErr != nil {
			// Log error but don't fail - audit logging failure shouldn't block operation
			fmt.Fprintf(os.Stderr, "Warning: Failed to log banner acknowledgment: %v\n", logErr)
		}
	}

	return nil
}

// GetBannerText returns the appropriate banner text based on terminal width.
// If customBanner is provided in config, it returns that instead.
func GetBannerText(config BannerConfig) string {
	if config.CustomBanner != "" {
		return config.CustomBanner
	}

	width := getTerminalWidth()
	if width < 100 {
		return CompactDoDBanner
	}
	return DoDConsentBanner
}
