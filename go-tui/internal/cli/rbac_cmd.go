// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// rbac_cmd.go - CLI commands for RBAC (Role-Based Access Control) management.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 AC-5 (Separation of Duties) and AC-6 (Least Privilege)
// controls for DoD IL5 compliance.
//
// Command: rbac [subcommand]
// Short:   Role-based access control (IL5 AC-5, AC-6)
// Aliases: roles
//
// Subcommands:
//   status (default)    Show current user role and permissions
//   assign <user> <role>  Assign role to user (admin only)
//   revoke <user> <role>  Revoke role from user (admin only)
//   list-roles          List all available roles
//   list-users          List users and their roles
//   check <action>      Check if current user can perform action
//
// Examples:
//   rigrun rbac                           Show status (default)
//   rigrun rbac status                    Show current role/permissions
//   rigrun rbac status --json             Status in JSON format
//   rigrun rbac assign admin@example admin  Assign admin role
//   rigrun rbac revoke user@example admin   Revoke admin role
//   rigrun rbac list-roles                List available roles
//   rigrun rbac list-users                List all users and roles
//   rigrun rbac check cloud_access        Check permission
//
// Available Roles:
//   admin               Full administrative access
//   operator            Operational access (no config changes)
//   auditor             Read-only access to audit logs
//   user                Standard user access
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

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// RBAC COMMAND STYLES
// =============================================================================

var (
	rbacTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	rbacSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	rbacLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(22)

	rbacValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	rbacGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	rbacRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	rbacYellowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // Yellow

	rbacDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	rbacSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	rbacErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

// =============================================================================
// RBAC ARGUMENTS
// =============================================================================

// RBACArgs holds parsed RBAC command arguments.
type RBACArgs struct {
	Subcommand string
	UserID     string
	Role       string
	Permission string
	JSON       bool
}

// parseRBACCmdArgs parses RBAC command specific arguments.
func parseRBACCmdArgs(args *Args, remaining []string) RBACArgs {
	rbacArgs := RBACArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		rbacArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	// Parse additional arguments based on subcommand
	switch rbacArgs.Subcommand {
	case "assign", "revoke":
		if len(remaining) > 0 {
			rbacArgs.UserID = remaining[0]
		}
		if len(remaining) > 1 {
			rbacArgs.Role = remaining[1]
		}
	case "check":
		if len(remaining) > 0 {
			rbacArgs.Permission = strings.Join(remaining, ":")
		}
	}

	return rbacArgs
}

// =============================================================================
// HANDLE RBAC
// =============================================================================

// HandleRBAC handles the "rbac" command with various subcommands.
// Implements AC-5 and AC-6 management commands.
func HandleRBAC(args Args) error {
	rbacArgs := parseRBACCmdArgs(&args, args.Raw)

	switch rbacArgs.Subcommand {
	case "", "status":
		return handleRBACStatus(rbacArgs)
	case "assign":
		return handleRBACAssign(rbacArgs)
	case "revoke":
		return handleRBACRevoke(rbacArgs)
	case "list-roles", "roles":
		return handleRBACListRoles(rbacArgs)
	case "list-users", "users":
		return handleRBACListUsers(rbacArgs)
	case "check":
		return handleRBACCheck(rbacArgs)
	default:
		return fmt.Errorf("unknown RBAC subcommand: %s\n\nUsage:\n"+
			"  rigrun rbac status              Show current user role and permissions\n"+
			"  rigrun rbac assign USER ROLE    Assign role to user (admin only)\n"+
			"  rigrun rbac revoke USER         Revoke user's role (admin only)\n"+
			"  rigrun rbac list-roles          List all available roles\n"+
			"  rigrun rbac list-users          List users and their roles\n"+
			"  rigrun rbac check PERMISSION    Check if current user can perform action", rbacArgs.Subcommand)
	}
}

// =============================================================================
// RBAC STATUS
// =============================================================================

// RBACStatusOutput is the JSON output format for RBAC status.
type RBACStatusOutput struct {
	UserID            string   `json:"user_id"`
	Role              string   `json:"role,omitempty"`
	RoleAssigned      bool     `json:"role_assigned"`
	AssignedBy        string   `json:"assigned_by,omitempty"`
	AssignedAt        string   `json:"assigned_at,omitempty"`
	Permissions       []string `json:"permissions"`
	PermissionCount   int      `json:"permission_count"`
	AC5Compliant      bool     `json:"ac5_compliant"`
	AC6Compliant      bool     `json:"ac6_compliant"`
	SeparationOfDuties bool    `json:"separation_of_duties"`
}

// handleRBACStatus shows the current user's role and permissions.
func handleRBACStatus(args RBACArgs) error {
	rbacMgr := security.GlobalRBACManager()
	cfg := config.Global()

	// Get current user ID (from config or environment)
	userID := getCurrentUserIDFromConfig(cfg)

	// Get role details
	userRole, hasRole := rbacMgr.GetUserRoleDetails(userID)
	permissions := rbacMgr.GetUserPermissions(userID)

	// Determine compliance status
	ac5Compliant := hasRole // AC-5: User has an assigned role
	ac6Compliant := hasRole // AC-6: Least privilege enforced if role assigned

	output := RBACStatusOutput{
		UserID:             userID,
		RoleAssigned:       hasRole,
		Permissions:        permissionsToStrings(permissions),
		PermissionCount:    len(permissions),
		AC5Compliant:       ac5Compliant,
		AC6Compliant:       ac6Compliant,
		SeparationOfDuties: ac5Compliant,
	}

	if hasRole {
		output.Role = string(userRole.Role)
		output.AssignedBy = userRole.AssignedBy
		output.AssignedAt = userRole.AssignedAt.Format(time.RFC3339)
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
	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(rbacTitleStyle.Render("NIST 800-53 AC-5/AC-6 RBAC Status"))
	fmt.Println(rbacDimStyle.Render(separator))
	fmt.Println()

	// User section
	fmt.Println(rbacSectionStyle.Render("User Information"))
	fmt.Printf("  %s%s\n", rbacLabelStyle.Render("User ID:"), rbacValueStyle.Render(userID))

	if hasRole {
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Role:"), rbacGreenStyle.Render(string(userRole.Role)))
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Assigned By:"), rbacValueStyle.Render(userRole.AssignedBy))
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Assigned At:"),
			rbacValueStyle.Render(userRole.AssignedAt.Format("2006-01-02 15:04:05")))
	} else {
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Role:"), rbacRedStyle.Render("NO ROLE ASSIGNED"))
		fmt.Println()
		fmt.Println(rbacYellowStyle.Render("  WARNING: No role assigned. Access is restricted to read-only operations."))
		fmt.Println(rbacDimStyle.Render("  An administrator must assign you a role to grant additional permissions."))
	}
	fmt.Println()

	// Permissions section
	fmt.Println(rbacSectionStyle.Render("Permissions"))
	if len(permissions) > 0 {
		fmt.Printf("  %s%d\n", rbacLabelStyle.Render("Permission Count:"), len(permissions))
		fmt.Println()
		for _, perm := range permissions {
			fmt.Printf("    %s %s\n", rbacGreenStyle.Render("[OK]"), rbacValueStyle.Render(string(perm)))
		}
	} else {
		fmt.Printf("  %s\n", rbacRedStyle.Render("No permissions granted (least privilege enforced)"))
	}
	fmt.Println()

	// Compliance section
	fmt.Println(rbacSectionStyle.Render("NIST 800-53 Compliance"))

	// AC-5: Separation of Duties
	ac5Status := rbacRedStyle.Render("NON-COMPLIANT")
	if ac5Compliant {
		ac5Status = rbacGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", rbacLabelStyle.Render("AC-5 (Sep. of Duties):"), ac5Status)

	// AC-6: Least Privilege
	ac6Status := rbacRedStyle.Render("NON-COMPLIANT")
	if ac6Compliant {
		ac6Status = rbacGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", rbacLabelStyle.Render("AC-6 (Least Privilege):"), ac6Status)

	if !ac5Compliant || !ac6Compliant {
		fmt.Println()
		fmt.Println(rbacYellowStyle.Render("  To achieve compliance:"))
		if !hasRole {
			fmt.Println("    * An administrator must assign you a role")
		}
		fmt.Println("    * Roles follow least privilege principle (AC-6)")
		fmt.Println("    * Separation of duties enforced between roles (AC-5)")
	}

	fmt.Println()
	return nil
}

// =============================================================================
// RBAC ASSIGN
// =============================================================================

// handleRBACAssign assigns a role to a user (admin only).
func handleRBACAssign(args RBACArgs) error {
	if args.UserID == "" {
		return fmt.Errorf("user ID required\n\nUsage: rigrun rbac assign USER ROLE")
	}
	if args.Role == "" {
		return fmt.Errorf("role required\n\nUsage: rigrun rbac assign USER ROLE\n\nAvailable roles: admin, operator, auditor, user")
	}

	rbacMgr := security.GlobalRBACManager()
	cfg := config.Global()

	// Get current user (who is performing the assignment)
	currentUserID := getCurrentUserIDFromConfig(cfg)

	// Convert string to RBACRole
	role := security.RBACRole(args.Role)

	// Attempt to assign the role
	err := rbacMgr.AssignRole(args.UserID, role, currentUserID)
	if err != nil {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Printf("%s Failed to assign role: %s\n", rbacErrorStyle.Render("[ERROR]"), err.Error())
		fmt.Println()
		return nil
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"success":     true,
			"user_id":     args.UserID,
			"role":        args.Role,
			"assigned_by": currentUserID,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Role assigned successfully\n", rbacSuccessStyle.Render("[OK]"))
	fmt.Printf("  User:        %s\n", args.UserID)
	fmt.Printf("  Role:        %s\n", rbacGreenStyle.Render(args.Role))
	fmt.Printf("  Assigned by: %s\n", currentUserID)
	fmt.Println()

	return nil
}

// =============================================================================
// RBAC REVOKE
// =============================================================================

// handleRBACRevoke revokes a user's role (admin only).
func handleRBACRevoke(args RBACArgs) error {
	if args.UserID == "" {
		return fmt.Errorf("user ID required\n\nUsage: rigrun rbac revoke USER")
	}

	rbacMgr := security.GlobalRBACManager()
	cfg := config.Global()

	// Get current user (who is performing the revocation)
	currentUserID := getCurrentUserIDFromConfig(cfg)

	// Attempt to revoke the role
	err := rbacMgr.RevokeRole(args.UserID, currentUserID)
	if err != nil {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Printf("%s Failed to revoke role: %s\n", rbacErrorStyle.Render("[ERROR]"), err.Error())
		fmt.Println()
		return nil
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"success":    true,
			"user_id":    args.UserID,
			"revoked_by": currentUserID,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Role revoked successfully\n", rbacSuccessStyle.Render("[OK]"))
	fmt.Printf("  User:       %s\n", args.UserID)
	fmt.Printf("  Revoked by: %s\n", currentUserID)
	fmt.Println()

	return nil
}

// =============================================================================
// RBAC LIST ROLES
// =============================================================================

// handleRBACListRoles lists all available roles and their permissions.
func handleRBACListRoles(args RBACArgs) error {
	rbacMgr := security.GlobalRBACManager()
	roles := rbacMgr.ListRoles()

	if args.JSON {
		roleDetails := make([]map[string]interface{}, 0, len(roles))
		for _, role := range roles {
			desc := rbacMgr.GetRoleDescription(role)
			permissions := rbacMgr.GetRolePermissions(role)
			roleDetails = append(roleDetails, map[string]interface{}{
				"role":              string(role),
				"name":              desc.Name,
				"description":       desc.Description,
				"permission_count":  len(permissions),
				"permissions":       permissionsToStrings(permissions),
			})
		}
		data, err := json.MarshalIndent(map[string]interface{}{
			"roles": roleDetails,
			"count": len(roles),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Display human-readable role list
	fmt.Println()
	fmt.Println(rbacTitleStyle.Render("Available Roles (AC-5/AC-6)"))
	fmt.Println(rbacDimStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	for _, role := range roles {
		desc := rbacMgr.GetRoleDescription(role)
		permissions := rbacMgr.GetRolePermissions(role)

		fmt.Println(rbacSectionStyle.Render(desc.Name + " (" + string(role) + ")"))
		fmt.Printf("  %s\n", rbacDimStyle.Render(desc.Description))
		fmt.Printf("  %s%d\n", rbacLabelStyle.Render("Permission Count:"), len(permissions))
		fmt.Println()

		// Display key permissions
		fmt.Println(rbacDimStyle.Render("  Key Permissions:"))
		for _, perm := range desc.Permissions {
			fmt.Printf("    %s %s\n", rbacGreenStyle.Render("*"), perm)
		}
		fmt.Println()
	}

	fmt.Println(rbacDimStyle.Render("Use 'rigrun rbac assign USER ROLE' to assign a role (admin only)"))
	fmt.Println()

	return nil
}

// =============================================================================
// RBAC LIST USERS
// =============================================================================

// handleRBACListUsers lists all users and their role assignments.
func handleRBACListUsers(args RBACArgs) error {
	rbacMgr := security.GlobalRBACManager()
	users := rbacMgr.ListUsers()

	if args.JSON {
		userDetails := make([]map[string]interface{}, 0, len(users))
		for _, user := range users {
			permissions := rbacMgr.GetUserPermissions(user.UserID)
			userDetails = append(userDetails, map[string]interface{}{
				"user_id":          user.UserID,
				"role":             string(user.Role),
				"assigned_by":      user.AssignedBy,
				"assigned_at":      user.AssignedAt.Format(time.RFC3339),
				"permission_count": len(permissions),
			})
		}
		data, err := json.MarshalIndent(map[string]interface{}{
			"users": userDetails,
			"count": len(users),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Display human-readable user list
	fmt.Println()
	fmt.Println(rbacTitleStyle.Render("User Role Assignments"))
	fmt.Println(rbacDimStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	if len(users) == 0 {
		fmt.Println(rbacDimStyle.Render("  No users have been assigned roles."))
		fmt.Println()
		fmt.Println(rbacYellowStyle.Render("  Use 'rigrun rbac assign USER ROLE' to assign roles (admin only)"))
		fmt.Println()
		return nil
	}

	// Table header
	fmt.Printf("  %-25s %-12s %-20s %-15s\n", "User ID", "Role", "Assigned By", "Assigned At")
	fmt.Println(rbacDimStyle.Render("  " + strings.Repeat("-", 68)))

	for _, user := range users {
		roleColor := rbacValueStyle
		switch user.Role {
		case security.RBACRoleAdmin:
			roleColor = rbacGreenStyle
		case security.RBACRoleOperator:
			roleColor = lipgloss.NewStyle().Foreground(lipgloss.Color("75")) // Blue
		case security.RBACRoleAuditor:
			roleColor = rbacYellowStyle
		case security.RBACRoleUser:
			roleColor = rbacDimStyle
		}

		fmt.Printf("  %-25s %s %-20s %-15s\n",
			truncate(user.UserID, 25),
			roleColor.Render(padRight(string(user.Role), 12)),
			truncate(user.AssignedBy, 20),
			user.AssignedAt.Format("2006-01-02"))
	}

	fmt.Println()
	fmt.Printf("  Total: %d user(s) with role assignments\n", len(users))
	fmt.Println()

	return nil
}

// =============================================================================
// RBAC CHECK
// =============================================================================

// handleRBACCheck checks if the current user has a specific permission.
func handleRBACCheck(args RBACArgs) error {
	if args.Permission == "" {
		return fmt.Errorf("permission required\n\nUsage: rigrun rbac check PERMISSION\n\n" +
			"Example permissions:\n" +
			"  query:run\n" +
			"  config:write\n" +
			"  audit:view\n" +
			"  user:create")
	}

	rbacMgr := security.GlobalRBACManager()
	cfg := config.Global()

	// Get current user
	userID := getCurrentUserIDFromConfig(cfg)

	// Check permission
	permission := security.Permission(args.Permission)
	hasPermission := rbacMgr.CheckPermission(userID, permission)

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"user_id":        userID,
			"permission":     args.Permission,
			"has_permission": hasPermission,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(rbacTitleStyle.Render("Permission Check"))
	fmt.Println(rbacDimStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	fmt.Printf("  %s%s\n", rbacLabelStyle.Render("User ID:"), rbacValueStyle.Render(userID))
	fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Permission:"), rbacValueStyle.Render(args.Permission))

	if hasPermission {
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Status:"), rbacGreenStyle.Render("GRANTED"))
	} else {
		fmt.Printf("  %s%s\n", rbacLabelStyle.Render("Status:"), rbacRedStyle.Render("DENIED (AC-6)"))
		fmt.Println()
		fmt.Println(rbacYellowStyle.Render("  Access denied: Your role does not have this permission."))
	}

	fmt.Println()
	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getCurrentUserIDFromConfig gets the current user ID from config or generates one.
// This is a special version that takes config parameter, different from the shared helper.
func getCurrentUserIDFromConfig(cfg *config.Config) string {
	// Try to get from environment first
	if envUser := os.Getenv("RIGRUN_USER_ID"); envUser != "" {
		return envUser
	}

	// Try to get from config
	if cfg != nil && cfg.Security.UserID != "" {
		return cfg.Security.UserID
	}

	// Derive from system user
	if username := os.Getenv("USER"); username != "" {
		return username
	}
	if username := os.Getenv("USERNAME"); username != "" {
		return username
	}

	// Fallback
	return "default_user"
}

// permissionsToStrings converts permissions to string slice.
func permissionsToStrings(permissions []security.Permission) []string {
	result := make([]string, len(permissions))
	for i, perm := range permissions {
		result[i] = string(perm)
	}
	return result
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// padRight pads a string to the right with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
