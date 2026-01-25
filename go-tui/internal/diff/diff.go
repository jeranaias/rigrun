// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package diff provides diff computation and formatting for file changes.
package diff

import (
	"fmt"
	"strings"
)

// =============================================================================
// DIFF TYPES
// =============================================================================

// DiffLineType represents the type of a diff line.
type DiffLineType int

const (
	// DiffLineContext represents unchanged context lines
	DiffLineContext DiffLineType = iota
	// DiffLineAdded represents added lines
	DiffLineAdded
	// DiffLineRemoved represents removed lines
	DiffLineRemoved
)

// String returns the string representation of a diff line type.
func (t DiffLineType) String() string {
	switch t {
	case DiffLineContext:
		return "context"
	case DiffLineAdded:
		return "added"
	case DiffLineRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

// Prefix returns the diff prefix character for this line type.
func (t DiffLineType) Prefix() string {
	switch t {
	case DiffLineContext:
		return " "
	case DiffLineAdded:
		return "+"
	case DiffLineRemoved:
		return "-"
	default:
		return " "
	}
}

// =============================================================================
// DIFF LINE
// =============================================================================

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    DiffLineType // Type of line (added, removed, context)
	Content string       // The actual line content
	OldLine int          // Line number in old file (0 if added)
	NewLine int          // Line number in new file (0 if removed)
}

// =============================================================================
// DIFF HUNK
// =============================================================================

// DiffHunk represents a contiguous section of changes.
type DiffHunk struct {
	OldStart int        // Starting line in old file
	OldCount int        // Number of lines in old file
	NewStart int        // Starting line in new file
	NewCount int        // Number of lines in new file
	Lines    []DiffLine // The actual diff lines
}

// =============================================================================
// DIFF STATS
// =============================================================================

// DiffStats holds statistics about a diff.
type DiffStats struct {
	Additions int    // Number of added lines
	Deletions int    // Number of removed lines
	FileMode  string // "new", "modified", "deleted"
}

// =============================================================================
// DIFF
// =============================================================================

// Diff represents a complete file diff.
type Diff struct {
	FilePath   string     // Path to the file being diffed
	OldContent string     // Original file content
	NewContent string     // New file content
	Hunks      []DiffHunk // The diff hunks
	Stats      DiffStats  // Statistics
}

// =============================================================================
// DIFF COMPUTATION
// =============================================================================

// ComputeDiff creates a diff between old and new content using a simple
// line-by-line comparison algorithm (similar to Myers diff).
func ComputeDiff(filePath, oldContent, newContent string) *Diff {
	diff := &Diff{
		FilePath:   filePath,
		OldContent: oldContent,
		NewContent: newContent,
	}

	// Split into lines
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Determine file mode
	if oldContent == "" && newContent != "" {
		diff.Stats.FileMode = "new"
	} else if oldContent != "" && newContent == "" {
		diff.Stats.FileMode = "deleted"
	} else {
		diff.Stats.FileMode = "modified"
	}

	// Compute the diff using a simple LCS-based algorithm
	diffLines := computeLineDiff(oldLines, newLines)

	// Group diff lines into hunks with context
	diff.Hunks = groupIntoHunks(diffLines, oldLines, newLines)

	// Calculate statistics
	for _, line := range diffLines {
		switch line.Type {
		case DiffLineAdded:
			diff.Stats.Additions++
		case DiffLineRemoved:
			diff.Stats.Deletions++
		}
	}

	return diff
}

// splitLines splits content into lines, preserving empty lines.
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	// Split but preserve empty lines
	lines := strings.Split(content, "\n")
	// Remove trailing empty line if it exists (from final newline)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// computeLineDiff computes line-by-line differences using a simple algorithm.
// This is a simplified version that works well for most cases.
func computeLineDiff(oldLines, newLines []string) []DiffLine {
	var result []DiffLine

	// Simple case: both empty
	if len(oldLines) == 0 && len(newLines) == 0 {
		return result
	}

	// Simple case: only additions (new file)
	if len(oldLines) == 0 {
		for i, line := range newLines {
			result = append(result, DiffLine{
				Type:    DiffLineAdded,
				Content: line,
				OldLine: 0,
				NewLine: i + 1,
			})
		}
		return result
	}

	// Simple case: only deletions (file deleted)
	if len(newLines) == 0 {
		for i, line := range oldLines {
			result = append(result, DiffLine{
				Type:    DiffLineRemoved,
				Content: line,
				OldLine: i + 1,
				NewLine: 0,
			})
		}
		return result
	}

	// Use a simple LCS (Longest Common Subsequence) approach
	lcs := computeLCS(oldLines, newLines)

	oldIdx := 0
	newIdx := 0
	lcsIdx := 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		// Check if we're at a common line
		if lcsIdx < len(lcs) &&
		   oldIdx < len(oldLines) && newIdx < len(newLines) &&
		   oldLines[oldIdx] == newLines[newIdx] &&
		   oldLines[oldIdx] == lcs[lcsIdx] {
			// Context line (unchanged)
			result = append(result, DiffLine{
				Type:    DiffLineContext,
				Content: oldLines[oldIdx],
				OldLine: oldIdx + 1,
				NewLine: newIdx + 1,
			})
			oldIdx++
			newIdx++
			lcsIdx++
		} else if oldIdx < len(oldLines) && (lcsIdx >= len(lcs) || oldLines[oldIdx] != lcs[lcsIdx]) {
			// Line was removed
			result = append(result, DiffLine{
				Type:    DiffLineRemoved,
				Content: oldLines[oldIdx],
				OldLine: oldIdx + 1,
				NewLine: 0,
			})
			oldIdx++
		} else if newIdx < len(newLines) {
			// Line was added
			result = append(result, DiffLine{
				Type:    DiffLineAdded,
				Content: newLines[newIdx],
				OldLine: 0,
				NewLine: newIdx + 1,
			})
			newIdx++
		}
	}

	return result
}

// computeLCS computes the Longest Common Subsequence of two string slices.
// This is a simplified implementation for line-based diffing.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)

	// Create DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// Fill DP table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to find LCS
	var lcs []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append([]string{a[i-1]}, lcs...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// groupIntoHunks groups diff lines into hunks with context.
// Each hunk includes a few lines of context before and after changes.
func groupIntoHunks(diffLines []DiffLine, oldLines, newLines []string) []DiffHunk {
	if len(diffLines) == 0 {
		return nil
	}

	const contextLines = 3 // Number of context lines before/after changes

	var hunks []DiffHunk
	var currentHunk *DiffHunk

	for i, line := range diffLines {
		isChange := line.Type != DiffLineContext

		if currentHunk == nil && isChange {
			// Start a new hunk
			currentHunk = &DiffHunk{}

			// Add context before
			contextStart := max(0, i-contextLines)
			for j := contextStart; j < i; j++ {
				currentHunk.Lines = append(currentHunk.Lines, diffLines[j])
				// Count context lines
				if diffLines[j].OldLine > 0 {
					currentHunk.OldCount++
				}
				if diffLines[j].NewLine > 0 {
					currentHunk.NewCount++
				}
			}

			// Set start positions based on first line in hunk (including context)
			if len(currentHunk.Lines) > 0 {
				firstLine := currentHunk.Lines[0]
				if firstLine.OldLine > 0 {
					currentHunk.OldStart = firstLine.OldLine
				} else {
					currentHunk.OldStart = line.OldLine
				}
				if firstLine.NewLine > 0 {
					currentHunk.NewStart = firstLine.NewLine
				} else {
					currentHunk.NewStart = line.NewLine
				}
			} else {
				currentHunk.OldStart = line.OldLine
				currentHunk.NewStart = line.NewLine
			}
		}

		if currentHunk != nil {
			currentHunk.Lines = append(currentHunk.Lines, line)

			// Update counts
			if line.OldLine > 0 {
				currentHunk.OldCount++
			}
			if line.NewLine > 0 {
				currentHunk.NewCount++
			}

			// Check if we should close this hunk
			// Close if we've seen enough context lines after the last change
			contextAfter := 0
			for j := i + 1; j < len(diffLines) && j < i+1+contextLines; j++ {
				if diffLines[j].Type != DiffLineContext {
					contextAfter = -1 // More changes coming
					break
				}
				contextAfter++
			}

			if contextAfter >= 0 && (i == len(diffLines)-1 || contextAfter < contextLines) {
				// Add remaining context and close hunk
				for j := i + 1; j <= i+contextAfter && j < len(diffLines); j++ {
					currentHunk.Lines = append(currentHunk.Lines, diffLines[j])
					if diffLines[j].OldLine > 0 {
						currentHunk.OldCount++
					}
					if diffLines[j].NewLine > 0 {
						currentHunk.NewCount++
					}
				}
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}

	// Close any remaining hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// =============================================================================
// UNIFIED DIFF FORMAT
// =============================================================================

// FormatUnifiedDiff returns the diff in standard unified diff format.
func FormatUnifiedDiff(diff *Diff) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("--- a/%s\n", diff.FilePath))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", diff.FilePath))

	// Hunks
	for _, hunk := range diff.Hunks {
		// Hunk header: @@ -oldStart,oldCount +newStart,newCount @@
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			hunk.OldStart, hunk.OldCount,
			hunk.NewStart, hunk.NewCount))

		// Lines
		for _, line := range hunk.Lines {
			sb.WriteString(line.Type.Prefix())
			sb.WriteString(line.Content)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// =============================================================================
// SUMMARY
// =============================================================================

// Summary returns a human-readable summary of the diff.
func (d *Diff) Summary() string {
	var parts []string

	if d.Stats.FileMode == "new" {
		parts = append(parts, "New file")
	} else if d.Stats.FileMode == "deleted" {
		parts = append(parts, "File deleted")
	} else {
		parts = append(parts, "Modified")
	}

	if d.Stats.Additions > 0 {
		parts = append(parts, fmt.Sprintf("+%d", d.Stats.Additions))
	}
	if d.Stats.Deletions > 0 {
		parts = append(parts, fmt.Sprintf("-%d", d.Stats.Deletions))
	}

	return strings.Join(parts, " ")
}
