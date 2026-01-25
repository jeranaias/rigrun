// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// backup_cmd.go - Backup management CLI commands for rigrun (NIST 800-53 CP-9/CP-10).
//
// CLI: Comprehensive help and examples for all commands
//
// Command: backup [subcommand]
// Short:   Backup and recovery management (IL5 CP-9, CP-10)
// Aliases: (none)
//
// Subcommands:
//   list (default)      List all available backups (aliases: ls, l)
//   create [type]       Create a new backup
//   restore <id>        Restore from a backup
//   verify <id>         Verify backup integrity
//   delete <id>         Delete a backup
//   schedule [interval] Set backup schedule
//   status              Show backup status and schedule
//
// Backup Types:
//   config              Configuration files only
//   cache               Response cache only
//   audit               Audit logs only
//   full                All data (default)
//
// Examples:
//   rigrun backup                             List backups (default)
//   rigrun backup list                        List all available backups
//   rigrun backup ls                          List (short alias)
//   rigrun backup create                      Create full backup (default)
//   rigrun backup create config               Create config-only backup
//   rigrun backup create cache                Create cache-only backup
//   rigrun backup create audit                Create audit-only backup
//   rigrun backup create full                 Create full backup
//   rigrun backup restore abc123              Restore backup (interactive confirm)
//   rigrun backup restore abc123 --confirm    Restore without prompt
//   rigrun backup verify abc123               Verify backup integrity
//   rigrun backup delete abc123               Delete backup (interactive confirm)
//   rigrun backup delete abc123 --confirm     Delete without prompt
//   rigrun backup schedule 24h                Set daily schedule
//   rigrun backup schedule 7d full            Set weekly full backup
//   rigrun backup schedule 1h config          Hourly config backup
//   rigrun backup status                      Show status and schedule
//   rigrun backup status --json               Status in JSON format
//
// Flags:
//   --confirm           Skip confirmation prompts
//   --json              Output in JSON format
//
// Schedule Intervals:
//   1h, 2h, 6h, 12h, 24h, 7d, 30d
//
// Security Notes (CP-9/CP-10):
//   - Backups are encrypted with AES-256
//   - Integrity verified with SHA-256 checksums
//   - Secure deletion uses DoD 5220.22-M standard
//   - Stored in ~/.rigrun/backups/
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// BACKUP COMMAND HANDLER
// =============================================================================

// HandleBackup handles the "backup" command with various subcommands.
// Subcommands:
//   - backup create [type]: Create a backup (config/cache/audit/full)
//   - backup restore [id]: Restore from backup
//   - backup list: List all backups
//   - backup verify [id]: Verify backup integrity
//   - backup delete [id]: Delete a backup
//   - backup schedule [interval] [type]: Set backup schedule
//   - backup status: Show backup status and schedule
//
// Implements NIST 800-53 CP-9 (System Backup) and CP-10 (System Recovery).
func HandleBackup(args Args) error {
	// Parse additional flags from raw args
	var backupType string
	var backupID string
	var interval string
	confirmFlag := false

	for i, arg := range args.Raw {
		switch arg {
		case "--confirm":
			confirmFlag = true
		default:
			// Capture positional arguments
			if args.Subcommand == "create" && !strings.HasPrefix(arg, "-") && backupType == "" {
				backupType = arg
			} else if (args.Subcommand == "restore" || args.Subcommand == "verify" || args.Subcommand == "delete") && !strings.HasPrefix(arg, "-") && backupID == "" {
				backupID = arg
			} else if args.Subcommand == "schedule" && !strings.HasPrefix(arg, "-") {
				if interval == "" {
					interval = arg
				} else if backupType == "" {
					backupType = arg
				}
			}
		}
		_ = i // Suppress unused variable warning
	}

	switch args.Subcommand {
	case "create":
		return createBackup(backupType, args.JSON)
	case "restore":
		return restoreBackup(backupID, confirmFlag)
	case "", "list":
		return listBackups(args.JSON)
	case "verify":
		return verifyBackup(backupID)
	case "delete":
		return deleteBackup(backupID, confirmFlag)
	case "schedule":
		return scheduleBackup(interval, backupType)
	case "status":
		return showBackupStatus(args.JSON)
	default:
		return fmt.Errorf("unknown backup subcommand: %s", args.Subcommand)
	}
}

// =============================================================================
// CREATE BACKUP
// =============================================================================

// createBackup creates a new backup of the specified type.
func createBackup(backupType string, jsonOutput bool) error {
	// Default to full backup if no type specified
	if backupType == "" {
		backupType = "full"
	}

	// Validate backup type
	var bt security.BackupType
	switch strings.ToLower(backupType) {
	case "config":
		bt = security.BackupTypeConfig
	case "cache":
		bt = security.BackupTypeCache
	case "audit":
		bt = security.BackupTypeAudit
	case "full":
		bt = security.BackupTypeFull
	default:
		return fmt.Errorf("invalid backup type: %s (valid: config, cache, audit, full)", backupType)
	}

	// Get backup manager
	bm, err := security.GlobalBackupManager()
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup create", err)
			resp.Print()
			return err
		}
		return err
	}

	if !jsonOutput {
		fmt.Println()
		fmt.Printf("Creating %s backup...\n", backupType)
	}

	// Create backup
	backup, err := bm.CreateBackup(bt)
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup create", err)
			resp.Print()
			return err
		}
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if jsonOutput {
		data := map[string]interface{}{
			"backup_id":  backup.ID,
			"type":       string(backup.Type),
			"timestamp":  backup.Timestamp,
			"size":       backup.Size,
			"size_human": formatBytes(backup.Size),
			"checksum":   backup.Checksum,
			"files":      len(backup.Files),
		}
		resp := NewJSONResponse("backup create", data)
		return resp.Print()
	}

	// Show success message
	fmt.Println()
	fmt.Println("Backup created successfully!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Backup ID:  %s\n", backup.ID)
	fmt.Printf("  Type:       %s\n", backup.Type)
	fmt.Printf("  Size:       %s\n", formatBytes(backup.Size))
	fmt.Printf("  Files:      %d\n", len(backup.Files))
	fmt.Printf("  Checksum:   %s\n", backup.Checksum[:16]+"...")
	fmt.Printf("  Timestamp:  %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Printf("Use 'rigrun backup restore %s' to restore this backup\n", backup.ID)
	fmt.Println()

	return nil
}

// =============================================================================
// RESTORE BACKUP
// =============================================================================

// restoreBackup restores a backup with confirmation.
func restoreBackup(backupID string, confirmFlag bool) error {
	if backupID == "" {
		return fmt.Errorf("backup ID required: rigrun backup restore <id>")
	}

	// Get backup manager
	bm, err := security.GlobalBackupManager()
	if err != nil {
		return err
	}

	// List backups to find the one to restore
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var backup *security.Backup
	for _, b := range backups {
		if b.ID == backupID {
			backup = b
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// Show backup info and confirm
	if !confirmFlag {
		fmt.Println()
		fmt.Println("Restore Backup Confirmation")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Printf("  Backup ID:  %s\n", backup.ID)
		fmt.Printf("  Type:       %s\n", backup.Type)
		fmt.Printf("  Created:    %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Files:      %d\n", len(backup.Files))
		fmt.Println()
		fmt.Println("WARNING: This will overwrite current files!")
		fmt.Println()
		fmt.Print("Continue with restore? [y/N]: ")

		input := promptInput("")
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("Restore cancelled.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("Restoring backup...")

	// Restore backup
	if err := bm.RestoreBackup(backupID); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	fmt.Println()
	fmt.Println("Backup restored successfully!")
	fmt.Printf("  Restored %d files from backup %s\n", len(backup.Files), backupID)
	fmt.Println()

	return nil
}

// =============================================================================
// LIST BACKUPS
// =============================================================================

// listBackups lists all available backups.
func listBackups(jsonOutput bool) error {
	bm, err := security.GlobalBackupManager()
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup list", err)
			resp.Print()
			return err
		}
		return err
	}

	backups, err := bm.ListBackups()
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup list", err)
			resp.Print()
			return err
		}
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		if jsonOutput {
			data := map[string]interface{}{
				"backups": []interface{}{},
				"total":   0,
			}
			resp := NewJSONResponse("backup list", data)
			return resp.Print()
		}

		fmt.Println()
		fmt.Println("No backups found.")
		fmt.Println()
		fmt.Println("Create a backup with: rigrun backup create [type]")
		fmt.Println()
		return nil
	}

	if jsonOutput {
		var backupList []map[string]interface{}
		for _, b := range backups {
			backupList = append(backupList, map[string]interface{}{
				"id":         b.ID,
				"type":       string(b.Type),
				"timestamp":  b.Timestamp,
				"size":       b.Size,
				"size_human": formatBytes(b.Size),
				"files":      len(b.Files),
				"checksum":   b.Checksum,
			})
		}

		data := map[string]interface{}{
			"backups": backupList,
			"total":   len(backups),
		}
		resp := NewJSONResponse("backup list", data)
		return resp.Print()
	}

	// Display backups
	fmt.Println()
	fmt.Println("Available Backups")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	for i, backup := range backups {
		age := formatTimeAgo(backup.Timestamp)
		fmt.Printf("[%d] %s\n", i+1, backup.ID)
		fmt.Printf("    Type:      %s\n", backup.Type)
		fmt.Printf("    Created:   %s (%s)\n", backup.Timestamp.Format("2006-01-02 15:04:05"), age)
		fmt.Printf("    Size:      %s\n", formatBytes(backup.Size))
		fmt.Printf("    Files:     %d\n", len(backup.Files))
		fmt.Printf("    Checksum:  %s...\n", backup.Checksum[:16])
		fmt.Println()
	}

	fmt.Printf("Total backups: %d\n", len(backups))
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  rigrun backup restore <id>  - Restore a backup")
	fmt.Println("  rigrun backup verify <id>   - Verify backup integrity")
	fmt.Println("  rigrun backup delete <id>   - Delete a backup")
	fmt.Println()

	return nil
}

// =============================================================================
// VERIFY BACKUP
// =============================================================================

// verifyBackup verifies the integrity of a backup.
func verifyBackup(backupID string) error {
	if backupID == "" {
		return fmt.Errorf("backup ID required: rigrun backup verify <id>")
	}

	bm, err := security.GlobalBackupManager()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Verifying backup: %s...\n", backupID)

	// Verify backup
	if err := bm.VerifyBackup(backupID); err != nil {
		fmt.Println()
		fmt.Println("FAILED: Backup integrity check failed!")
		fmt.Printf("  Error: %s\n", err.Error())
		fmt.Println()
		return err
	}

	fmt.Println()
	fmt.Println("SUCCESS: Backup integrity verified!")
	fmt.Printf("  Backup %s is valid and can be restored safely.\n", backupID)
	fmt.Println()

	return nil
}

// =============================================================================
// DELETE BACKUP
// =============================================================================

// deleteBackup deletes a backup with confirmation.
func deleteBackup(backupID string, confirmFlag bool) error {
	if backupID == "" {
		return fmt.Errorf("backup ID required: rigrun backup delete <id>")
	}

	bm, err := security.GlobalBackupManager()
	if err != nil {
		return err
	}

	// List backups to find the one to delete
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var backup *security.Backup
	for _, b := range backups {
		if b.ID == backupID {
			backup = b
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// Confirm deletion
	if !confirmFlag {
		fmt.Println()
		fmt.Println("Delete Backup Confirmation")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Printf("  Backup ID:  %s\n", backup.ID)
		fmt.Printf("  Type:       %s\n", backup.Type)
		fmt.Printf("  Created:    %s\n", backup.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Size:       %s\n", formatBytes(backup.Size))
		fmt.Println()
		fmt.Println("This backup will be securely deleted using DoD 5220.22-M standard.")
		fmt.Println()
		fmt.Print("Continue with deletion? [y/N]: ")

		input := promptInput("")
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("Securely deleting backup...")

	// Delete backup
	if err := bm.DeleteBackup(backupID); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	fmt.Println()
	fmt.Println("Backup deleted successfully!")
	fmt.Println()

	return nil
}

// =============================================================================
// SCHEDULE BACKUP
// =============================================================================

// scheduleBackup configures automated backup schedule.
func scheduleBackup(interval string, backupType string) error {
	if interval == "" {
		return fmt.Errorf("interval required: rigrun backup schedule <interval> [type]")
	}

	// Default to full backup if no type specified
	if backupType == "" {
		backupType = "full"
	}

	// Parse interval
	duration, err := parseBackupDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	// Validate backup type
	var bt security.BackupType
	switch strings.ToLower(backupType) {
	case "config":
		bt = security.BackupTypeConfig
	case "cache":
		bt = security.BackupTypeCache
	case "audit":
		bt = security.BackupTypeAudit
	case "full":
		bt = security.BackupTypeFull
	default:
		return fmt.Errorf("invalid backup type: %s (valid: config, cache, audit, full)", backupType)
	}

	// Get backup manager
	bm, err := security.GlobalBackupManager()
	if err != nil {
		return err
	}

	// Schedule backup
	if err := bm.ScheduleBackup(duration, bt); err != nil {
		return fmt.Errorf("failed to schedule backup: %w", err)
	}

	fmt.Println()
	fmt.Println("Backup schedule configured!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Interval:  %s\n", interval)
	fmt.Printf("  Type:      %s\n", backupType)
	fmt.Println()
	fmt.Println("Backups will be created automatically at the specified interval.")
	fmt.Println()

	return nil
}

// =============================================================================
// BACKUP STATUS
// =============================================================================

// showBackupStatus shows the current backup status and schedule.
func showBackupStatus(jsonOutput bool) error {
	bm, err := security.GlobalBackupManager()
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup status", err)
			resp.Print()
			return err
		}
		return err
	}

	status, err := bm.GetBackupStatus()
	if err != nil {
		if jsonOutput {
			resp := NewJSONErrorResponse("backup status", err)
			resp.Print()
			return err
		}
		return fmt.Errorf("failed to get backup status: %w", err)
	}

	if jsonOutput {
		data := map[string]interface{}{
			"total_backups":     status.TotalBackups,
			"total_size":        status.TotalSize,
			"total_size_human":  formatBytes(status.TotalSize),
			"schedule_enabled":  status.ScheduleEnabled,
			"schedule_interval": status.ScheduleInterval,
			"schedule_type":     string(status.ScheduleType),
			"retention_policy":  status.RetentionPolicy,
		}

		if status.LastBackup != nil {
			data["last_backup"] = map[string]interface{}{
				"id":        status.LastBackup.ID,
				"type":      string(status.LastBackup.Type),
				"timestamp": status.LastBackup.Timestamp,
				"size":      status.LastBackup.Size,
			}
		}

		if status.ScheduleEnabled {
			data["next_scheduled"] = status.NextScheduled
		}

		resp := NewJSONResponse("backup status", data)
		return resp.Print()
	}

	// Display status
	fmt.Println()
	fmt.Println("Backup Status")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Last backup
	if status.LastBackup != nil {
		age := formatTimeAgo(status.LastBackup.Timestamp)
		fmt.Println("Last Backup:")
		fmt.Printf("  ID:        %s\n", status.LastBackup.ID)
		fmt.Printf("  Type:      %s\n", status.LastBackup.Type)
		fmt.Printf("  Created:   %s (%s)\n", status.LastBackup.Timestamp.Format("2006-01-02 15:04:05"), age)
		fmt.Printf("  Size:      %s\n", formatBytes(status.LastBackup.Size))
		fmt.Println()
	} else {
		fmt.Println("Last Backup: None")
		fmt.Println()
	}

	// Schedule
	if status.ScheduleEnabled {
		fmt.Println("Schedule:")
		fmt.Printf("  Enabled:   Yes\n")
		fmt.Printf("  Interval:  %s\n", status.ScheduleInterval)
		fmt.Printf("  Type:      %s\n", status.ScheduleType)
		if !status.NextScheduled.IsZero() {
			timeUntil := time.Until(status.NextScheduled)
			fmt.Printf("  Next:      %s (in %s)\n", status.NextScheduled.Format("2006-01-02 15:04:05"), formatDuration(timeUntil))
		}
		fmt.Println()
	} else {
		fmt.Println("Schedule:    Not configured")
		fmt.Println()
	}

	// Statistics
	fmt.Println("Statistics:")
	fmt.Printf("  Total Backups:  %d\n", status.TotalBackups)
	fmt.Printf("  Total Size:     %s\n", formatBytes(status.TotalSize))
	fmt.Printf("  Retention:      Keep last %d backups\n", status.RetentionPolicy)
	fmt.Println()

	// Backup location
	home, _ := os.UserHomeDir()
	backupDir := filepath.Join(home, ".rigrun", security.BackupDirName)
	fmt.Printf("Backup Location: %s\n", backupDir)
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// parseBackupDuration parses a duration string (supports common formats).
func parseBackupDuration(s string) (time.Duration, error) {
	// Try standard Go duration parsing first
	duration, err := time.ParseDuration(s)
	if err == nil {
		return duration, nil
	}

	// Try common formats: "1h", "24h", "7d", "30d"
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid duration format: %s", s)
}
