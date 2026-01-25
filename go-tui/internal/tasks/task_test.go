// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package tasks

import (
	"errors"
	"testing"
)

func TestNewTask(t *testing.T) {
	task := NewTask("Test task", "bash", []string{"echo", "hello"})

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}

	if task.Description != "Test task" {
		t.Errorf("Expected description 'Test task', got '%s'", task.Description)
	}

	if task.Command != "bash" {
		t.Errorf("Expected command 'bash', got '%s'", task.Command)
	}

	if task.GetStatus() != TaskStatusQueued {
		t.Errorf("Expected status Queued, got %s", task.GetStatus())
	}
}

func TestTaskStatus(t *testing.T) {
	task := NewTask("Test", "bash", []string{})

	task.MarkStarted()
	if task.GetStatus() != TaskStatusRunning {
		t.Error("Task should be running after MarkStarted()")
	}

	task.MarkComplete()
	if task.GetStatus() != TaskStatusComplete {
		t.Error("Task should be complete after MarkComplete()")
	}

	// Duration might be very small but should not be negative
	if task.Duration() < 0 {
		t.Error("Task duration should not be negative")
	}
}

func TestTaskProgress(t *testing.T) {
	task := NewTask("Test", "bash", []string{})

	task.SetProgress(50)
	if task.GetProgress() != 50 {
		t.Errorf("Expected progress 50, got %d", task.GetProgress())
	}

	task.SetProgress(150) // Should cap at 100
	if task.GetProgress() != 100 {
		t.Errorf("Expected progress capped at 100, got %d", task.GetProgress())
	}

	task.SetProgress(-10) // Should floor at 0
	if task.GetProgress() != 0 {
		t.Errorf("Expected progress floored at 0, got %d", task.GetProgress())
	}
}

func TestTaskOutput(t *testing.T) {
	task := NewTask("Test", "bash", []string{})

	task.AppendOutput("Line 1\n")
	task.AppendOutput("Line 2\n")

	output := task.GetOutput()
	if output != "Line 1\nLine 2\n" {
		t.Errorf("Expected 'Line 1\\nLine 2\\n', got '%s'", output)
	}
}

func TestQueueOperations(t *testing.T) {
	queue := NewQueue(10)

	task1 := NewTask("Task 1", "bash", []string{})
	task2 := NewTask("Task 2", "bash", []string{})

	queue.Add(task1)
	queue.Add(task2)

	if queue.Count() != 2 {
		t.Errorf("Expected 2 tasks, got %d", queue.Count())
	}

	retrieved := queue.Get(task1.ID)
	if retrieved == nil {
		t.Error("Should retrieve task by ID")
	}

	if retrieved.Description != "Task 1" {
		t.Errorf("Expected 'Task 1', got '%s'", retrieved.Description)
	}
}

func TestQueueFiltering(t *testing.T) {
	queue := NewQueue(10)

	task1 := NewTask("Running", "bash", []string{})
	task2 := NewTask("Complete", "bash", []string{})
	task3 := NewTask("Failed", "bash", []string{})

	queue.Add(task1)
	queue.Add(task2)
	queue.Add(task3)

	queue.MarkRunning(task1)
	queue.MarkComplete(task2)
	queue.MarkFailed(task3, errors.New("test error"))

	running := queue.Running()
	if len(running) != 1 {
		t.Errorf("Expected 1 running task, got %d", len(running))
	}

	completed := queue.Completed()
	if len(completed) != 2 { // Complete + Failed
		t.Errorf("Expected 2 completed tasks, got %d", len(completed))
	}
}

func TestTaskCancel(t *testing.T) {
	task := NewTask("Test", "bash", []string{})

	// Mark as running
	task.MarkStarted()

	// Cancel should succeed
	if !task.Cancel() {
		t.Error("Cancel should succeed for running task")
	}

	if task.GetStatus() != TaskStatusCanceled {
		t.Error("Task should be canceled")
	}

	// Second cancel should fail
	if task.Cancel() {
		t.Error("Second cancel should fail")
	}
}
