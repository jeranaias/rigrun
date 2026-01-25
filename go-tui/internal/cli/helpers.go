// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli provides command-line interface functionality.
// This file contains shared helper functions used across multiple CLI commands.
//
// CLI: Comprehensive help and examples for all commands
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// formatDuration formats a time.Duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// formatDurationShort formats a short duration string.
func formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}

// formatBytes formats a byte count for display.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// outputJSON outputs data as JSON.
func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// promptInput prompts the user for input.
func promptInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// getCurrentUserID gets the current user ID from environment or system.
func getCurrentUserID() string {
	// Try RIGRUN_USER_ID environment variable first
	if userID := os.Getenv("RIGRUN_USER_ID"); userID != "" {
		return userID
	}
	// Try USERNAME (Windows) or USER (Unix)
	if userID := os.Getenv("USERNAME"); userID != "" {
		return userID
	}
	if userID := os.Getenv("USER"); userID != "" {
		return userID
	}
	return "unknown"
}

// ValidateOutputPath ensures path is safe for writing.
// Prevents path traversal attacks by validating the path is within allowed directories.
// SECURITY: Uses isPathWithinDir to prevent HasPrefix bypass attacks.
func ValidateOutputPath(path string) (string, error) {
	// Clean the path
	cleaned := filepath.Clean(path)

	// Resolve to absolute
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check for traversal attempts
	if strings.Contains(path, "..") {
		return "", errors.New("path traversal not allowed")
	}

	// Ensure within allowed directories
	// SECURITY: Use isPathWithinDirCLI to prevent /home/userEVIL matching /home/user
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	allowed := []string{home, cwd, os.TempDir()}
	isAllowed := false
	for _, dir := range allowed {
		if dir == "" {
			continue
		}
		// SECURITY: Use proper path boundary checking instead of HasPrefix
		if isPathWithinDirCLI(abs, dir) {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return "", fmt.Errorf("path must be within home, cwd, or temp directory")
	}

	return abs, nil
}

// isPathWithinDirCLI checks if a path is within a directory, ensuring proper path boundaries.
// SECURITY: Prevents HasPrefix bypass where /home/userEVIL would pass check for /home/user.
func isPathWithinDirCLI(path, dir string) bool {
	// Clean both paths for consistent comparison
	cleanPath := filepath.Clean(path)
	cleanDir := filepath.Clean(dir)

	// Exact match - path is the directory itself
	if cleanPath == cleanDir {
		return true
	}

	// Ensure directory path ends with separator for prefix check
	// This prevents /home/userEVIL from matching /home/user
	dirWithSep := cleanDir + string(filepath.Separator)

	// Check if path starts with directory + separator
	return strings.HasPrefix(cleanPath, dirWithSep)
}

// ValidatePathSecure validates a path is safe for secure operations (like wiping).
// This is a stricter validation that also checks the path exists and is accessible.
func ValidatePathSecure(path string) (string, error) {
	// First do standard validation
	abs, err := ValidateOutputPath(path)
	if err != nil {
		return "", err
	}

	// Additional check: ensure the path doesn't contain suspicious patterns
	lowerPath := strings.ToLower(abs)
	suspiciousPatterns := []string{
		"\\windows\\",
		"\\system32\\",
		"\\program files\\",
		"/etc/",
		"/usr/",
		"/bin/",
		"/sbin/",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return "", fmt.Errorf("access to system directories not allowed")
		}
	}

	return abs, nil
}
