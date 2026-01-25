// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
package plan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// PROGRESS CALLBACK
// =============================================================================

// ProgressCallback is called when plan execution progress is made.
type ProgressCallback func(step int, total int, status string)

// =============================================================================
// PLAN EXECUTOR
// =============================================================================

// PlanExecutor executes a plan step by step.
type PlanExecutor struct {
	// plan is the plan being executed
	plan *Plan

	// executor is the tool executor
	executor *tools.Executor

	// mu protects concurrent access to plan state and onProgress
	mu sync.RWMutex

	// onProgress is called when progress is made
	onProgress ProgressCallback

	// ctx is the execution context
	ctx context.Context

	// cancel can be called to cancel execution
	cancel context.CancelFunc

	// continueOnError determines whether to continue on step failure
	continueOnError bool
}

// NewPlanExecutor creates a new plan executor.
func NewPlanExecutor(plan *Plan, executor *tools.Executor) *PlanExecutor {
	return &PlanExecutor{
		plan:            plan,
		executor:        executor,
		continueOnError: false,
	}
}

// SetProgressCallback sets the progress callback function.
func (e *PlanExecutor) SetProgressCallback(cb ProgressCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onProgress = cb
}

// SetContinueOnError sets whether to continue execution on step failure.
func (e *PlanExecutor) SetContinueOnError(continueOnError bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.continueOnError = continueOnError
}

// Execute runs the plan from the current step to completion.
func (e *PlanExecutor) Execute(ctx context.Context) error {
	e.mu.Lock()
	if !e.plan.CanExecute() {
		e.mu.Unlock()
		return fmt.Errorf("plan cannot be executed in status: %s", e.plan.Status)
	}

	// Create new context for this execution to avoid reuse issues
	execCtx, cancel := context.WithCancel(ctx)
	e.ctx = execCtx
	e.cancel = cancel
	continueOnError := e.continueOnError
	e.mu.Unlock()

	// Mark plan as running
	e.mu.Lock()
	e.plan.Start()
	e.mu.Unlock()

	// Execute steps sequentially
	for {
		e.mu.Lock()
		if e.plan.CurrentStep >= len(e.plan.Steps) {
			e.mu.Unlock()
			break
		}
		currentStepIdx := e.plan.CurrentStep
		e.mu.Unlock()

		// Check for cancellation
		select {
		case <-execCtx.Done():
			e.mu.Lock()
			e.plan.Pause()
			e.mu.Unlock()
			return fmt.Errorf("execution cancelled")
		default:
		}

		// Execute current step
		e.mu.Lock()
		step := &e.plan.Steps[currentStepIdx]
		e.mu.Unlock()

		if err := e.executeStep(execCtx, step); err != nil {
			if !continueOnError {
				e.mu.Lock()
				e.plan.Fail(err)
				e.mu.Unlock()
				return fmt.Errorf("step %d failed: %w", currentStepIdx+1, err)
			}
			// Mark step as failed but continue
			e.mu.Lock()
			step.Status = StepFailed
			step.Error = err
			step.EndTime = time.Now()
			e.mu.Unlock()
		}

		// Notify progress
		e.notifyProgress()

		// Move to next step
		e.mu.Lock()
		e.plan.CurrentStep++
		e.mu.Unlock()
	}

	// Mark plan as complete
	e.mu.Lock()
	e.plan.Complete()
	e.mu.Unlock()
	e.notifyProgress()

	return nil
}

// ExecuteNext executes the next step in the plan.
// Returns true if there are more steps to execute.
func (e *PlanExecutor) ExecuteNext(ctx context.Context) (bool, error) {
	e.mu.Lock()
	if !e.plan.CanExecute() {
		e.mu.Unlock()
		return false, fmt.Errorf("plan cannot be executed in status: %s", e.plan.Status)
	}

	// Check if there are more steps
	if e.plan.CurrentStep >= len(e.plan.Steps) {
		e.plan.Complete()
		e.mu.Unlock()
		e.notifyProgress()
		return false, nil
	}

	// Mark plan as running if not already
	if e.plan.Status != StatusRunning {
		e.plan.Start()
	}

	// Create new context for this execution if not set
	if e.ctx == nil || e.ctx.Err() != nil {
		execCtx, cancel := context.WithCancel(ctx)
		e.ctx = execCtx
		e.cancel = cancel
	}

	currentStepIdx := e.plan.CurrentStep
	step := &e.plan.Steps[currentStepIdx]
	continueOnError := e.continueOnError
	execCtx := e.ctx
	e.mu.Unlock()

	// Execute current step
	if err := e.executeStep(execCtx, step); err != nil {
		if !continueOnError {
			e.mu.Lock()
			e.plan.Fail(err)
			e.mu.Unlock()
			return false, fmt.Errorf("step %d failed: %w", currentStepIdx+1, err)
		}
		// Mark step as failed but continue
		e.mu.Lock()
		step.Status = StepFailed
		step.Error = err
		step.EndTime = time.Now()
		e.mu.Unlock()
	}

	// Notify progress
	e.notifyProgress()

	// Move to next step
	e.mu.Lock()
	e.plan.CurrentStep++

	// Check if there are more steps
	hasMore := e.plan.CurrentStep < len(e.plan.Steps)
	if !hasMore {
		e.plan.Complete()
	}
	e.mu.Unlock()

	if !hasMore {
		e.notifyProgress()
	}

	return hasMore, nil
}

// Pause pauses plan execution.
func (e *PlanExecutor) Pause() {
	e.mu.Lock()
	e.plan.Pause()
	e.mu.Unlock()
	e.notifyProgress()
}

// Resume resumes plan execution.
func (e *PlanExecutor) Resume(ctx context.Context) error {
	e.mu.Lock()
	if !e.plan.CanResume() {
		e.mu.Unlock()
		return fmt.Errorf("plan cannot be resumed from status: %s", e.plan.Status)
	}
	e.mu.Unlock()
	return e.Execute(ctx)
}

// Cancel cancels plan execution.
func (e *PlanExecutor) Cancel() {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
	}
	e.plan.Cancel()
	e.mu.Unlock()
	e.notifyProgress()
}

// executeStep executes a single step in the plan.
func (e *PlanExecutor) executeStep(ctx context.Context, step *PlanStep) error {
	// Mark step as running
	e.mu.Lock()
	step.Status = StepRunning
	step.StartTime = time.Now()
	e.mu.Unlock()
	e.notifyProgress()

	// Execute each tool call in the step
	for i := range step.ToolCalls {
		toolCall := &step.ToolCalls[i]

		// Check for cancellation
		select {
		case <-ctx.Done():
			e.mu.Lock()
			step.Status = StepFailed
			step.Error = fmt.Errorf("step cancelled")
			step.EndTime = time.Now()
			e.mu.Unlock()
			return step.Error
		default:
		}

		// Execute tool call
		e.mu.RLock()
		hasExecutor := e.executor != nil
		e.mu.RUnlock()

		if !hasExecutor {
			e.mu.Lock()
			step.Status = StepFailed
			step.Error = fmt.Errorf("tool executor not configured")
			step.EndTime = time.Now()
			e.mu.Unlock()
			return step.Error
		}

		result, err := e.executeToolCall(toolCall)
		if err != nil {
			e.mu.Lock()
			step.Status = StepFailed
			step.Error = fmt.Errorf("tool %s failed: %w", toolCall.Name, err)
			step.EndTime = time.Now()
			e.mu.Unlock()
			return step.Error
		}

		// Append result to step result
		e.mu.Lock()
		if step.Result != "" {
			step.Result += "\n\n"
		}
		step.Result += fmt.Sprintf("[%s] %s", toolCall.Name, result)
		e.mu.Unlock()
	}

	// Mark step as complete
	e.mu.Lock()
	step.Status = StepComplete
	step.EndTime = time.Now()
	e.mu.Unlock()
	e.notifyProgress()

	return nil
}

// executeToolCall executes a single tool call.
func (e *PlanExecutor) executeToolCall(toolCall *ToolCall) (string, error) {
	// TODO: Implement actual tool execution using the tool executor
	// This is a stub implementation that should be replaced with real tool execution
	return "", fmt.Errorf("tool execution not implemented: %s", toolCall.Name)
}

// notifyProgress calls the progress callback if set.
func (e *PlanExecutor) notifyProgress() {
	e.mu.RLock()
	cb := e.onProgress
	e.mu.RUnlock()

	if cb != nil {
		e.mu.RLock()
		status := fmt.Sprintf("Step %s: %s",
			e.plan.Progress(),
			e.plan.CurrentStepDescription())
		currentStep := e.plan.CurrentStep
		totalSteps := len(e.plan.Steps)
		e.mu.RUnlock()

		cb(currentStep, totalSteps, status)
	}
}

// GetPlan returns the plan being executed.
func (e *PlanExecutor) GetPlan() *Plan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plan
}

// IsRunning returns true if the plan is currently executing.
func (e *PlanExecutor) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plan.Status == StatusRunning
}

// IsPaused returns true if the plan is paused.
func (e *PlanExecutor) IsPaused() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plan.Status == StatusPaused
}

// IsComplete returns true if the plan has completed.
func (e *PlanExecutor) IsComplete() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plan.IsComplete()
}
