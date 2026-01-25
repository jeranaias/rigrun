//go:build !windows
// +build !windows

// audit_hmac_key_acl_unix.go - Unix stub for Windows ACL verification
//
// On Unix systems, file permissions are checked using standard mode bits
// in the main audit_hmac_key.go file. This stub exists to satisfy the
// compiler when building for non-Windows platforms.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

// verifyWindowsACL is a stub for Unix systems.
// On Unix, file permissions are verified using standard mode bits in checkFilePermissions.
// This function should never be called on Unix systems as the runtime.GOOS check
// prevents it, but exists for compilation purposes.
func verifyWindowsACL(path string) error {
	// This should never be called on Unix - the runtime.GOOS check in
	// checkFilePermissions prevents it. Panic if somehow reached.
	panic("verifyWindowsACL called on non-Windows platform")
}
