// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"context"

	"github.com/jeranaias/rigrun-tui/internal/diff"
)

// =============================================================================
// DIFF SUPPORT
// =============================================================================

// DiffResult extends Result with diff information.
type DiffResult struct {
	Result
	Diff *diff.Diff // The computed diff (if applicable)
}

// ComputeEditDiff computes a diff for Edit tool operations.
// This is called before executing the edit to show the user what will change.
func ComputeEditDiff(filePath, oldContent, newContent string) *diff.Diff {
	return diff.ComputeDiff(filePath, oldContent, newContent)
}

// ComputeWriteDiff computes a diff for Write tool operations.
// For new files, oldContent will be empty.
func ComputeWriteDiff(filePath, oldContent, newContent string) *diff.Diff {
	return diff.ComputeDiff(filePath, oldContent, newContent)
}

// DiffCallback is a callback function that receives a diff before execution.
// Return true to proceed with execution, false to cancel.
type DiffCallback func(d *diff.Diff) bool

// DiffAwareExecutor wraps a ToolExecutor to add diff preview support.
type DiffAwareExecutor struct {
	Executor     ToolExecutor
	DiffCallback DiffCallback
	ToolName     string // "Edit" or "Write"
}

// Execute wraps the underlying executor with diff support.
func (d *DiffAwareExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// Only compute diff for Edit and Write tools
	if d.ToolName != "Edit" && d.ToolName != "Write" {
		return d.Executor.Execute(ctx, params)
	}

	// Extract parameters
	filePath, _ := params["file_path"].(string)

	var oldContent, newContent string

	if d.ToolName == "Edit" {
		// For Edit, the diff callback will handle reading the file
		// and computing the actual diff based on the replacement
		// This is a simplified version - the actual implementation
		// would need to read the file and apply the edit logic

		// Skip diff computation for now - this will be handled
		// by the Edit executor itself
		return d.Executor.Execute(ctx, params)
	}

	if d.ToolName == "Write" {
		newContent, _ = params["content"].(string)
		// For Write, old content is empty for new files,
		// or the existing file content for overwrites
		// This requires reading the file if it exists
	}

	// Compute diff
	if filePath != "" {
		diffObj := diff.ComputeDiff(filePath, oldContent, newContent)

		// Call the diff callback if provided
		if d.DiffCallback != nil && !d.DiffCallback(diffObj) {
			return Result{
				Success: false,
				Output:  "Operation cancelled by user",
			}, nil
		}
	}

	// Execute the underlying tool
	return d.Executor.Execute(ctx, params)
}

// =============================================================================
// DIFF PREVIEW MODE
// =============================================================================

// PreviewEdit returns a diff preview for an Edit operation without executing it.
func PreviewEdit(filePath, oldContent, oldString, newString string, useRegex, replaceAll bool) (*diff.Diff, error) {
	// This would implement the same logic as EditExecutor but only compute the diff
	// For now, we'll use the basic diff computation
	// In a full implementation, this would apply the edit logic first

	// TODO: Implement proper edit preview that applies the replacement logic
	// before computing the diff

	return diff.ComputeDiff(filePath, oldContent, oldContent), nil
}

// PreviewWrite returns a diff preview for a Write operation without executing it.
func PreviewWrite(filePath, oldContent, newContent string) *diff.Diff {
	return diff.ComputeDiff(filePath, oldContent, newContent)
}
