// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// search.go implements GlobTool and GrepTool for file searching.
package tools

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// GLOB TOOL EXECUTOR
// =============================================================================

// GlobExecutor implements file pattern matching.
// Supports glob patterns like "**/*.go" or "src/**/*.ts".
// Returns matching file paths sorted by modification time (newest first).
type GlobExecutor struct {
	// MaxResults limits the number of results (default: 100)
	MaxResults int

	// IgnorePatterns are patterns to ignore (e.g., .git, node_modules)
	IgnorePatterns []string
}

// Execute finds files matching a glob pattern.
func (e *GlobExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxResults == 0 {
		e.MaxResults = 100
	}
	if len(e.IgnorePatterns) == 0 {
		e.IgnorePatterns = []string{
			".git",
			"node_modules",
			"__pycache__",
			".venv",
			"venv",
			".idea",
			".vscode",
			"target",
			"dist",
			"build",
			".cache",
		}
	}

	// Extract parameters
	pattern, _ := params["pattern"].(string)
	basePath := getStringParam(params, "path", ".")

	// Validate pattern
	if pattern == "" {
		return Result{
			Success:  false,
			Error:    "pattern is required",
			Duration: time.Since(start),
		}, nil
	}

	// Validate pattern security
	if err := validateGlobPattern(pattern); err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Handle absolute paths in the pattern
	// If pattern contains an absolute path, extract basePath from it
	pattern, basePath = extractBasePathFromPattern(pattern, basePath)

	// TOOL-9 fix: Validate path security using comprehensive secure validation
	validatedPath, err := ValidatePathSecure(basePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	basePath = validatedPath

	// Check if base path exists
	if _, err := os.Stat(basePath); err != nil {
		if os.IsNotExist(err) {
			return Result{
				Success:  false,
				Error:    "path not found: " + basePath,
				Duration: time.Since(start),
			}, nil
		}
		return Result{
			Success:  false,
			Error:    "cannot access path: " + err.Error(),
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

	// Collect matching files
	var matches []fileEntry
	truncated := false
	totalCount := 0

	walkErr := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		// Skip ignored directories
		if d.IsDir() && e.shouldIgnore(d.Name()) {
			return filepath.SkipDir
		}

		// Skip directories in results
		if d.IsDir() {
			return nil
		}

		// Get relative path for matching
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			relPath = path
		}

		// Match against pattern
		matched, err := matchGlobPattern(pattern, relPath)
		if err != nil {
			return nil // Invalid pattern, skip
		}

		if matched {
			totalCount++

			// Check limit
			if len(matches) >= e.MaxResults {
				truncated = true
				return nil // Continue counting but don't add more
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			matches = append(matches, fileEntry{
				path:    path,
				modTime: info.ModTime(),
			})
		}

		return nil
	})

	if walkErr != nil && walkErr != context.Canceled && walkErr != filepath.SkipAll {
		return Result{
			Success:  false,
			Error:    "error walking directory: " + walkErr.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Sort by modification time (most recent first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime.After(matches[j].modTime)
	})

	// Build output
	var builder strings.Builder
	for _, m := range matches {
		builder.WriteString(m.path)
		builder.WriteString("\n")
	}

	output := builder.String()
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}

	// Handle no matches case
	if len(matches) == 0 {
		output = "No files found matching pattern '" + pattern + "' in '" + basePath + "'"
	}

	// Add count info if truncated
	if truncated {
		output = output + "\n\n[Results limited to " + util.IntToStr(e.MaxResults) + " files. Total matches: " + util.IntToStr(totalCount) + "]"
	}

	return Result{
		Success:      true,
		Output:       output,
		FilesMatched: len(matches),
		Truncated:    truncated,
		Duration:     time.Since(start),
	}, nil
}

// shouldIgnore checks if a name should be ignored.
func (e *GlobExecutor) shouldIgnore(name string) bool {
	for _, ignore := range e.IgnorePatterns {
		if name == ignore {
			return true
		}
	}
	return false
}

// =============================================================================
// GLOB HELPER TYPES AND FUNCTIONS
// =============================================================================

// fileEntry holds file path and modification time.
type fileEntry struct {
	path    string
	modTime time.Time
}

// extractBasePathFromPattern handles patterns that contain absolute paths.
// If the pattern is "C:/foo/bar/**/*.go", this extracts:
//   - basePath: "C:/foo/bar"
//   - pattern: "**/*.go"
//
// This allows users to specify full paths in the pattern parameter.
func extractBasePathFromPattern(pattern, defaultBasePath string) (newPattern, basePath string) {
	// Normalize to forward slashes for processing
	normalized := strings.ReplaceAll(pattern, "\\", "/")

	// Check if this is an absolute path (Unix "/" or Windows drive letter "C:")
	isAbsolute := strings.HasPrefix(normalized, "/") ||
		(len(normalized) >= 2 && normalized[1] == ':')

	if !isAbsolute {
		return pattern, defaultBasePath
	}

	// Find the first glob wildcard character
	wildcardIdx := -1
	for i, c := range normalized {
		if c == '*' || c == '?' || c == '[' {
			wildcardIdx = i
			break
		}
	}

	// No wildcard found - treat entire pattern as a path pattern
	if wildcardIdx == -1 {
		// This is a literal path, use its directory as basePath
		lastSlash := strings.LastIndex(normalized, "/")
		if lastSlash > 0 {
			basePath = pattern[:lastSlash]
			newPattern = pattern[lastSlash+1:]
			return newPattern, basePath
		}
		return pattern, defaultBasePath
	}

	// Find the last slash before the wildcard - that's where basePath ends
	lastSlashBeforeWildcard := strings.LastIndex(normalized[:wildcardIdx], "/")
	if lastSlashBeforeWildcard > 0 {
		// Use original pattern (may have backslashes on Windows) for basePath
		basePath = pattern[:lastSlashBeforeWildcard]
		newPattern = pattern[lastSlashBeforeWildcard+1:]
		return newPattern, basePath
	}

	// Wildcard is at the beginning or right after drive letter, use current pattern
	return pattern, defaultBasePath
}

// validateGlobPattern validates a glob pattern for security concerns.
// Allows absolute paths (including Windows drive letters) but prevents path traversal.
// Comprehensive path security is handled by ValidatePathSecure in the Execute function.
func validateGlobPattern(pattern string) error {
	// NOTE: Absolute paths (Unix "/" or Windows "C:") are allowed.
	// The ValidatePathSecure function provides comprehensive security validation
	// including checking against blocked/system directories.

	// Check for patterns that escape the workspace with ..
	if strings.HasPrefix(pattern, "..") {
		return &PatternError{
			Pattern: pattern,
			Reason:  "pattern cannot escape the workspace directory",
		}
	}

	// Check for .. anywhere in the pattern (path traversal attack)
	normalized := strings.ReplaceAll(pattern, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return &PatternError{
				Pattern: pattern,
				Reason:  "pattern contains '..' which could escape the workspace",
			}
		}
	}

	return nil
}

// PatternError represents an invalid glob pattern.
type PatternError struct {
	Pattern string
	Reason  string
}

func (e *PatternError) Error() string {
	return "invalid glob pattern '" + e.Pattern + "': " + e.Reason
}

// matchGlobPattern matches a path against a glob pattern.
// Supports:
// - * matches any sequence of characters within a path segment
// - ** matches any sequence of characters including path separators
// - ? matches any single character
func matchGlobPattern(pattern, path string) (bool, error) {
	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle ** pattern
	if strings.Contains(pattern, "**") {
		return matchDoublestarPattern(pattern, path)
	}

	// Use standard filepath.Match for simple patterns
	return filepath.Match(pattern, path)
}

// matchDoublestarPattern handles ** patterns.
func matchDoublestarPattern(pattern, path string) (bool, error) {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 1 {
		// No **, use standard match
		return filepath.Match(pattern, path)
	}

	// Handle pattern like **/*.go
	if parts[0] == "" {
		// Pattern starts with **
		suffix := parts[1]
		if len(suffix) > 0 && suffix[0] == '/' {
			suffix = suffix[1:]
		}

		// Try matching suffix against path and all subpaths
		return matchSuffixPattern(suffix, path)
	}

	// Handle pattern like src/**/*.go
	prefix := parts[0]
	if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
		prefix = prefix[:len(prefix)-1]
	}

	// Check if path starts with prefix
	if !strings.HasPrefix(path, prefix) {
		return false, nil
	}

	// Get remaining path
	remaining := path[len(prefix):]
	if len(remaining) > 0 && remaining[0] == '/' {
		remaining = remaining[1:]
	}

	// Match remaining against suffix pattern
	suffix := parts[1]
	if len(suffix) > 0 && suffix[0] == '/' {
		suffix = suffix[1:]
	}

	return matchSuffixPattern(suffix, remaining)
}

// matchSuffixPattern matches a suffix pattern against a path.
func matchSuffixPattern(suffix, path string) (bool, error) {
	if suffix == "" {
		return true, nil
	}

	// Try matching from the end
	parts := strings.Split(path, "/")
	for i := 0; i <= len(parts); i++ {
		subpath := strings.Join(parts[i:], "/")
		matched, err := filepath.Match(suffix, subpath)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

// =============================================================================
// GREP TOOL EXECUTOR
// =============================================================================

// GrepExecutor implements content searching with regex.
// Searches file contents and returns matches in filepath:line:content format.
type GrepExecutor struct {
	// MaxResults limits the number of matches (default: 50)
	MaxResults int

	// MaxFileSize is the maximum file size to search (default: 5MB)
	MaxFileSize int64

	// MaxContextLines for context option (default: 10)
	MaxContextLines int

	// IgnorePatterns are directories to ignore
	IgnorePatterns []string

	// SensitivePatterns are file patterns to skip for security
	SensitivePatterns []string
}

// Execute searches for a pattern in files.
func (e *GrepExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.MaxResults == 0 {
		e.MaxResults = 50
	}
	if e.MaxFileSize == 0 {
		e.MaxFileSize = 5 * 1024 * 1024 // 5MB
	}
	if e.MaxContextLines == 0 {
		e.MaxContextLines = 10
	}
	if len(e.IgnorePatterns) == 0 {
		e.IgnorePatterns = []string{
			".git",
			"node_modules",
			"__pycache__",
			".venv",
			"venv",
			".idea",
			".vscode",
			"target",
			"dist",
			"build",
			".cache",
		}
	}
	if len(e.SensitivePatterns) == 0 {
		e.SensitivePatterns = []string{
			".env",
			".env.local",
			".git/config",
			".gitconfig",
			".ssh",
			".aws",
			"credentials",
			"secrets",
			".npmrc",
			"id_rsa",
			"id_ed25519",
		}
	}

	// Extract parameters
	pattern, _ := params["pattern"].(string)
	basePath := getStringParam(params, "path", ".")
	contextLines := getIntParam(params, "context", 0)
	globFilter := getStringParam(params, "glob", "")
	outputMode := getStringParam(params, "output_mode", "content")
	caseInsensitive := getBoolParam(params, "case_insensitive", false)

	// Validate output_mode
	if outputMode != "content" && outputMode != "files_with_matches" && outputMode != "count" {
		outputMode = "content" // Default to content if invalid
	}

	// Validate pattern
	if pattern == "" {
		return Result{
			Success:  false,
			Error:    "pattern is required",
			Duration: time.Since(start),
		}, nil
	}

	// Add case-insensitive flag if needed
	if caseInsensitive && !strings.HasPrefix(pattern, "(?i)") {
		pattern = "(?i)" + pattern
	}

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Result{
			Success:  false,
			Error:    "invalid regex pattern: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Clamp context lines
	if contextLines > e.MaxContextLines {
		contextLines = e.MaxContextLines
	}
	if contextLines < 0 {
		contextLines = 0
	}

	// TOOL-10 fix: Validate path security using comprehensive secure validation
	validatedPath, err := ValidatePathSecure(basePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	basePath = validatedPath

	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{
				Success:  false,
				Error:    "path not found: " + basePath,
				Duration: time.Since(start),
			}, nil
		}
		return Result{
			Success:  false,
			Error:    "cannot access path: " + err.Error(),
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

	// If it's a file, search just that file
	if !info.IsDir() {
		return e.searchSingleFile(ctx, basePath, re, contextLines, outputMode, start)
	}

	// Search directory
	return e.searchDirectoryFiles(ctx, basePath, re, contextLines, globFilter, outputMode, start)
}

// searchSingleFile searches a single file.
func (e *GrepExecutor) searchSingleFile(ctx context.Context, filePath string, re *regexp.Regexp, contextLines int, outputMode string, start time.Time) (Result, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    "cannot access file: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Skip large files
	if info.Size() > e.MaxFileSize {
		return Result{
			Success:  false,
			Error:    "file too large (max " + formatSize(e.MaxFileSize) + ")",
			Duration: time.Since(start),
		}, nil
	}

	// Check if file is sensitive
	if e.isSensitivePath(filePath) {
		return Result{
			Success:  false,
			Error:    "cannot search sensitive file",
			Duration: time.Since(start),
		}, nil
	}

	// Check if binary
	if isBinaryFileByExtension(filePath) {
		return Result{
			Success:      true,
			Output:       "No matches found (binary file skipped)",
			MatchCount:   0,
			FilesMatched: 0,
			Duration:     time.Since(start),
		}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return Result{
			Success:  false,
			Error:    "cannot open file: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}
	defer file.Close()

	matches, err := e.searchFileContent(ctx, file, filePath, re, contextLines)
	if err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	output := e.formatMatchesWithMode(matches, contextLines, outputMode, filePath)
	if len(matches) == 0 {
		output = "No matches found for pattern: " + re.String()
	}

	return Result{
		Success:      true,
		Output:       output,
		MatchCount:   len(matches),
		FilesMatched: boolToInt(len(matches) > 0),
		Duration:     time.Since(start),
	}, nil
}

// searchDirectoryFiles searches files in a directory.
func (e *GrepExecutor) searchDirectoryFiles(ctx context.Context, basePath string, re *regexp.Regexp, contextLines int, globFilter string, outputMode string, start time.Time) (Result, error) {
	var allMatches []grepMatch
	matchedFiles := make(map[string]int) // file -> match count for files_with_matches and count modes
	filesSearched := 0
	filesMatched := 0
	truncated := false

	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		// Skip ignored directories
		if d.IsDir() && e.shouldIgnoreDir(d.Name()) {
			return filepath.SkipDir
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip sensitive files
		if e.isSensitivePath(path) {
			return nil
		}

		// Skip binary files
		if isBinaryFileByExtension(path) {
			return nil
		}

		// Apply glob filter if specified
		if globFilter != "" {
			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				relPath = filepath.Base(path)
			}
			matched, err := matchGlobPattern(globFilter, relPath)
			if err != nil || !matched {
				// Also try matching just the filename
				matched, _ = matchGlobPattern(globFilter, filepath.Base(path))
				if !matched {
					return nil
				}
			}
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip large files
		if info.Size() > e.MaxFileSize {
			return nil
		}

		// TOOLS: Proper timeout, validation, and resource cleanup
		// Search file using helper function to ensure file is always closed
		matches, err := e.searchFileWithCleanup(ctx, path, re, contextLines)
		if err != nil {
			return nil
		}

		filesSearched++

		if len(matches) > 0 {
			filesMatched++
			matchedFiles[path] = len(matches)
			allMatches = append(allMatches, matches...)

			// Check limit
			if len(allMatches) >= e.MaxResults {
				truncated = true
				allMatches = allMatches[:e.MaxResults]
				return filepath.SkipAll
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled && err != filepath.SkipAll {
		return Result{
			Success:  false,
			Error:    "error walking directory: " + err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Format output based on output mode
	var output string
	switch outputMode {
	case "files_with_matches":
		if len(matchedFiles) == 0 {
			output = "No matches found for pattern: " + re.String()
		} else {
			var builder strings.Builder
			for file := range matchedFiles {
				builder.WriteString(file)
				builder.WriteString("\n")
			}
			output = strings.TrimSuffix(builder.String(), "\n")
		}
	case "count":
		if len(matchedFiles) == 0 {
			output = "No matches found for pattern: " + re.String()
		} else {
			var builder strings.Builder
			totalMatches := 0
			for file, count := range matchedFiles {
				builder.WriteString(file)
				builder.WriteString(":")
				builder.WriteString(util.IntToStr(count))
				builder.WriteString("\n")
				totalMatches += count
			}
			builder.WriteString("\nTotal: ")
			builder.WriteString(util.IntToStr(totalMatches))
			builder.WriteString(" matches in ")
			builder.WriteString(util.IntToStr(len(matchedFiles)))
			builder.WriteString(" files")
			output = builder.String()
		}
	default: // "content"
		output = e.formatMatches(allMatches, contextLines)
		if len(allMatches) == 0 {
			output = "No matches found for pattern: " + re.String()
		}
	}

	if truncated {
		output = output + "\n\n[Results limited to " + util.IntToStr(e.MaxResults) + " matches]"
	}

	return Result{
		Success:      true,
		Output:       output,
		MatchCount:   len(allMatches),
		FilesMatched: filesMatched,
		Truncated:    truncated,
		Duration:     time.Since(start),
	}, nil
}

// grepMatch represents a single grep match.
type grepMatch struct {
	file       string
	lineNumber int
	content    string
	context    []contextLine
}

// contextLine represents a context line around a match.
type contextLine struct {
	lineNumber int
	content    string
	isBefore   bool // true = before match, false = after match
}

// TOOLS: Proper timeout, validation, and resource cleanup
// searchFileWithCleanup opens a file, searches it, and ensures cleanup via defer.
// This prevents goroutine leaks and resource leaks if a panic occurs.
func (e *GrepExecutor) searchFileWithCleanup(ctx context.Context, filePath string, re *regexp.Regexp, contextLines int) ([]grepMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close() // Ensures cleanup even on panic

	return e.searchFileContent(ctx, file, filePath, re, contextLines)
}

// searchFileContent searches file content for matches.
func (e *GrepExecutor) searchFileContent(ctx context.Context, file *os.File, filePath string, re *regexp.Regexp, contextLines int) ([]grepMatch, error) {
	var matches []grepMatch
	var lines []string

	scanner := bufio.NewScanner(file)
	// Increase buffer for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for i, line := range lines {
		// Check context periodically
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, context.Canceled
			default:
			}
		}

		if re.MatchString(line) {
			match := grepMatch{
				file:       filePath,
				lineNumber: i + 1,
				content:    truncateLine(line, 500),
			}

			// Add context lines
			if contextLines > 0 {
				// Before context
				for j := max(0, i-contextLines); j < i; j++ {
					match.context = append(match.context, contextLine{
						lineNumber: j + 1,
						content:    truncateLine(lines[j], 500),
						isBefore:   true,
					})
				}

				// After context
				for j := i + 1; j <= min(len(lines)-1, i+contextLines); j++ {
					match.context = append(match.context, contextLine{
						lineNumber: j + 1,
						content:    truncateLine(lines[j], 500),
						isBefore:   false,
					})
				}
			}

			matches = append(matches, match)

			if len(matches) >= e.MaxResults {
				break
			}
		}
	}

	return matches, nil
}

// formatMatches formats grep matches for output.
func (e *GrepExecutor) formatMatches(matches []grepMatch, contextLines int) string {
	if len(matches) == 0 {
		return ""
	}

	var builder strings.Builder

	for i, m := range matches {
		// Add separator between matches with context
		if contextLines > 0 && i > 0 {
			builder.WriteString("--\n")
		}

		// Show context before
		for _, ctx := range m.context {
			if ctx.isBefore {
				builder.WriteString(m.file)
				builder.WriteString(":")
				builder.WriteString(util.IntToStr(ctx.lineNumber))
				builder.WriteString("-")
				builder.WriteString(ctx.content)
				builder.WriteString("\n")
			}
		}

		// Show match
		builder.WriteString(m.file)
		builder.WriteString(":")
		builder.WriteString(util.IntToStr(m.lineNumber))
		builder.WriteString(":")
		builder.WriteString(m.content)
		builder.WriteString("\n")

		// Show context after
		for _, ctx := range m.context {
			if !ctx.isBefore {
				builder.WriteString(m.file)
				builder.WriteString(":")
				builder.WriteString(util.IntToStr(ctx.lineNumber))
				builder.WriteString("+")
				builder.WriteString(ctx.content)
				builder.WriteString("\n")
			}
		}
	}

	output := builder.String()
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}

	return output
}

// formatMatchesWithMode formats grep matches based on output mode (for single file search).
func (e *GrepExecutor) formatMatchesWithMode(matches []grepMatch, contextLines int, outputMode string, filePath string) string {
	if len(matches) == 0 {
		return ""
	}

	switch outputMode {
	case "files_with_matches":
		return filePath
	case "count":
		return filePath + ":" + util.IntToStr(len(matches))
	default: // "content"
		return e.formatMatches(matches, contextLines)
	}
}

// shouldIgnoreDir checks if a directory should be ignored.
func (e *GrepExecutor) shouldIgnoreDir(name string) bool {
	for _, ignore := range e.IgnorePatterns {
		if name == ignore {
			return true
		}
	}
	return false
}

// isSensitivePath checks if a path is sensitive.
func (e *GrepExecutor) isSensitivePath(path string) bool {
	pathLower := strings.ToLower(path)
	pathNormalized := strings.ReplaceAll(pathLower, "\\", "/")

	for _, sensitive := range e.SensitivePatterns {
		if strings.Contains(pathNormalized, strings.ToLower(sensitive)) {
			return true
		}
	}

	// Check for key files
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" {
		return true
	}

	return false
}

// isBinaryFileByExtension checks if a file is likely binary by extension.
func isBinaryFileByExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".bin": true, ".dat": true, ".db": true, ".sqlite": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".bmp": true, ".tiff": true, ".webp": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true,
		".xlsx": true, ".ppt": true, ".pptx": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true,
		".7z": true, ".bz2": true, ".xz": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
		".wav": true, ".flac": true, ".ogg": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".pyc": true, ".pyo": true, ".class": true,
		".o": true, ".a": true, ".lib": true,
	}

	return binaryExts[ext]
}

// truncateLine truncates a line if it exceeds the max length.
// UNICODE: Rune-aware truncation preserves multi-byte characters.
func truncateLine(line string, maxLen int) string {
	return util.TruncateRunes(line, maxLen)
}

