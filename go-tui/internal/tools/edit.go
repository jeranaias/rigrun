// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// EDIT EXECUTOR
// =============================================================================

// EditExecutor implements file editing via search and replace.
type EditExecutor struct {
	// MaxFileSize is the maximum file size to edit (default: 10MB)
	MaxFileSize int64

	// BackupOriginal creates a backup before editing (legacy field, use create_backup param instead)
	BackupOriginal bool
}

// Execute edits a file using search and replace.
func (e *EditExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// Set defaults
	if e.MaxFileSize == 0 {
		e.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}

	// Extract parameters
	filePath, _ := params["file_path"].(string)
	oldString, _ := params["old_string"].(string)
	newString, _ := params["new_string"].(string)
	replaceAll := getBoolParam(params, "replace_all", false)
	useRegex := getBoolParam(params, "use_regex", false)
	createBackup := getBoolParam(params, "create_backup", false)
	restoreBackup := getBoolParam(params, "restore_backup", false)
	dryRun := getBoolParam(params, "dry_run", false)

	// Validate file_path
	if filePath == "" {
		return Result{
			Success: false,
			Error:   "file_path is required - please provide the absolute path to the file you want to edit",
		}, nil
	}

	// Handle restore_backup operation first (needs path validation but not file open)
	if restoreBackup {
		// Validate path security for restore operation
		validatedPath, err := ValidatePathSecure(filePath)
		if err != nil {
			return Result{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return e.restoreFromBackup(validatedPath)
	}

	// Validate old_string for edit operations
	if oldString == "" {
		return Result{
			Success: false,
			Error:   "old_string is required - provide the exact text you want to find and replace",
		}, nil
	}

	// Allow same old_string and new_string only if new_string is empty (deletion)
	// or if they're actually the same (which is a no-op)
	if oldString == newString && newString != "" {
		return Result{
			Success: false,
			Error:   "old_string and new_string are identical - no changes would be made",
		}, nil
	}

	// ==========================================================================
	// SECURITY: Atomic validation and file open to prevent TOCTOU race condition
	// SECURITY: Atomic validation prevents TOCTOU race (TOOL-8 fix)
	// ==========================================================================
	// Use OpenSecureFile instead of ValidatePathSecure + os.Open to prevent
	// race conditions where the file could be swapped between validation and open.
	file, err := OpenSecureFile(filePath, os.O_RDONLY)
	if err != nil {
		// Provide more specific error messages
		if secErr, ok := err.(*SecurityError); ok {
			if secErr.Type == "file_open" && strings.Contains(secErr.Message, "no such file") {
				return Result{
					Success: false,
					Error:   "file not found: " + filePath + " - verify the path is correct and the file exists",
				}, nil
			}
		}
		return Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Get file info from the open file handle (not a separate stat call)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return Result{
			Success: false,
			Error:   "cannot access file: " + err.Error(),
		}, nil
	}

	// Get the validated path from the opened file
	validatedPath := file.Name()

	// Check if it's a directory
	if info.IsDir() {
		file.Close()
		return Result{
			Success: false,
			Error:   "cannot edit directory - provide a file path, not a directory path",
		}, nil
	}

	// Check file size
	if info.Size() > e.MaxFileSize {
		file.Close()
		return Result{
			Success: false,
			Error:   "file too large (max " + formatSize(e.MaxFileSize) + ") - consider editing in smaller chunks",
		}, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		file.Close()
		return Result{
			Success: false,
			Error:   "operation cancelled",
		}, nil
	default:
	}

	// Read file content from the open handle
	content, err := io.ReadAll(file)
	file.Close() // Close after reading, we'll reopen for write
	if err != nil {
		return Result{
			Success: false,
			Error:   "cannot read file: " + err.Error(),
		}, nil
	}

	// Use the validated path for the rest of the operation
	filePath = validatedPath

	contentStr := string(content)

	// Perform the replacement
	var newContent string
	var replacements int
	var matchInfo string

	if useRegex {
		// Regex-based replacement
		re, err := regexp.Compile(oldString)
		if err != nil {
			return Result{
				Success: false,
				Error:   "invalid regular expression: " + err.Error() + " - check your regex syntax",
			}, nil
		}

		matches := re.FindAllString(contentStr, -1)
		if len(matches) == 0 {
			return Result{
				Success: false,
				Error:   "regex pattern not found in file - the pattern '" + oldString + "' did not match any text",
			}, nil
		}

		// Check uniqueness if not replacing all
		if !replaceAll && len(matches) > 1 {
			return Result{
				Success: false,
				Error:   "regex pattern matched " + util.IntToStr(len(matches)) + " times - use replace_all=true or refine your pattern for a unique match",
			}, nil
		}

		if replaceAll {
			replacements = len(matches)
			newContent = re.ReplaceAllString(contentStr, newString)
		} else {
			replacements = 1
			newContent = re.ReplaceAllStringFunc(contentStr, func(match string) string {
				if replacements > 0 {
					replacements--
					return re.ReplaceAllString(match, newString)
				}
				return match
			})
			replacements = 1 // Reset for reporting
		}

		// Build match info for output
		if len(matches) == 1 {
			matchInfo = "matched: '" + truncateString(matches[0], 50) + "'"
		} else {
			matchInfo = "matched " + util.IntToStr(len(matches)) + " occurrences"
		}
	} else {
		// Exact string replacement
		if !strings.Contains(contentStr, oldString) {
			// Provide helpful error message with suggestions
			errMsg := "old_string not found in file - the exact text was not found"

			// Check for case-insensitive match
			if strings.Contains(strings.ToLower(contentStr), strings.ToLower(oldString)) {
				errMsg += ". Note: A case-insensitive match exists - check your capitalization"
			}

			// Check for whitespace issues
			trimmed := strings.TrimSpace(oldString)
			if trimmed != oldString && strings.Contains(contentStr, trimmed) {
				errMsg += ". Note: The text exists but with different whitespace - check leading/trailing spaces"
			}

			return Result{
				Success: false,
				Error:   errMsg,
			}, nil
		}

		// Check uniqueness if not replacing all
		if !replaceAll {
			count := strings.Count(contentStr, oldString)
			if count > 1 {
				return Result{
					Success: false,
					Error:   "old_string found " + util.IntToStr(count) + " times - use replace_all=true to replace all occurrences, or include more surrounding context in old_string for a unique match",
				}, nil
			}
		}

		if replaceAll {
			replacements = strings.Count(contentStr, oldString)
			newContent = strings.ReplaceAll(contentStr, oldString, newString)
		} else {
			newContent = strings.Replace(contentStr, oldString, newString, 1)
			replacements = 1
		}

		matchInfo = "replaced " + util.IntToStr(replacements) + " occurrence(s)"
	}

	// Handle dry run - show what would change without actually changing
	if dryRun {
		output := "DRY RUN - No changes made\n\n"
		output += "File: " + filePath + "\n"
		output += "Operation: " + matchInfo + "\n\n"

		if useRegex {
			output += "Regex pattern: " + oldString + "\n"
		} else {
			output += "Find: '" + truncateString(oldString, 100) + "'\n"
		}
		output += "Replace with: '" + truncateString(newString, 100) + "'\n"
		output += "\nWould make " + util.IntToStr(replacements) + " replacement(s)"

		// Include diff information in metadata for UI to display
		// The UI can extract this and show a visual diff
		output += "\n\n--- DIFF ---\n"
		output += "Old content length: " + util.IntToStr(len(contentStr)) + " bytes\n"
		output += "New content length: " + util.IntToStr(len(newContent)) + " bytes\n"

		return Result{
			Success:    true,
			Output:     output,
			MatchCount: replacements,
		}, nil
	}

	// Create backup if requested (either via parameter or executor config)
	if createBackup || e.BackupOriginal {
		backupPath := filePath + ".bak"
		// TOOL-11 fix: Validate backup path before writing
		validatedBackupPath, err := ValidatePathSecure(backupPath)
		if err != nil {
			return Result{
				Success: false,
				Error:   "backup path validation failed: " + err.Error(),
			}, nil
		}
		if err := os.WriteFile(validatedBackupPath, content, info.Mode()); err != nil {
			return Result{
				Success: false,
				Error:   "failed to create backup: " + err.Error(),
			}, nil
		}
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(newContent), info.Mode()); err != nil {
		return Result{
			Success: false,
			Error:   "cannot write file: " + err.Error(),
		}, nil
	}

	// Build success message
	var output string
	if replacements == 1 {
		output = "Edited " + filePath + " (1 replacement)"
	} else {
		output = "Edited " + filePath + " (" + util.IntToStr(replacements) + " replacements)"
	}

	if createBackup || e.BackupOriginal {
		output += "\nBackup saved to: " + filePath + ".bak"
	}

	return Result{
		Success:      true,
		Output:       output,
		BytesWritten: int64(len(newContent)),
		LinesCount:   countLines(newContent),
		MatchCount:   replacements,
	}, nil
}

// restoreFromBackup restores a file from its .bak backup
func (e *EditExecutor) restoreFromBackup(filePath string) (Result, error) {
	backupPath := filePath + ".bak"

	// TOOL-11 fix: Validate backup path before reading
	validatedBackupPath, err := ValidatePathSecure(backupPath)
	if err != nil {
		return Result{
			Success: false,
			Error:   "backup path validation failed: " + err.Error(),
		}, nil
	}

	// Check if backup exists
	backupInfo, err := os.Stat(validatedBackupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{
				Success: false,
				Error:   "backup file not found: " + backupPath + " - no backup exists to restore from",
			}, nil
		}
		return Result{
			Success: false,
			Error:   "cannot access backup file: " + err.Error(),
		}, nil
	}

	// Read backup content
	backupContent, err := os.ReadFile(validatedBackupPath)
	if err != nil {
		return Result{
			Success: false,
			Error:   "cannot read backup file: " + err.Error(),
		}, nil
	}

	// Get original file info for permissions (if it exists)
	mode := backupInfo.Mode()
	if originalInfo, err := os.Stat(filePath); err == nil {
		mode = originalInfo.Mode()
	}

	// Restore the file
	if err := os.WriteFile(filePath, backupContent, mode); err != nil {
		return Result{
			Success: false,
			Error:   "cannot restore file: " + err.Error(),
		}, nil
	}

	return Result{
		Success:      true,
		Output:       "Restored " + filePath + " from backup",
		BytesWritten: int64(len(backupContent)),
		LinesCount:   countLines(string(backupContent)),
	}, nil
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

// truncateString truncates a string to maxLen runes (characters), adding "..." if truncated.
// UNICODE: Rune-aware truncation preserves multi-byte characters.
func truncateString(s string, maxLen int) string {
	return util.TruncateRunes(s, maxLen)
}

// =============================================================================
// DIFF PREVIEW
// =============================================================================

// GetDiffPreview returns a diff preview for an Edit operation without executing it.
// This allows the UI to show what will change before applying the edit.
func (e *EditExecutor) GetDiffPreview(params map[string]interface{}) (oldContent, newContent string, err error) {
	// Extract parameters (same as Execute)
	filePath, _ := params["file_path"].(string)
	oldString, _ := params["old_string"].(string)
	newString, _ := params["new_string"].(string)
	replaceAll := getBoolParam(params, "replace_all", false)
	useRegex := getBoolParam(params, "use_regex", false)

	if filePath == "" {
		return "", "", &SecurityError{
			Type:    "validation",
			Message: "file_path is required",
		}
	}

	if oldString == "" {
		return "", "", &SecurityError{
			Type:    "validation",
			Message: "old_string is required",
		}
	}

	// Open and validate file
	file, err := OpenSecureFile(filePath, os.O_RDONLY)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", err
	}

	contentStr := string(content)
	oldContent = contentStr

	// Perform the replacement (same logic as Execute)
	if useRegex {
		re, err := regexp.Compile(oldString)
		if err != nil {
			return "", "", err
		}

		if replaceAll {
			newContent = re.ReplaceAllString(contentStr, newString)
		} else {
			replacements := 0
			newContent = re.ReplaceAllStringFunc(contentStr, func(match string) string {
				if replacements == 0 {
					replacements++
					return re.ReplaceAllString(match, newString)
				}
				return match
			})
		}
	} else {
		if replaceAll {
			newContent = strings.ReplaceAll(contentStr, oldString, newString)
		} else {
			newContent = strings.Replace(contentStr, oldString, newString, 1)
		}
	}

	return oldContent, newContent, nil
}
