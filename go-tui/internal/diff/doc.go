// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package diff provides diff computation and formatting for file changes.
//
// This package computes unified diffs between old and new content,
// with support for syntax highlighting and various output formats.
//
// # Key Types
//
//   - DiffLineType: Type of diff line (context, added, removed)
//   - DiffLine: Single line in a diff with type and content
//   - DiffHunk: Group of related diff lines with line numbers
//   - Diff: Complete diff result with hunks and metadata
//
// # Usage
//
// Compute a diff between two strings:
//
//	result := diff.ComputeDiff(oldContent, newContent)
//	fmt.Println(result.Format())
//
// Format with syntax highlighting:
//
//	formatted := diff.FormatWithSyntax(result, "go")
package diff
