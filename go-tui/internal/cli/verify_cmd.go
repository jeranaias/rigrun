// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// verify_cmd.go - Integrity verification CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 SI-7 (Software, Firmware, and Information Integrity)
// controls for DoD IL5 compliance.
//
// Command: verify [subcommand]
// Short:   Integrity verification (IL5 SI-7)
// Aliases: (none)
//
// Subcommands:
//   status (default)    Show integrity verification status
//   binary              Verify binary integrity
//   config              Verify configuration file integrity
//   audit               Verify audit log integrity
//   all                 Verify all components
//   baseline            Create integrity baseline
//
// Examples:
//   rigrun verify                     Show status (default)
//   rigrun verify status              Show integrity status
//   rigrun verify status --json       Status in JSON format
//   rigrun verify binary              Verify binary checksums
//   rigrun verify config              Verify config integrity
//   rigrun verify audit               Verify audit log chain
//   rigrun verify all                 Verify all components
//   rigrun verify baseline            Create/update baseline
//
// SI-7 Integrity Checks:
//   - Binary hash verification
//   - Configuration file tampering detection
//   - Audit log chain integrity
//   - Runtime integrity monitoring
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
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// VERIFY COMMAND STYLES
// =============================================================================

var (
	// Verify title style
	verifyTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")). // Cyan
		MarginBottom(1)

	// Verify section style
	verifySectionStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")). // White
		MarginTop(1)

	// Verify label style
	verifyLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")). // Light gray
		Width(20)

	// Verify value styles
	verifyValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")) // White

	verifyGreenStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")) // Green

	verifyYellowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")) // Yellow

	verifyRedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")) // Red

	verifyDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")) // Dim

	// Verify separator style
	verifySeparatorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
)

// =============================================================================
// VERIFY ARGUMENTS
// =============================================================================

// VerifyArgs holds parsed verify command arguments.
type VerifyArgs struct {
	Subcommand string
	All        bool
	Binary     bool
	Config     bool
	Audit      bool
	Checksum   bool
	Update     bool
	JSON       bool
}

// parseVerifyArgs parses verify command specific arguments from raw args.
func parseVerifyArgs(args *Args, remaining []string) VerifyArgs {
	verifyArgs := VerifyArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		verifyArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for _, arg := range remaining {
		switch arg {
		case "--all", "-a":
			verifyArgs.All = true
		case "--binary", "-b":
			verifyArgs.Binary = true
		case "--config", "-c":
			verifyArgs.Config = true
		case "--audit":
			verifyArgs.Audit = true
		case "--checksum":
			verifyArgs.Checksum = true
		case "--update":
			verifyArgs.Update = true
		case "--json":
			verifyArgs.JSON = true
		}
	}

	// Handle subcommand as flags
	switch verifyArgs.Subcommand {
	case "all":
		verifyArgs.All = true
	case "binary":
		verifyArgs.Binary = true
	case "config":
		verifyArgs.Config = true
	case "audit":
		verifyArgs.Audit = true
	case "checksum":
		verifyArgs.Checksum = true
	case "update", "baseline":
		verifyArgs.Update = true
	}

	// Default to all if no specific flag set
	if !verifyArgs.Binary && !verifyArgs.Config && !verifyArgs.Audit && !verifyArgs.Checksum && !verifyArgs.Update {
		verifyArgs.All = true
	}

	return verifyArgs
}

// =============================================================================
// HANDLE VERIFY
// =============================================================================

// HandleVerify handles the "verify" command with various subcommands.
// Subcommands:
//   - verify or verify all: Verify everything
//   - verify binary: Verify binary integrity
//   - verify config: Verify config file integrity
//   - verify audit: Verify audit log integrity
//   - verify checksum: Verify all file checksums
//   - verify update: Update baseline checksums
func HandleVerify(args Args) error {
	verifyArgs := parseVerifyArgs(&args, args.Raw)

	// Handle update/baseline command
	if verifyArgs.Update {
		return handleVerifyUpdate(verifyArgs)
	}

	// Handle specific verifications
	if verifyArgs.Binary && !verifyArgs.All {
		return handleVerifyBinary(verifyArgs)
	}
	if verifyArgs.Config && !verifyArgs.All {
		return handleVerifyConfig(verifyArgs)
	}
	if verifyArgs.Audit && !verifyArgs.All {
		return handleVerifyAudit(verifyArgs)
	}
	if verifyArgs.Checksum && !verifyArgs.All {
		return handleVerifyChecksum(verifyArgs)
	}

	// Default: verify all
	return handleVerifyAll(verifyArgs)
}

// =============================================================================
// VERIFY ALL
// =============================================================================

// VerifyAllOutput is the JSON output format for verify all.
type VerifyAllOutput struct {
	Timestamp     time.Time                    `json:"timestamp"`
	AllPassed     bool                         `json:"all_passed"`
	BinaryResult  *security.IntegrityResult    `json:"binary_result,omitempty"`
	ConfigResults []security.IntegrityResult   `json:"config_results,omitempty"`
	AuditResult   *security.IntegrityResult    `json:"audit_result,omitempty"`
	SystemInfo    map[string]string            `json:"system_info,omitempty"`
}

// handleVerifyAll performs all integrity verifications.
func handleVerifyAll(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	output := VerifyAllOutput{
		Timestamp:  time.Now(),
		AllPassed:  true,
		SystemInfo: security.GetSystemInfo(),
	}

	// Get binary path
	binaryPath, _ := security.GetBinaryPath()

	// 1. Binary verification
	binaryValid, binaryErr := im.VerifyBinary()
	binaryResult := &security.IntegrityResult{
		Path:  binaryPath,
		Valid: binaryValid,
	}
	if binaryErr != nil {
		binaryResult.Error = binaryErr.Error()
		// Don't fail on missing baseline
		if !strings.Contains(binaryErr.Error(), "no checksum record") {
			output.AllPassed = false
		}
	}
	output.BinaryResult = binaryResult

	// 2. Config verification
	configResults, configErr := im.VerifyConfig()
	if configErr == nil {
		output.ConfigResults = configResults
		for _, r := range configResults {
			if !r.Valid && r.Error != "no baseline checksum (file may be new)" {
				output.AllPassed = false
			}
		}
	}

	// 3. Audit log verification
	auditResult, auditErr := im.VerifyAuditLog()
	if auditErr == nil {
		output.AuditResult = auditResult
		if !auditResult.Valid {
			output.AllPassed = false
		}
	}

	// Log the verification event
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		status := "PASS"
		if !output.AllPassed {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "all",
			"status": status,
		})
	}

	// Output
	if verifyArgs.JSON {
		return outputVerifyJSON(output)
	}

	return outputVerifyAllText(output)
}

// outputVerifyJSON outputs verify results as JSON.
func outputVerifyJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// outputVerifyAllText outputs verify all results in human-readable format.
func outputVerifyAllText(output VerifyAllOutput) error {
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(verifyTitleStyle.Render("SI-7 Integrity Verification"))
	fmt.Println(verifySeparatorStyle.Render(separator))
	fmt.Println()

	// Overall status
	if output.AllPassed {
		fmt.Println(verifyGreenStyle.Render("All integrity checks PASSED"))
	} else {
		fmt.Println(verifyRedStyle.Render("Some integrity checks FAILED"))
	}
	fmt.Println()

	// Binary verification
	fmt.Println(verifySectionStyle.Render("Binary Integrity"))
	if output.BinaryResult != nil {
		printIntegrityResult(*output.BinaryResult)
	} else {
		fmt.Println(verifyDimStyle.Render("  (not checked)"))
	}
	fmt.Println()

	// Config verification
	fmt.Println(verifySectionStyle.Render("Configuration Integrity"))
	if len(output.ConfigResults) > 0 {
		for _, r := range output.ConfigResults {
			printIntegrityResult(r)
		}
	} else {
		fmt.Println(verifyDimStyle.Render("  (no config files found)"))
	}
	fmt.Println()

	// Audit log verification
	fmt.Println(verifySectionStyle.Render("Audit Log Integrity"))
	if output.AuditResult != nil {
		printIntegrityResult(*output.AuditResult)
	} else {
		fmt.Println(verifyDimStyle.Render("  (not checked)"))
	}
	fmt.Println()

	// System info
	fmt.Println(verifySectionStyle.Render("System Information"))
	for k, v := range output.SystemInfo {
		fmt.Printf("  %s%s\n", verifyLabelStyle.Render(k+":"), verifyValueStyle.Render(v))
	}
	fmt.Println()

	return nil
}

// printIntegrityResult prints a single integrity result.
func printIntegrityResult(r security.IntegrityResult) {
	// Status indicator
	var status string
	if r.Valid {
		status = verifyGreenStyle.Render("[PASS]")
	} else if r.Error != "" && strings.Contains(r.Error, "no checksum record") {
		status = verifyYellowStyle.Render("[SKIP]")
	} else if r.Error != "" && strings.Contains(r.Error, "no baseline") {
		status = verifyYellowStyle.Render("[SKIP]")
	} else {
		status = verifyRedStyle.Render("[FAIL]")
	}

	// Path (truncate if too long)
	path := r.Path
	if len(path) > 45 {
		path = "..." + path[len(path)-42:]
	}

	fmt.Printf("  %s %s\n", status, path)

	if r.Error != "" && !strings.Contains(r.Error, "no checksum record") && !strings.Contains(r.Error, "no baseline") {
		fmt.Printf("       %s %s\n", verifyRedStyle.Render("Error:"), r.Error)
	}

	if !r.Valid && r.Expected != "" && r.Actual != "" {
		fmt.Printf("       %s %s\n", verifyDimStyle.Render("Expected:"), r.Expected[:16]+"...")
		fmt.Printf("       %s %s\n", verifyDimStyle.Render("Actual:  "), r.Actual[:16]+"...")
	}
}

// =============================================================================
// VERIFY BINARY
// =============================================================================

// handleVerifyBinary verifies only binary integrity.
func handleVerifyBinary(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	binaryPath, pathErr := security.GetBinaryPath()
	if pathErr != nil {
		return fmt.Errorf("failed to get binary path: %w", pathErr)
	}

	valid, err := im.VerifyBinary()
	result := security.IntegrityResult{
		Path:  binaryPath,
		Valid: valid,
	}
	if err != nil {
		result.Error = err.Error()
	}

	// Log the verification
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		status := "PASS"
		if !valid {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "binary",
			"status": status,
			"path":   binaryPath,
		})
	}

	if verifyArgs.JSON {
		return outputVerifyJSON(result)
	}

	fmt.Println()
	fmt.Println(verifyTitleStyle.Render("Binary Integrity Check"))
	fmt.Println()
	printIntegrityResult(result)
	fmt.Println()

	return nil
}

// =============================================================================
// VERIFY CONFIG
// =============================================================================

// handleVerifyConfig verifies only config file integrity.
func handleVerifyConfig(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	results, err := im.VerifyConfig()
	if err != nil {
		return fmt.Errorf("failed to verify config: %w", err)
	}

	// Log the verification
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		allPassed := true
		for _, r := range results {
			if !r.Valid && r.Error != "no baseline checksum (file may be new)" {
				allPassed = false
				break
			}
		}
		status := "PASS"
		if !allPassed {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "config",
			"status": status,
		})
	}

	if verifyArgs.JSON {
		return outputVerifyJSON(results)
	}

	fmt.Println()
	fmt.Println(verifyTitleStyle.Render("Configuration Integrity Check"))
	fmt.Println()
	if len(results) == 0 {
		fmt.Println(verifyDimStyle.Render("  No config files found"))
	} else {
		for _, r := range results {
			printIntegrityResult(r)
		}
	}
	fmt.Println()

	return nil
}

// =============================================================================
// VERIFY AUDIT
// =============================================================================

// handleVerifyAudit verifies only audit log integrity.
func handleVerifyAudit(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	result, err := im.VerifyAuditLog()
	if err != nil {
		return fmt.Errorf("failed to verify audit log: %w", err)
	}

	// Log the verification
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		status := "PASS"
		if !result.Valid {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "audit",
			"status": status,
		})
	}

	if verifyArgs.JSON {
		return outputVerifyJSON(result)
	}

	fmt.Println()
	fmt.Println(verifyTitleStyle.Render("Audit Log Integrity Check"))
	fmt.Println()
	printIntegrityResult(*result)
	fmt.Println()

	return nil
}

// =============================================================================
// VERIFY CHECKSUM
// =============================================================================

// handleVerifyChecksum verifies all tracked file checksums.
func handleVerifyChecksum(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	results, err := im.VerifyAll()
	if err != nil {
		return fmt.Errorf("failed to verify checksums: %w", err)
	}

	// Calculate overall status
	allPassed := true
	for _, r := range results {
		if !r.Valid {
			allPassed = false
			break
		}
	}

	// Log the verification
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		status := "PASS"
		if !allPassed {
			status = "FAIL"
		}
		logger.LogEvent("CLI", "INTEGRITY_CHECK", map[string]string{
			"scope":  "checksum",
			"status": status,
			"count":  fmt.Sprintf("%d", len(results)),
		})
	}

	if verifyArgs.JSON {
		return outputVerifyJSON(map[string]interface{}{
			"all_passed": allPassed,
			"results":    results,
			"count":      len(results),
		})
	}

	fmt.Println()
	fmt.Println(verifyTitleStyle.Render("Checksum Verification"))
	fmt.Println()

	if len(results) == 0 {
		fmt.Println(verifyDimStyle.Render("  No files tracked in baseline"))
		fmt.Println()
		fmt.Println("Use 'rigrun verify update' to create a baseline")
	} else {
		for _, r := range results {
			printIntegrityResult(r)
		}
		fmt.Println()
		if allPassed {
			fmt.Println(verifyGreenStyle.Render(fmt.Sprintf("All %d checksums verified", len(results))))
		} else {
			fmt.Println(verifyRedStyle.Render("Some checksums failed verification"))
		}
	}
	fmt.Println()

	return nil
}

// =============================================================================
// VERIFY UPDATE (BASELINE)
// =============================================================================

// handleVerifyUpdate updates the baseline checksums.
func handleVerifyUpdate(verifyArgs VerifyArgs) error {
	im := security.GlobalIntegrityManager()

	// Update baseline
	if err := im.UpdateBaseline(); err != nil {
		return fmt.Errorf("failed to update baseline: %w", err)
	}

	// Save
	if err := im.Save(); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	// Get baseline info
	baseline := im.GetBaseline()

	// Log the update
	logger := security.GlobalAuditLogger()
	if logger != nil && logger.IsEnabled() {
		logger.LogEvent("CLI", "INTEGRITY_BASELINE_UPDATE", map[string]string{
			"files_count": fmt.Sprintf("%d", len(baseline.Records)),
		})
	}

	if verifyArgs.JSON {
		return outputVerifyJSON(map[string]interface{}{
			"updated":      true,
			"checksum_file": im.GetChecksumFile(),
			"files_count":  len(baseline.Records),
			"updated_at":   baseline.UpdatedAt,
		})
	}

	fmt.Println()
	fmt.Println(verifyGreenStyle.Render("Baseline updated successfully"))
	fmt.Println()
	fmt.Printf("  %s%s\n", verifyLabelStyle.Render("Checksum file:"), verifyValueStyle.Render(im.GetChecksumFile()))
	fmt.Printf("  %s%d\n", verifyLabelStyle.Render("Files tracked:"), len(baseline.Records))
	fmt.Printf("  %s%s\n", verifyLabelStyle.Render("Updated at:"), verifyValueStyle.Render(baseline.UpdatedAt.Format(time.RFC3339)))
	fmt.Println()

	// List tracked files
	if len(baseline.Records) > 0 {
		fmt.Println(verifySectionStyle.Render("Tracked Files:"))
		for path, record := range baseline.Records {
			shortPath := path
			if len(shortPath) > 50 {
				shortPath = "..." + shortPath[len(shortPath)-47:]
			}
			fmt.Printf("  %s %s\n", verifyDimStyle.Render(record.Checksum[:8]+"..."), shortPath)
		}
		fmt.Println()
	}

	return nil
}
