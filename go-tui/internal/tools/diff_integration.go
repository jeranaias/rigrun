// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"context"

	"github.com/jeranaias/rigrun-tui/internal/diff"
)

// =============================================================================
// DIFF INTEGRATION FOR TOOL EXECUTION
// =============================================================================

// DiffPreview represents a diff preview before tool execution.
type DiffPreview struct {
	ToolName   string      // "Edit" or "Write"
	FilePath   string      // File being modified
	OldContent string      // Original content
	NewContent string      // New content
	Diff       *diff.Diff  // Computed diff
}

// GetToolDiffPreview returns a diff preview for Edit or Write tools.
// This should be called before executing the tool to show the user what will change.
func GetToolDiffPreview(toolName string, params map[string]interface{}) (*DiffPreview, error) {
	filePath, _ := params["file_path"].(string)

	var oldContent, newContent string
	var err error

	switch toolName {
	case "Edit":
		executor := &EditExecutor{}
		oldContent, newContent, err = executor.GetDiffPreview(params)
		if err != nil {
			return nil, err
		}

	case "Write":
		executor := &WriteExecutor{}
		oldContent, newContent, err = executor.GetDiffPreview(params)
		if err != nil {
			return nil, err
		}

	default:
		// Not a tool that supports diff preview
		return nil, nil
	}

	// Compute the diff
	d := diff.ComputeDiff(filePath, oldContent, newContent)

	return &DiffPreview{
		ToolName:   toolName,
		FilePath:   filePath,
		OldContent: oldContent,
		NewContent: newContent,
		Diff:       d,
	}, nil
}

// ShouldShowDiff determines if a diff should be shown for this tool call.
// Returns true for Edit and Write tools that will modify files.
func ShouldShowDiff(toolName string, params map[string]interface{}) bool {
	switch toolName {
	case "Edit":
		// Show diff unless it's a restore_backup operation
		restoreBackup, _ := params["restore_backup"].(bool)
		return !restoreBackup

	case "Write":
		// Always show diff for writes
		return true

	default:
		return false
	}
}

// =============================================================================
// DIFF-AWARE TOOL EXECUTOR WRAPPER
// =============================================================================

// DiffApprovalFunc is called when a diff is ready for approval.
// It should return true to proceed, false to cancel.
// This is typically implemented by the UI layer.
type DiffApprovalFunc func(preview *DiffPreview) bool

// ExecutorWithDiffApproval wraps a tool executor to require diff approval.
type ExecutorWithDiffApproval struct {
	ToolName     string
	Executor     ToolExecutor
	ApprovalFunc DiffApprovalFunc
}

// Execute wraps the tool execution with diff approval.
func (e *ExecutorWithDiffApproval) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// Check if this tool supports diff preview
	if !ShouldShowDiff(e.ToolName, params) {
		// No diff needed, execute directly
		return e.Executor.Execute(ctx, params)
	}

	// Get diff preview
	preview, err := GetToolDiffPreview(e.ToolName, params)
	if err != nil {
		return Result{
			Success: false,
			Error:   "Failed to generate diff preview: " + err.Error(),
		}, nil
	}

	if preview == nil {
		// No preview available, execute directly
		return e.Executor.Execute(ctx, params)
	}

	// Request approval if approval func is set
	if e.ApprovalFunc != nil {
		approved := e.ApprovalFunc(preview)
		if !approved {
			return Result{
				Success: false,
				Output:  "Operation cancelled by user",
			}, nil
		}
	}

	// User approved or no approval needed, execute the tool
	return e.Executor.Execute(ctx, params)
}

// =============================================================================
// DIFF METADATA FOR RESULTS
// =============================================================================

// ResultWithDiff extends Result to include diff information.
// This is used to pass diff data back to the UI after execution.
type ResultWithDiff struct {
	Result
	DiffPreview *DiffPreview
}

// ExecuteWithDiffMetadata executes a tool and includes diff metadata in the result.
// This is useful for saving diffs to session history.
func ExecuteWithDiffMetadata(ctx context.Context, toolName string, params map[string]interface{}, executor ToolExecutor) (*ResultWithDiff, error) {
	// Get diff preview if applicable
	var preview *DiffPreview
	if ShouldShowDiff(toolName, params) {
		var err error
		preview, err = GetToolDiffPreview(toolName, params)
		if err != nil {
			// Don't fail the execution, just log that preview failed
			preview = nil
		}
	}

	// Execute the tool
	result, err := executor.Execute(ctx, params)

	return &ResultWithDiff{
		Result:      result,
		DiffPreview: preview,
	}, err
}

// =============================================================================
// SESSION DIFF STORAGE
// =============================================================================

// DiffHistoryEntry represents a diff stored in session history.
type DiffHistoryEntry struct {
	Timestamp  string       // When the diff was created
	ToolName   string       // "Edit" or "Write"
	FilePath   string       // File that was modified
	Diff       *diff.Diff   // The diff
	Applied    bool         // Whether the diff was applied
	MessageID  string       // Associated message ID
}

// DiffHistory stores diffs for a session.
type DiffHistory struct {
	Entries []DiffHistoryEntry
}

// NewDiffHistory creates a new diff history.
func NewDiffHistory() *DiffHistory {
	return &DiffHistory{
		Entries: make([]DiffHistoryEntry, 0),
	}
}

// Add adds a diff to the history.
func (h *DiffHistory) Add(entry DiffHistoryEntry) {
	h.Entries = append(h.Entries, entry)
}

// GetByMessageID retrieves all diffs for a message.
func (h *DiffHistory) GetByMessageID(messageID string) []DiffHistoryEntry {
	var results []DiffHistoryEntry
	for _, entry := range h.Entries {
		if entry.MessageID == messageID {
			results = append(results, entry)
		}
	}
	return results
}

// GetByFile retrieves all diffs for a file.
func (h *DiffHistory) GetByFile(filePath string) []DiffHistoryEntry {
	var results []DiffHistoryEntry
	for _, entry := range h.Entries {
		if entry.FilePath == filePath {
			results = append(results, entry)
		}
	}
	return results
}

// Count returns the number of diffs in history.
func (h *DiffHistory) Count() int {
	return len(h.Entries)
}
