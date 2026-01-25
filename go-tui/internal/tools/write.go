// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// write.go implements secure file writing with comprehensive protections.
package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SECURITY CONSTANTS
// =============================================================================

// SensitiveFilePatterns are file patterns that should NEVER be written to.
// These files often contain secrets, credentials, or sensitive configuration.
var SensitiveFilePatterns = []string{
	// Environment and secrets
	".env",
	".env.local",
	".env.production",
	".env.development",
	".env.*",
	"*.pem",
	"*.key",
	"*.crt",
	"*.p12",
	"*.pfx",

	// Credentials and tokens
	"credentials.json",
	"credentials.yaml",
	"credentials.yml",
	"secrets.json",
	"secrets.yaml",
	"secrets.yml",
	"*_secret*",
	"*_token*",
	"*.secret",
	".npmrc",
	".pypirc",
	".netrc",
	".gitconfig",

	// SSH keys
	"id_rsa",
	"id_rsa.pub",
	"id_ed25519",
	"id_ed25519.pub",
	"id_dsa",
	"id_ecdsa",
	"authorized_keys",
	"known_hosts",

	// Cloud credentials
	".aws/credentials",
	".aws/config",
	".gcloud/*",
	".azure/*",
	"kubeconfig",
	".kube/config",

	// Database and service configs with potential secrets
	"database.yml",
	"database.yaml",
	"config/database.yml",
	"config/secrets.yml",

	// Password files
	"passwd",
	"shadow",
	"*.password",
	"*.passwd",
}

// BlockedWritePaths are system directories that should never be written to.
var blockedWritePathsLinux = []string{
	"/etc",
	"/sys",
	"/proc",
	"/boot",
	"/dev",
	"/sbin",
	"/bin",
	"/usr/bin",
	"/usr/sbin",
	"/lib",
	"/lib64",
	"/var/log",
	"/root",
}

var blockedWritePathsWindows = []string{
	"C:\\Windows",
	"C:\\Program Files",
	"C:\\Program Files (x86)",
	"C:\\ProgramData",
	"C:\\System Volume Information",
	"C:\\$Recycle.Bin",
}

// =============================================================================
// WRITE EXECUTOR
// =============================================================================

// WriteExecutor implements secure file writing with comprehensive protections.
type WriteExecutor struct {
	// MaxFileSize is the maximum file size to write (default: 10MB)
	MaxFileSize int64

	// CreateDirs automatically creates parent directories
	CreateDirs bool

	// BackupOriginal creates a backup of existing files
	BackupOriginal bool
}

// Execute writes content to a file with security validations.
func (e *WriteExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	// CreateDirs defaults to true
	e.CreateDirs = true

	// Extract parameters
	filePath, _ := params["file_path"].(string)
	content, _ := params["content"].(string)

	// ==========================================================================
	// VALIDATION: Required parameters
	// ==========================================================================

	if filePath == "" {
		return Result{
			Success:  false,
			Error:    "file_path is required",
			Duration: time.Since(start),
		}, nil
	}

	// ==========================================================================
	// SECURITY: Comprehensive path validation with symlink protection
	// ==========================================================================
	// CRITICAL FIX: Use ValidatePathSecure instead of checking ".." before canonicalization.
	// The old approach was vulnerable to symlink attacks where:
	//   1. Attacker creates symlink: /safe/path/link -> /etc
	//   2. Old code: "/safe/path/link/passwd" passes ".." check
	//   3. After canonicalization: becomes "/etc/passwd" - BYPASSED!
	// ValidatePathSecure resolves symlinks FIRST, then checks for path traversal.

	absPath, err := ValidatePathSecure(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    "security: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// ==========================================================================
	// SECURITY: Sensitive file protection
	// ==========================================================================

	fileName := filepath.Base(absPath)
	fileNameLower := strings.ToLower(fileName)
	pathLower := strings.ToLower(absPath)

	for _, pattern := range SensitiveFilePatterns {
		patternLower := strings.ToLower(pattern)

		// Check exact match
		if fileNameLower == patternLower {
			return Result{
				Success:  false,
				Error:    "security: cannot write to sensitive file '" + fileName + "' - this file may contain secrets or credentials",
				Duration: time.Since(start),
			}, nil
		}

		// Check pattern match with wildcards
		if strings.Contains(pattern, "*") {
			// Convert glob to simple prefix/suffix match
			if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
				// *pattern* - contains
				middle := strings.Trim(patternLower, "*")
				if strings.Contains(fileNameLower, middle) {
					return Result{
						Success:  false,
						Error:    "security: cannot write to sensitive file matching pattern '" + pattern + "'",
						Duration: time.Since(start),
					}, nil
				}
			} else if strings.HasPrefix(pattern, "*") {
				// *.ext - suffix match
				suffix := strings.TrimPrefix(patternLower, "*")
				if strings.HasSuffix(fileNameLower, suffix) {
					return Result{
						Success:  false,
						Error:    "security: cannot write to sensitive file matching pattern '" + pattern + "'",
						Duration: time.Since(start),
					}, nil
				}
			} else if strings.HasSuffix(pattern, "*") {
				// prefix* - prefix match
				prefix := strings.TrimSuffix(patternLower, "*")
				if strings.HasPrefix(fileNameLower, prefix) {
					return Result{
						Success:  false,
						Error:    "security: cannot write to sensitive file matching pattern '" + pattern + "'",
						Duration: time.Since(start),
					}, nil
				}
			}
		}

		// Check path-based patterns (e.g., ".aws/credentials")
		if strings.Contains(pattern, "/") || strings.Contains(pattern, "\\") {
			normalizedPattern := strings.ReplaceAll(patternLower, "\\", "/")
			normalizedPathCheck := strings.ReplaceAll(pathLower, "\\", "/")
			if strings.HasSuffix(normalizedPathCheck, normalizedPattern) {
				return Result{
					Success:  false,
					Error:    "security: cannot write to sensitive path matching '" + pattern + "'",
					Duration: time.Since(start),
				}, nil
			}
		}
	}

	// ==========================================================================
	// VALIDATION: Content size
	// ==========================================================================

	if int64(len(content)) > e.MaxFileSize {
		return Result{
			Success:  false,
			Error:    "content too large (" + formatSize(int64(len(content))) + "), max " + formatSize(e.MaxFileSize),
			Duration: time.Since(start),
		}, nil
	}

	// ==========================================================================
	// CHECK: Context cancellation
	// ==========================================================================

	select {
	case <-ctx.Done():
		return Result{
			Success:  false,
			Error:    "operation cancelled",
			Duration: time.Since(start),
		}, nil
	default:
	}

	// ==========================================================================
	// CREATE: Parent directories if needed
	// ==========================================================================

	dir := filepath.Dir(absPath)
	if e.CreateDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return Result{
				Success:  false,
				Error:    "cannot create directory '" + dir + "': " + err.Error(),
				Duration: time.Since(start),
			}, nil
		}
	}

	// ==========================================================================
	// CHECK: File existence and handle overwrite
	// ==========================================================================

	existed := false
	var existingSize int64
	if info, err := os.Stat(absPath); err == nil {
		if info.IsDir() {
			return Result{
				Success:  false,
				Error:    "cannot write to '" + absPath + "': path is a directory",
				Duration: time.Since(start),
			}, nil
		}
		existed = true
		existingSize = info.Size()

		// Create backup if configured
		if e.BackupOriginal {
			backupPath := absPath + ".bak"
			if err := copyFile(absPath, backupPath); err != nil {
				// Non-fatal, log but continue with write
			}
		}
	}

	// ==========================================================================
	// WRITE: File content
	// ==========================================================================

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return Result{
			Success:  false,
			Error:    "cannot write file: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// ==========================================================================
	// BUILD: Success message
	// ==========================================================================

	lines := countLines(content)
	var output string

	if existed {
		output = "Overwrote " + absPath + " (" + util.IntToStr(lines) + " lines, " + formatSize(int64(len(content))) + ")"
		if existingSize > 0 {
			output += " [was " + formatSize(existingSize) + "]"
		}
	} else {
		output = "Created " + absPath + " (" + util.IntToStr(lines) + " lines, " + formatSize(int64(len(content))) + ")"
	}

	return Result{
		Success:      true,
		Output:       output,
		Duration:     time.Since(start),
		BytesWritten: int64(len(content)),
		LinesCount:   lines,
	}, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// =============================================================================
// DIFF PREVIEW
// =============================================================================

// GetDiffPreview returns a diff preview for a Write operation without executing it.
// This allows the UI to show what will change before applying the write.
// For new files, oldContent will be empty.
func (e *WriteExecutor) GetDiffPreview(params map[string]interface{}) (oldContent, newContent string, err error) {
	// Extract parameters
	filePath, _ := params["file_path"].(string)
	content, _ := params["content"].(string)

	if filePath == "" {
		return "", "", &SecurityError{
			Type:    "validation",
			Message: "file_path is required",
		}
	}

	// Validate path
	absPath, err := ValidatePathSecure(filePath)
	if err != nil {
		return "", "", err
	}

	// Check if file exists and read old content
	if info, err := os.Stat(absPath); err == nil {
		if info.IsDir() {
			return "", "", &SecurityError{
				Type:    "validation",
				Message: "cannot write to directory",
			}
		}

		// Read existing content
		data, err := os.ReadFile(absPath)
		if err != nil {
			// File exists but can't read - return empty old content
			oldContent = ""
		} else {
			oldContent = string(data)
		}
	}
	// If file doesn't exist, oldContent remains empty (new file)

	newContent = content
	return oldContent, newContent, nil
}
