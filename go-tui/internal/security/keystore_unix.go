// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows
// +build !windows

// Package security provides Unix key storage implementation.
//
// On Unix systems (Linux, macOS, BSD), we use an encrypted file-based
// storage as a fallback. For enhanced security on macOS, users can
// leverage Keychain through the CLI.
//
// The key file is stored with restricted permissions (0600) and the
// directory with 0700 permissions.
package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// =============================================================================
// UNIX KEY STORE
// =============================================================================

// UnixKeyStore provides file-based key storage on Unix systems.
// The key is stored in a file with restricted permissions.
//
// For enhanced security, consider using:
// - macOS: Keychain Access via `security` command
// - Linux: libsecret/gnome-keyring via `secret-tool`
//
// This implementation serves as a portable fallback.
type UnixKeyStore struct {
	path string
}

// NewKeyStore returns a Unix file-based key store.
func NewKeyStore() KeyStore {
	return &UnixKeyStore{
		path: defaultKeyStorePath(),
	}
}

// Store saves the key to a file with restricted permissions.
// On Unix, the file is protected by filesystem permissions (0600).
func (u *UnixKeyStore) Store(key []byte) error {
	// Ensure directory exists with restricted permissions (0700 = rwx------)
	dir := filepath.Dir(u.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Check if directory permissions are correct
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat key directory: %w", err)
	}

	// IL5 SECURITY: Verify directory has no group/world permissions
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return fmt.Errorf("IL5 SECURITY ERROR: key directory has insecure permissions (%o). "+
			"Directory must have mode 0700 or more restrictive. "+
			"Fix with: chmod 700 %s", mode, dir)
	}

	// Write key with restricted permissions (0600 = rw-------)
	if err := os.WriteFile(u.path, key, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	// IL5 SECURITY: Verify file permissions are strict
	fileInfo, err := os.Stat(u.path)
	if err != nil {
		return fmt.Errorf("failed to stat key file: %w", err)
	}

	fileMode := fileInfo.Mode().Perm()
	if fileMode&0077 != 0 {
		// Delete the insecure key file immediately
		_ = os.Remove(u.path)
		return fmt.Errorf("IL5 SECURITY ERROR: key file was created with insecure permissions (%o). "+
			"File must have mode 0600 or more restrictive. "+
			"The insecure file has been deleted. Please retry the operation.", fileMode)
	}

	return nil
}

// Retrieve reads the key from the file.
func (u *UnixKeyStore) Retrieve() ([]byte, error) {
	// IL5 SECURITY: Verify directory permissions before reading
	dir := filepath.Dir(u.path)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat key directory: %w", err)
	}

	dirMode := dirInfo.Mode().Perm()
	if dirMode&0077 != 0 {
		return nil, fmt.Errorf("IL5 SECURITY ERROR: key directory has insecure permissions (%o). "+
			"Directory must have mode 0700 or more restrictive. "+
			"Fix with: chmod 700 %s", dirMode, dir)
	}

	// IL5 SECURITY: Verify file permissions before reading
	info, err := os.Stat(u.path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat key file: %w", err)
	}

	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return nil, fmt.Errorf("IL5 SECURITY ERROR: key file has insecure permissions (%o). "+
			"File must have mode 0600 or more restrictive. "+
			"Fix with: chmod 600 %s", mode, u.path)
	}

	key, err := os.ReadFile(u.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	return key, nil
}

// Delete removes the key file using secure deletion.
// Attempts to overwrite the file before deletion.
func (u *UnixKeyStore) Delete() error {
	// Read file size
	info, err := os.Stat(u.path)
	if os.IsNotExist(err) {
		return nil // Already deleted
	}
	if err != nil {
		return fmt.Errorf("failed to stat key file for deletion: %w", err)
	}

	// Attempt secure deletion by overwriting with zeros
	size := info.Size()
	if size > 0 {
		zeros := make([]byte, size)
		if f, err := os.OpenFile(u.path, os.O_WRONLY, 0600); err == nil {
			_, _ = f.Write(zeros)
			_ = f.Sync()
			_ = f.Close()
		}
	}

	// Delete the file
	if err := os.Remove(u.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key file: %w", err)
	}

	return nil
}

// Exists checks if the key file exists.
func (u *UnixKeyStore) Exists() bool {
	_, err := os.Stat(u.path)
	return err == nil
}

// =============================================================================
// MACOS KEYCHAIN HELPER (OPTIONAL)
// =============================================================================

// NOTE: For enhanced security on macOS, you can use the Keychain.
// This would require executing the `security` command or using
// the go-keychain library.
//
// Example using security command:
//   security add-generic-password -a "rigrun" -s "encryption-key" -w <key>
//   security find-generic-password -a "rigrun" -s "encryption-key" -w
//   security delete-generic-password -a "rigrun" -s "encryption-key"
//
// For Linux, you can use libsecret via:
//   secret-tool store --label='rigrun encryption key' service rigrun key master
//   secret-tool lookup service rigrun key master
//   secret-tool clear service rigrun key master
