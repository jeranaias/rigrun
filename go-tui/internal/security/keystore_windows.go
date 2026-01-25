// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows
// +build windows

// Package security provides Windows-specific key storage using DPAPI.
//
// DPAPI (Data Protection API) is a Windows built-in encryption mechanism that
// encrypts data using credentials derived from the current user's logon credentials.
// This provides secure storage without requiring a separate password.
package security

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// =============================================================================
// WINDOWS DPAPI KEY STORE
// =============================================================================

// WindowsKeyStore provides DPAPI-based key storage on Windows.
// The key is encrypted using the current user's credentials and stored in a file.
type WindowsKeyStore struct {
	path string
}

// NewKeyStore returns a Windows DPAPI-based key store.
func NewKeyStore() KeyStore {
	return &WindowsKeyStore{
		path: defaultKeyStorePath(),
	}
}

// Store encrypts the key using DPAPI and saves it to a file.
func (w *WindowsKeyStore) Store(key []byte) error {
	// Encrypt using DPAPI
	encrypted, err := dpAPIEncrypt(key)
	if err != nil {
		return fmt.Errorf("DPAPI encryption failed: %w", err)
	}

	// Ensure directory exists with restricted permissions
	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write encrypted key
	if err := os.WriteFile(w.path, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted key: %w", err)
	}

	return nil
}

// Retrieve reads the encrypted key and decrypts it using DPAPI.
func (w *WindowsKeyStore) Retrieve() ([]byte, error) {
	// Read encrypted key
	encrypted, err := os.ReadFile(w.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted key: %w", err)
	}

	// Decrypt using DPAPI
	key, err := dpAPIDecrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("DPAPI decryption failed: %w", err)
	}

	return key, nil
}

// Delete removes the encrypted key file.
func (w *WindowsKeyStore) Delete() error {
	if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key file: %w", err)
	}
	return nil
}

// Exists checks if the encrypted key file exists.
func (w *WindowsKeyStore) Exists() bool {
	_, err := os.Stat(w.path)
	return err == nil
}

// =============================================================================
// DPAPI IMPLEMENTATION
// =============================================================================

// DATA_BLOB is the Windows DPAPI data structure.
type dataBLOB struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree       = kernel32.NewProc("LocalFree")
)

// dpAPIEncrypt encrypts data using Windows DPAPI.
// The encryption is bound to the current user's credentials.
func dpAPIEncrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Prepare input blob
	dataIn := dataBLOB{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}

	// Prepare output blob
	var dataOut dataBLOB

	// Call CryptProtectData
	// Flags: CRYPTPROTECT_UI_FORBIDDEN (0x01) - don't show UI prompts
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&dataIn)),  // pDataIn
		0,                                  // szDataDescr (optional description)
		0,                                  // pOptionalEntropy (additional password)
		0,                                  // pvReserved
		0,                                  // pPromptStruct
		0x01,                               // dwFlags (CRYPTPROTECT_UI_FORBIDDEN)
		uintptr(unsafe.Pointer(&dataOut)), // pDataOut
	)

	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %w", err)
	}

	// Copy output data
	encrypted := make([]byte, dataOut.cbData)
	copy(encrypted, unsafe.Slice(dataOut.pbData, dataOut.cbData))

	// Free the output buffer
	procLocalFree.Call(uintptr(unsafe.Pointer(dataOut.pbData)))

	return encrypted, nil
}

// dpAPIDecrypt decrypts data using Windows DPAPI.
func dpAPIDecrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Prepare input blob
	dataIn := dataBLOB{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}

	// Prepare output blob
	var dataOut dataBLOB

	// Call CryptUnprotectData
	// Flags: CRYPTPROTECT_UI_FORBIDDEN (0x01) - don't show UI prompts
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&dataIn)),  // pDataIn
		0,                                  // ppszDataDescr (optional, receives description)
		0,                                  // pOptionalEntropy (additional password, must match encrypt)
		0,                                  // pvReserved
		0,                                  // pPromptStruct
		0x01,                               // dwFlags (CRYPTPROTECT_UI_FORBIDDEN)
		uintptr(unsafe.Pointer(&dataOut)), // pDataOut
	)

	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %w", err)
	}

	// Copy output data
	decrypted := make([]byte, dataOut.cbData)
	copy(decrypted, unsafe.Slice(dataOut.pbData, dataOut.cbData))

	// Free the output buffer
	procLocalFree.Call(uintptr(unsafe.Pointer(dataOut.pbData)))

	return decrypted, nil
}
