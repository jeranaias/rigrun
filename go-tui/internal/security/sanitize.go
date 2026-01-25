// sanitize.go - Secure data sanitization for NIST 800-53 IR-9 compliance.
//
// Provides secure deletion (DoD 5220.22-M standard), memory sanitization,
// and cache/session cleanup capabilities for spillage response procedures.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// SecureDeletePasses is the number of overwrite passes per DoD 5220.22-M standard.
// 3-pass overwrite: pattern, complement, random
const SecureDeletePasses = 3

// =============================================================================
// DATA SANITIZER
// =============================================================================

// DataSanitizer provides secure data sanitization capabilities.
type DataSanitizer struct {
	mu sync.Mutex
}

// NewDataSanitizer creates a new data sanitizer.
func NewDataSanitizer() *DataSanitizer {
	return &DataSanitizer{}
}

// =============================================================================
// SECURE FILE DELETION
// =============================================================================

// SecureDeleteFile securely deletes a file using DoD 5220.22-M standard.
// This performs a 3-pass overwrite: pattern (0x00), complement (0xFF), random.
func (d *DataSanitizer) SecureDeleteFile(path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("cannot secure delete directory, use SecureDeleteDirectory")
	}

	size := info.Size()

	// Open file for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open file for overwrite: %w", err)
	}
	defer file.Close()

	// Pass 1: Write zeros (0x00)
	if err := overwriteFile(file, size, 0x00); err != nil {
		return fmt.Errorf("pass 1 (zeros) failed: %w", err)
	}

	// Pass 2: Write ones (0xFF)
	if err := overwriteFile(file, size, 0xFF); err != nil {
		return fmt.Errorf("pass 2 (ones) failed: %w", err)
	}

	// Pass 3: Write random data
	if err := overwriteFileRandom(file, size); err != nil {
		return fmt.Errorf("pass 3 (random) failed: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close before delete
	file.Close()

	// Delete the file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// Log the secure deletion
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "secure_delete",
		"path":   path,
		"size":   fmt.Sprintf("%d", size),
	})

	return nil
}

// SecureDeleteDirectory securely deletes all files in a directory recursively.
func (d *DataSanitizer) SecureDeleteDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	var errors []error

	// Walk through all files
	err = filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, err)
			return nil // Continue walking despite errors
		}

		// Skip directories for now - we'll remove them after files
		if fileInfo.IsDir() {
			return nil
		}

		// Securely delete each file
		if err := d.SecureDeleteFile(filePath); err != nil {
			errors = append(errors, fmt.Errorf("failed to delete %s: %w", filePath, err))
		}

		return nil
	})

	if err != nil {
		errors = append(errors, err)
	}

	// Remove empty directories bottom-up
	if err := removeEmptyDirs(path); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("secure delete completed with %d errors", len(errors))
	}

	// Log the directory deletion
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "secure_delete_dir",
		"path":   path,
	})

	return nil
}

// =============================================================================
// MEMORY SANITIZATION
// =============================================================================

// ClearMemory zeros out a byte slice to prevent memory-based data recovery.
func (d *DataSanitizer) ClearMemory(data []byte) {
	if data == nil {
		return
	}

	// Zero out the memory
	for i := range data {
		data[i] = 0
	}

	// Optional: Overwrite with random data for extra security
	rand.Read(data)

	// Zero out again
	for i := range data {
		data[i] = 0
	}
}

// ClearString attempts to clear a string from memory.
// Note: Go strings are immutable, so this creates a new zeroed string
// and relies on GC to clean up the original.
func (d *DataSanitizer) ClearString(s *string) {
	if s == nil || *s == "" {
		return
	}

	// Create a zeroed string of the same length
	*s = string(make([]byte, len(*s)))
}

// =============================================================================
// CACHE/SESSION SANITIZATION
// =============================================================================

// SanitizeCache clears the rigrun cache files.
func (d *DataSanitizer) SanitizeCache() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(home, ".rigrun")
	cacheFiles := []string{
		"cache.json",
		"semantic_cache.json",
	}

	var errors []error

	for _, file := range cacheFiles {
		path := filepath.Join(cacheDir, file)
		if _, err := os.Stat(path); err == nil {
			if err := d.SecureDeleteFile(path); err != nil {
				errors = append(errors, err)
			}
		}
	}

	// Log cache sanitization
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "cache_clear",
	})

	if len(errors) > 0 {
		return fmt.Errorf("cache sanitization completed with %d errors", len(errors))
	}

	return nil
}

// SanitizeSession clears session-related data.
func (d *DataSanitizer) SanitizeSession() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Session files to clear
	sessionPaths := []string{
		filepath.Join(home, ".rigrun", "session.json"),
		filepath.Join(home, ".rigrun", "current_session.json"),
	}

	var errors []error

	for _, path := range sessionPaths {
		if _, err := os.Stat(path); err == nil {
			if err := d.SecureDeleteFile(path); err != nil {
				errors = append(errors, err)
			}
		}
	}

	// Log session sanitization
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "session_clear",
	})

	if len(errors) > 0 {
		return fmt.Errorf("session sanitization completed with %d errors", len(errors))
	}

	return nil
}

// SanitizeConversations clears all saved conversations.
func (d *DataSanitizer) SanitizeConversations() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	convDir := filepath.Join(home, ".rigrun", "conversations")
	if _, err := os.Stat(convDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to do
	}

	if err := d.SecureDeleteDirectory(convDir); err != nil {
		return fmt.Errorf("failed to sanitize conversations: %w", err)
	}

	// Recreate empty directory
	os.MkdirAll(convDir, 0700)

	// Log conversation sanitization
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "conversations_clear",
	})

	return nil
}

// SanitizeAll performs a full cleanup of all rigrun data.
// This is the most thorough sanitization option.
func (d *DataSanitizer) SanitizeAll() error {
	var errors []error

	// Sanitize cache
	if err := d.SanitizeCache(); err != nil {
		errors = append(errors, fmt.Errorf("cache: %w", err))
	}

	// Sanitize session
	if err := d.SanitizeSession(); err != nil {
		errors = append(errors, fmt.Errorf("session: %w", err))
	}

	// Sanitize conversations
	if err := d.SanitizeConversations(); err != nil {
		errors = append(errors, fmt.Errorf("conversations: %w", err))
	}

	// Log full sanitization
	AuditLogEvent("", "DATA_SANITIZED", map[string]string{
		"action": "full_sanitize",
	})

	if len(errors) > 0 {
		return fmt.Errorf("full sanitization completed with %d errors", len(errors))
	}

	return nil
}

// SpillageResponse executes the complete spillage response procedure.
// This is the IR-9 mandated response to a spillage event.
func (d *DataSanitizer) SpillageResponse() error {
	// Log initiation
	AuditLogEvent("", "SPILLAGE_RESPONSE_INITIATED", map[string]string{
		"action": "full_spillage_response",
	})

	var errors []error

	// Step 1: Clear all caches
	if err := d.SanitizeCache(); err != nil {
		errors = append(errors, fmt.Errorf("cache clear: %w", err))
	}

	// Step 2: Clear current session
	if err := d.SanitizeSession(); err != nil {
		errors = append(errors, fmt.Errorf("session clear: %w", err))
	}

	// Step 3: Clear all conversations (may contain spillage)
	if err := d.SanitizeConversations(); err != nil {
		errors = append(errors, fmt.Errorf("conversation clear: %w", err))
	}

	// Step 4: Clear temporary files
	if err := d.clearTempFiles(); err != nil {
		errors = append(errors, fmt.Errorf("temp clear: %w", err))
	}

	// Log completion
	AuditLogEvent("", "SPILLAGE_RESPONSE_COMPLETED", map[string]string{
		"errors": fmt.Sprintf("%d", len(errors)),
	})

	if len(errors) > 0 {
		return fmt.Errorf("spillage response completed with %d errors", len(errors))
	}

	return nil
}

// clearTempFiles clears any temporary files created by rigrun.
func (d *DataSanitizer) clearTempFiles() error {
	// Get temp directory
	tempDir := os.TempDir()
	rigrunTemp := filepath.Join(tempDir, "rigrun")

	if _, err := os.Stat(rigrunTemp); os.IsNotExist(err) {
		return nil // No temp directory
	}

	return d.SecureDeleteDirectory(rigrunTemp)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// overwriteFile overwrites a file with a specific byte pattern.
func overwriteFile(file *os.File, size int64, pattern byte) error {
	// Seek to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	// Create buffer of the pattern
	bufSize := int64(64 * 1024) // 64KB buffer
	buf := make([]byte, bufSize)
	for i := range buf {
		buf[i] = pattern
	}

	// Write in chunks
	written := int64(0)
	for written < size {
		toWrite := bufSize
		if written+toWrite > size {
			toWrite = size - written
		}

		n, err := file.Write(buf[:toWrite])
		if err != nil {
			return err
		}
		written += int64(n)
	}

	return nil
}

// overwriteFileRandom overwrites a file with random data.
func overwriteFileRandom(file *os.File, size int64) error {
	// Seek to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	// Create random buffer
	bufSize := int64(64 * 1024) // 64KB buffer
	buf := make([]byte, bufSize)

	// Write in chunks
	written := int64(0)
	for written < size {
		toWrite := bufSize
		if written+toWrite > size {
			toWrite = size - written
		}

		// Generate random data
		if _, err := rand.Read(buf[:toWrite]); err != nil {
			return err
		}

		n, err := file.Write(buf[:toWrite])
		if err != nil {
			return err
		}
		written += int64(n)
	}

	return nil
}

// removeEmptyDirs removes empty directories recursively bottom-up.
func removeEmptyDirs(path string) error {
	var dirs []string

	// Collect all directories
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})

	// Remove directories in reverse order (deepest first)
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // Ignore errors - dir may not be empty
	}

	return nil
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalSanitizer     *DataSanitizer
	globalSanitizerOnce sync.Once
)

// GlobalSanitizer returns the global data sanitizer instance.
func GlobalSanitizer() *DataSanitizer {
	globalSanitizerOnce.Do(func() {
		globalSanitizer = NewDataSanitizer()
	})
	return globalSanitizer
}
