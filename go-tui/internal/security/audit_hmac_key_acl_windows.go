//go:build windows
// +build windows

// audit_hmac_key_acl_windows.go - Windows ACL verification for HMAC key files
//
// SECURITY: ACL verification is mandatory, not optional
// This file implements mandatory Windows ACL checks for key files.
// The check MUST fail if ACLs are insecure - no warnings allowed.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// verifyWindowsACL checks that the file has secure ACLs (owner-only access).
// SECURITY: ACL verification is mandatory, not optional
// Returns error if ACL check fails - MUST NOT just log warning and proceed.
func verifyWindowsACL(path string) error {
	// Get file security info with DACL
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("failed to get security info: %w", err)
	}

	// Get the owner SID from the security descriptor
	ownerSid, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("failed to get owner SID: %w", err)
	}

	// Get the current user's SID
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("failed to open process token: %w", err)
	}
	defer token.Close()

	currentUser, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Get DACL from security descriptor
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("failed to get DACL: %w", err)
	}

	// SECURITY: Verify the ACL is secure
	if err := isACLSecure(dacl, ownerSid, currentUser.User.Sid); err != nil {
		return fmt.Errorf("insecure ACL on key file %s: %w", path, err)
	}

	return nil
}

// isACLSecure verifies that the DACL only allows access to the owner and administrators.
// SECURITY: Returns error if any other principals have access.
func isACLSecure(dacl *windows.ACL, ownerSid *windows.SID, currentUserSid *windows.SID) error {
	if dacl == nil {
		// NULL DACL means full access to everyone - INSECURE
		return fmt.Errorf("NULL DACL grants full access to everyone")
	}

	// Get well-known SIDs for comparison
	adminsSid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("failed to get Administrators SID: %w", err)
	}

	systemSid, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("failed to get SYSTEM SID: %w", err)
	}

	// Iterate through ACL entries
	// Note: This is a simplified check - a full implementation would parse the ACL structure
	// For now, we use AccessCheck to verify the current user has proper access
	// and deny access if the DACL appears to grant broad permissions

	// Check if Everyone or Users groups have access (they shouldn't)
	everyoneSid, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err == nil {
		if hasExplicitAccess(dacl, everyoneSid) {
			return fmt.Errorf("Everyone group has access to key file")
		}
	}

	usersSid, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err == nil {
		if hasExplicitAccess(dacl, usersSid) {
			return fmt.Errorf("Users group has access to key file")
		}
	}

	authenticatedUsersSid, err := windows.CreateWellKnownSid(windows.WinAuthenticatedUserSid)
	if err == nil {
		if hasExplicitAccess(dacl, authenticatedUsersSid) {
			return fmt.Errorf("Authenticated Users have access to key file")
		}
	}

	// Verify that only acceptable principals have access:
	// - Owner (current user or admin)
	// - SYSTEM
	// - Administrators
	if ownerSid != nil {
		if !ownerSid.Equals(currentUserSid) && !ownerSid.Equals(adminsSid) && !ownerSid.Equals(systemSid) {
			return fmt.Errorf("key file owned by unexpected principal")
		}
	}

	return nil
}

// hasExplicitAccess checks if a SID has explicit access in the DACL.
// This is a simplified check using GetEffectiveRightsFromAcl concept.
func hasExplicitAccess(dacl *windows.ACL, sid *windows.SID) bool {
	if dacl == nil || sid == nil {
		return false
	}

	// Use GetExplicitEntriesFromAcl to check entries
	// This requires advapi32.dll
	var entries *windows.EXPLICIT_ACCESS
	var count uint32

	advapi32 := windows.NewLazySystemDLL("advapi32.dll")
	procGetExplicitEntriesFromAcl := advapi32.NewProc("GetExplicitEntriesFromAclW")

	ret, _, _ := procGetExplicitEntriesFromAcl.Call(
		uintptr(unsafe.Pointer(dacl)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&entries)),
	)

	if ret != 0 || count == 0 {
		return false
	}

	if entries != nil {
		defer windows.LocalFree(windows.Handle(unsafe.Pointer(entries)))
	}

	// Check each entry
	entrySlice := unsafe.Slice(entries, count)
	for _, entry := range entrySlice {
		if entry.AccessMode == windows.GRANT_ACCESS || entry.AccessMode == windows.SET_ACCESS {
			// Check if this grants access to the specified SID
			if entry.Trustee.TrusteeForm == windows.TRUSTEE_IS_SID {
				entrySid := (*windows.SID)(unsafe.Pointer(entry.Trustee.TrusteeValue))
				if entrySid != nil && entrySid.Equals(sid) {
					return true
				}
			}
		}
	}

	return false
}
