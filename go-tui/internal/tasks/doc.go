// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasks provides a background task system for long-running operations.
//
// This package implements a task queue and runner for executing background
// operations like indexing, exports, and other long-running processes.
//
// # Key Types
//
//   - Task: Represents a background task with status and progress
//   - Queue: Task queue with concurrency control
//   - Runner: Executes tasks with timeout and cancellation support
//   - Status: Task status enumeration (Pending, Running, Completed, Failed)
//
// # Usage
//
// Create and queue a task:
//
//	queue := tasks.NewQueue(4) // 4 concurrent workers
//	task := tasks.NewTask("Build index", "index", []string{"--dir", "."})
//	queue.Add(task)
//
// Monitor task progress:
//
//	for update := range task.Updates() {
//	    fmt.Printf("Progress: %d%%\n", update.Progress)
//	}
//
// Wait for completion:
//
//	result := <-task.Done()
//	if result.Err != nil {
//	    log.Printf("Task failed: %v", result.Err)
//	}
package tasks
