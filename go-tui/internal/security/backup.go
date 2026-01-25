// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides NIST 800-53 CP-9 (System Backup) and CP-10 (System Recovery) implementation.
//
// This implements:
// - CP-9: System Backup with encrypted backups, integrity verification, and retention policy
// - CP-10: System Recovery with validated restoration from encrypted backups
// - Automated backup scheduling with configurable intervals
// - Secure deletion using DoD 5220.22-M standard
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// BackupDirName is the directory name for storing backups
	BackupDirName = "backups"

	// BackupFileExt is the file extension for backup files
	BackupFileExt = ".backup"

	// BackupMetadataExt is the file extension for backup metadata
	BackupMetadataExt = ".meta"

	// DefaultMaxBackups is the default number of backups to retain
	DefaultMaxBackups = 10

	// DefaultBackupRetentionDays is the default retention period in days
	DefaultBackupRetentionDays = 30
)

// BackupType represents the type of backup.
type BackupType string

const (
	// BackupTypeConfig backs up configuration files
	BackupTypeConfig BackupType = "config"

	// BackupTypeCache backs up conversation cache
	BackupTypeCache BackupType = "cache"

	// BackupTypeAudit backs up audit logs
	BackupTypeAudit BackupType = "audit"

	// BackupTypeFull backs up everything
	BackupTypeFull BackupType = "full"
)

// =============================================================================
// BACKUP STRUCTURES
// =============================================================================

// Backup represents a single backup with metadata.
type Backup struct {
	ID            string     `json:"id"`             // Unique backup ID (timestamp-based)
	Type          BackupType `json:"type"`           // Type of backup
	Timestamp     time.Time  `json:"timestamp"`      // When backup was created
	Size          int64      `json:"size"`           // Size in bytes
	Checksum      string     `json:"checksum"`       // SHA-256 checksum for integrity
	EncryptedPath string     `json:"encrypted_path"` // Path to encrypted backup file
	Verified      bool       `json:"verified"`       // Whether backup has been verified
	LastVerified  time.Time  `json:"last_verified"`  // Last verification time
	Files         []string   `json:"files"`          // List of files included in backup
}

// BackupStatus represents the current backup status.
type BackupStatus struct {
	LastBackup       *Backup    `json:"last_backup"`       // Most recent backup
	NextScheduled    time.Time  `json:"next_scheduled"`    // Next scheduled backup time
	TotalBackups     int        `json:"total_backups"`     // Total number of backups
	TotalSize        int64      `json:"total_size"`        // Total size of all backups
	ScheduleEnabled  bool       `json:"schedule_enabled"`  // Whether automated backup is enabled
	ScheduleInterval string     `json:"schedule_interval"` // Backup interval (e.g., "24h")
	ScheduleType     BackupType `json:"schedule_type"`     // Type of scheduled backup
	RetentionPolicy  int        `json:"retention_policy"`  // Number of backups to retain
}

// BackupMetadata stores metadata about a backup (saved alongside encrypted file).
type BackupMetadata struct {
	ID            string     `json:"id"`
	Type          BackupType `json:"type"`
	Timestamp     time.Time  `json:"timestamp"`
	OriginalSize  int64      `json:"original_size"`
	EncryptedSize int64      `json:"encrypted_size"`
	Checksum      string     `json:"checksum"`
	Files         []string   `json:"files"`
	Version       string     `json:"version"` // rigrun version
}

// =============================================================================
// BACKUP MANAGER
// =============================================================================

// BackupManager manages system backups with encryption and integrity verification.
type BackupManager struct {
	mu               sync.RWMutex
	backupDir        string
	encryptionMgr    *EncryptionManager
	maxBackups       int
	retentionDays    int
	scheduleInterval time.Duration
	scheduleType     BackupType
	scheduleEnabled  bool
	stopScheduler    chan struct{}
}

// NewBackupManager creates a new backup manager.
func NewBackupManager() (*BackupManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(home, ".rigrun", BackupDirName)

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get encryption manager
	encMgr := GlobalEncryptionManager()
	if encMgr == nil || !encMgr.IsInitialized() {
		return nil, fmt.Errorf("encryption not initialized: run 'rigrun encrypt init' first")
	}

	bm := &BackupManager{
		backupDir:     backupDir,
		encryptionMgr: encMgr,
		maxBackups:    DefaultMaxBackups,
		retentionDays: DefaultBackupRetentionDays,
		stopScheduler: make(chan struct{}),
	}

	return bm, nil
}

// =============================================================================
// BACKUP CREATION
// =============================================================================

// CreateBackup creates a new backup of the specified type.
// Returns the backup ID and any error.
func (bm *BackupManager) CreateBackup(backupType BackupType) (*Backup, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Generate backup ID (timestamp-based)
	backupID := fmt.Sprintf("%s_%s", backupType, time.Now().Format("20060102_150405"))

	// Determine what files to backup
	files, err := bm.getFilesToBackup(backupType)
	if err != nil {
		return nil, fmt.Errorf("failed to determine backup files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found for backup type: %s", backupType)
	}

	// Create temporary archive
	archivePath, originalSize, err := bm.createArchive(backupID, files)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}
	defer os.Remove(archivePath) // Clean up temp archive

	// Calculate checksum of original data
	checksum, err := bm.calculateChecksum(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Encrypt the archive
	encryptedPath := filepath.Join(bm.backupDir, backupID+BackupFileExt)
	if err := bm.encryptionMgr.EncryptFile(archivePath, encryptedPath); err != nil {
		return nil, fmt.Errorf("failed to encrypt backup: %w", err)
	}

	// Get encrypted file size
	encInfo, err := os.Stat(encryptedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat encrypted backup: %w", err)
	}

	// Create backup metadata
	backup := &Backup{
		ID:            backupID,
		Type:          backupType,
		Timestamp:     time.Now(),
		Size:          encInfo.Size(),
		Checksum:      checksum,
		EncryptedPath: encryptedPath,
		Verified:      false,
		Files:         files,
	}

	// Save metadata
	metadata := &BackupMetadata{
		ID:            backupID,
		Type:          backupType,
		Timestamp:     backup.Timestamp,
		OriginalSize:  originalSize,
		EncryptedSize: encInfo.Size(),
		Checksum:      checksum,
		Files:         files,
		Version:       "0.3.0-wired", // TODO: Get from build info
	}

	metaPath := filepath.Join(bm.backupDir, backupID+BackupMetadataExt)
	if err := bm.saveMetadata(metaPath, metadata); err != nil {
		// Clean up encrypted backup if metadata save fails
		os.Remove(encryptedPath)
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Log backup creation
	AuditLogEvent("", "BACKUP_CREATED", map[string]string{
		"backup_id": backupID,
		"type":      string(backupType),
		"size":      fmt.Sprintf("%d", encInfo.Size()),
		"files":     fmt.Sprintf("%d", len(files)),
	})

	// Apply retention policy
	if err := bm.applyRetentionPolicyLocked(); err != nil {
		// Log but don't fail the backup
		AuditLogEvent("", "RETENTION_POLICY_ERROR", map[string]string{
			"error": err.Error(),
		})
	}

	return backup, nil
}

// getFilesToBackup determines which files to backup based on type.
func (bm *BackupManager) getFilesToBackup(backupType BackupType) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rigrunDir := filepath.Join(home, ".rigrun")
	var files []string

	switch backupType {
	case BackupTypeConfig:
		// Backup config files
		configFiles := []string{
			"config.toml",
			"consent.json",
		}
		for _, f := range configFiles {
			path := filepath.Join(rigrunDir, f)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}

	case BackupTypeCache:
		// Backup cache files
		cacheFiles := []string{
			"cache.json",
			"semantic_cache.json",
		}
		for _, f := range cacheFiles {
			path := filepath.Join(rigrunDir, f)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}

	case BackupTypeAudit:
		// Backup audit logs
		auditFiles := []string{
			"audit.log",
		}
		for _, f := range auditFiles {
			path := filepath.Join(rigrunDir, f)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}

	case BackupTypeFull:
		// Backup everything except backups and temp files
		err := filepath.Walk(rigrunDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files with errors
			}

			// Skip directories
			if info.IsDir() {
				// Skip backup directory
				if filepath.Base(path) == BackupDirName {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip backup files themselves
			if strings.HasSuffix(path, BackupFileExt) || strings.HasSuffix(path, BackupMetadataExt) {
				return nil
			}

			// Skip temp files
			if strings.HasPrefix(filepath.Base(path), ".") && filepath.Base(path) != ".gitkeep" {
				return nil
			}

			files = append(files, path)
			return nil
		})

		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown backup type: %s", backupType)
	}

	return files, nil
}

// createArchive creates a tar-like archive of files (simple concatenation with metadata).
func (bm *BackupManager) createArchive(backupID string, files []string) (string, int64, error) {
	archivePath := filepath.Join(os.TempDir(), backupID+".tar")

	archive, err := os.Create(archivePath)
	if err != nil {
		return "", 0, err
	}
	defer archive.Close()

	var totalSize int64

	// Write a simple archive format: [file count]\n[file1 path]\n[file1 size]\n[file1 data]...
	if _, err := fmt.Fprintf(archive, "%d\n", len(files)); err != nil {
		return "", 0, err
	}

	for _, filePath := range files {
		// Read file
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", 0, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// Get relative path from .rigrun directory
		home, _ := os.UserHomeDir()
		rigrunDir := filepath.Join(home, ".rigrun")
		relPath, err := filepath.Rel(rigrunDir, filePath)
		if err != nil {
			relPath = filepath.Base(filePath)
		}

		// Write file metadata and data
		if _, err := fmt.Fprintf(archive, "%s\n%d\n", relPath, len(data)); err != nil {
			return "", 0, err
		}

		if _, err := archive.Write(data); err != nil {
			return "", 0, err
		}

		totalSize += int64(len(data))
	}

	return archivePath, totalSize, nil
}

// calculateChecksum calculates SHA-256 checksum of a file.
func (bm *BackupManager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// saveMetadata saves backup metadata to a JSON file.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (bm *BackupManager) saveMetadata(path string, metadata *BackupMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	return util.AtomicWriteFile(path, data, 0600)
}

// =============================================================================
// BACKUP RESTORATION
// =============================================================================

// RestoreBackup restores a backup with integrity verification.
func (bm *BackupManager) RestoreBackup(backupID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Load backup metadata
	metaPath := filepath.Join(bm.backupDir, backupID+BackupMetadataExt)
	metadata, err := bm.loadMetadata(metaPath)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata: %w", err)
	}

	// Verify backup exists
	encryptedPath := filepath.Join(bm.backupDir, backupID+BackupFileExt)
	if _, err := os.Stat(encryptedPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Decrypt backup to temp file
	tempArchive := filepath.Join(os.TempDir(), backupID+".tar")
	if err := bm.encryptionMgr.DecryptFile(encryptedPath, tempArchive); err != nil {
		return fmt.Errorf("failed to decrypt backup: %w", err)
	}
	defer os.Remove(tempArchive)

	// Verify checksum
	checksum, err := bm.calculateChecksum(tempArchive)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if checksum != metadata.Checksum {
		return fmt.Errorf("backup integrity check failed: checksum mismatch")
	}

	// Extract archive
	if err := bm.extractArchive(tempArchive); err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}

	// Log restoration
	AuditLogEvent("", "BACKUP_RESTORED", map[string]string{
		"backup_id": backupID,
		"type":      string(metadata.Type),
		"files":     fmt.Sprintf("%d", len(metadata.Files)),
	})

	return nil
}

// extractArchive extracts files from a backup archive.
func (bm *BackupManager) extractArchive(archivePath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rigrunDir := filepath.Join(home, ".rigrun")

	// Read file count
	var fileCount int
	if _, err := fmt.Fscanf(archive, "%d\n", &fileCount); err != nil {
		return err
	}

	// Extract each file
	for i := 0; i < fileCount; i++ {
		var relPath string
		var size int

		if _, err := fmt.Fscanf(archive, "%s\n%d\n", &relPath, &size); err != nil {
			return err
		}

		// Read file data
		data := make([]byte, size)
		if _, err := io.ReadFull(archive, data); err != nil {
			return err
		}

		// Write to destination
		destPath := filepath.Join(rigrunDir, relPath)

		// RELIABILITY: Atomic write with fsync prevents data loss on crash
		if err := util.AtomicWriteFileWithDir(destPath, data, 0600, 0700); err != nil {
			return err
		}
	}

	return nil
}

// loadMetadata loads backup metadata from a JSON file.
func (bm *BackupManager) loadMetadata(path string) (*BackupMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// =============================================================================
// BACKUP VERIFICATION
// =============================================================================

// VerifyBackup verifies the integrity of a backup.
func (bm *BackupManager) VerifyBackup(backupID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Load metadata
	metaPath := filepath.Join(bm.backupDir, backupID+BackupMetadataExt)
	metadata, err := bm.loadMetadata(metaPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Verify backup file exists
	encryptedPath := filepath.Join(bm.backupDir, backupID+BackupFileExt)
	if _, err := os.Stat(encryptedPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Decrypt to temp file
	tempArchive := filepath.Join(os.TempDir(), backupID+"_verify.tar")
	if err := bm.encryptionMgr.DecryptFile(encryptedPath, tempArchive); err != nil {
		return fmt.Errorf("failed to decrypt backup: %w", err)
	}
	defer os.Remove(tempArchive)

	// Verify checksum
	checksum, err := bm.calculateChecksum(tempArchive)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if checksum != metadata.Checksum {
		return fmt.Errorf("integrity check failed: checksum mismatch")
	}

	// Log verification
	AuditLogEvent("", "BACKUP_VERIFIED", map[string]string{
		"backup_id": backupID,
		"status":    "success",
	})

	return nil
}

// =============================================================================
// BACKUP LISTING AND MANAGEMENT
// =============================================================================

// ListBackups returns all available backups, sorted by timestamp (newest first).
func (bm *BackupManager) ListBackups() ([]*Backup, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []*Backup

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), BackupMetadataExt) {
			continue
		}

		// Load metadata
		metaPath := filepath.Join(bm.backupDir, entry.Name())
		metadata, err := bm.loadMetadata(metaPath)
		if err != nil {
			continue // Skip invalid metadata
		}

		// Get encrypted file info
		encryptedPath := filepath.Join(bm.backupDir, metadata.ID+BackupFileExt)
		info, err := os.Stat(encryptedPath)
		if err != nil {
			continue // Skip if encrypted file missing
		}

		backup := &Backup{
			ID:            metadata.ID,
			Type:          metadata.Type,
			Timestamp:     metadata.Timestamp,
			Size:          info.Size(),
			Checksum:      metadata.Checksum,
			EncryptedPath: encryptedPath,
			Files:         metadata.Files,
		}

		backups = append(backups, backup)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// DeleteBackup securely deletes a backup using DoD 5220.22-M standard.
func (bm *BackupManager) DeleteBackup(backupID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	sanitizer := GlobalSanitizer()

	// Delete encrypted backup file
	encryptedPath := filepath.Join(bm.backupDir, backupID+BackupFileExt)
	if _, err := os.Stat(encryptedPath); err == nil {
		if err := sanitizer.SecureDeleteFile(encryptedPath); err != nil {
			return fmt.Errorf("failed to delete backup file: %w", err)
		}
	}

	// Delete metadata file
	metaPath := filepath.Join(bm.backupDir, backupID+BackupMetadataExt)
	if _, err := os.Stat(metaPath); err == nil {
		if err := sanitizer.SecureDeleteFile(metaPath); err != nil {
			return fmt.Errorf("failed to delete metadata: %w", err)
		}
	}

	// Log deletion
	AuditLogEvent("", "BACKUP_DELETED", map[string]string{
		"backup_id": backupID,
	})

	return nil
}

// =============================================================================
// BACKUP SCHEDULING
// =============================================================================

// ScheduleBackup configures automated backup schedule.
func (bm *BackupManager) ScheduleBackup(interval time.Duration, backupType BackupType) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if interval < time.Hour {
		return fmt.Errorf("backup interval must be at least 1 hour")
	}

	bm.scheduleInterval = interval
	bm.scheduleType = backupType
	bm.scheduleEnabled = true

	// Start scheduler if not already running
	go bm.runScheduler()

	// Log scheduling
	AuditLogEvent("", "BACKUP_SCHEDULED", map[string]string{
		"interval": interval.String(),
		"type":     string(backupType),
	})

	return nil
}

// runScheduler runs the backup scheduler.
func (bm *BackupManager) runScheduler() {
	ticker := time.NewTicker(bm.scheduleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bm.mu.RLock()
			if !bm.scheduleEnabled {
				bm.mu.RUnlock()
				return
			}
			scheduleType := bm.scheduleType
			bm.mu.RUnlock()

			// Create backup
			if _, err := bm.CreateBackup(scheduleType); err != nil {
				AuditLogEvent("", "SCHEDULED_BACKUP_FAILED", map[string]string{
					"error": err.Error(),
				})
			}

		case <-bm.stopScheduler:
			return
		}
	}
}

// StopSchedule stops the automated backup schedule.
func (bm *BackupManager) StopSchedule() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// BUGFIX: Check if already stopped to prevent double-close panic
	if !bm.scheduleEnabled {
		return // Already stopped
	}

	bm.scheduleEnabled = false
	close(bm.stopScheduler)
	bm.stopScheduler = make(chan struct{})

	AuditLogEvent("", "BACKUP_SCHEDULE_STOPPED", nil)
}

// =============================================================================
// BACKUP STATUS
// =============================================================================

// GetBackupStatus returns the current backup status.
func (bm *BackupManager) GetBackupStatus() (*BackupStatus, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	backups, err := bm.ListBackups()
	if err != nil {
		return nil, err
	}

	status := &BackupStatus{
		TotalBackups:     len(backups),
		ScheduleEnabled:  bm.scheduleEnabled,
		ScheduleInterval: bm.scheduleInterval.String(),
		ScheduleType:     bm.scheduleType,
		RetentionPolicy:  bm.maxBackups,
	}

	// Calculate total size
	for _, backup := range backups {
		status.TotalSize += backup.Size
	}

	// Get last backup
	if len(backups) > 0 {
		status.LastBackup = backups[0]
	}

	// Calculate next scheduled backup
	if bm.scheduleEnabled && status.LastBackup != nil {
		status.NextScheduled = status.LastBackup.Timestamp.Add(bm.scheduleInterval)
	}

	return status, nil
}

// =============================================================================
// RETENTION POLICY
// =============================================================================

// applyRetentionPolicyLocked applies the backup retention policy (caller must hold lock).
func (bm *BackupManager) applyRetentionPolicyLocked() error {
	backups, err := bm.ListBackups()
	if err != nil {
		return err
	}

	// Delete backups exceeding max count
	if len(backups) > bm.maxBackups {
		for i := bm.maxBackups; i < len(backups); i++ {
			backup := backups[i]

			// Delete old backup
			sanitizer := GlobalSanitizer()

			encryptedPath := filepath.Join(bm.backupDir, backup.ID+BackupFileExt)
			if err := sanitizer.SecureDeleteFile(encryptedPath); err == nil {
				AuditLogEvent("", "BACKUP_RETENTION_DELETE", map[string]string{
					"backup_id": backup.ID,
					"reason":    "max_backups_exceeded",
				})
			}

			metaPath := filepath.Join(bm.backupDir, backup.ID+BackupMetadataExt)
			sanitizer.SecureDeleteFile(metaPath)
		}
	}

	// Delete backups exceeding retention days
	cutoffDate := time.Now().AddDate(0, 0, -bm.retentionDays)
	for _, backup := range backups {
		if backup.Timestamp.Before(cutoffDate) {
			sanitizer := GlobalSanitizer()

			encryptedPath := filepath.Join(bm.backupDir, backup.ID+BackupFileExt)
			if err := sanitizer.SecureDeleteFile(encryptedPath); err == nil {
				AuditLogEvent("", "BACKUP_RETENTION_DELETE", map[string]string{
					"backup_id": backup.ID,
					"reason":    "retention_days_exceeded",
				})
			}

			metaPath := filepath.Join(bm.backupDir, backup.ID+BackupMetadataExt)
			sanitizer.SecureDeleteFile(metaPath)
		}
	}

	return nil
}

// SetRetentionPolicy sets the backup retention policy.
func (bm *BackupManager) SetRetentionPolicy(maxBackups int, retentionDays int) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if maxBackups < 1 {
		return fmt.Errorf("max backups must be at least 1")
	}

	if retentionDays < 1 {
		return fmt.Errorf("retention days must be at least 1")
	}

	bm.maxBackups = maxBackups
	bm.retentionDays = retentionDays

	// Apply new policy
	return bm.applyRetentionPolicyLocked()
}

// =============================================================================
// GLOBAL BACKUP MANAGER
// =============================================================================

var (
	globalBackupManager     *BackupManager
	globalBackupManagerOnce sync.Once
	globalBackupManagerMu   sync.Mutex
)

// GlobalBackupManager returns the global backup manager instance.
func GlobalBackupManager() (*BackupManager, error) {
	var err error
	globalBackupManagerOnce.Do(func() {
		globalBackupManager, err = NewBackupManager()
	})
	return globalBackupManager, err
}
