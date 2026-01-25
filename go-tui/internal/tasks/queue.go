// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasks provides a background task system for long-running operations.
package tasks

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// =============================================================================
// TASK QUEUE
// =============================================================================

// Queue manages a queue of background tasks with thread-safe operations.
type Queue struct {
	// tasks is the list of all tasks (both queued and completed)
	tasks []*Task

	// running tracks currently running tasks by ID
	running map[string]*Task

	// maxHistory is the maximum number of completed tasks to keep
	maxHistory int

	// maxQueueSize is the maximum number of queued tasks allowed (0 = unlimited)
	maxQueueSize int

	// mu protects concurrent access to the queue
	mu sync.RWMutex

	// notifyChan sends notifications when tasks complete
	notifyChan chan TaskNotification
}

// TaskNotification represents a notification about a task state change.
type TaskNotification struct {
	TaskID      string
	Description string
	Status      TaskStatus
	Error       string
	Duration    time.Duration
}

// =============================================================================
// QUEUE CREATION
// =============================================================================

// NewQueue creates a new task queue.
// maxHistory sets the maximum number of completed tasks to keep (0 = unlimited).
func NewQueue(maxHistory int) *Queue {
	return NewQueueWithOptions(maxHistory, 0)
}

// NewQueueWithOptions creates a new task queue with custom settings.
// maxHistory: maximum number of completed tasks to keep (0 = unlimited)
// maxQueueSize: maximum number of queued tasks allowed (0 = unlimited)
func NewQueueWithOptions(maxHistory, maxQueueSize int) *Queue {
	return &Queue{
		tasks:        make([]*Task, 0),
		running:      make(map[string]*Task),
		maxHistory:   maxHistory,
		maxQueueSize: maxQueueSize,
		notifyChan:   make(chan TaskNotification, 100),
	}
}

// =============================================================================
// TASK MANAGEMENT
// =============================================================================

// Add adds a new task to the queue.
// Returns an error if the queue has reached its maximum size.
func (q *Queue) Add(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue size limit if configured
	if q.maxQueueSize > 0 {
		queuedCount := 0
		for _, t := range q.tasks {
			if t.GetStatus() == TaskStatusQueued {
				queuedCount++
			}
		}
		if queuedCount >= q.maxQueueSize {
			return fmt.Errorf("queue is full: %d queued tasks (max: %d)", queuedCount, q.maxQueueSize)
		}
	}

	// Set initial status (ignore error since we're setting to initial state)
	_ = task.SetStatus(TaskStatusQueued)
	q.tasks = append(q.tasks, task)
	return nil
}

// Get retrieves a task by ID.
// Returns nil if the task is not found.
func (q *Queue) Get(id string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, task := range q.tasks {
		if task.ID == id {
			return task.Clone()
		}
	}
	return nil
}

// GetRunning retrieves a running task by ID.
// Returns nil if the task is not found or not running.
func (q *Queue) GetRunning(id string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if task, ok := q.running[id]; ok {
		return task.Clone()
	}
	return nil
}

// Cancel cancels a task by ID.
// Returns true if the task was successfully canceled.
// Uses write lock to prevent race conditions with tasks transitioning states.
func (q *Queue) Cancel(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if it's a running task
	if task, ok := q.running[id]; ok {
		return task.Cancel()
	}

	// Check if it's a queued task
	for _, task := range q.tasks {
		if task.ID == id {
			if task.GetStatus() == TaskStatusQueued {
				task.MarkCanceled()
				return true
			}
		}
	}

	return false
}

// MarkRunning marks a task as running.
func (q *Queue) MarkRunning(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.MarkStarted()
	q.running[task.ID] = task
}

// MarkComplete marks a task as complete and removes it from running.
func (q *Queue) MarkComplete(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.MarkComplete()
	delete(q.running, task.ID)

	// Send notification
	q.notify(TaskNotification{
		TaskID:      task.ID,
		Description: task.Description,
		Status:      TaskStatusComplete,
		Duration:    task.Duration(),
	})

	// Cleanup old tasks
	q.cleanupLocked()
}

// MarkFailed marks a task as failed and removes it from running.
func (q *Queue) MarkFailed(task *Task, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.SetError(err)
	delete(q.running, task.ID)

	// Send notification
	q.notify(TaskNotification{
		TaskID:      task.ID,
		Description: task.Description,
		Status:      TaskStatusFailed,
		Error:       err.Error(),
		Duration:    task.Duration(),
	})

	// Cleanup old tasks
	q.cleanupLocked()
}

// MarkCanceled marks a task as canceled and removes it from running.
func (q *Queue) MarkCanceled(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.MarkCanceled()
	delete(q.running, task.ID)

	// Send notification
	q.notify(TaskNotification{
		TaskID:      task.ID,
		Description: task.Description,
		Status:      TaskStatusCanceled,
		Duration:    task.Duration(),
	})

	// Cleanup old tasks
	q.cleanupLocked()
}

// =============================================================================
// QUEUE QUERIES
// =============================================================================

// All returns a copy of all tasks.
func (q *Queue) All() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, len(q.tasks))
	for i, task := range q.tasks {
		result[i] = task.Clone()
	}
	return result
}

// Running returns a copy of all running tasks.
func (q *Queue) Running() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, 0, len(q.running))
	for _, task := range q.running {
		result = append(result, task.Clone())
	}
	return result
}

// Queued returns all queued (not yet started) tasks.
// IMPORTANT: Returns original task pointers (not clones) that are atomically
// marked as running to prevent race conditions where the runner would execute
// clones while originals remain in the queued state.
func (q *Queue) Queued() []*Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	result := make([]*Task, 0)
	for _, task := range q.tasks {
		if task.GetStatus() == TaskStatusQueued {
			// Return original task pointer
			result = append(result, task)
		}
	}
	return result
}

// Completed returns all completed tasks (success, failure, or canceled).
func (q *Queue) Completed() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range q.tasks {
		if task.IsComplete() {
			result = append(result, task.Clone())
		}
	}
	return result
}

// Count returns the total number of tasks.
func (q *Queue) Count() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks)
}

// RunningCount returns the number of running tasks.
func (q *Queue) RunningCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.running)
}

// =============================================================================
// NOTIFICATIONS
// =============================================================================

// Notifications returns the notification channel.
// Consumers can read from this channel to receive task completion notifications.
func (q *Queue) Notifications() <-chan TaskNotification {
	return q.notifyChan
}

// notify sends a notification (must be called with lock held).
func (q *Queue) notify(notification TaskNotification) {
	select {
	case q.notifyChan <- notification:
		// Notification sent successfully
	default:
		// Channel full, drop notification and log warning
		log.Printf("WARNING: Notification channel full, dropped notification for task %s (status: %s)",
			notification.TaskID, notification.Status)
	}
}

// =============================================================================
// CLEANUP
// =============================================================================

// cleanupLocked removes old completed tasks to keep history size manageable.
// Must be called with lock held.
// Note: Uses FIFO removal order based on task slice position, NOT time-based removal.
// Completed tasks are removed in the order they appear in the tasks slice, which may
// not correspond to completion time if tasks finish out of order. The first N completed
// tasks in the slice will be removed, where N = (completedCount - maxHistory).
func (q *Queue) cleanupLocked() {
	if q.maxHistory <= 0 {
		return
	}

	// Count completed tasks
	completedCount := 0
	for _, task := range q.tasks {
		if task.IsComplete() {
			completedCount++
		}
	}

	// If we have too many completed tasks, remove the oldest
	if completedCount > q.maxHistory {
		toRemove := completedCount - q.maxHistory
		newTasks := make([]*Task, 0, len(q.tasks)-toRemove)

		for _, task := range q.tasks {
			if task.IsComplete() && toRemove > 0 {
				toRemove--
				continue
			}
			newTasks = append(newTasks, task)
		}

		q.tasks = newTasks
	}
}

// Clear removes all completed tasks from the history.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Keep only running and queued tasks
	newTasks := make([]*Task, 0)
	for _, task := range q.tasks {
		if !task.IsComplete() {
			newTasks = append(newTasks, task)
		}
	}
	q.tasks = newTasks
}

// =============================================================================
// FORMATTING
// =============================================================================

// Summary returns a formatted summary of the queue.
func (q *Queue) Summary() string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	running := len(q.running)
	queued := 0
	completed := 0
	failed := 0

	for _, task := range q.tasks {
		status := task.GetStatus()
		switch status {
		case TaskStatusQueued:
			queued++
		case TaskStatusComplete:
			completed++
		case TaskStatusFailed:
			failed++
		}
	}

	return fmt.Sprintf("Running: %d | Queued: %d | Completed: %d | Failed: %d",
		running, queued, completed, failed)
}
