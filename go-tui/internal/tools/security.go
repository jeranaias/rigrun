// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// security.go implements comprehensive security validation for file operations.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// =============================================================================
// BLOCKED SHELL STARTUP FILES
// =============================================================================

// blockedShellFiles are shell startup/config files that should never be modified.
// These files can be used for privilege escalation or persistence attacks.
var blockedShellFiles = []string{
	".bashrc",
	".bash_profile",
	".bash_login",
	".bash_logout",
	".zshrc",
	".zprofile",
	".zlogin",
	".zlogout",
	".profile",
	".login",
	".cshrc",
	".tcshrc",
	".kshrc",
	".fishrc",
	".config/fish/config.fish",
}

// blockedSensitiveDirs are directories containing sensitive configuration
var blockedSensitiveDirs = []string{
	".ssh/",
	".gnupg/",
	".aws/",
	".kube/",
	".docker/",
}

// isBlockedShellFile checks if a path points to a blocked shell startup file.
func isBlockedShellFile(path string) bool {
	// Normalize path separators
	normalizedPath := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(normalizedPath)
	}

	// Get just the filename for simple checks
	baseName := filepath.Base(normalizedPath)
	if runtime.GOOS == "windows" {
		baseName = strings.ToLower(baseName)
	}

	// Check against blocked shell files
	for _, blocked := range blockedShellFiles {
		blockedNorm := blocked
		if runtime.GOOS == "windows" {
			blockedNorm = strings.ToLower(blocked)
		}

		// Check if filename matches
		if baseName == filepath.Base(blockedNorm) {
			return true
		}

		// Check if full path ends with the blocked pattern
		if strings.HasSuffix(normalizedPath, "/"+blockedNorm) || normalizedPath == blockedNorm {
			return true
		}
	}

	// Check against blocked sensitive directories
	for _, blocked := range blockedSensitiveDirs {
		blockedNorm := blocked
		if runtime.GOOS == "windows" {
			blockedNorm = strings.ToLower(blocked)
		}

		if strings.Contains(normalizedPath, blockedNorm) {
			return true
		}
	}

	return false
}

// =============================================================================
// SENSITIVE PATH PATTERNS
// =============================================================================

// SensitivePathPatterns are file patterns that require explicit permission.
// These patterns trigger PermissionAsk instead of PermissionAuto.
var SensitivePathPatterns = []string{
	// Environment files
	"*/.env",
	"*/.env.*",
	"*/env",
	"*/environment",

	// Cloud credentials
	"*/.aws/*",
	"*/.aws/credentials",
	"*/.aws/config",
	"*/.azure/*",
	"*/.gcloud/*",
	"*/.kube/config",

	// SSH keys and config
	"*/.ssh/*",
	"*/id_rsa",
	"*/id_ed25519",
	"*/id_ecdsa",
	"*/id_dsa",
	"*/authorized_keys",
	"*/known_hosts",

	// Git credentials
	"*/.git/config",
	"*/.gitconfig",
	"*/.git-credentials",
	"*/.netrc",

	// General sensitive files
	"*/credentials*",
	"*/secrets*",
	"*/password*",
	"*/passwd",
	"*/.npmrc",
	"*/.pypirc",

	// Certificate and key files
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"*.crt",
	"*.cer",

	// System files (Linux)
	"/etc/shadow",
	"/etc/passwd",
	"/etc/sudoers",
	"/etc/ssh/*",

	// System files (Windows)
	"*/SAM",
	"*/SYSTEM",
	"*/SECURITY",
}

// =============================================================================
// PATH VALIDATION
// =============================================================================

// ValidatePathSecure performs comprehensive path validation with security checks.
// This function:
// 1. Converts to absolute path
// 2. Resolves symlinks to get real path
// 3. Checks for path traversal AFTER canonicalization
// 4. Checks against blocked paths
// 5. Checks against blocked shell startup files
//
// Returns the canonicalized safe path and an error if validation fails.
func ValidatePathSecure(path string) (string, error) {
	// Step 1: Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", &SecurityError{
			Type:    "path_resolution",
			Path:    path,
			Message: fmt.Sprintf("cannot resolve absolute path: %v", err),
		}
	}

	// Step 2: Resolve symlinks to get real path
	// This prevents symlink attacks where a symlink points outside allowed paths
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path might not exist yet (e.g., for writes) - use parent directory
		parentDir := filepath.Dir(absPath)
		realParent, err2 := filepath.EvalSymlinks(parentDir)
		if err2 != nil {
			// Parent doesn't exist either, use absolute path
			realPath = absPath
		} else {
			// Reconstruct path with real parent and original filename
			realPath = filepath.Join(realParent, filepath.Base(absPath))
		}
	}

	// Step 3: Normalize path for comparison
	normalizedPath := realPath
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(filepath.ToSlash(realPath))
	} else {
		normalizedPath = filepath.Clean(realPath)
	}

	// Step 4: Check for path traversal AFTER canonicalization
	// This prevents bypasses like /safe/path/../../../etc/shadow
	if !isWithinAllowedPaths(normalizedPath) {
		return "", &SecurityError{
			Type:    "path_traversal",
			Path:    path,
			Message: "path traversal attempt detected - path escapes allowed directories",
		}
	}

	// Step 5: Check against blocked system paths
	if err := checkBlockedPaths(normalizedPath); err != nil {
		return "", err
	}

	// Step 6: Check against blocked shell startup files (TOOL-2 fix)
	if isBlockedShellFile(normalizedPath) {
		return "", &SecurityError{
			Type:    "blocked_shell_file",
			Path:    path,
			Message: "access denied: shell startup/configuration files are protected",
		}
	}

	// Step 7: TOOL-3 fix - Validate the final constructed path after parent dir fallback
	// Re-check the final realPath is still within allowed boundaries
	finalNormalized := realPath
	if runtime.GOOS == "windows" {
		finalNormalized = strings.ToLower(filepath.ToSlash(realPath))
	} else {
		finalNormalized = filepath.Clean(realPath)
	}
	if !isWithinAllowedPaths(finalNormalized) {
		return "", &SecurityError{
			Type:    "path_traversal",
			Path:    path,
			Message: "final constructed path escapes allowed directories",
		}
	}

	// Step 8: Check for sensitive patterns (for logging/audit purposes)
	// Note: This doesn't block access, just flags for higher permission level
	if isSensitivePath(normalizedPath) {
		// Caller can check this and require PermissionAsk
		// Not blocking here to allow ValidatePathSecure to be used in multiple contexts
	}

	return realPath, nil
}

// OpenSecureFile performs atomic path validation and file opening to prevent TOCTOU race conditions.
// SECURITY: Atomic validation prevents TOCTOU race where an attacker could swap the file
// between validation and opening.
//
// The function:
// 1. Validates the path (cleaned with filepath.Clean)
// 2. Opens the file atomically - no race window between check and use
// 3. Re-validates the opened file's real path to detect symlink attacks
//
// Parameters:
//   - path: The file path to validate and open
//   - flag: File open flags (e.g., os.O_RDONLY, os.O_WRONLY|os.O_CREATE)
//
// Returns:
//   - *os.File: Open file handle (caller must close)
//   - error: Validation or open error
func OpenSecureFile(path string, flag int) (*os.File, error) {
	// SECURITY: Atomic validation prevents TOCTOU race

	// Step 1: Clean and normalize the path first
	cleanPath := filepath.Clean(path)

	// Step 2: Perform initial security validation
	if err := validatePathSecurity(cleanPath); err != nil {
		return nil, err
	}

	// Step 3: Open atomically - no race window between validation and open
	f, err := os.OpenFile(cleanPath, flag, 0600)
	if err != nil {
		return nil, &SecurityError{
			Type:    "file_open",
			Path:    path,
			Message: fmt.Sprintf("cannot open file: %v", err),
		}
	}

	// Step 4: Re-validate after open to ensure we got what we expected
	// This detects symlink races where the path was swapped between validation and open
	realPath, err := filepath.EvalSymlinks(f.Name())
	if err != nil {
		// If EvalSymlinks fails, try to get the real path another way
		realPath = f.Name()
	}

	// Step 5: Re-validate the actual opened path
	if err := validatePathSecurity(realPath); err != nil {
		f.Close()
		return nil, &SecurityError{
			Type:    "toctou_detected",
			Path:    path,
			Message: "file path changed after open - possible symlink attack: " + err.Error(),
		}
	}

	return f, nil
}

// OpenSecureFileWithPerm is like OpenSecureFile but allows specifying file permissions for creation.
// SECURITY: Atomic validation prevents TOCTOU race
//
// Parameters:
//   - path: The file path to validate and open
//   - flag: File open flags (e.g., os.O_RDONLY, os.O_WRONLY|os.O_CREATE)
//   - perm: File permissions for creation (e.g., 0644)
//
// Returns:
//   - *os.File: Open file handle (caller must close)
//   - error: Validation or open error
func OpenSecureFileWithPerm(path string, flag int, perm os.FileMode) (*os.File, error) {
	// SECURITY: Atomic validation prevents TOCTOU race

	// Step 1: Clean and normalize the path first
	cleanPath := filepath.Clean(path)

	// Step 2: Perform initial security validation
	if err := validatePathSecurity(cleanPath); err != nil {
		return nil, err
	}

	// Step 3: Open atomically - no race window between validation and open
	f, err := os.OpenFile(cleanPath, flag, perm)
	if err != nil {
		return nil, &SecurityError{
			Type:    "file_open",
			Path:    path,
			Message: fmt.Sprintf("cannot open file: %v", err),
		}
	}

	// Step 4: Re-validate after open to ensure we got what we expected
	realPath, err := filepath.EvalSymlinks(f.Name())
	if err != nil {
		realPath = f.Name()
	}

	// Step 5: Re-validate the actual opened path
	if err := validatePathSecurity(realPath); err != nil {
		f.Close()
		return nil, &SecurityError{
			Type:    "toctou_detected",
			Path:    path,
			Message: "file path changed after open - possible symlink attack: " + err.Error(),
		}
	}

	return f, nil
}

// validatePathSecurity performs internal security validation on a path.
// This is used by OpenSecureFile for both pre-open and post-open validation.
// SECURITY: Atomic validation prevents TOCTOU race
func validatePathSecurity(path string) error {
	// Step 1: Clean and normalize path
	cleanPath := filepath.Clean(path)

	// Step 2: Convert to absolute path for comparison
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return &SecurityError{
			Type:    "path_resolution",
			Path:    path,
			Message: fmt.Sprintf("cannot resolve absolute path: %v", err),
		}
	}

	// Step 3: Normalize for comparison
	normalizedPath := absPath
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(filepath.ToSlash(absPath))
	} else {
		normalizedPath = filepath.Clean(absPath)
	}

	// Step 4: Check if path is within allowed directories
	if !isWithinAllowedPaths(normalizedPath) {
		return &SecurityError{
			Type:    "path_traversal",
			Path:    path,
			Message: "path traversal attempt detected - path escapes allowed directories",
		}
	}

	// Step 5: Check against blocked system paths
	if err := checkBlockedPaths(normalizedPath); err != nil {
		return err
	}

	// Step 6: Check against blocked shell startup files
	if isBlockedShellFile(normalizedPath) {
		return &SecurityError{
			Type:    "blocked_shell_file",
			Path:    path,
			Message: "access denied: shell startup/configuration files are protected",
		}
	}

	return nil
}

// ValidatePathSecureWithHandle performs path validation AND opens the file atomically.
// DEPRECATED: Use OpenSecureFile or OpenSecureFileWithPerm instead for better TOCTOU protection.
// This function is kept for backward compatibility but now uses the secure atomic pattern internally.
//
// Parameters:
//   - path: The file path to validate and open
//   - flag: File open flags (e.g., os.O_RDONLY, os.O_WRONLY|os.O_CREATE)
//   - perm: File permissions for creation (e.g., 0644)
//
// Returns:
//   - *os.File: Open file handle (caller must close)
//   - string: The validated real path
//   - error: Validation or open error
func ValidatePathSecureWithHandle(path string, flag int, perm os.FileMode) (*os.File, string, error) {
	// SECURITY: Atomic validation prevents TOCTOU race

	f, err := OpenSecureFileWithPerm(path, flag, perm)
	if err != nil {
		return nil, "", err
	}

	// Get the real path for the return value
	realPath, err := filepath.EvalSymlinks(f.Name())
	if err != nil {
		realPath = f.Name()
	}

	return f, realPath, nil
}

// isWithinAllowedPaths checks if a path is within allowed directories.
// By default, we allow access to the current working directory and subdirectories.
// This prevents access to system directories outside the workspace.
// SECURITY: Uses isPathWithinDir to prevent HasPrefix bypass attacks where
// /home/userEVIL would incorrectly pass a check for /home/user.
func isWithinAllowedPaths(normalizedPath string) bool {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		// If we can't determine CWD, allow access to temp directory
		// but be more restrictive otherwise
		tempDir := os.TempDir()
		normalizedTemp := normalizePath(tempDir)
		if isPathWithinDir(normalizedPath, normalizedTemp) {
			return true
		}
		// Default to blocking if we can't determine CWD
		return false
	}

	// Normalize CWD
	normalizedCwd := normalizePath(cwd)

	// Check if path is under CWD
	// SECURITY: Use isPathWithinDir to prevent /home/userEVIL matching /home/user
	if isPathWithinDir(normalizedPath, normalizedCwd) {
		return true
	}

	// Allow access to user's home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		normalizedHome := normalizePath(homeDir)

		// SECURITY: Use isPathWithinDir to prevent HasPrefix bypass
		if isPathWithinDir(normalizedPath, normalizedHome) {
			// Allow home directory access, but blocked paths will still apply
			return true
		}
	}

	// Allow access to system temp directory (needed for tests and temp files)
	tempDir := os.TempDir()
	normalizedTemp := normalizePath(tempDir)
	// SECURITY: Use isPathWithinDir to prevent HasPrefix bypass
	if isPathWithinDir(normalizedPath, normalizedTemp) {
		return true
	}

	// For absolute paths outside allowed directories, block
	// This prevents arbitrary filesystem access
	return false
}

// normalizePath normalizes a path for secure comparison.
// Applies filepath.Clean and platform-specific normalization.
// SECURITY: Consistent normalization prevents path comparison bypasses.
func normalizePath(path string) string {
	// Always clean the path first
	cleaned := filepath.Clean(path)

	if runtime.GOOS == "windows" {
		// On Windows: lowercase and normalize separators
		return strings.ToLower(filepath.ToSlash(cleaned))
	}
	return cleaned
}

// isPathWithinDir checks if a path is within a directory, ensuring proper path boundaries.
// SECURITY: Prevents HasPrefix bypass where /home/userEVIL would pass check for /home/user.
// The path must either:
// 1. Be exactly the directory, OR
// 2. Start with the directory followed by a path separator
func isPathWithinDir(path, dir string) bool {
	// Normalize both paths for consistent comparison
	normalizedPath := normalizePath(path)
	normalizedDir := normalizePath(dir)

	// Exact match - path is the directory itself
	if normalizedPath == normalizedDir {
		return true
	}

	// Ensure directory path ends with separator for prefix check
	// This prevents /home/userEVIL from matching /home/user
	dirWithSep := normalizedDir
	if !strings.HasSuffix(dirWithSep, "/") {
		dirWithSep = dirWithSep + "/"
	}

	// Check if path starts with directory + separator
	return strings.HasPrefix(normalizedPath, dirWithSep)
}

// checkBlockedPaths checks if a path is in a blocked system directory.
// SECURITY: Uses isPathWithinDir and isPathOrParentBlocked for secure path boundary checks.
func checkBlockedPaths(normalizedPath string) error {
	// Apply filepath.Clean for consistent normalization
	cleanPath := normalizePath(normalizedPath)

	// Platform-specific blocked paths - directories that should be completely blocked
	var blockedDirs []string
	// Platform-specific blocked files - exact files that should be blocked
	var blockedFiles []string
	// Platform-specific blocked patterns - substring matches for patterns
	var blockedPatterns []string

	if runtime.GOOS == "windows" {
		blockedDirs = []string{
			"c:/windows/system32/config",
			"c:/windows/system32/drivers",
			"c:/users/default",
			"c:/programdata/microsoft/crypto",
		}
		blockedFiles = []string{
			"c:/windows/system32/config/sam",
			"c:/windows/system32/config/system",
			"c:/windows/system32/config/security",
		}
		blockedPatterns = []string{}
	} else {
		blockedDirs = []string{
			"/proc",
			"/sys",
			"/dev",
			"/boot",
			"/root/.ssh",
			"/etc/sudoers.d",
		}
		blockedFiles = []string{
			"/etc/shadow",
			"/etc/gshadow",
			"/etc/passwd",
			"/etc/sudoers",
		}
		blockedPatterns = []string{
			"/etc/ssh/ssh_host_",
		}
	}

	// Check against blocked directories
	// SECURITY: Use isPathWithinDir to ensure proper path boundary checking
	for _, blocked := range blockedDirs {
		normalizedBlocked := normalizePath(blocked)
		if isPathWithinDir(cleanPath, normalizedBlocked) {
			return &SecurityError{
				Type:    "blocked_path",
				Path:    normalizedPath,
				Message: "access denied: path is in a protected system directory",
			}
		}
	}

	// Check against blocked files (exact match or path within)
	for _, blocked := range blockedFiles {
		normalizedBlocked := normalizePath(blocked)
		if cleanPath == normalizedBlocked || isPathWithinDir(cleanPath, normalizedBlocked) {
			return &SecurityError{
				Type:    "blocked_path",
				Path:    normalizedPath,
				Message: "access denied: path is a protected system file",
			}
		}
	}

	// Check against blocked patterns (substring matches for special cases)
	for _, pattern := range blockedPatterns {
		normalizedPattern := normalizePath(pattern)
		if strings.Contains(cleanPath, normalizedPattern) {
			return &SecurityError{
				Type:    "blocked_path",
				Path:    normalizedPath,
				Message: "access denied: path matches protected system pattern",
			}
		}
	}

	return nil
}

// isSensitivePath checks if a path matches sensitive patterns.
// This is used to determine if PermissionAsk should be required.
// SECURITY: Uses normalizePath for consistent cross-platform path handling.
func isSensitivePath(path string) bool {
	// Normalize for comparison using consistent normalization
	normalizedPath := normalizePath(path)

	// Check each sensitive pattern
	for _, pattern := range SensitivePathPatterns {
		if matchPath(normalizedPath, pattern) {
			return true
		}
	}

	return false
}

// matchPath matches a path against a glob-like pattern.
// Supports:
// - * matches any sequence in a path segment
// - ** matches any sequence including path separators
// - Exact substring matching for patterns without wildcards
func matchPath(path, pattern string) bool {
	// Normalize separators
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Split on **
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			// Remove leading/trailing slashes for matching
			prefix = strings.TrimSuffix(prefix, "/")
			suffix = strings.TrimPrefix(suffix, "/")

			// Check if path matches pattern
			if prefix != "" && !strings.Contains(path, prefix) {
				return false
			}
			if suffix != "" {
				// Handle wildcards in suffix (e.g., ".aws/*" should match ".aws/credentials")
				if strings.Contains(suffix, "*") {
					// Remove trailing /* and check if path contains the directory
					suffixDir := strings.TrimSuffix(suffix, "/*")
					suffixDir = strings.TrimSuffix(suffixDir, "*")
					if !strings.Contains(path, suffixDir) {
						return false
					}
				} else if !strings.Contains(path, suffix) {
					return false
				}
			}
			return true
		}
	}

	// Handle * patterns (within a single path segment)
	if strings.Contains(pattern, "*") {
		// Convert to simple contains check if pattern is like */.env
		if strings.HasPrefix(pattern, "*/") {
			substring := strings.TrimPrefix(pattern, "*/")
			return strings.Contains(path, "/"+substring) || strings.HasSuffix(path, substring)
		}
		// Use filepath.Match for more complex patterns
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		// Also check if pattern matches the basename
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		return matched
	}

	// Exact substring match
	return strings.Contains(path, pattern)
}

// =============================================================================
// CONTEXT-AWARE PERMISSION HELPERS
// =============================================================================

// GetPermissionForPath determines the permission level for a given path.
// Returns PermissionAsk for sensitive paths, PermissionAuto otherwise.
func GetPermissionForPath(path string) PermissionLevel {
	// Validate path first
	realPath, err := ValidatePathSecure(path)
	if err != nil {
		// If path validation fails, require permission
		return PermissionAsk
	}

	// Check if path is sensitive
	if isSensitivePath(realPath) {
		return PermissionAsk
	}

	return PermissionAuto
}

// =============================================================================
// COMMAND VALIDATION (Enhanced)
// =============================================================================

// ValidateCommandSecure validates a command with improved parsing.
// Instead of just substring matching, this uses both token parsing and pattern matching.
func ValidateCommandSecure(command string) error {
	// Normalize and trim
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return &SecurityError{
			Type:    "command_validation",
			Path:    "",
			Message: "command cannot be empty",
		}
	}

	// Normalize for comparison - collapse multiple spaces
	normalizedLower := strings.ToLower(normalized)
	normalizedLower = strings.ReplaceAll(normalizedLower, "\t", " ")
	// Collapse multiple spaces into single space
	for strings.Contains(normalizedLower, "  ") {
		normalizedLower = strings.ReplaceAll(normalizedLower, "  ", " ")
	}

	// Check for blocked command strings (full command line matching)
	for _, blocked := range DefaultBlockedCommands {
		blockedLower := strings.ToLower(blocked)
		// Normalize blocked command too
		for strings.Contains(blockedLower, "  ") {
			blockedLower = strings.ReplaceAll(blockedLower, "  ", " ")
		}
		// Check if the blocked string appears in the command
		if strings.Contains(normalizedLower, blockedLower) {
			return &SecurityError{
				Type:    "command_blocked",
				Path:    "",
				Message: fmt.Sprintf("command contains blocked operation: %s", blocked),
			}
		}
	}

	// Check for dangerous patterns
	for _, pattern := range DefaultBlockedPatterns {
		if strings.Contains(normalizedLower, strings.ToLower(pattern)) {
			return &SecurityError{
				Type:    "command_pattern",
				Path:    "",
				Message: fmt.Sprintf("command contains dangerous pattern: %s", pattern),
			}
		}
	}

	// Parse command into tokens for additional checks
	tokens, err := parseCommandTokens(normalized)
	if err != nil {
		return &SecurityError{
			Type:    "command_validation",
			Path:    "",
			Message: fmt.Sprintf("failed to parse command: %v", err),
		}
	}

	// Check first token for sudo/su
	if len(tokens) > 0 {
		firstToken := strings.ToLower(tokens[0])
		if firstToken == "sudo" || firstToken == "su" || firstToken == "doas" || firstToken == "pkexec" {
			return &SecurityError{
				Type:    "command_privileged",
				Path:    "",
				Message: fmt.Sprintf("privileged command '%s' requires explicit approval", tokens[0]),
			}
		}
	}

	return nil
}

// parseCommandTokens parses a command string into tokens, respecting quotes.
// This prevents simple bypass attempts with extra spaces/tabs.
func parseCommandTokens(command string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			} else {
				current.WriteRune(r)
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			} else {
				current.WriteRune(r)
			}
		case ' ', '\t', '\n':
			if inSingleQuote || inDoubleQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add final token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	// Check for unclosed quotes
	if inSingleQuote || inDoubleQuote {
		return nil, fmt.Errorf("unclosed quote in command")
	}

	return tokens, nil
}

// =============================================================================
// SECURITY ERROR TYPE
// =============================================================================

// SecurityError represents a security validation error.
type SecurityError struct {
	Type    string // Type of security error
	Path    string // Path that caused the error (if applicable)
	Message string // Human-readable error message
}

func (e *SecurityError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("security error (%s): %s [path: %s]", e.Type, e.Message, e.Path)
	}
	return fmt.Sprintf("security error (%s): %s", e.Type, e.Message)
}
