// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasks provides a background task system for long-running operations.
package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// TASK STATUS
// =============================================================================

// TaskStatus represents the current state of a background task.
type TaskStatus string

const (
	// TaskStatusQueued indicates the task is waiting to be executed
	TaskStatusQueued TaskStatus = "Queued"

	// TaskStatusRunning indicates the task is currently executing
	TaskStatusRunning TaskStatus = "Running"

	// TaskStatusComplete indicates the task finished successfully
	TaskStatusComplete TaskStatus = "Complete"

	// TaskStatusFailed indicates the task encountered an error
	TaskStatusFailed TaskStatus = "Failed"

	// TaskStatusCanceled indicates the task was canceled by the user
	TaskStatusCanceled TaskStatus = "Canceled"
)

// String returns the string representation of the task status.
func (s TaskStatus) String() string {
	return string(s)
}

// =============================================================================
// TASK STRUCTURE
// =============================================================================

// Task represents a background task that can run without blocking the UI.
type Task struct {
	// ID is a unique identifier for this task
	ID string

	// Description is a human-readable description of what this task does
	Description string

	// Command is the command being executed (e.g., "bash")
	Command string

	// Args are the arguments to the command
	Args []string

	// Status is the current state of the task
	Status TaskStatus

	// StartTime is when the task started running
	StartTime time.Time

	// EndTime is when the task completed or failed
	EndTime time.Time

	// Output is the standard output from the task
	Output string

	// Error is the error message if the task failed
	Error string

	// Progress is an optional progress percentage (0-100)
	Progress int

	// ConversationID is the conversation this task was started from
	ConversationID string

	// Metadata stores additional task-specific data
	Metadata map[string]interface{}

	// cancel is the context cancel function for this task
	cancel context.CancelFunc

	// mu protects concurrent access to the task
	mu sync.RWMutex
}

// =============================================================================
// TASK CREATION
// =============================================================================

// NewTask creates a new task with the given description and command.
func NewTask(description, command string, args []string) *Task {
	return &Task{
		ID:          uuid.New().String(),
		Description: description,
		Command:     command,
		Args:        args,
		Status:      TaskStatusQueued,
		Metadata:    make(map[string]interface{}),
	}
}

// =============================================================================
// TASK METHODS
// =============================================================================

// SetStatus updates the task status (thread-safe).
// Validates state transitions to prevent invalid status changes.
// Valid transitions: Queued -> Running -> Complete/Failed/Canceled
func (t *Task) SetStatus(status TaskStatus) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate transition
	if !t.isValidTransition(t.Status, status) {
		return fmt.Errorf("invalid status transition from %s to %s", t.Status, status)
	}

	t.Status = status
	return nil
}

// isValidTransition checks if a status transition is valid (must be called with lock held).
func (t *Task) isValidTransition(from, to TaskStatus) bool {
	// Allow setting the same status (idempotent)
	if from == to {
		return true
	}

	// Valid transitions
	switch from {
	case TaskStatusQueued:
		// Queued can transition to Running or Canceled
		return to == TaskStatusRunning || to == TaskStatusCanceled
	case TaskStatusRunning:
		// Running can transition to Complete, Failed, or Canceled
		return to == TaskStatusComplete || to == TaskStatusFailed || to == TaskStatusCanceled
	case TaskStatusComplete, TaskStatusFailed, TaskStatusCanceled:
		// Terminal states - no transitions allowed
		return false
	default:
		return false
	}
}

// GetStatus returns the current task status (thread-safe).
func (t *Task) GetStatus() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// SetProgress updates the task progress (thread-safe).
func (t *Task) SetProgress(progress int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	t.Progress = progress
}

// GetProgress returns the current progress (thread-safe).
func (t *Task) GetProgress() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Progress
}

// AppendOutput appends text to the task output (thread-safe).
func (t *Task) AppendOutput(output string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Output += output
}

// GetOutput returns the current output (thread-safe).
func (t *Task) GetOutput() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Output
}

// SetError sets the error message and marks the task as failed (thread-safe).
// This bypasses status transition validation for internal use.
func (t *Task) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err != nil {
		t.Error = err.Error()
		t.Status = TaskStatusFailed
		t.EndTime = time.Now()
	}
}

// GetError returns the error message (thread-safe).
func (t *Task) GetError() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Error
}

// MarkStarted marks the task as running (thread-safe).
// This bypasses status transition validation for internal use.
func (t *Task) MarkStarted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusRunning
	t.StartTime = time.Now()
}

// MarkComplete marks the task as successfully completed (thread-safe).
// This bypasses status transition validation for internal use.
func (t *Task) MarkComplete() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusComplete
	t.EndTime = time.Now()
	t.Progress = 100
}

// MarkCanceled marks the task as canceled (thread-safe).
// This bypasses status transition validation for internal use.
func (t *Task) MarkCanceled() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusCanceled
	t.EndTime = time.Now()
}

// SetCancelFunc stores the context cancel function for this task.
// WARNING: This function must only be called once during task initialization.
// Multiple calls will overwrite the cancel function, potentially causing race
// conditions where the wrong context is canceled. The current implementation
// uses a simple field assignment which is not atomic. Consider using atomic.Value
// if you need to support dynamic cancel function updates.
func (t *Task) SetCancelFunc(cancel context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cancel = cancel
}

// Cancel cancels the task if it's running.
// Returns true if the task was canceled, false if it couldn't be canceled.
func (t *Task) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != TaskStatusRunning && t.Status != TaskStatusQueued {
		return false
	}

	if t.cancel != nil {
		t.cancel()
	}

	t.Status = TaskStatusCanceled
	t.EndTime = time.Now()
	return true
}

// Duration returns how long the task has been running or took to complete.
func (t *Task) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.StartTime.IsZero() {
		return 0
	}

	if t.EndTime.IsZero() {
		return time.Since(t.StartTime)
	}

	return t.EndTime.Sub(t.StartTime)
}

// IsRunning returns true if the task is currently running.
func (t *Task) IsRunning() bool {
	return t.GetStatus() == TaskStatusRunning
}

// IsComplete returns true if the task has finished (success, failure, or canceled).
func (t *Task) IsComplete() bool {
	status := t.GetStatus()
	return status == TaskStatusComplete || status == TaskStatusFailed || status == TaskStatusCanceled
}

// Summary returns a one-line summary of the task.
func (t *Task) Summary() string {
	status := t.GetStatus()
	duration := t.Duration()

	summary := fmt.Sprintf("[%s] %s - %s",
		t.ID[:8],
		t.Description,
		status,
	)

	if duration > 0 {
		summary += fmt.Sprintf(" (%.1fs)", duration.Seconds())
	}

	return summary
}

// Clone creates a thread-safe copy of the task for reading.
// WARNING: This performs a shallow copy of the Metadata map. Map values themselves
// are not deep-copied, only the map structure is copied. If Metadata contains
// pointers, slices, or other reference types, modifications to those values will
// affect both the original and the clone. For full isolation, implement deep copying
// for complex Metadata values or use immutable types.
func (t *Task) Clone() *Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	metadata := make(map[string]interface{})
	for k, v := range t.Metadata {
		metadata[k] = v
	}

	return &Task{
		ID:             t.ID,
		Description:    t.Description,
		Command:        t.Command,
		Args:           append([]string{}, t.Args...),
		Status:         t.Status,
		StartTime:      t.StartTime,
		EndTime:        t.EndTime,
		Output:         t.Output,
		Error:          t.Error,
		Progress:       t.Progress,
		ConversationID: t.ConversationID,
		Metadata:       metadata,
	}
}
