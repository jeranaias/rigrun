// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// sanitize_cmd.go - CLI commands for NIST 800-53 IR-9 spillage and data sanitization.
//
// CLI: Comprehensive help and examples for all commands
//
// Provides data sanitization and spillage response capabilities for
// DoD IL5 compliance. Supports secure deletion, cache clearing, and
// spillage scanning.
//
// Command: sanitize [subcommand]
// Short:   Data sanitization and spillage response (IL5 IR-9)
// Aliases: wipe, clean
//
// Subcommands:
//   status (default)    Show sanitization status
//   cache               Securely wipe response cache
//   audit               Securely wipe audit logs
//   sessions            Securely wipe session data
//   all                 Securely wipe all data
//   file <path>         Securely wipe specific file
//   spillage-scan       Scan for potential data spillage
//
// Examples:
//   rigrun sanitize                         Show status (default)
//   rigrun sanitize status                  Show sanitization status
//   rigrun sanitize status --json           Status in JSON format
//   rigrun sanitize cache --confirm         Wipe cache (requires confirm)
//   rigrun sanitize audit --confirm         Wipe audit logs
//   rigrun sanitize sessions --confirm      Wipe sessions
//   rigrun sanitize all --confirm           Wipe all data
//   rigrun sanitize file /path/to/file --confirm  Wipe specific file
//   rigrun sanitize spillage-scan           Scan for spillage
//
// Secure Deletion (DoD 5220.22-M):
//   - 3-pass overwrite with random data
//   - Verification of overwrite success
//   - All sanitization operations logged
//   - Spillage scanning for sensitive patterns
//
// Flags:
//   --confirm           Required for destructive operations
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// SANITIZE COMMAND STYLES
// =============================================================================

var (
	// Sanitize title style
	sanitizeTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Sanitize warning style
	sanitizeWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")). // Yellow
				Bold(true)

	// Sanitize error style
	sanitizeErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")). // Red
				Bold(true)

	// Sanitize success style
	sanitizeSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")). // Green
				Bold(true)

	// Sanitize label style
	sanitizeLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(16)

	// Sanitize value style
	sanitizeValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	// Sanitize separator style
	sanitizeSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// Spillage detection styles
	spillageCriticalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true).
				Background(lipgloss.Color("52"))

	spillageHighStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true)

	spillageWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220"))
)

// =============================================================================
// SANITIZE ARGUMENTS
// =============================================================================

// SanitizeArgs holds parsed sanitize command arguments.
type SanitizeArgs struct {
	Subcommand       string
	Path             string
	Confirm          bool
	JSON             bool
	Cache            bool
	Session          bool
	All              bool
	SpillageResponse bool
	Recursive        bool
}

// parseSanitizeArgs parses sanitize command specific arguments.
func parseSanitizeArgs(args *Args) SanitizeArgs {
	sanitizeArgs := SanitizeArgs{}

	if len(args.Raw) > 0 {
		sanitizeArgs.Subcommand = args.Raw[0]
	}

	remaining := args.Raw[1:]
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--cache":
			sanitizeArgs.Cache = true
		case "--session":
			sanitizeArgs.Session = true
		case "--all":
			sanitizeArgs.All = true
		case "--spillage-response":
			sanitizeArgs.SpillageResponse = true
		case "--confirm", "-y":
			sanitizeArgs.Confirm = true
		case "--json":
			sanitizeArgs.JSON = true
		case "--recursive", "-r":
			sanitizeArgs.Recursive = true
		case "--secure":
			// Next arg should be the path
			if i+1 < len(remaining) {
				i++
				sanitizeArgs.Path = remaining[i]
			}
		default:
			// Non-flag argument is treated as path
			if !strings.HasPrefix(arg, "-") && sanitizeArgs.Path == "" {
				sanitizeArgs.Path = arg
			}
		}
	}

	// Inherit global JSON flag
	if args.JSON {
		sanitizeArgs.JSON = true
	}

	return sanitizeArgs
}

// =============================================================================
// HANDLE DATA COMMAND
// =============================================================================

// HandleData handles the "data" command with sanitize and wipe subcommands.
// Subcommands:
//   - data sanitize --cache: Sanitize cache
//   - data sanitize --session: Sanitize current session
//   - data sanitize --all: Sanitize everything
//   - data sanitize --spillage-response: Full IR-9 spillage response
//   - data wipe --secure <path>: Securely delete file/directory
//   - data scan --spillage [path]: Scan for spillage
func HandleData(args Args) error {
	sanitizeArgs := parseSanitizeArgs(&args)

	switch sanitizeArgs.Subcommand {
	case "sanitize":
		return handleDataSanitize(sanitizeArgs)
	case "wipe":
		return handleDataWipe(sanitizeArgs)
	case "scan":
		return handleDataScan(sanitizeArgs)
	default:
		return fmt.Errorf("unknown data subcommand: %s\n\nUsage:\n"+
			"  rigrun data sanitize --cache             Sanitize cache\n"+
			"  rigrun data sanitize --session          Sanitize current session\n"+
			"  rigrun data sanitize --all              Sanitize everything\n"+
			"  rigrun data sanitize --spillage-response Full IR-9 spillage response\n"+
			"  rigrun data wipe --secure <path>        Securely delete file/directory\n"+
			"  rigrun data scan --spillage [path]      Scan for spillage", sanitizeArgs.Subcommand)
	}
}

// =============================================================================
// DATA SANITIZE
// =============================================================================

// handleDataSanitize handles the sanitize subcommand.
func handleDataSanitize(sanitizeArgs SanitizeArgs) error {
	sanitizer := security.GlobalSanitizer()
	if sanitizer == nil {
		return fmt.Errorf("sanitizer not available")
	}

	// Handle spillage response (most severe)
	if sanitizeArgs.SpillageResponse {
		return handleSpillageResponse(sanitizeArgs)
	}

	// Handle sanitize --all
	if sanitizeArgs.All {
		if !sanitizeArgs.Confirm {
			fmt.Println()
			fmt.Println(sanitizeWarningStyle.Render("WARNING: Full Data Sanitization"))
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println()
			fmt.Println("This will securely delete:")
			fmt.Println("  - All cache data")
			fmt.Println("  - All session data")
			fmt.Println("  - All saved conversations")
			fmt.Println()
			fmt.Println(sanitizeErrorStyle.Render("This action cannot be undone."))
			fmt.Println()
			fmt.Println("To proceed, run:")
			fmt.Println("  rigrun data sanitize --all --confirm")
			fmt.Println()
			return nil
		}

		fmt.Println()
		fmt.Println(sanitizeTitleStyle.Render("Full Data Sanitization"))
		fmt.Println(sanitizeSeparatorStyle.Render(strings.Repeat("=", 40)))
		fmt.Println()

		if err := sanitizer.SanitizeAll(); err != nil {
			fmt.Printf("  %s %s\n", sanitizeErrorStyle.Render("[ERROR]"), err.Error())
			return err
		}

		fmt.Printf("  %s All data sanitized\n", sanitizeSuccessStyle.Render("[OK]"))
		fmt.Println()
		return nil
	}

	// Handle individual sanitize options
	if sanitizeArgs.Cache {
		fmt.Println()
		fmt.Println(sanitizeTitleStyle.Render("Cache Sanitization"))
		fmt.Println()

		if err := sanitizer.SanitizeCache(); err != nil {
			fmt.Printf("  %s %s\n", sanitizeErrorStyle.Render("[ERROR]"), err.Error())
			return err
		}

		fmt.Printf("  %s Cache sanitized\n", sanitizeSuccessStyle.Render("[OK]"))
		fmt.Println()
		return nil
	}

	if sanitizeArgs.Session {
		fmt.Println()
		fmt.Println(sanitizeTitleStyle.Render("Session Sanitization"))
		fmt.Println()

		if err := sanitizer.SanitizeSession(); err != nil {
			fmt.Printf("  %s %s\n", sanitizeErrorStyle.Render("[ERROR]"), err.Error())
			return err
		}

		fmt.Printf("  %s Session data sanitized\n", sanitizeSuccessStyle.Render("[OK]"))
		fmt.Println()
		return nil
	}

	// No specific option - show help
	return fmt.Errorf("no sanitize option specified\n\nUsage:\n"+
		"  rigrun data sanitize --cache             Sanitize cache\n"+
		"  rigrun data sanitize --session          Sanitize current session\n"+
		"  rigrun data sanitize --all              Sanitize everything\n"+
		"  rigrun data sanitize --spillage-response Full IR-9 spillage response")
}

// handleSpillageResponse executes the full IR-9 spillage response procedure.
func handleSpillageResponse(sanitizeArgs SanitizeArgs) error {
	if !sanitizeArgs.Confirm {
		fmt.Println()
		fmt.Println(spillageCriticalStyle.Render(" IR-9 SPILLAGE RESPONSE "))
		fmt.Println()
		fmt.Println(sanitizeWarningStyle.Render("WARNING: This is a NIST 800-53 IR-9 Spillage Response"))
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println()
		fmt.Println("This procedure will:")
		fmt.Println("  1. Clear all caches (may contain classified data)")
		fmt.Println("  2. Clear current session data")
		fmt.Println("  3. Clear all saved conversations")
		fmt.Println("  4. Clear temporary files")
		fmt.Println("  5. Log all actions for audit trail")
		fmt.Println()
		fmt.Println(sanitizeErrorStyle.Render("This action cannot be undone."))
		fmt.Println()
		fmt.Println("Execute only if spillage has been detected.")
		fmt.Println()
		fmt.Println("To proceed, run:")
		fmt.Println("  rigrun data sanitize --spillage-response --confirm")
		fmt.Println()
		return nil
	}

	sanitizer := security.GlobalSanitizer()
	if sanitizer == nil {
		return fmt.Errorf("sanitizer not available")
	}

	fmt.Println()
	fmt.Println(spillageCriticalStyle.Render(" IR-9 SPILLAGE RESPONSE INITIATED "))
	fmt.Println()
	fmt.Printf("  Started: %s\n", time.Now().Format(time.RFC1123))
	fmt.Println()

	// Execute spillage response
	if err := sanitizer.SpillageResponse(); err != nil {
		fmt.Printf("  %s Spillage response completed with errors: %s\n",
			sanitizeWarningStyle.Render("[WARN]"), err.Error())
	} else {
		fmt.Printf("  %s Spillage response completed successfully\n",
			sanitizeSuccessStyle.Render("[OK]"))
	}

	fmt.Println()
	fmt.Printf("  Completed: %s\n", time.Now().Format(time.RFC1123))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Document the spillage incident")
	fmt.Println("  2. Report to your security officer")
	fmt.Println("  3. File an incident report: rigrun incident report --category spillage ...")
	fmt.Println()

	return nil
}

// =============================================================================
// DATA WIPE
// =============================================================================

// handleDataWipe securely deletes a file or directory.
func handleDataWipe(sanitizeArgs SanitizeArgs) error {
	if sanitizeArgs.Path == "" {
		return fmt.Errorf("path required\nUsage: rigrun data wipe --secure <path>")
	}

	// Validate path to prevent path traversal attacks
	validatedPath, err := ValidatePathSecure(sanitizeArgs.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists (using validated path)
	info, err := os.Stat(validatedPath)
	if err != nil {
		return fmt.Errorf("path not found: %s", validatedPath)
	}

	// Update sanitizeArgs.Path to use validated path
	sanitizeArgs.Path = validatedPath

	sanitizer := security.GlobalSanitizer()
	if sanitizer == nil {
		return fmt.Errorf("sanitizer not available")
	}

	// Get absolute path for display
	absPath, _ := filepath.Abs(sanitizeArgs.Path)

	// Confirmation
	if !sanitizeArgs.Confirm {
		fmt.Println()
		fmt.Println(sanitizeWarningStyle.Render("WARNING: Secure Deletion"))
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println()
		fmt.Printf("  Path: %s\n", absPath)

		if info.IsDir() {
			fmt.Printf("  Type: Directory\n")
		} else {
			fmt.Printf("  Type: File\n")
			fmt.Printf("  Size: %s\n", formatBytes(info.Size()))
		}

		fmt.Println()
		fmt.Println("This will perform DoD 5220.22-M secure deletion:")
		fmt.Println("  - 3-pass overwrite (zeros, ones, random)")
		fmt.Println("  - File removal")
		fmt.Println()
		fmt.Println(sanitizeErrorStyle.Render("This action cannot be undone."))
		fmt.Println()
		fmt.Println("To proceed, run:")
		fmt.Printf("  rigrun data wipe --secure \"%s\" --confirm\n", sanitizeArgs.Path)
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println(sanitizeTitleStyle.Render("Secure Deletion"))
	fmt.Println()

	var deleteErr error
	if info.IsDir() {
		deleteErr = sanitizer.SecureDeleteDirectory(sanitizeArgs.Path)
	} else {
		deleteErr = sanitizer.SecureDeleteFile(sanitizeArgs.Path)
	}

	if deleteErr != nil {
		fmt.Printf("  %s %s\n", sanitizeErrorStyle.Render("[ERROR]"), deleteErr.Error())
		return deleteErr
	}

	fmt.Printf("  %s Securely deleted: %s\n", sanitizeSuccessStyle.Render("[OK]"), absPath)
	fmt.Println()

	if sanitizeArgs.JSON {
		output := map[string]interface{}{
			"success": true,
			"path":    absPath,
			"action":  "secure_delete",
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	}

	return nil
}

// =============================================================================
// DATA SCAN (SPILLAGE)
// =============================================================================

// handleDataScan scans for spillage in files.
func handleDataScan(sanitizeArgs SanitizeArgs) error {
	spillageManager := security.GlobalSpillageManager()
	if spillageManager == nil {
		return fmt.Errorf("spillage manager not available")
	}

	// Default to current directory if no path specified
	scanPath := sanitizeArgs.Path
	if scanPath == "" {
		scanPath = "."
	}

	// Validate path to prevent path traversal attacks
	validatedPath, err := ValidateOutputPath(scanPath)
	if err != nil {
		return fmt.Errorf("invalid scan path: %w", err)
	}

	// Use validated path from here on
	absPath := validatedPath
	scanPath = validatedPath

	// Check if path exists
	info, err := os.Stat(scanPath)
	if err != nil {
		return fmt.Errorf("path not found: %s", scanPath)
	}

	fmt.Println()
	fmt.Println(sanitizeTitleStyle.Render("Spillage Scan"))
	fmt.Println(sanitizeSeparatorStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()
	fmt.Printf("  Scanning: %s\n", absPath)
	fmt.Printf("  Started:  %s\n", time.Now().Format("15:04:05"))
	fmt.Println()

	var report *security.SpillageReport

	if info.IsDir() {
		report, err = spillageManager.ScanDirectory(scanPath)
	} else {
		events, scanErr := spillageManager.DetectInFile(scanPath)
		if scanErr != nil {
			return scanErr
		}
		report = &security.SpillageReport{
			ScanTime:     time.Now(),
			Location:     absPath,
			FilesScanned: 1,
			EventsFound:  len(events),
			Events:       events,
		}
	}

	if err != nil {
		return err
	}

	// Output results
	if sanitizeArgs.JSON {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("  Files scanned: %d\n", report.FilesScanned)
	fmt.Printf("  Completed:     %s\n", time.Now().Format("15:04:05"))
	fmt.Println()

	if report.EventsFound == 0 {
		fmt.Printf("  %s No classification markers detected\n", sanitizeSuccessStyle.Render("[OK]"))
		fmt.Println()
		return nil
	}

	// Display detected spillage
	fmt.Println(spillageCriticalStyle.Render(fmt.Sprintf(" SPILLAGE DETECTED: %d MARKERS FOUND ", report.EventsFound)))
	fmt.Println()

	for _, event := range report.Events {
		var style lipgloss.Style
		switch event.Classification {
		case "TOP SECRET", "SCI":
			style = spillageCriticalStyle
		case "SECRET", "NATO":
			style = spillageHighStyle
		default:
			style = spillageWarningStyle
		}

		fmt.Printf("  %s [%s]\n", style.Render(event.Classification), event.PatternName)
		if event.Location != "memory" {
			fmt.Printf("    File: %s (line %d)\n", event.Location, event.LineNumber)
		}
		fmt.Printf("    Match: %s\n", event.MatchedText)
		fmt.Println()
	}

	// Recommendations
	fmt.Println(sanitizeSeparatorStyle.Render(strings.Repeat("-", 60)))
	fmt.Println()
	fmt.Println(sanitizeWarningStyle.Render("Recommended Actions:"))
	fmt.Println("  1. Do not transmit this data over untrusted networks")
	fmt.Println("  2. Report to your security officer")
	fmt.Println("  3. Consider running: rigrun data sanitize --spillage-response")
	fmt.Println("  4. File an incident: rigrun incident report --category spillage ...")
	fmt.Println()

	// Auto-create incident if spillage detected
	if report.IncidentID != "" {
		fmt.Printf("  %s Auto-created incident: %s\n", sanitizeSuccessStyle.Render("[OK]"), report.IncidentID)
		fmt.Println()
	}

	return nil
}
