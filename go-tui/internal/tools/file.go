// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// This file implements file manipulation tools: Read, Write, Edit.
// It provides path validation and enhanced security features.
package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// FileMaxSize is the maximum file size for read/edit operations (100KB as per requirements)
	FileMaxSize = 100 * 1024

	// FileMaxWriteSize is the maximum content size for write operations (10MB)
	FileMaxWriteSize = 10 * 1024 * 1024

	// FileDefaultLineLimit is the default number of lines to read
	FileDefaultLineLimit = 2000

	// FileMaxLineLength is the maximum length of a single line before truncation
	FileMaxLineLength = 2000
)

// Note: Blocked path prefixes are now defined in security.go via ValidatePathSecure

// =============================================================================
// PATH VALIDATION
// =============================================================================

// ValidatePath checks if a path is safe to access.
// Deprecated: Use ValidatePathSecure instead for comprehensive security validation.
// This function now redirects to ValidatePathSecure for backward compatibility.
func ValidatePath(path string) error {
	_, err := ValidatePathSecure(path)
	return err
}

// =============================================================================
// FILE READ EXECUTOR (Enhanced)
// =============================================================================

// FileReadExecutor implements file reading with enhanced security.
type FileReadExecutor struct {
	// MaxFileSize is the maximum file size to read (default: 100KB)
	MaxFileSize int64

	// MaxLines is the maximum number of lines to read (default: 2000)
	MaxLines int

	// MaxLineLength is the maximum length of a single line (default: 2000)
	MaxLineLength int
}

// Execute reads a file and returns its contents with line numbers.
func (e *FileReadExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = FileMaxSize
	}
	if e.MaxLines == 0 {
		e.MaxLines = FileDefaultLineLimit
	}
	if e.MaxLineLength == 0 {
		e.MaxLineLength = FileMaxLineLength
	}

	// Extract parameters - support both 'path' and 'file_path'
	filePath, _ := params["path"].(string)
	if filePath == "" {
		filePath, _ = params["file_path"].(string)
	}
	offset := getIntParam(params, "offset", 1)
	limit := getIntParam(params, "limit", e.MaxLines)

	// Validate path parameter
	if filePath == "" {
		return Result{
			Success:  false,
			Error:    "path is required",
			Duration: time.Since(start),
		}, nil
	}

	// ==========================================================================
	// SECURITY: Atomic validation and file open to prevent TOCTOU race condition
	// SECURITY: Atomic validation prevents TOCTOU race (TOOL-5 fix)
	// ==========================================================================
	// Use OpenSecureFile instead of ValidatePathSecure + os.Open to prevent
	// race conditions where the file could be swapped between validation and open.
	file, err := OpenSecureFile(filePath, os.O_RDONLY)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	defer file.Close()

	// Get file info from the open file handle (not a separate stat call)
	// This ensures we're checking the same file we opened
	info, err := file.Stat()
	if err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("cannot access file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return Result{
			Success:  false,
			Error:    "cannot read directory, use Glob or Bash 'ls' instead",
			Duration: time.Since(start),
		}, nil
	}

	// Check file size
	if info.Size() > e.MaxFileSize {
		return Result{
			Success:   false,
			Error:     fmt.Sprintf("file too large (%s), max %s. Use offset and limit to read portions.", formatSize(info.Size()), formatSize(e.MaxFileSize)),
			Duration:  time.Since(start),
			Truncated: true,
		}, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return Result{
			Success:  false,
			Error:    "operation cancelled",
			Duration: time.Since(start),
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
		builder.WriteString(fileFormatLineNum(lineNum))
		builder.WriteString("\t")
		builder.WriteString(line)
		builder.WriteString("\n")

		linesRead++

		// Check for context cancellation periodically
		if linesRead%100 == 0 {
			select {
			case <-ctx.Done():
				return Result{
					Success:  false,
					Error:    "operation cancelled",
					Duration: time.Since(start),
				}, nil
			default:
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("error reading file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	output := builder.String()
	if output == "" {
		output = "(empty file)"
	}

	return Result{
		Success:    true,
		Output:     output,
		Duration:   time.Since(start),
		BytesRead:  info.Size(),
		LinesCount: linesRead,
		Truncated:  truncated,
	}, nil
}

// =============================================================================
// FILE WRITE EXECUTOR (Enhanced)
// =============================================================================

// FileWriteExecutor implements file writing with enhanced security.
type FileWriteExecutor struct {
	// MaxFileSize is the maximum file size to write (default: 10MB)
	MaxFileSize int64

	// CreateDirs automatically creates parent directories
	CreateDirs bool

	// BackupOriginal creates a backup of existing files
	BackupOriginal bool
}

// Execute writes content to a file.
func (e *FileWriteExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = FileMaxWriteSize
	}
	if !e.CreateDirs {
		e.CreateDirs = true // Default to creating directories
	}

	// Extract parameters - support both 'path' and 'file_path'
	filePath, _ := params["path"].(string)
	if filePath == "" {
		filePath, _ = params["file_path"].(string)
	}
	content, _ := params["content"].(string)

	// Validate path parameter
	if filePath == "" {
		return Result{
			Success:  false,
			Error:    "path is required",
			Duration: time.Since(start),
		}, nil
	}

	// Validate path security using comprehensive secure validation (TOOL-6 fix)
	validatedPath, err := ValidatePathSecure(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	filePath = validatedPath

	// Validate content size
	if int64(len(content)) > e.MaxFileSize {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("content too large (%s), max %s", formatSize(int64(len(content))), formatSize(e.MaxFileSize)),
			Duration: time.Since(start),
		}, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return Result{
			Success:  false,
			Error:    "operation cancelled",
			Duration: time.Since(start),
		}, nil
	default:
	}

	// Create parent directories if needed
	dir := filepath.Dir(filePath)
	if e.CreateDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return Result{
				Success:  false,
				Error:    fmt.Sprintf("cannot create directory: %v", err),
				Duration: time.Since(start),
			}, nil
		}
	}

	// Check if file exists (for backup and reporting)
	existed := false
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		existed = true

		// Create backup if configured
		if e.BackupOriginal {
			backupPath := filePath + ".bak"
			if err := os.Rename(filePath, backupPath); err != nil {
				// Non-fatal, continue with write
			}
		}
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	writeErr := util.AtomicWriteFile(filePath, []byte(content), 0644)
	if writeErr != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("cannot write file: %v", writeErr),
			Duration: time.Since(start),
		}, nil
	}

	// Build success message
	action := "Created"
	if existed {
		action = "Wrote"
	}

	lines := fileCountLines(content)
	output := fmt.Sprintf("%s %s (%d lines, %s)", action, filePath, lines, formatSize(int64(len(content))))

	return Result{
		Success:      true,
		Output:       output,
		Duration:     time.Since(start),
		BytesWritten: int64(len(content)),
		LinesCount:   lines,
	}, nil
}

// =============================================================================
// FILE EDIT EXECUTOR (Enhanced)
// =============================================================================

// FileEditExecutor implements file editing via search and replace with enhanced security.
type FileEditExecutor struct {
	// MaxFileSize is the maximum file size to edit (default: 100KB)
	MaxFileSize int64

	// BackupOriginal creates a backup before editing
	BackupOriginal bool
}

// Execute edits a file using search and replace.
func (e *FileEditExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = FileMaxSize
	}

	// Extract parameters - support both 'path' and 'file_path'
	filePath, _ := params["path"].(string)
	if filePath == "" {
		filePath, _ = params["file_path"].(string)
	}
	oldString, _ := params["old_string"].(string)
	newString, _ := params["new_string"].(string)
	replaceAll := getBoolParam(params, "replace_all", false)

	// Validate parameters
	if filePath == "" {
		return Result{
			Success:  false,
			Error:    "path is required",
			Duration: time.Since(start),
		}, nil
	}

	if oldString == "" {
		return Result{
			Success:  false,
			Error:    "old_string is required",
			Duration: time.Since(start),
		}, nil
	}

	if oldString == newString {
		return Result{
			Success:  false,
			Error:    "old_string and new_string must be different",
			Duration: time.Since(start),
		}, nil
	}

	// Validate path security using comprehensive secure validation (TOOL-7 fix)
	validatedPath, err := ValidatePathSecure(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	filePath = validatedPath

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{
				Success:  false,
				Error:    fmt.Sprintf("file not found: %s", filePath),
				Duration: time.Since(start),
			}, nil
		}
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("cannot access file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return Result{
			Success:  false,
			Error:    "cannot edit directory",
			Duration: time.Since(start),
		}, nil
	}

	// Check file size
	if info.Size() > e.MaxFileSize {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("file too large (%s), max %s", formatSize(info.Size()), formatSize(e.MaxFileSize)),
			Duration: time.Since(start),
		}, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return Result{
			Success:  false,
			Error:    "operation cancelled",
			Duration: time.Since(start),
		}, nil
	default:
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("cannot read file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	contentStr := string(content)

	// Check if old_string exists
	if !strings.Contains(contentStr, oldString) {
		return Result{
			Success:  false,
			Error:    "old_string not found in file",
			Duration: time.Since(start),
		}, nil
	}

	// Check uniqueness if not replacing all
	count := strings.Count(contentStr, oldString)
	if !replaceAll && count > 1 {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("old_string found %d times - use replace_all=true or provide more context for unique match", count),
			Duration: time.Since(start),
		}, nil
	}

	// Create backup if configured
	if e.BackupOriginal {
		backupPath := filePath + ".bak"
		os.WriteFile(backupPath, content, info.Mode())
	}

	// Perform replacement
	var newContent string
	var replacements int

	if replaceAll {
		replacements = count
		newContent = strings.ReplaceAll(contentStr, oldString, newString)
	} else {
		newContent = strings.Replace(contentStr, oldString, newString, 1)
		replacements = 1
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(newContent), info.Mode()); err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("cannot write file: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Build diff-style output showing the change
	var diffOutput strings.Builder
	diffOutput.WriteString(fmt.Sprintf("Edited %s (%d replacement", filePath, replacements))
	if replacements != 1 {
		diffOutput.WriteString("s")
	}
	diffOutput.WriteString(")\n\n")

	// Show before/after for the change
	diffOutput.WriteString("--- Before:\n")
	diffOutput.WriteString(fileDiffContext(oldString))
	diffOutput.WriteString("\n+++ After:\n")
	diffOutput.WriteString(fileDiffContext(newString))

	return Result{
		Success:      true,
		Output:       diffOutput.String(),
		Duration:     time.Since(start),
		BytesWritten: int64(len(newContent)),
		LinesCount:   fileCountLines(newContent),
		MatchCount:   replacements,
	}, nil
}

// =============================================================================
// FILE HELPER FUNCTIONS
// =============================================================================

// fileFormatLineNum formats a line number with right-padding (6 chars).
func fileFormatLineNum(n int) string {
	s := fmt.Sprintf("%d", n)
	padding := 6 - len(s)
	if padding > 0 {
		return strings.Repeat(" ", padding) + s
	}
	return s
}

// fileCountLines counts the number of lines in content.
func fileCountLines(content string) int {
	if len(content) == 0 {
		return 0
	}

	lines := 1
	for _, c := range content {
		if c == '\n' {
			lines++
		}
	}

	// Don't count trailing newline as extra line
	if len(content) > 0 && content[len(content)-1] == '\n' {
		lines--
	}

	return lines
}

// fileDiffContext formats a string for diff-style output.
// Shows each line with a prefix and truncates if too long.
func fileDiffContext(s string) string {
	if s == "" {
		return "  (empty)\n"
	}

	lines := strings.Split(s, "\n")
	var result strings.Builder

	maxLines := 10
	for i, line := range lines {
		if i >= maxLines {
			result.WriteString(fmt.Sprintf("  ... (%d more lines)\n", len(lines)-maxLines))
			break
		}

		// UNICODE: Rune-aware truncation preserves multi-byte characters
		line = util.TruncateRunes(line, 80)

		result.WriteString("  ")
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
