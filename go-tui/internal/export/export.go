// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package export provides conversation export functionality for rigrun TUI.
// Supports exporting conversations to various formats with styling and metadata.
package export

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// =============================================================================
// EXPORT INTERFACE
// =============================================================================

// Exporter defines the interface for conversation exporters.
type Exporter interface {
	// Export converts a conversation to the target format and returns the content.
	Export(conv *storage.StoredConversation) ([]byte, error)

	// FileExtension returns the appropriate file extension (e.g., ".md", ".html").
	FileExtension() string

	// MimeType returns the MIME type for the exported format.
	MimeType() string
}

// =============================================================================
// EXPORT OPTIONS
// =============================================================================

// Options configures export behavior.
type Options struct {
	// OutputDir is the directory where files will be saved.
	// Default: current working directory
	OutputDir string

	// OpenAfterExport opens the file in the default application.
	OpenAfterExport bool

	// IncludeMetadata includes metadata header (timestamp, model, stats).
	IncludeMetadata bool

	// IncludeTimestamps includes per-message timestamps.
	IncludeTimestamps bool

	// Theme for HTML export ("light" or "dark").
	// Default: "dark"
	Theme string
}

// DefaultOptions returns default export options.
func DefaultOptions() *Options {
	return &Options{
		OutputDir:         ".",
		OpenAfterExport:   true,
		IncludeMetadata:   true,
		IncludeTimestamps: true,
		Theme:             "dark",
	}
}

// =============================================================================
// EXPORT FUNCTIONS
// =============================================================================

// ExportToFile exports a conversation to a file using the specified exporter.
// Returns the output file path or an error.
//
// NOTE: Large conversations may require significant memory during export. The entire
// conversation is loaded into memory and formatted as a string before writing to disk.
// For very large conversations (>10K messages or >100MB), consider exporting in smaller
// chunks or monitoring memory usage.
//
// TIMEZONE: Per-message timestamps are formatted without timezone information. The
// conversation's CreatedAt timestamp in metadata includes timezone (RFC3339 format).
func ExportToFile(conv *storage.StoredConversation, exporter Exporter, opts *Options) (string, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Export the content
	content, err := exporter.Export(conv)
	if err != nil {
		return "", fmt.Errorf("export failed: %w", err)
	}

	// Generate output filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("conversation_%s_%s%s",
		sanitizeFilename(conv.Summary),
		timestamp,
		exporter.FileExtension(),
	)

	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// Write to file
	outputPath := filepath.Join(opts.OutputDir, filename)
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// Open in default application if requested
	if opts.OpenAfterExport {
		if err := openFile(outputPath); err != nil {
			// Non-fatal - file was still created successfully
			fmt.Printf("Warning: Could not open file: %v\n", err)
		}
	}

	return outputPath, nil
}

// ExportMarkdown exports to Markdown format.
func ExportMarkdown(conv *storage.StoredConversation, opts *Options) (string, error) {
	exporter := NewMarkdownExporter(opts)
	return ExportToFile(conv, exporter, opts)
}

// ExportHTML exports to HTML format.
func ExportHTML(conv *storage.StoredConversation, opts *Options) (string, error) {
	exporter := NewHTMLExporter(opts)
	return ExportToFile(conv, exporter, opts)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// sanitizeFilename removes or replaces characters that are invalid in filenames.
func sanitizeFilename(s string) string {
	// Limit length
	maxLen := 50
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen])
	}

	// Replace problematic characters (Windows and Unix)
	replacer := map[rune]rune{
		'/':  '-',
		'\\': '-',
		':':  '-',
		'*':  '-',
		'?':  '-',
		'"':  '-',
		'<':  '-',
		'>':  '-',
		'|':  '-',
		' ':  '_',
		'\t': '_',
		'\n': '_',
		'\r': '_',
	}

	result := []rune{}
	for _, r := range s {
		if replacement, found := replacer[r]; found {
			result = append(result, replacement)
		} else if r < 32 || r == 127 {
			// Replace control characters
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}

	if len(result) == 0 {
		return "conversation"
	}

	return string(result)
}

// openFile opens a file in the default application for the OS.
func openFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Properly quote path for Windows cmd - use quoted empty string for window title
		// and the path should be the last argument
		cmd = exec.Command("cmd", "/c", "start", `""`, path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// formatDuration formats a duration in milliseconds to a human-readable string.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	if seconds < 60 {
		return fmt.Sprintf("%.2fs", seconds)
	}
	minutes := int(seconds / 60)
	remainingSeconds := int(seconds) % 60
	return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
}

// formatTokensPerSec formats tokens per second for display.
func formatTokensPerSec(tps float64) string {
	if tps == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f tok/s", tps)
}

// formatTimestamp formats a timestamp for display.
func formatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// formatShortTimestamp formats a timestamp for inline display.
func formatShortTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}
