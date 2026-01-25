// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SECURITY: SENSITIVE FILE PATTERNS
// =============================================================================

// sensitiveFilePatterns are file patterns that contain credentials or secrets.
// These files are blocked from reading to prevent credential exposure.
// CRITICAL FOR MILITARY APPLICATIONS: Prevents accidental leakage of classified credentials.
var sensitiveFilePatterns = []string{
	// Environment and configuration files with secrets
	".env",
	".env.local",
	".env.development",
	".env.production",
	".env.staging",

	// SSH keys and configuration
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
	".ssh/config",
	"authorized_keys",
	"known_hosts",

	// Cloud provider credentials
	".aws/credentials",
	".aws/config",
	".azure/",
	".kube/config",
	".gcloud/",

	// Git credentials
	".git/config",
	".gitconfig",
	".git-credentials",
	".netrc",

	// Package manager credentials
	".npmrc",
	".pypirc",

	// Certificate and key files
	".pem",
	".key",
	".p12",
	".pfx",
	".crt",

	// General sensitive patterns
	"credentials",
	"secrets",
	"password",
	"passwd",
	"/etc/shadow",
	"/etc/passwd",
	"/etc/sudoers",
}

// blockedSystemPaths are system directories that should not be accessed.
// Platform-specific lists are used based on runtime.GOOS.
var blockedLinuxSystemPaths = []string{
	"/etc/shadow",
	"/etc/passwd",
	"/etc/sudoers",
	"/etc/ssh",
	"/proc",
	"/sys",
	"/boot",
	"/root/.ssh",
}

var blockedWindowsSystemPaths = []string{
	"C:\\Windows\\System32\\config",
	"C:\\Windows\\System32\\drivers",
	"C:\\Users\\Default",
}

// =============================================================================
// READ EXECUTOR
// =============================================================================

// ReadExecutor implements file reading with security protections.
type ReadExecutor struct {
	// MaxFileSize is the maximum file size to read (default: 10MB)
	MaxFileSize int64

	// MaxLines is the maximum number of lines to read (default: 2000)
	MaxLines int

	// MaxLineLength is the maximum length of a single line (default: 2000)
	MaxLineLength int

	// SensitivePatterns are additional patterns to block (merged with defaults)
	SensitivePatterns []string

	// AllowSensitiveFiles disables sensitive file checking (DANGEROUS - use only for testing)
	AllowSensitiveFiles bool
}

// Execute reads a file and returns its contents.
func (e *ReadExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	if e.MaxLines == 0 {
		e.MaxLines = 2000
	}
	if e.MaxLineLength == 0 {
		e.MaxLineLength = 2000
	}

	// Extract parameters
	filePath, _ := params["file_path"].(string)
	offset := getIntParam(params, "offset", 1)
	limit := getIntParam(params, "limit", e.MaxLines)

	// Validate path is provided
	if filePath == "" {
		return Result{
			Success: false,
			Error:   "file_path is required",
		}, nil
	}

	// ==========================================================================
	// SECURITY: Atomic validation and file open to prevent TOCTOU race condition
	// SECURITY: Atomic validation prevents TOCTOU race
	// ==========================================================================
	// Use OpenSecureFile instead of ValidatePathSecure + os.Open to prevent
	// race conditions where the file could be swapped between validation and open.
	file, err := OpenSecureFile(filePath, os.O_RDONLY)
	if err != nil {
		return Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer file.Close()

	// Get the actual validated path from the opened file
	validPath := file.Name()

	// ==========================================================================
	// SECURITY CHECK: Sensitive file patterns
	// ==========================================================================
	if !e.AllowSensitiveFiles {
		if isSensitivePath(validPath) {
			return Result{
				Success: false,
				Error:   "access denied: file contains sensitive data (credentials, keys, or secrets)",
			}, nil
		}
	}

	// Get file info from the open file handle (not a separate stat call)
	// This ensures we're checking the same file we opened
	info, err := file.Stat()
	if err != nil {
		return Result{
			Success: false,
			Error:   "cannot access file: " + err.Error(),
		}, nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return Result{
			Success: false,
			Error:   "cannot read directory, use Glob or Bash 'ls' instead",
		}, nil
	}

	// Check file size
	if info.Size() > e.MaxFileSize {
		return Result{
			Success: false,
			Error:   "file too large (max " + formatSize(e.MaxFileSize) + "). Use offset and limit parameters to read portions.",
		}, nil
	}

	// ==========================================================================
	// SECURITY CHECK: Binary file detection
	// ==========================================================================
	if isBinaryFileFromHandle(file) {
		return Result{
			Success: false,
			Error:   "cannot read binary file. Use appropriate tools for binary files.",
		}, nil
	}

	// Reset file position after binary check
	file.Seek(0, 0)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return Result{
			Success: false,
			Error:   "operation cancelled",
		}, nil
	default:
	}

	// Read file with line numbers
	var builder strings.Builder
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	linesRead := 0
	truncated := false

	for scanner.Scan() {
		lineNum++

		// Skip lines before offset
		if lineNum < offset {
			continue
		}

		// Check limit
		if linesRead >= limit {
			truncated = true
			break
		}

		line := scanner.Text()

		// Truncate long lines
		if len(line) > e.MaxLineLength {
			line = line[:e.MaxLineLength] + "..."
		}

		// Format with line number (cat -n style)
		builder.WriteString(formatLineNumber(lineNum))
		builder.WriteString("\t")
		builder.WriteString(line)
		builder.WriteString("\n")

		linesRead++

		// Check for context cancellation periodically
		if linesRead%100 == 0 {
			select {
			case <-ctx.Done():
				return Result{
					Success: false,
					Error:   "operation cancelled",
				}, nil
			default:
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Result{
			Success: false,
			Error:   "error reading file: " + err.Error(),
		}, nil
	}

	return Result{
		Success:    true,
		Output:     builder.String(),
		BytesRead:  info.Size(),
		LinesCount: linesRead,
		Truncated:  truncated,
	}, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getIntParam extracts an integer parameter with a default value.
func getIntParam(params map[string]interface{}, name string, defaultVal int) int {
	if val, ok := params[name]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// formatLineNumber formats a line number with right-padding.
func formatLineNumber(n int) string {
	s := util.IntToStr(n)
	// Right-align in 6 characters
	padding := 6 - len(s)
	if padding > 0 {
		return strings.Repeat(" ", padding) + s
	}
	return s
}

// formatSize formats a byte size in human-readable form.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return util.IntToStr(int(bytes/GB)) + "GB"
	case bytes >= MB:
		return util.IntToStr(int(bytes/MB)) + "MB"
	case bytes >= KB:
		return util.IntToStr(int(bytes/KB)) + "KB"
	default:
		return util.IntToStr(int(bytes)) + "B"
	}
}

// getStringParam extracts a string parameter with a default value.
func getStringParam(params map[string]interface{}, name string, defaultVal string) string {
	if val, ok := params[name]; ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return defaultVal
}

// getBoolParam extracts a boolean parameter with a default value.
func getBoolParam(params map[string]interface{}, name string, defaultVal bool) bool {
	if val, ok := params[name]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// boolToInt converts a boolean to 0 or 1.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// SECURITY HELPER FUNCTIONS
// =============================================================================

// checkBlockedSystemPaths checks if a path is in a blocked system directory.
// Returns an error if the path should be blocked.
func (e *ReadExecutor) checkBlockedSystemPaths(absPath string) error {
	// Normalize path for comparison
	normalizedPath := absPath
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(absPath)
	}

	// Get platform-specific blocked paths
	var blockedPaths []string
	if runtime.GOOS == "windows" {
		blockedPaths = blockedWindowsSystemPaths
	} else {
		blockedPaths = blockedLinuxSystemPaths
	}

	for _, blocked := range blockedPaths {
		blockedNormalized := blocked
		if runtime.GOOS == "windows" {
			blockedNormalized = strings.ToLower(blocked)
		}

		if strings.HasPrefix(normalizedPath, blockedNormalized) {
			return &ReadSecurityError{
				Type:    "blocked_path",
				Path:    absPath,
				Message: "access denied: path is in a protected system directory",
			}
		}
	}

	return nil
}

// checkSensitiveFile checks if a file matches sensitive patterns.
// Returns an error if the file should not be read.
func (e *ReadExecutor) checkSensitiveFile(absPath string) error {
	// Normalize path for comparison (case-insensitive on Windows)
	normalizedPath := absPath
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(absPath)
	}

	// Get base name and extension
	baseName := filepath.Base(absPath)
	ext := filepath.Ext(absPath)

	// Combine default and custom patterns
	patterns := sensitiveFilePatterns
	if len(e.SensitivePatterns) > 0 {
		patterns = append(patterns, e.SensitivePatterns...)
	}

	for _, pattern := range patterns {
		patternNormalized := pattern
		if runtime.GOOS == "windows" {
			patternNormalized = strings.ToLower(pattern)
		}

		// Check if pattern matches the full path, base name, or extension
		if strings.Contains(normalizedPath, patternNormalized) {
			return &ReadSecurityError{
				Type:    "sensitive_file",
				Path:    absPath,
				Message: "access denied: file contains sensitive data (credentials, keys, or secrets)",
			}
		}

		// Check base name exact match
		baseNormalized := baseName
		if runtime.GOOS == "windows" {
			baseNormalized = strings.ToLower(baseName)
		}
		if baseNormalized == patternNormalized {
			return &ReadSecurityError{
				Type:    "sensitive_file",
				Path:    absPath,
				Message: "access denied: file contains sensitive data (credentials, keys, or secrets)",
			}
		}

		// Check extension match (for patterns like ".pem", ".key")
		if strings.HasPrefix(pattern, ".") && strings.ToLower(ext) == strings.ToLower(pattern) {
			return &ReadSecurityError{
				Type:    "sensitive_file",
				Path:    absPath,
				Message: "access denied: file type may contain sensitive data (certificate or key file)",
			}
		}
	}

	return nil
}

// isBinaryFile checks if a file is likely a binary file.
// It reads the first 512 bytes and checks for null bytes or high concentration of non-printable characters.
func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false // Can't check, assume not binary
	}
	defer file.Close()

	return isBinaryFileFromHandle(file)
}

// isBinaryFileFromHandle checks if an already-opened file is likely a binary file.
// SECURITY: This function works with an existing file handle to prevent TOCTOU race conditions.
// It reads the first 512 bytes and checks for null bytes or high concentration of non-printable characters.
func isBinaryFileFromHandle(file *os.File) bool {
	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return false // Empty file or error, assume not binary
	}
	buf = buf[:n]

	// Check for null bytes (strong indicator of binary)
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}

	// Check for high concentration of non-printable characters
	nonPrintable := 0
	for _, b := range buf {
		// Allow common text characters: printable ASCII, newlines, tabs
		if (b < 32 || b > 126) && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, likely binary
	return float64(nonPrintable)/float64(n) > 0.30
}

// ReadSecurityError represents a security-related error in file operations.
type ReadSecurityError struct {
	Type    string // "blocked_path", "sensitive_file", "path_traversal"
	Path    string // The path that caused the error
	Message string // Human-readable error message
}

func (e *ReadSecurityError) Error() string {
	return e.Message
}
