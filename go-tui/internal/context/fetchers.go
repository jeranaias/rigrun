// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system for including context in messages.
package context

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/index"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// ERRORS
// =============================================================================

var (
	// ErrFileNotFound is returned when a file doesn't exist.
	ErrFileNotFound = errors.New("file not found")

	// ErrFileTooLarge is returned when a file exceeds size limits.
	ErrFileTooLarge = errors.New("file too large")

	// ErrClipboardEmpty is returned when the clipboard is empty.
	ErrClipboardEmpty = errors.New("clipboard is empty")

	// ErrClipboardUnavailable is returned when clipboard access fails.
	ErrClipboardUnavailable = errors.New("clipboard unavailable")

	// ErrNotGitRepo is returned when not in a git repository.
	ErrNotGitRepo = errors.New("not a git repository")

	// ErrNoError is returned when there's no stored error.
	ErrNoError = errors.New("no recent error stored")
)

// =============================================================================
// FETCHER CONFIG
// =============================================================================

// FetcherConfig holds configuration for context fetchers.
type FetcherConfig struct {
	// MaxFileSize is the maximum file size to read (default: 100KB)
	MaxFileSize int64

	// MaxLines is the maximum number of lines to read (default: 1000)
	MaxLines int

	// MaxCodebaseDepth is the maximum directory depth for @codebase
	MaxCodebaseDepth int

	// MaxCodebaseFiles is the maximum number of files to list
	MaxCodebaseFiles int

	// WorkingDirectory is the base directory for relative paths
	WorkingDirectory string

	// GitCommitCount is the number of commits to show for @git
	GitCommitCount int

	// IgnorePatterns for @codebase (gitignore-style)
	IgnorePatterns []string
}

// DefaultConfig returns the default fetcher configuration.
func DefaultConfig() *FetcherConfig {
	wd, _ := os.Getwd()
	return &FetcherConfig{
		MaxFileSize:      100 * 1024, // 100KB
		MaxLines:         1000,
		MaxCodebaseDepth: 5,
		MaxCodebaseFiles: 100,
		WorkingDirectory: wd,
		GitCommitCount:   10,
		IgnorePatterns: []string{
			".git",
			"node_modules",
			"__pycache__",
			".venv",
			"vendor",
			"dist",
			"build",
			".idea",
			".vscode",
		},
	}
}

// =============================================================================
// FETCHER
// =============================================================================

// Fetcher handles fetching content for @ mentions.
type Fetcher struct {
	config *FetcherConfig

	// LastError stores the last error for @error
	lastError string

	// FileCache for caching file reads (optional)
	fileCache *FileCache

	// CodebaseIndex for intelligent @codebase (optional)
	codebaseIndex *index.CodebaseIndex
}

// NewFetcher creates a new fetcher with the given config.
func NewFetcher(config *FetcherConfig) *Fetcher {
	if config == nil {
		config = DefaultConfig()
	}
	return &Fetcher{
		config:    config,
		fileCache: DefaultFileCache, // Use global cache by default
	}
}

// NewFetcherWithCache creates a new fetcher with a custom file cache.
func NewFetcherWithCache(config *FetcherConfig, cache *FileCache) *Fetcher {
	if config == nil {
		config = DefaultConfig()
	}
	return &Fetcher{
		config:    config,
		fileCache: cache,
	}
}

// SetFileCache sets the file cache to use.
func (f *Fetcher) SetFileCache(cache *FileCache) {
	f.fileCache = cache
}

// GetFileCacheStats returns file cache statistics.
func (f *Fetcher) GetFileCacheStats() FileCacheStats {
	if f.fileCache == nil {
		return FileCacheStats{}
	}
	return f.fileCache.Stats()
}

// SetCodebaseIndex sets the codebase index to use for @codebase mentions.
func (f *Fetcher) SetCodebaseIndex(idx *index.CodebaseIndex) {
	f.codebaseIndex = idx
}

// GetCodebaseIndex returns the codebase index if set.
func (f *Fetcher) GetCodebaseIndex() *index.CodebaseIndex {
	return f.codebaseIndex
}

// FetchAll fetches content for all mentions.
func (f *Fetcher) FetchAll(mentions []Mention) []Mention {
	result := make([]Mention, len(mentions))
	copy(result, mentions)

	for i := range result {
		f.Fetch(&result[i])
	}

	return result
}

// Fetch fetches content for a single mention.
func (f *Fetcher) Fetch(m *Mention) {
	switch m.Type {
	case MentionFile:
		m.Content, m.Error = f.FetchFile(m.Path)
	case MentionClipboard:
		m.Content, m.Error = f.FetchClipboard()
	case MentionGit:
		m.Content, m.Error = f.FetchGit(m.Range)
	case MentionCodebase:
		m.Content, m.Error = f.FetchCodebase()
	case MentionLastError:
		m.Content, m.Error = f.FetchError()
	case MentionURL:
		m.Content, m.Error = f.FetchURL(m.URL)
	}
}

// =============================================================================
// FILE FETCHER
// =============================================================================

// FetchFile reads the contents of a file.
// Uses cache for faster repeated reads.
func (f *Fetcher) FetchFile(path string) (string, error) {
	// Resolve path
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.config.WorkingDirectory, path)
	}

	// Check cache first (if available)
	if f.fileCache != nil {
		if content, _, cacheHit := f.fileCache.Get(path); cacheHit {
			return content, nil
		}
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrFileNotFound
		}
		return "", err
	}

	// Check if it's a directory
	if info.IsDir() {
		return "", errors.New("path is a directory")
	}

	// Check size limit
	if info.Size() > f.config.MaxFileSize {
		return "", ErrFileTooLarge
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Convert to string and limit lines
	text := string(content)
	lines := strings.Split(text, "\n")

	truncated := false
	if len(lines) > f.config.MaxLines {
		lines = lines[:f.config.MaxLines]
		lines = append(lines, "... (truncated)")
		truncated = true
	}

	// Format with line numbers
	result := formatWithLineNumbers(lines, path)

	// Cache the result (only if not truncated)
	if f.fileCache != nil && !truncated {
		f.fileCache.Put(path, result, info.ModTime(), len(lines))
	}

	return result, nil
}

// FetchFileCached is like FetchFile but returns whether result was cached.
func (f *Fetcher) FetchFileCached(path string) (string, bool, error) {
	// Resolve path
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.config.WorkingDirectory, path)
	}

	// Check cache first
	if f.fileCache != nil {
		if content, _, cacheHit := f.fileCache.Get(path); cacheHit {
			return content, true, nil
		}
	}

	// Fall back to regular fetch
	content, err := f.FetchFile(path)
	return content, false, err
}

// formatWithLineNumbers formats content with line numbers.
func formatWithLineNumbers(lines []string, path string) string {
	var sb strings.Builder

	// Header with filename
	sb.WriteString("File: ")
	sb.WriteString(path)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", 40))
	sb.WriteString("\n")

	// Content with line numbers
	for i, line := range lines {
		lineNum := i + 1
		sb.WriteString(padInt(lineNum, 4))
		sb.WriteString("| ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// padInt pads an integer to the specified width.
func padInt(n, width int) string {
	s := util.IntToString(n)
	for len(s) < width {
		s = " " + s
	}
	return s
}

// =============================================================================
// CLIPBOARD FETCHER
// =============================================================================

// clipboardTimeout is the default timeout for clipboard operations.
const clipboardTimeout = 5 * time.Second

// FetchClipboard reads the clipboard contents.
func (f *Fetcher) FetchClipboard() (string, error) {
	return f.FetchClipboardWithContext(context.Background())
}

// FetchClipboardWithContext reads the clipboard contents with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) FetchClipboardWithContext(ctx context.Context) (string, error) {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, clipboardTimeout)
		defer cancel()
	}

	// Try different clipboard commands based on platform
	var cmd *exec.Cmd

	// Try xclip first (Linux)
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd = exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-o")
	} else if _, err := exec.LookPath("xsel"); err == nil {
		// Try xsel (Linux)
		cmd = exec.CommandContext(ctx, "xsel", "--clipboard", "--output")
	} else if _, err := exec.LookPath("pbpaste"); err == nil {
		// macOS
		cmd = exec.CommandContext(ctx, "pbpaste")
	} else if _, err := exec.LookPath("powershell.exe"); err == nil {
		// Windows
		cmd = exec.CommandContext(ctx, "powershell.exe", "-command", "Get-Clipboard")
	} else {
		return "", ErrClipboardUnavailable
	}

	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		return "", ErrClipboardUnavailable
	}

	content := strings.TrimSpace(string(output))
	if content == "" {
		return "", ErrClipboardEmpty
	}

	return "Clipboard content:\n" + strings.Repeat("-", 40) + "\n" + content, nil
}

// =============================================================================
// GIT FETCHER
// =============================================================================

// gitTimeout is the default timeout for git operations.
const gitTimeout = 10 * time.Second

// FetchGit fetches git context (commits, diff, status).
func (f *Fetcher) FetchGit(gitRange string) (string, error) {
	return f.FetchGitWithContext(context.Background(), gitRange)
}

// FetchGitWithContext fetches git context with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) FetchGitWithContext(ctx context.Context, gitRange string) (string, error) {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, gitTimeout)
		defer cancel()
	}

	// Check if we're in a git repo
	if !f.isGitRepoWithContext(ctx) {
		return "", ErrNotGitRepo
	}

	var sb strings.Builder
	sb.WriteString("Git Context\n")
	sb.WriteString(strings.Repeat("=", 40))
	sb.WriteString("\n\n")

	// Get recent commits
	commits, err := f.getGitCommitsWithContext(ctx, gitRange)
	if err == nil && commits != "" {
		sb.WriteString("Recent Commits:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString(commits)
		sb.WriteString("\n")
	}

	// Get status
	status, err := f.getGitStatusWithContext(ctx)
	if err == nil && status != "" {
		sb.WriteString("\nStatus:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString(status)
		sb.WriteString("\n")
	}

	// Get diff summary
	diff, err := f.getGitDiffWithContext(ctx)
	if err == nil && diff != "" {
		sb.WriteString("\nChanges:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString(diff)
		sb.WriteString("\n")
	}

	result := sb.String()
	if result == "Git Context\n"+strings.Repeat("=", 40)+"\n\n" {
		return "No git information available", nil
	}

	return result, nil
}

// isGitRepo checks if the current directory is in a git repo.
func (f *Fetcher) isGitRepo() bool {
	return f.isGitRepoWithContext(context.Background())
}

// isGitRepoWithContext checks if the current directory is in a git repo with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) isGitRepoWithContext(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = f.config.WorkingDirectory
	return cmd.Run() == nil
}

// getGitCommits returns recent commit history.
func (f *Fetcher) getGitCommits(gitRange string) (string, error) {
	return f.getGitCommitsWithContext(context.Background(), gitRange)
}

// getGitCommitsWithContext returns recent commit history with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) getGitCommitsWithContext(ctx context.Context, gitRange string) (string, error) {
	args := []string{"log", "--oneline"}

	if gitRange != "" {
		args = append(args, gitRange)
	} else {
		args = append(args, "-n", util.IntToString(f.config.GitCommitCount))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.config.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// getGitStatus returns the current git status.
func (f *Fetcher) getGitStatus() (string, error) {
	return f.getGitStatusWithContext(context.Background())
}

// getGitStatusWithContext returns the current git status with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) getGitStatusWithContext(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--short")
	cmd.Dir = f.config.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// getGitDiff returns a summary of changes.
func (f *Fetcher) getGitDiff() (string, error) {
	return f.getGitDiffWithContext(context.Background())
}

// getGitDiffWithContext returns a summary of changes with context support.
// CANCELLATION: Context enables timeout and cancellation
func (f *Fetcher) getGitDiffWithContext(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = f.config.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// =============================================================================
// CODEBASE FETCHER
// =============================================================================

// FetchCodebase generates a summary of the codebase structure.
// If a codebase index is available, uses it for intelligent search.
// Otherwise, falls back to simple directory tree.
func (f *Fetcher) FetchCodebase() (string, error) {
	// Use index if available
	if f.codebaseIndex != nil && f.codebaseIndex.IsIndexed() {
		return f.fetchCodebaseWithIndex()
	}

	// Fallback to simple directory tree
	return f.fetchCodebaseSimple()
}

// fetchCodebaseWithIndex generates codebase summary using the index.
func (f *Fetcher) fetchCodebaseWithIndex() (string, error) {
	var sb strings.Builder

	sb.WriteString("Codebase Index\n")
	sb.WriteString(strings.Repeat("=", 40))
	sb.WriteString("\n\n")

	// Get statistics
	stats := f.codebaseIndex.Stats()
	sb.WriteString(fmt.Sprintf("Files: %d\n", stats.FileCount))
	sb.WriteString(fmt.Sprintf("Symbols: %d\n", stats.SymbolCount))
	if !stats.LastIndexed.IsZero() {
		sb.WriteString(fmt.Sprintf("Last Indexed: %s\n", stats.LastIndexed.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("\n")

	// Get language statistics
	langStats, err := f.codebaseIndex.GetLanguageStats()
	if err == nil && len(langStats) > 0 {
		sb.WriteString("Languages:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		for lang, count := range langStats {
			sb.WriteString(fmt.Sprintf("  %s: %d files\n", lang, count))
		}
		sb.WriteString("\n")
	}

	// Get repository structure (top-level summary)
	structure, err := f.codebaseIndex.GetRepositoryStructure()
	if err == nil && len(structure) > 0 {
		sb.WriteString("Repository Structure:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")

		// Show top-level directories
		dirs := make([]string, 0, len(structure))
		for dir := range structure {
			if strings.Count(dir, "/") <= 1 {
				dirs = append(dirs, dir)
			}
		}

		for _, dir := range dirs {
			files := structure[dir]
			sb.WriteString(fmt.Sprintf("  %s/  (%d files)\n", dir, len(files)))

			// Show some key files
			shown := 0
			for _, file := range files {
				if shown >= 3 {
					break
				}
				if file.SymbolCount > 0 {
					sb.WriteString(fmt.Sprintf("    - %s (%d symbols)\n", filepath.Base(file.Path), file.SymbolCount))
					shown++
				}
			}
			if len(files) > shown {
				sb.WriteString(fmt.Sprintf("    ... and %d more files\n", len(files)-shown))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Tip: Use search to find specific symbols by name or type.\n")
	sb.WriteString("Example: Search for 'NewServer' to find server constructors.\n")

	return sb.String(), nil
}

// fetchCodebaseSimple generates a simple directory tree (fallback).
func (f *Fetcher) fetchCodebaseSimple() (string, error) {
	var sb strings.Builder

	sb.WriteString("Codebase Structure\n")
	sb.WriteString(strings.Repeat("=", 40))
	sb.WriteString("\n\n")

	// Build directory tree
	tree, err := f.buildDirectoryTree(f.config.WorkingDirectory, "", 0)
	if err != nil {
		return "", err
	}

	sb.WriteString(tree)

	// File statistics
	stats := f.getFileStats(f.config.WorkingDirectory)
	if stats != "" {
		sb.WriteString("\n\nFile Statistics:\n")
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString(stats)
	}

	return sb.String(), nil
}

// buildDirectoryTree builds a tree representation of the directory.
func (f *Fetcher) buildDirectoryTree(dir string, prefix string, depth int) (string, error) {
	if depth > f.config.MaxCodebaseDepth {
		return prefix + "... (max depth reached)\n", nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	fileCount := 0

	for i, entry := range entries {
		if fileCount >= f.config.MaxCodebaseFiles {
			sb.WriteString(prefix + "... (more files)\n")
			break
		}

		name := entry.Name()

		// Skip ignored patterns
		if f.shouldIgnore(name) {
			continue
		}

		isLast := i == len(entries)-1
		connector := "+-- "
		childPrefix := prefix + "|   "
		if isLast {
			connector = "`-- "
			childPrefix = prefix + "    "
		}

		if entry.IsDir() {
			sb.WriteString(prefix + connector + name + "/\n")

			// Recurse into directory
			subtree, err := f.buildDirectoryTree(
				filepath.Join(dir, name),
				childPrefix,
				depth+1,
			)
			if err == nil {
				sb.WriteString(subtree)
			}
		} else {
			sb.WriteString(prefix + connector + name + "\n")
			fileCount++
		}
	}

	return sb.String(), nil
}

// shouldIgnore checks if a file/directory should be ignored.
func (f *Fetcher) shouldIgnore(name string) bool {
	// Always ignore hidden files except .gitignore
	if strings.HasPrefix(name, ".") && name != ".gitignore" {
		return true
	}

	for _, pattern := range f.config.IgnorePatterns {
		if name == pattern {
			return true
		}
	}

	return false
}

// getFileStats returns statistics about file types.
func (f *Fetcher) getFileStats(dir string) string {
	stats := make(map[string]int)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip ignored directories
		if info.IsDir() {
			name := info.Name()
			if f.shouldIgnore(name) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == "" {
			ext = "(no extension)"
		}
		stats[ext]++

		return nil
	})

	if len(stats) == 0 {
		return ""
	}

	var sb strings.Builder
	for ext, count := range stats {
		sb.WriteString(ext + ": " + util.IntToString(count) + " files\n")
	}

	return sb.String()
}

// =============================================================================
// ERROR FETCHER
// =============================================================================

// FetchError returns the last stored error.
// It checks both the instance error and the global error storage.
func (f *Fetcher) FetchError() (string, error) {
	// Check instance error first
	if f.lastError != "" {
		return "Last Error:\n" + strings.Repeat("-", 40) + "\n" + f.lastError, nil
	}

	// Fall back to global error storage
	if globalLastError != "" {
		return "Last Error:\n" + strings.Repeat("-", 40) + "\n" + globalLastError, nil
	}

	return "", ErrNoError
}

// StoreError stores an error message for @error retrieval.
func (f *Fetcher) StoreError(err string) {
	f.lastError = err
}

// ClearError clears the stored error.
func (f *Fetcher) ClearError() {
	f.lastError = ""
}

// =============================================================================
// URL FETCHER
// =============================================================================

// FetchURL fetches content from a URL (stub - would need HTTP client).
func (f *Fetcher) FetchURL(url string) (string, error) {
	// This is a simplified implementation
	// A full implementation would use net/http to fetch the URL

	// For now, just return a placeholder
	return "URL content for: " + url + "\n(URL fetching not yet implemented)", nil
}

// =============================================================================
// GLOBAL ERROR STORAGE
// =============================================================================

// Global error storage for @error mentions
var globalLastError string

// StoreLastError stores the last error globally.
func StoreLastError(err string) {
	globalLastError = err
}

// GetLastError returns the last stored error.
func GetLastError() string {
	return globalLastError
}

// ClearLastError clears the global error storage.
func ClearLastError() {
	globalLastError = ""
}
