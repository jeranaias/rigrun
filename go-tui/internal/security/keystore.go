// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides platform-agnostic key storage interface.
//
// This file defines the KeyStore interface and provides the factory function
// that returns the appropriate implementation based on the platform.
package security

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// KEYSTORE INTERFACE
// =============================================================================

// KeyStore defines the interface for secure key storage.
// Implementations provide platform-specific secure storage:
// - Windows: DPAPI (Data Protection API)
// - macOS: Keychain
// - Linux: libsecret or encrypted file fallback
type KeyStore interface {
	// Store securely stores the encryption key.
	Store(key []byte) error
	// Retrieve retrieves the encryption key from secure storage.
	Retrieve() ([]byte, error)
	// Delete removes the key from secure storage.
	Delete() error
	// Exists checks if a key is stored.
	Exists() bool
}

// =============================================================================
// FILE-BASED KEYSTORE (FALLBACK)
// =============================================================================

// FileKeyStore provides a file-based key storage implementation.
// This is used as a fallback when platform-specific secure storage is not available.
// The key file is stored with restricted permissions (0600).
type FileKeyStore struct {
	path string
}

// NewFileKeyStore creates a new file-based key store.
func NewFileKeyStore(path string) *FileKeyStore {
	return &FileKeyStore{path: path}
}

// Store saves the key to a file with restricted permissions.
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func (f *FileKeyStore) Store(key []byte) error {
	// Ensure directory exists with restricted permissions
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	// Write key with restricted permissions (owner read/write only)
	if err := util.AtomicWriteFile(f.path, key, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// Retrieve reads the key from the file.
func (f *FileKeyStore) Retrieve() ([]byte, error) {
	key, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	return key, nil
}

// Delete removes the key file.
func (f *FileKeyStore) Delete() error {
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key file: %w", err)
	}
	return nil
}

// Exists checks if the key file exists.
func (f *FileKeyStore) Exists() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// defaultKeyStorePath returns the default path for key storage.
func defaultKeyStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".rigrun", "master.key")
	}
	return filepath.Join(home, ".rigrun", "master.key")
}
