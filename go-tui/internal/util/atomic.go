// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package util provides utility functions for the go-tui application.
package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// RELIABILITY: Atomic write with fsync prevents data loss on crash
//
// AtomicWriteFile writes data to a file atomically using the following pattern:
// 1. Write to a temporary file in the same directory
// 2. Sync the data to disk using fsync
// 3. Close the file
// 4. Atomically rename the temp file to the target path
//
// This ensures that:
// - The file is never partially written
// - Data is persisted to disk before the operation completes
// - On crash, either the old file or the new complete file exists
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Get absolute path and ensure parent directory exists
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create temporary file in the same directory (required for atomic rename)
	// Using the same directory ensures the rename is atomic on the same filesystem
	f, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := f.Name()

	// Ensure cleanup on any error
	success := false
	defer func() {
		if !success {
			f.Close()
			os.Remove(tempPath)
		}
	}()

	// Write data to temp file
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// RELIABILITY: Sync to disk - ensures data is persisted before rename
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync data to disk: %w", err)
	}

	// Close before rename - required on some systems (Windows)
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions on temp file before rename
	if err := os.Chmod(tempPath, perm); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename - replaces target file atomically
	if err := os.Rename(tempPath, absPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// AtomicWriteFileWithDir is like AtomicWriteFile but also allows specifying
// the permissions for the parent directory if it needs to be created.
func AtomicWriteFileWithDir(path string, data []byte, filePerm, dirPerm os.FileMode) error {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create temporary file in the same directory
	f, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := f.Name()

	// Ensure cleanup on any error
	success := false
	defer func() {
		if !success {
			f.Close()
			os.Remove(tempPath)
		}
	}()

	// Write data
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// RELIABILITY: Sync to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync data to disk: %w", err)
	}

	// Close before rename
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(tempPath, filePerm); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, absPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}
