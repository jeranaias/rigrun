// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// maintenance_cmd.go - CLI commands for NIST 800-53 MA-4 (Nonlocal Maintenance)
// and MA-5 (Maintenance Personnel) compliance.
//
// CLI: Comprehensive help and examples for all commands
//
// Provides maintenance mode management, personnel authorization, and session
// tracking for DoD IL5 compliance.
//
// Command: maintenance [subcommand]
// Short:   Maintenance mode management (IL5 MA-4, MA-5)
// Aliases: maint
//
// Subcommands:
//   status (default)    Show maintenance mode status
//   enable              Enable maintenance mode
//   disable             Disable maintenance mode
//   personnel           List authorized personnel
//   authorize <user>    Authorize maintenance personnel
//   revoke <user>       Revoke authorization
//   history             Show maintenance session history
//
// Examples:
//   rigrun maintenance                      Show status (default)
//   rigrun maintenance status               Show maintenance status
//   rigrun maintenance status --json        Status in JSON format
//   rigrun maintenance enable               Enter maintenance mode
//   rigrun maintenance disable              Exit maintenance mode
//   rigrun maintenance personnel            List authorized personnel
//   rigrun maintenance authorize admin      Authorize user
//   rigrun maintenance revoke user1         Revoke authorization
//   rigrun maintenance history              Show session history
//
// MA-4/MA-5 Compliance:
//   - Maintenance sessions are fully logged
//   - Personnel must be pre-authorized
//   - All maintenance activities are audited
//   - Session tracking with timeout enforcement
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// MAINTENANCE COMMAND STYLES
// =============================================================================

var (
	// Maintenance title style
	maintenanceTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Maintenance section style
	maintenanceSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	// Maintenance label style
	maintenanceLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(16)

	// Maintenance value style
	maintenanceValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	// Status styles
	maintenanceActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true) // Green

	maintenanceInactiveStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("245")) // Gray

	maintenanceWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Bold(true) // Yellow

	maintenanceErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true) // Red

	// Separator style
	maintenanceSeparatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("240"))
)

// =============================================================================
// MAINTENANCE ARGUMENTS
// =============================================================================

// MaintenanceArgs holds parsed maintenance command arguments.
type MaintenanceArgs struct {
	Subcommand   string
	SessionID    string
	OperatorID   string
	PersonnelID  string
	Name         string
	Role         string
	Type         string
	Reason       string
	Action       string
	Description  string
	Target       string
	SupervisorID string
	JSON         bool
	Limit        int
}

// parseMaintenanceArgs parses maintenance command specific arguments.
func parseMaintenanceArgs(args *Args) MaintenanceArgs {
	maintenanceArgs := MaintenanceArgs{
		Limit: 50,
		Type:  "routine", // Default maintenance type
	}

	if len(args.Raw) > 0 {
		maintenanceArgs.Subcommand = args.Raw[0]
	}

	remaining := args.Raw[1:]
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--operator", "-o":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.OperatorID = remaining[i]
			}
		case "--session", "-s":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.SessionID = remaining[i]
			}
		case "--id":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.PersonnelID = remaining[i]
			}
		case "--name", "-n":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Name = remaining[i]
			}
		case "--role", "-r":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Role = strings.ToLower(remaining[i])
			}
		case "--type", "-t":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Type = strings.ToLower(remaining[i])
			}
		case "--reason":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Reason = remaining[i]
			}
		case "--action", "-a":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Action = remaining[i]
			}
		case "--description", "-d":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Description = remaining[i]
			}
		case "--target":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.Target = remaining[i]
			}
		case "--supervisor":
			if i+1 < len(remaining) {
				i++
				maintenanceArgs.SupervisorID = remaining[i]
			}
		case "--limit", "-l":
			if i+1 < len(remaining) {
				i++
				if n, err := strconv.Atoi(remaining[i]); err == nil && n > 0 {
					maintenanceArgs.Limit = n
				}
			}
		case "--json":
			maintenanceArgs.JSON = true
		default:
			// First non-flag argument after subcommand
			if !strings.HasPrefix(arg, "-") {
				if maintenanceArgs.Subcommand == "start" && maintenanceArgs.Reason == "" {
					maintenanceArgs.Reason = arg
				} else if maintenanceArgs.Subcommand == "log" && maintenanceArgs.Action == "" {
					maintenanceArgs.Action = arg
				} else if maintenanceArgs.Subcommand == "add-personnel" && maintenanceArgs.PersonnelID == "" {
					maintenanceArgs.PersonnelID = arg
				} else if maintenanceArgs.Subcommand == "remove-personnel" && maintenanceArgs.PersonnelID == "" {
					maintenanceArgs.PersonnelID = arg
				}
			}
		}
	}

	// Inherit global JSON flag
	if args.JSON {
		maintenanceArgs.JSON = true
	}

	return maintenanceArgs
}

// =============================================================================
// HANDLE MAINTENANCE COMMAND
// =============================================================================

// HandleMaintenance handles the "maintenance" command.
func HandleMaintenance(args Args) error {
	maintenanceArgs := parseMaintenanceArgs(&args)
	manager := security.GlobalMaintenanceManager()

	// Check for auto-expiration before any command
	manager.CheckSessionExpiration()

	switch maintenanceArgs.Subcommand {
	case "start":
		return handleMaintenanceStart(manager, maintenanceArgs)
	case "end", "stop":
		return handleMaintenanceEnd(manager, maintenanceArgs)
	case "status":
		return handleMaintenanceStatus(manager, maintenanceArgs)
	case "history":
		return handleMaintenanceHistory(manager, maintenanceArgs)
	case "log":
		return handleMaintenanceLog(manager, maintenanceArgs)
	case "personnel", "list-personnel":
		return handleMaintenancePersonnel(manager, maintenanceArgs)
	case "add-personnel":
		return handleMaintenanceAddPersonnel(manager, maintenanceArgs)
	case "remove-personnel":
		return handleMaintenanceRemovePersonnel(manager, maintenanceArgs)
	case "help", "":
		printMaintenanceHelp()
		return nil
	default:
		return fmt.Errorf("unknown maintenance subcommand: %s (try 'maintenance help')", maintenanceArgs.Subcommand)
	}
}

// =============================================================================
// START MAINTENANCE MODE
// =============================================================================

func handleMaintenanceStart(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	// Validate required arguments
	if args.OperatorID == "" {
		return fmt.Errorf("operator ID is required (use --operator)")
	}
	if args.Reason == "" {
		return fmt.Errorf("reason is required")
	}

	// Start maintenance mode
	session, err := manager.StartMaintenanceMode(args.OperatorID, args.Type, args.Reason)
	if err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"success":    true,
			"session_id": session.ID,
			"operator":   session.OperatorID,
			"type":       session.Type,
			"start_time": session.StartTime,
			"reason":     session.Reason,
		})
	}

	// Pretty output
	fmt.Println(maintenanceTitleStyle.Render("Maintenance Mode Started"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("Session ID:") + " " + maintenanceActiveStyle.Render(session.ID))
	fmt.Println(maintenanceLabelStyle.Render("Operator:") + " " + maintenanceValueStyle.Render(session.OperatorID))
	fmt.Println(maintenanceLabelStyle.Render("Type:") + " " + maintenanceValueStyle.Render(session.Type))
	fmt.Println(maintenanceLabelStyle.Render("Start Time:") + " " + maintenanceValueStyle.Render(session.StartTime.Format("2006-01-02 15:04:05")))
	fmt.Println(maintenanceLabelStyle.Render("Reason:") + " " + maintenanceValueStyle.Render(session.Reason))
	fmt.Println()
	fmt.Println(maintenanceWarningStyle.Render("[!!] Maintenance mode will auto-expire after 4 hours"))
	fmt.Println()
	fmt.Println("Use 'maintenance log' to record actions during this session.")
	fmt.Println("Use 'maintenance end' when maintenance is complete.")

	return nil
}

// =============================================================================
// END MAINTENANCE MODE
// =============================================================================

func handleMaintenanceEnd(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	// Get current session
	session := manager.GetCurrentSession()
	if session == nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   "no active maintenance session",
			})
		}
		return fmt.Errorf("no active maintenance session")
	}

	// End maintenance mode
	if err := manager.EndMaintenanceMode(session.ID); err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	duration := time.Since(session.StartTime)

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"success":      true,
			"session_id":   session.ID,
			"operator":     session.OperatorID,
			"duration_min": duration.Minutes(),
			"actions":      len(session.Actions),
		})
	}

	// Pretty output
	fmt.Println(maintenanceTitleStyle.Render("Maintenance Mode Ended"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("Session ID:") + " " + maintenanceValueStyle.Render(session.ID))
	fmt.Println(maintenanceLabelStyle.Render("Operator:") + " " + maintenanceValueStyle.Render(session.OperatorID))
	fmt.Println(maintenanceLabelStyle.Render("Duration:") + " " + maintenanceValueStyle.Render(fmt.Sprintf("%.2f minutes", duration.Minutes())))
	fmt.Println(maintenanceLabelStyle.Render("Actions:") + " " + maintenanceValueStyle.Render(fmt.Sprintf("%d", len(session.Actions))))
	fmt.Println()
	fmt.Println("Maintenance session has been logged to audit trail.")

	return nil
}

// =============================================================================
// MAINTENANCE STATUS
// =============================================================================

func handleMaintenanceStatus(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	session := manager.GetCurrentSession()

	if args.JSON {
		if session == nil {
			return outputJSON(map[string]interface{}{
				"active":  false,
				"session": nil,
			})
		}

		return outputJSON(map[string]interface{}{
			"active":       true,
			"session_id":   session.ID,
			"operator":     session.OperatorID,
			"type":         session.Type,
			"start_time":   session.StartTime,
			"elapsed_min":  time.Since(session.StartTime).Minutes(),
			"actions":      len(session.Actions),
			"reason":       session.Reason,
		})
	}

	// Pretty output
	fmt.Println(maintenanceTitleStyle.Render("Maintenance Mode Status"))
	fmt.Println()

	if session == nil {
		fmt.Println(maintenanceInactiveStyle.Render("Status: INACTIVE"))
		fmt.Println()
		fmt.Println("No active maintenance session.")
		fmt.Println("Use 'maintenance start' to enter maintenance mode.")
		return nil
	}

	elapsed := time.Since(session.StartTime)
	remaining := security.MaxMaintenanceSessionDuration - elapsed

	fmt.Println(maintenanceActiveStyle.Render("Status: ACTIVE"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("Session ID:") + " " + maintenanceValueStyle.Render(session.ID))
	fmt.Println(maintenanceLabelStyle.Render("Operator:") + " " + maintenanceValueStyle.Render(session.OperatorID))
	fmt.Println(maintenanceLabelStyle.Render("Type:") + " " + maintenanceValueStyle.Render(session.Type))
	fmt.Println(maintenanceLabelStyle.Render("Start Time:") + " " + maintenanceValueStyle.Render(session.StartTime.Format("2006-01-02 15:04:05")))
	fmt.Println(maintenanceLabelStyle.Render("Elapsed:") + " " + maintenanceValueStyle.Render(fmt.Sprintf("%.2f minutes", elapsed.Minutes())))
	fmt.Println(maintenanceLabelStyle.Render("Remaining:") + " " + formatRemaining(remaining))
	fmt.Println(maintenanceLabelStyle.Render("Actions:") + " " + maintenanceValueStyle.Render(fmt.Sprintf("%d", len(session.Actions))))
	fmt.Println(maintenanceLabelStyle.Render("Reason:") + " " + maintenanceValueStyle.Render(session.Reason))
	fmt.Println()

	if remaining < 30*time.Minute {
		fmt.Println(maintenanceWarningStyle.Render("[!!] Session will auto-expire in less than 30 minutes"))
		fmt.Println()
	}

	return nil
}

// =============================================================================
// MAINTENANCE HISTORY
// =============================================================================

func handleMaintenanceHistory(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	sessions := manager.GetMaintenanceHistory(args.Limit)

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"sessions": sessions,
			"count":    len(sessions),
		})
	}

	// Pretty output
	fmt.Println(maintenanceTitleStyle.Render("Maintenance Session History"))
	fmt.Println()

	if len(sessions) == 0 {
		fmt.Println("No maintenance sessions recorded.")
		return nil
	}

	for i, session := range sessions {
		if i > 0 {
			fmt.Println(maintenanceSeparatorStyle.Render(strings.Repeat("-", 70)))
		}

		duration := "unknown"
		if session.EndTime != nil {
			d := session.EndTime.Sub(session.StartTime)
			duration = fmt.Sprintf("%.2f minutes", d.Minutes())
		}

		status := "completed"
		if session.AutoExpired {
			status = maintenanceWarningStyle.Render("auto-expired")
		} else if session.EndTime == nil {
			status = maintenanceActiveStyle.Render("active")
		}

		fmt.Println(maintenanceLabelStyle.Render("Session ID:") + " " + maintenanceValueStyle.Render(session.ID))
		fmt.Println(maintenanceLabelStyle.Render("Operator:") + " " + maintenanceValueStyle.Render(session.OperatorID))
		fmt.Println(maintenanceLabelStyle.Render("Type:") + " " + maintenanceValueStyle.Render(session.Type))
		fmt.Println(maintenanceLabelStyle.Render("Start:") + " " + maintenanceValueStyle.Render(session.StartTime.Format("2006-01-02 15:04:05")))
		if session.EndTime != nil {
			fmt.Println(maintenanceLabelStyle.Render("End:") + " " + maintenanceValueStyle.Render(session.EndTime.Format("2006-01-02 15:04:05")))
		}
		fmt.Println(maintenanceLabelStyle.Render("Duration:") + " " + maintenanceValueStyle.Render(duration))
		fmt.Println(maintenanceLabelStyle.Render("Status:") + " " + status)
		fmt.Println(maintenanceLabelStyle.Render("Actions:") + " " + maintenanceValueStyle.Render(fmt.Sprintf("%d", len(session.Actions))))
		fmt.Println(maintenanceLabelStyle.Render("Reason:") + " " + maintenanceValueStyle.Render(session.Reason))
		fmt.Println()
	}

	return nil
}

// =============================================================================
// LOG MAINTENANCE ACTION
// =============================================================================

func handleMaintenanceLog(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	// Check if in maintenance mode
	if !manager.IsMaintenanceMode() {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   "not in maintenance mode",
			})
		}
		return fmt.Errorf("not in maintenance mode (use 'maintenance start' first)")
	}

	// Validate required arguments
	if args.Action == "" {
		return fmt.Errorf("action is required")
	}
	if args.Description == "" {
		return fmt.Errorf("description is required (use --description)")
	}

	// Check if action requires supervisor approval
	if err := manager.RequireApproval(args.Action, args.SupervisorID); err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	// Log the action
	if err := manager.LogMaintenanceAction(args.Action, args.Description, args.Target, true, ""); err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"action":  args.Action,
			"target":  args.Target,
		})
	}

	// Pretty output
	fmt.Println(maintenanceActiveStyle.Render("[OK] Maintenance action logged"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("Action:") + " " + maintenanceValueStyle.Render(args.Action))
	fmt.Println(maintenanceLabelStyle.Render("Description:") + " " + maintenanceValueStyle.Render(args.Description))
	if args.Target != "" {
		fmt.Println(maintenanceLabelStyle.Render("Target:") + " " + maintenanceValueStyle.Render(args.Target))
	}
	if args.SupervisorID != "" {
		fmt.Println(maintenanceLabelStyle.Render("Approved By:") + " " + maintenanceValueStyle.Render(args.SupervisorID))
	}

	return nil
}

// =============================================================================
// PERSONNEL MANAGEMENT
// =============================================================================

func handleMaintenancePersonnel(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	personnel := manager.GetAuthorizedPersonnel()

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"personnel": personnel,
			"count":     len(personnel),
		})
	}

	// Pretty output
	fmt.Println(maintenanceTitleStyle.Render("Authorized Maintenance Personnel"))
	fmt.Println()

	if len(personnel) == 0 {
		fmt.Println("No authorized maintenance personnel.")
		fmt.Println("Use 'maintenance add-personnel' to authorize personnel.")
		return nil
	}

	for i, p := range personnel {
		if i > 0 {
			fmt.Println(maintenanceSeparatorStyle.Render(strings.Repeat("-", 70)))
		}

		roleStyle := maintenanceValueStyle
		if p.Role == "admin" {
			roleStyle = maintenanceActiveStyle
		} else if p.Role == "supervisor" {
			roleStyle = maintenanceWarningStyle
		}

		lastActivity := "never"
		if p.LastActivity != nil {
			lastActivity = p.LastActivity.Format("2006-01-02 15:04:05")
		}

		fmt.Println(maintenanceLabelStyle.Render("ID:") + " " + maintenanceValueStyle.Render(p.ID))
		fmt.Println(maintenanceLabelStyle.Render("Name:") + " " + maintenanceValueStyle.Render(p.Name))
		fmt.Println(maintenanceLabelStyle.Render("Role:") + " " + roleStyle.Render(p.Role))
		fmt.Println(maintenanceLabelStyle.Render("Added:") + " " + maintenanceValueStyle.Render(p.AddedAt.Format("2006-01-02 15:04:05")))
		fmt.Println(maintenanceLabelStyle.Render("Added By:") + " " + maintenanceValueStyle.Render(p.AddedBy))
		fmt.Println(maintenanceLabelStyle.Render("Last Activity:") + " " + maintenanceValueStyle.Render(lastActivity))
		fmt.Println()
	}

	return nil
}

func handleMaintenanceAddPersonnel(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	// Validate required arguments
	if args.PersonnelID == "" {
		return fmt.Errorf("personnel ID is required")
	}
	if args.Name == "" {
		return fmt.Errorf("name is required (use --name)")
	}
	if args.Role == "" {
		return fmt.Errorf("role is required (use --role: technician, supervisor, or admin)")
	}

	// Default added by to system if not specified
	addedBy := args.OperatorID
	if addedBy == "" {
		addedBy = "system"
	}

	// Add personnel
	if err := manager.AddPersonnel(args.PersonnelID, args.Name, args.Role, addedBy); err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"id":      args.PersonnelID,
			"name":    args.Name,
			"role":    args.Role,
		})
	}

	// Pretty output
	fmt.Println(maintenanceActiveStyle.Render("[OK] Personnel authorized"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("ID:") + " " + maintenanceValueStyle.Render(args.PersonnelID))
	fmt.Println(maintenanceLabelStyle.Render("Name:") + " " + maintenanceValueStyle.Render(args.Name))
	fmt.Println(maintenanceLabelStyle.Render("Role:") + " " + maintenanceValueStyle.Render(args.Role))
	fmt.Println()
	fmt.Println("This person can now perform maintenance on the system.")

	return nil
}

func handleMaintenanceRemovePersonnel(manager *security.MaintenanceManager, args MaintenanceArgs) error {
	// Validate required arguments
	if args.PersonnelID == "" {
		return fmt.Errorf("personnel ID is required")
	}

	// Default removed by to system if not specified
	removedBy := args.OperatorID
	if removedBy == "" {
		removedBy = "system"
	}

	// Remove personnel
	if err := manager.RemovePersonnel(args.PersonnelID, removedBy); err != nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return err
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"id":      args.PersonnelID,
		})
	}

	// Pretty output
	fmt.Println(maintenanceActiveStyle.Render("[OK] Personnel authorization removed"))
	fmt.Println()
	fmt.Println(maintenanceLabelStyle.Render("ID:") + " " + maintenanceValueStyle.Render(args.PersonnelID))
	fmt.Println()
	fmt.Println("This person can no longer perform maintenance on the system.")

	return nil
}

// =============================================================================
// HELP
// =============================================================================

func printMaintenanceHelp() {
	help := `NIST 800-53 MA-4/MA-5: Maintenance Management

Usage:
  rigrun maintenance <subcommand> [options]

Subcommands:
  start [reason]              Enter maintenance mode
    --operator ID             Operator performing maintenance (required)
    --type TYPE               Type: routine, emergency, diagnostic, update (default: routine)

  end                         Exit maintenance mode

  status                      Show current maintenance mode status

  history                     Show maintenance session history
    --limit N                 Limit to N most recent sessions (default: 50)

  log <action>                Log a maintenance action (must be in maintenance mode)
    --description TEXT        Description of action (required)
    --target PATH             Target file/resource
    --supervisor ID           Supervisor approval (required for sensitive actions)

  personnel                   List authorized maintenance personnel

  add-personnel <id>          Authorize maintenance personnel
    --name NAME               Full name (required)
    --role ROLE               Role: technician, supervisor, admin (required)
    --operator ID             Who is authorizing (optional)

  remove-personnel <id>       Remove maintenance authorization
    --operator ID             Who is removing (optional)

Action Types (for 'log' subcommand):
  config_change               Configuration modification
  data_access                 Data file access
  system_modify               System file modification (requires supervisor approval)
  log_access                  Log file access
  user_management             User/personnel management (requires supervisor approval)
  key_management              Cryptographic key operations (requires supervisor approval)

Examples:
  # Start maintenance session
  rigrun maintenance start "Routine system update" --operator john.doe --type routine

  # Check maintenance status
  rigrun maintenance status

  # Log a configuration change
  rigrun maintenance log config_change --description "Updated timeout settings" --target config.toml

  # Log a sensitive action with supervisor approval
  rigrun maintenance log key_management --description "Rotated encryption keys" --supervisor jane.supervisor

  # End maintenance session
  rigrun maintenance end

  # View maintenance history
  rigrun maintenance history --limit 10

  # Authorize maintenance personnel
  rigrun maintenance add-personnel john.doe --name "John Doe" --role technician

  # List authorized personnel
  rigrun maintenance personnel

Compliance:
  MA-4 (Nonlocal Maintenance): All maintenance sessions are tracked, time-limited
                                (4 hours max), and fully audited.
  MA-5 (Maintenance Personnel): Only authorized personnel can perform maintenance.
                                 Sensitive actions require supervisor approval.

All maintenance activities are automatically logged to the audit trail.
`
	fmt.Println(help)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func formatRemaining(d time.Duration) string {
	if d < 0 {
		return maintenanceErrorStyle.Render("EXPIRED")
	}
	if d < 30*time.Minute {
		return maintenanceWarningStyle.Render(fmt.Sprintf("%.2f minutes", d.Minutes()))
	}
	return maintenanceValueStyle.Render(fmt.Sprintf("%.2f minutes", d.Minutes()))
}

