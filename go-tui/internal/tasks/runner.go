// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasks provides a background task system for long-running operations.
package tasks

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// TASK RUNNER
// =============================================================================

// Runner executes background tasks from a queue.
type Runner struct {
	queue         *Queue
	wg            sync.WaitGroup
	stop          chan struct{}
	stopped       atomic.Bool     // Flag to prevent new tasks after Stop() is called
	mu            sync.Mutex
	maxConcurrent int             // Maximum number of concurrent tasks
	semaphore     chan struct{}   // Semaphore to limit concurrency
	taskTimeout   time.Duration   // Timeout for each task (0 = no timeout)
}

// NewRunner creates a new task runner for the given queue.
// Uses default concurrency limit of 5 and default timeout of 30 minutes.
func NewRunner(queue *Queue) *Runner {
	return NewRunnerWithOptions(queue, 5, 30*time.Minute)
}

// NewRunnerWithOptions creates a new task runner with custom settings.
// maxConcurrent: maximum number of tasks to run concurrently (default: 5)
// taskTimeout: timeout for each task (0 = no timeout, default: 30 minutes)
func NewRunnerWithOptions(queue *Queue, maxConcurrent int, taskTimeout time.Duration) *Runner {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &Runner{
		queue:         queue,
		stop:          make(chan struct{}),
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		taskTimeout:   taskTimeout,
	}
}

// =============================================================================
// RUNNER LIFECYCLE
// =============================================================================

// Start begins processing tasks from the queue.
func (r *Runner) Start() {
	go r.processLoop()
}

// Stop gracefully stops the runner.
// Waits for currently running tasks to complete.
func (r *Runner) Stop() {
	r.stopped.Store(true) // Set flag to prevent new task spawns
	close(r.stop)
	r.wg.Wait()
}

// =============================================================================
// TASK PROCESSING
// =============================================================================

// processLoop continuously processes tasks from the queue.
func (r *Runner) processLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			// Don't start new tasks if stopped
			if r.stopped.Load() {
				return
			}

			// Check for queued tasks
			queued := r.queue.Queued()
			for _, task := range queued {
				// Check again in case Stop() was called during iteration
				if r.stopped.Load() {
					return
				}

				// Acquire semaphore (blocks if at max concurrency)
				select {
				case r.semaphore <- struct{}{}:
					// Successfully acquired semaphore, start task
					r.wg.Add(1)
					go r.executeTask(task)
				case <-r.stop:
					// Stop signal received while waiting for semaphore
					return
				}
			}
		}
	}
}

// executeTask executes a single task.
func (r *Runner) executeTask(task *Task) {
	defer r.wg.Done()
	defer func() { <-r.semaphore }() // Release semaphore when done

	// Mark as running
	r.queue.MarkRunning(task)

	// Create context with timeout or cancel
	var ctx context.Context
	var cancel context.CancelFunc
	if r.taskTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), r.taskTimeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	task.SetCancelFunc(cancel)
	defer cancel()

	// Execute based on command type
	var err error
	switch strings.ToLower(task.Command) {
	case "bash", "sh", "shell":
		err = r.executeBashTask(ctx, task)
	case "sleep":
		err = r.executeSleepTask(ctx, task)
	default:
		err = fmt.Errorf("unknown command type: %s", task.Command)
	}

	// Mark task as complete or failed
	if err != nil {
		if ctx.Err() == context.Canceled {
			r.queue.MarkCanceled(task)
		} else if ctx.Err() == context.DeadlineExceeded {
			r.queue.MarkFailed(task, fmt.Errorf("task timeout after %v: %w", r.taskTimeout, err))
		} else {
			r.queue.MarkFailed(task, err)
		}
	} else {
		r.queue.MarkComplete(task)
	}
}

// =============================================================================
// COMMAND EXECUTORS
// =============================================================================

// executeBashTask executes a bash/shell command.
// Note: Progress tracking is not supported for bash tasks - progress will remain at 0%.
// Progress is only available for sleep tasks which can estimate completion time.
// Note: stdout/stderr output interleaving is non-deterministic due to concurrent reads
// from separate pipes. Lines from stdout and stderr may appear in any order.
func (r *Runner) executeBashTask(ctx context.Context, task *Task) error {
	// Determine shell to use
	shell := "/bin/bash"
	shellFlag := "-c"

	// On Windows, use cmd or powershell
	// We'll check for common shells and fall back to cmd
	if _, err := exec.LookPath("bash"); err != nil {
		if _, err := exec.LookPath("powershell"); err == nil {
			shell = "powershell"
			shellFlag = "-Command"
		} else {
			shell = "cmd"
			shellFlag = "/c"
		}
	}

	// Build command
	args := append([]string{shellFlag}, strings.Join(task.Args, " "))
	cmd := exec.CommandContext(ctx, shell, args...)

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Read output concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		r.streamOutput(stdout, task, false)
	}()

	go func() {
		defer wg.Done()
		r.streamOutput(stderr, task, true)
	}()

	// Wait for output to complete
	wg.Wait()

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Check if context was canceled
		if ctx.Err() == context.Canceled {
			return ctx.Err()
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// streamOutput reads from a pipe and appends to task output.
func (r *Runner) streamOutput(pipe io.ReadCloser, task *Task, isError bool) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if isError {
			line = "[STDERR] " + line
		}
		task.AppendOutput(line + "\n")
	}
}

// executeSleepTask executes a sleep task (for testing).
func (r *Runner) executeSleepTask(ctx context.Context, task *Task) error {
	if len(task.Args) == 0 {
		return fmt.Errorf("sleep requires duration argument")
	}

	duration, err := time.ParseDuration(task.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	// Sleep with progress updates
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			elapsed := time.Since(start)
			if elapsed >= duration {
				task.SetProgress(100)
				return nil
			}
			progress := int((elapsed.Seconds() / duration.Seconds()) * 100)
			task.SetProgress(progress)
		}
	}
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// Execute is a convenience function to execute a task immediately without queuing.
// This is useful for one-off tasks that don't need background processing.
func Execute(task *Task) error {
	runner := NewRunner(NewQueue(0))

	// Mark as running
	task.MarkStarted()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	task.SetCancelFunc(cancel)
	defer cancel()

	// Execute
	var err error
	switch strings.ToLower(task.Command) {
	case "bash", "sh", "shell":
		err = runner.executeBashTask(ctx, task)
	case "sleep":
		err = runner.executeSleepTask(ctx, task)
	default:
		err = fmt.Errorf("unknown command type: %s", task.Command)
	}

	// Mark complete or failed
	if err != nil {
		if ctx.Err() == context.Canceled {
			task.MarkCanceled()
		} else {
			task.SetError(err)
		}
	} else {
		task.MarkComplete()
	}

	return err
}
