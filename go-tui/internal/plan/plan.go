// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
package plan

import (
	"fmt"
	"time"
)

// =============================================================================
// PLAN STATUS
// =============================================================================

// PlanStatus represents the current state of a plan.
type PlanStatus int

const (
	// StatusDraft - Plan created but not yet approved
	StatusDraft PlanStatus = iota

	// StatusApproved - Plan approved by user, ready to execute
	StatusApproved

	// StatusRunning - Plan is currently executing
	StatusRunning

	// StatusPaused - Plan execution paused by user
	StatusPaused

	// StatusComplete - Plan finished successfully
	StatusComplete

	// StatusFailed - Plan execution failed
	StatusFailed

	// StatusCancelled - Plan cancelled by user
	StatusCancelled
)

// String returns the string representation of a plan status.
func (s PlanStatus) String() string {
	switch s {
	case StatusDraft:
		return "Draft"
	case StatusApproved:
		return "Approved"
	case StatusRunning:
		return "Running"
	case StatusPaused:
		return "Paused"
	case StatusComplete:
		return "Complete"
	case StatusFailed:
		return "Failed"
	case StatusCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// =============================================================================
// STEP STATUS
// =============================================================================

// StepStatus represents the current state of a plan step.
type StepStatus int

const (
	// StepPending - Step not yet started
	StepPending StepStatus = iota

	// StepRunning - Step currently executing
	StepRunning

	// StepComplete - Step completed successfully
	StepComplete

	// StepFailed - Step failed
	StepFailed

	// StepSkipped - Step skipped (due to earlier failure or user choice)
	StepSkipped
)

// String returns the string representation of a step status.
func (s StepStatus) String() string {
	switch s {
	case StepPending:
		return "Pending"
	case StepRunning:
		return "Running"
	case StepComplete:
		return "Complete"
	case StepFailed:
		return "Failed"
	case StepSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// =============================================================================
// TOOL CALL
// =============================================================================

// ToolCall represents a single tool invocation in a plan step.
type ToolCall struct {
	// Name of the tool to call
	Name string

	// Arguments for the tool (JSON-encoded)
	Arguments map[string]interface{}

	// Description of what this tool call does
	Description string
}

// =============================================================================
// PLAN STEP
// =============================================================================

// PlanStep represents a single step in an execution plan.
type PlanStep struct {
	// ID is a unique identifier for this step
	ID string

	// Description is a human-readable description of what this step does
	Description string

	// ToolCalls are the tool invocations to execute for this step
	ToolCalls []ToolCall

	// Status is the current execution status of this step
	Status StepStatus

	// Result contains the output/result of this step after execution
	Result string

	// Error contains any error that occurred during execution
	Error error

	// StartTime is when this step started executing
	StartTime time.Time

	// EndTime is when this step completed
	EndTime time.Time

	// Dependencies are step IDs that must complete before this step
	Dependencies []string

	// Editable indicates if the user can modify this step
	Editable bool
}

// Duration returns how long this step took to execute.
func (s *PlanStep) Duration() time.Duration {
	if s.StartTime.IsZero() {
		return 0
	}
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// IsComplete returns true if the step is in a terminal state.
func (s *PlanStep) IsComplete() bool {
	return s.Status == StepComplete || s.Status == StepFailed || s.Status == StepSkipped
}

// =============================================================================
// PLAN
// =============================================================================

// Plan represents a multi-step execution plan.
type Plan struct {
	// ID is a unique identifier for this plan
	ID string

	// Description is a high-level description of what this plan accomplishes
	Description string

	// Steps are the individual steps in the plan
	Steps []PlanStep

	// Status is the current status of the plan
	Status PlanStatus

	// CurrentStep is the index of the step currently being executed
	CurrentStep int

	// CreatedAt is when the plan was created
	CreatedAt time.Time

	// StartedAt is when execution started
	StartedAt time.Time

	// CompletedAt is when the plan finished (successfully or not)
	CompletedAt time.Time

	// Error contains any error that caused the plan to fail
	Error error

	// OriginalTask is the user's original task description
	OriginalTask string
}

// Progress returns the current progress as a string (e.g., "2/5")
func (p *Plan) Progress() string {
	total := len(p.Steps)
	if total == 0 {
		return "0/0"
	}
	current := p.CurrentStep
	if current >= total {
		current = total
	}
	return fmt.Sprintf("%d/%d", current, total)
}

// CurrentStepDescription returns the description of the current step.
func (p *Plan) CurrentStepDescription() string {
	if p.CurrentStep >= 0 && p.CurrentStep < len(p.Steps) {
		return p.Steps[p.CurrentStep].Description
	}
	return ""
}

// Duration returns how long the plan has been running.
func (p *Plan) Duration() time.Duration {
	if p.StartedAt.IsZero() {
		return 0
	}
	if p.CompletedAt.IsZero() {
		return time.Since(p.StartedAt)
	}
	return p.CompletedAt.Sub(p.StartedAt)
}

// IsComplete returns true if the plan is in a terminal state.
func (p *Plan) IsComplete() bool {
	return p.Status == StatusComplete || p.Status == StatusFailed || p.Status == StatusCancelled
}

// CompletedSteps returns the number of completed steps.
func (p *Plan) CompletedSteps() int {
	count := 0
	for _, step := range p.Steps {
		if step.Status == StepComplete {
			count++
		}
	}
	return count
}

// FailedSteps returns the number of failed steps.
func (p *Plan) FailedSteps() int {
	count := 0
	for _, step := range p.Steps {
		if step.Status == StepFailed {
			count++
		}
	}
	return count
}

// CanExecute returns true if the plan can be executed.
func (p *Plan) CanExecute() bool {
	return p.Status == StatusApproved || p.Status == StatusPaused
}

// CanPause returns true if the plan can be paused.
func (p *Plan) CanPause() bool {
	return p.Status == StatusRunning
}

// CanResume returns true if the plan can be resumed.
func (p *Plan) CanResume() bool {
	return p.Status == StatusPaused
}

// CanModify returns true if the plan can be modified.
func (p *Plan) CanModify() bool {
	return p.Status == StatusDraft || p.Status == StatusPaused
}

// Approve marks the plan as approved and ready to execute.
func (p *Plan) Approve() error {
	if p.Status != StatusDraft {
		return fmt.Errorf("can only approve draft plans, current status: %s", p.Status)
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("cannot approve plan with no steps")
	}
	p.Status = StatusApproved
	return nil
}

// Start marks the plan as running.
func (p *Plan) Start() {
	if p.Status == StatusApproved || p.Status == StatusPaused {
		p.Status = StatusRunning
		if p.StartedAt.IsZero() {
			p.StartedAt = time.Now()
		}
	}
}

// Pause marks the plan as paused.
func (p *Plan) Pause() {
	if p.Status == StatusRunning {
		p.Status = StatusPaused
	}
}

// Complete marks the plan as complete.
func (p *Plan) Complete() {
	p.Status = StatusComplete
	p.CompletedAt = time.Now()
}

// Fail marks the plan as failed.
func (p *Plan) Fail(err error) {
	p.Status = StatusFailed
	p.Error = err
	p.CompletedAt = time.Now()
}

// Cancel marks the plan as cancelled.
func (p *Plan) Cancel() {
	p.Status = StatusCancelled
	p.CompletedAt = time.Now()
}

// GetStep returns a step by ID.
func (p *Plan) GetStep(id string) *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}

// UpdateStep updates a step in the plan.
func (p *Plan) UpdateStep(step PlanStep) error {
	// Check for duplicate IDs
	for i := range p.Steps {
		if p.Steps[i].ID == step.ID && i != p.findStepIndex(step.ID) {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
	}

	for i := range p.Steps {
		if p.Steps[i].ID == step.ID {
			p.Steps[i] = step
			return nil
		}
	}
	return fmt.Errorf("step not found: %s", step.ID)
}

// InsertStep inserts a new step at the specified index.
func (p *Plan) InsertStep(index int, step PlanStep) error {
	// Validate index
	if index < 0 {
		return fmt.Errorf("invalid index: %d (must be >= 0)", index)
	}
	if index > len(p.Steps) {
		index = len(p.Steps)
	}

	// Check for duplicate IDs
	for i := range p.Steps {
		if p.Steps[i].ID == step.ID {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
	}

	p.Steps = append(p.Steps[:index], append([]PlanStep{step}, p.Steps[index:]...)...)
	return nil
}

// findStepIndex returns the index of a step by ID, or -1 if not found.
func (p *Plan) findStepIndex(id string) int {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return i
		}
	}
	return -1
}

// DeleteStep removes a step by ID.
func (p *Plan) DeleteStep(id string) {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			p.Steps = append(p.Steps[:i], p.Steps[i+1:]...)
			return
		}
	}
}
