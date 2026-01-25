// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// lockout_cmd.go - CLI commands for AC-7 lockout management in rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 AC-7 (Unsuccessful Logon Attempts) controls
// for DoD IL5 compliance.
//
// Command: lockout [subcommand]
// Short:   Account lockout management (IL5 AC-7)
// Aliases: lock
//
// Subcommands:
//   status (default)    Show lockout status
//   list                List all locked identifiers (alias: ls)
//   reset <id>          Reset lockout for identifier
//   stats               Show lockout statistics
//
// Examples:
//   rigrun lockout                     Show status (default)
//   rigrun lockout status              Show lockout status
//   rigrun lockout status --json       Status in JSON format
//   rigrun lockout list                List locked identifiers
//   rigrun lockout reset user@example  Reset specific lockout
//   rigrun lockout stats               Show statistics
//
// AC-7 Lockout Policy:
//   - Max attempts: 3 consecutive failures
//   - Lockout duration: 15 minutes
//   - Delay increases with each failure
//   - All lockout events logged to audit
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
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// LOCKOUT COMMAND STYLES
// =============================================================================

var (
	lockoutTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	lockoutSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	lockoutLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(20)

	lockoutValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	lockoutGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	lockoutRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	lockoutYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	lockoutDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	lockoutSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	lockoutErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)
)

// =============================================================================
// LOCKOUT ARGUMENTS
// =============================================================================

// LockoutArgs holds parsed lockout command arguments.
type LockoutArgs struct {
	Subcommand string
	Identifier string
	Confirm    bool
	JSON       bool
}

// parseLockoutArgs parses lockout command specific arguments.
func parseLockoutArgs(args *Args, remaining []string) LockoutArgs {
	lockoutArgs := LockoutArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		lockoutArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--confirm", "-y":
			lockoutArgs.Confirm = true
		case "--json":
			lockoutArgs.JSON = true
		default:
			// First non-flag argument is the identifier
			if !strings.HasPrefix(arg, "-") && lockoutArgs.Identifier == "" {
				lockoutArgs.Identifier = arg
			}
		}
	}

	return lockoutArgs
}

// =============================================================================
// HANDLE LOCKOUT
// =============================================================================

// HandleLockout handles the "lockout" command with various subcommands.
// Implements AC-7 management commands.
func HandleLockout(args Args) error {
	lockoutArgs := parseLockoutArgs(&args, args.Raw)

	switch lockoutArgs.Subcommand {
	case "", "status":
		return handleLockoutStatus(lockoutArgs)
	case "list":
		return handleLockoutList(lockoutArgs)
	case "reset":
		return handleLockoutReset(lockoutArgs)
	case "unlock":
		return handleLockoutUnlock(lockoutArgs)
	case "stats":
		return handleLockoutStats(lockoutArgs)
	case "clear":
		return handleLockoutClear(lockoutArgs)
	default:
		return fmt.Errorf("unknown lockout subcommand: %s\n\nUsage:\n"+
			"  rigrun lockout status           Show lockout configuration\n"+
			"  rigrun lockout list             List locked identifiers\n"+
			"  rigrun lockout reset <id>       Reset lockout counter for identifier\n"+
			"  rigrun lockout unlock <id>      Manually unlock an identifier\n"+
			"  rigrun lockout stats            Show lockout statistics\n"+
			"  rigrun lockout clear --confirm  Clear all lockout records", lockoutArgs.Subcommand)
	}
}

// =============================================================================
// LOCKOUT STATUS
// =============================================================================

// LockoutStatusOutput is the JSON output format for lockout status.
type LockoutStatusOutput struct {
	Enabled         bool          `json:"enabled"`
	MaxAttempts     int           `json:"max_attempts"`
	LockoutDuration string        `json:"lockout_duration"`
	LockedCount     int           `json:"locked_count"`
	TotalTracked    int           `json:"total_tracked"`
	AC7Compliant    bool          `json:"ac7_compliant"`
	Message         string        `json:"message,omitempty"`
}

// handleLockoutStatus shows the current lockout configuration and status.
func handleLockoutStatus(args LockoutArgs) error {
	cfg := config.Global()
	manager := security.GlobalLockoutManager()

	// Get current stats
	stats := manager.GetStats()

	// Determine AC-7 compliance
	ac7Compliant := stats.MaxAttempts <= 3 && stats.LockoutDuration >= 15*time.Minute

	output := LockoutStatusOutput{
		Enabled:         stats.Enabled,
		MaxAttempts:     stats.MaxAttempts,
		LockoutDuration: stats.LockoutDuration.String(),
		LockedCount:     stats.CurrentlyLocked,
		TotalTracked:    stats.TotalTracked,
		AC7Compliant:    ac7Compliant,
	}

	if args.JSON {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Display human-readable status
	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(lockoutTitleStyle.Render("AC-7 Lockout Status"))
	fmt.Println(lockoutDimStyle.Render(separator))
	fmt.Println()

	// Configuration section
	fmt.Println(lockoutSectionStyle.Render("Configuration"))

	enabledStr := lockoutRedStyle.Render("DISABLED")
	if stats.Enabled {
		enabledStr = lockoutGreenStyle.Render("ENABLED")
	}
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Enabled:"), enabledStr)
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Max Attempts:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.MaxAttempts)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Lockout Duration:"), lockoutValueStyle.Render(stats.LockoutDuration.String()))
	fmt.Println()

	// Current state section
	fmt.Println(lockoutSectionStyle.Render("Current State"))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Currently Locked:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.CurrentlyLocked)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Total Tracked:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.TotalTracked)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Total Lockouts:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.TotalLockouts)))
	fmt.Println()

	// Compliance section
	fmt.Println(lockoutSectionStyle.Render("NIST 800-53 AC-7 Compliance"))
	complianceStr := lockoutRedStyle.Render("NON-COMPLIANT")
	if ac7Compliant {
		complianceStr = lockoutGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Status:"), complianceStr)

	if !ac7Compliant {
		fmt.Println()
		fmt.Println(lockoutYellowStyle.Render("  AC-7 requires: max 3 attempts, 15+ minute lockout"))
		fmt.Println()
		fmt.Println("  To configure, update config.toml:")
		fmt.Printf("    security.max_login_attempts = %d\n", cfg.Security.MaxLoginAttempts)
		fmt.Printf("    security.lockout_duration_minutes = %d\n", cfg.Security.LockoutDurationMinutes)
	}

	fmt.Println()
	return nil
}

// =============================================================================
// LOCKOUT LIST
// =============================================================================

// handleLockoutList lists all currently locked identifiers.
func handleLockoutList(args LockoutArgs) error {
	manager := security.GlobalLockoutManager()
	locked := manager.ListLocked()

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"locked_identifiers": locked,
			"count":              len(locked),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(lockoutTitleStyle.Render("Locked Identifiers"))
	fmt.Println(lockoutDimStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	if len(locked) == 0 {
		fmt.Println(lockoutGreenStyle.Render("  No identifiers are currently locked."))
		fmt.Println()
		return nil
	}

	// Table header
	fmt.Printf("  %-20s %-20s %-15s\n", "Identifier", "Locked Until", "Time Remaining")
	fmt.Println(lockoutDimStyle.Render("  " + strings.Repeat("-", 55)))

	for _, entry := range locked {
		remaining := formatDurationShort(entry.TimeRemaining)
		untilStr := entry.LockedUntil.Format("15:04:05")

		fmt.Printf("  %-20s %-20s %s\n",
			lockoutRedStyle.Render(entry.Identifier),
			untilStr,
			lockoutYellowStyle.Render(remaining))
	}

	fmt.Println()
	fmt.Printf("  Total: %d locked identifier(s)\n", len(locked))
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    rigrun lockout unlock <identifier>  Manually unlock")
	fmt.Println("    rigrun lockout reset <identifier>   Reset attempt counter")
	fmt.Println()

	return nil
}

// =============================================================================
// LOCKOUT RESET
// =============================================================================

// handleLockoutReset resets the lockout counter for an identifier.
func handleLockoutReset(args LockoutArgs) error {
	if args.Identifier == "" {
		return fmt.Errorf("identifier required\nUsage: rigrun lockout reset <identifier>")
	}

	manager := security.GlobalLockoutManager()
	manager.Reset(args.Identifier)

	// Log the action
	security.AuditLogEvent("CLI", "LOCKOUT_RESET", map[string]string{
		"identifier": args.Identifier,
	})

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"reset":      true,
			"identifier": args.Identifier,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Lockout counter reset for: %s\n", lockoutSuccessStyle.Render("[OK]"), args.Identifier)
	fmt.Println()

	return nil
}

// =============================================================================
// LOCKOUT UNLOCK
// =============================================================================

// handleLockoutUnlock manually unlocks an identifier.
func handleLockoutUnlock(args LockoutArgs) error {
	if args.Identifier == "" {
		return fmt.Errorf("identifier required\nUsage: rigrun lockout unlock <identifier>")
	}

	manager := security.GlobalLockoutManager()
	err := manager.Unlock(args.Identifier)
	if err != nil {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"unlocked": false,
				"error":    err.Error(),
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		return fmt.Errorf("failed to unlock: %w", err)
	}

	// Log the action
	security.AuditLogEvent("CLI", "LOCKOUT_UNLOCK", map[string]string{
		"identifier": args.Identifier,
		"method":     "manual_cli",
	})

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"unlocked":   true,
			"identifier": args.Identifier,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Unlocked: %s\n", lockoutSuccessStyle.Render("[OK]"), args.Identifier)
	fmt.Println()

	return nil
}

// =============================================================================
// LOCKOUT STATS
// =============================================================================

// handleLockoutStats shows detailed lockout statistics.
func handleLockoutStats(args LockoutArgs) error {
	manager := security.GlobalLockoutManager()
	stats := manager.GetStats()

	if args.JSON {
		data, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(lockoutTitleStyle.Render("AC-7 Lockout Statistics"))
	fmt.Println(lockoutDimStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	fmt.Println(lockoutSectionStyle.Render("Configuration"))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Max Attempts:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.MaxAttempts)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Lockout Duration:"), lockoutValueStyle.Render(stats.LockoutDuration.String()))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Persist Path:"), lockoutDimStyle.Render(stats.PersistPath))
	fmt.Println()

	fmt.Println(lockoutSectionStyle.Render("Statistics"))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Total Tracked:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.TotalTracked)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Currently Locked:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.CurrentlyLocked)))
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Total Lockouts:"), lockoutValueStyle.Render(fmt.Sprintf("%d", stats.TotalLockouts)))
	fmt.Println()

	enabledStr := lockoutRedStyle.Render("DISABLED")
	if stats.Enabled {
		enabledStr = lockoutGreenStyle.Render("ENABLED")
	}
	fmt.Printf("  %s%s\n", lockoutLabelStyle.Render("Status:"), enabledStr)
	fmt.Println()

	return nil
}

// =============================================================================
// LOCKOUT CLEAR
// =============================================================================

// handleLockoutClear clears all lockout records.
func handleLockoutClear(args LockoutArgs) error {
	if !args.Confirm {
		fmt.Println()
		fmt.Println(lockoutYellowStyle.Render("WARNING: This will clear all lockout records."))
		fmt.Println()
		fmt.Println("To proceed, run:")
		fmt.Println("  rigrun lockout clear --confirm")
		fmt.Println()
		return nil
	}

	manager := security.GlobalLockoutManager()
	stats := manager.GetStats()
	count := stats.TotalTracked

	manager.Clear()

	// Log the action
	security.AuditLogEvent("CLI", "LOCKOUT_CLEAR", map[string]string{
		"records_cleared": fmt.Sprintf("%d", count),
	})

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"cleared": true,
			"count":   count,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Cleared %d lockout record(s)\n", lockoutSuccessStyle.Render("[OK]"), count)
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// =============================================================================
// PARSE LOCKOUT ARGS (FOR CLI.GO INTEGRATION)
// =============================================================================

// parseLockoutArgsFromCLI parses lockout command arguments from CLI.
func parseLockoutArgsFromCLI(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}
