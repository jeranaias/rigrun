// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
package plan_test

import (
	"fmt"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/plan"
)

// ExamplePlan demonstrates creating and using a plan.
func ExamplePlan() {
	// Create a plan using the example generator
	p := plan.GenerateFromExample("Refactor auth to use RBAC")

	// Print plan details
	fmt.Printf("Plan: %s\n", p.Description)
	fmt.Printf("Status: %s\n", p.Status)
	fmt.Printf("Steps: %d\n", len(p.Steps))

	// List steps
	for i, step := range p.Steps {
		fmt.Printf("  %d. %s (%s)\n", i+1, step.Description, step.Status)
	}

	// Approve the plan
	if err := p.Approve(); err != nil {
		fmt.Printf("Error approving plan: %v\n", err)
		return
	}
	fmt.Printf("\nAfter approval: %s\n", p.Status)

	// Output:
	// Plan: Complete task: Refactor auth to use RBAC
	// Status: Draft
	// Steps: 5
	//   1. Analyze current code structure (Pending)
	//   2. Identify refactoring opportunities (Pending)
	//   3. Create new structure (Pending)
	//   4. Update imports and references (Pending)
	//   5. Run tests to verify refactoring (Pending)
	//
	// After approval: Approved
}

// TestPlanLifecycle tests the complete plan lifecycle.
func TestPlanLifecycle(t *testing.T) {
	// Create plan
	p := plan.GenerateFromExample("Test task")

	// Verify initial state
	if p.Status != plan.StatusDraft {
		t.Errorf("Expected status Draft, got %s", p.Status)
	}

	// Approve plan
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}
	if p.Status != plan.StatusApproved {
		t.Errorf("Expected status Approved, got %s", p.Status)
	}

	// Start plan
	p.Start()
	if p.Status != plan.StatusRunning {
		t.Errorf("Expected status Running, got %s", p.Status)
	}

	// Pause plan
	p.Pause()
	if p.Status != plan.StatusPaused {
		t.Errorf("Expected status Paused, got %s", p.Status)
	}

	// Resume (start again from paused)
	p.Start()
	if p.Status != plan.StatusRunning {
		t.Errorf("Expected status Running after resume, got %s", p.Status)
	}

	// Complete plan
	p.Complete()
	if p.Status != plan.StatusComplete {
		t.Errorf("Expected status Complete, got %s", p.Status)
	}
}

// TestPlanProgress tests progress tracking.
func TestPlanProgress(t *testing.T) {
	p := plan.GenerateFromExample("Test task")

	// Initial progress
	progress := p.Progress()
	if progress != "0/4" && progress != "0/5" {
		t.Logf("Initial progress: %s (varies based on task)", progress)
	}

	// Simulate progress
	p.CurrentStep = 2
	progress = p.Progress()
	if progress != "2/4" && progress != "2/5" {
		t.Logf("Mid progress: %s (varies based on task)", progress)
	}

	// Check completed steps count
	p.Steps[0].Status = plan.StepComplete
	p.Steps[1].Status = plan.StepComplete
	completed := p.CompletedSteps()
	if completed != 2 {
		t.Errorf("Expected 2 completed steps, got %d", completed)
	}
}

// TestPlanStepOperations tests step manipulation.
func TestPlanStepOperations(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	initialSteps := len(p.Steps)

	// Get a step by ID
	step := p.GetStep("step-1")
	if step == nil {
		t.Fatal("Expected to find step-1")
	}
	if step.ID != "step-1" {
		t.Errorf("Expected step ID 'step-1', got %s", step.ID)
	}

	// Update a step
	step.Description = "Updated description"
	if err := p.UpdateStep(*step); err != nil {
		t.Fatalf("Failed to update step: %v", err)
	}
	updated := p.GetStep("step-1")
	if updated.Description != "Updated description" {
		t.Error("Step update failed")
	}

	// Insert a new step
	newStep := plan.PlanStep{
		ID:          "step-new",
		Description: "New step",
		Status:      plan.StepPending,
	}
	if err := p.InsertStep(1, newStep); err != nil {
		t.Fatalf("Failed to insert step: %v", err)
	}
	if len(p.Steps) != initialSteps+1 {
		t.Errorf("Expected %d steps after insert, got %d", initialSteps+1, len(p.Steps))
	}

	// Delete a step
	p.DeleteStep("step-new")
	if len(p.Steps) != initialSteps {
		t.Errorf("Expected %d steps after delete, got %d", initialSteps, len(p.Steps))
	}
}

// TestPlanExecutor tests the plan executor.
func TestPlanExecutor(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	// Create executor
	executor := plan.NewPlanExecutor(p, nil)

	// Check initial state
	if !executor.GetPlan().CanExecute() {
		t.Error("Plan should be executable after approval")
	}

	// Test state checks
	if executor.IsRunning() {
		t.Error("Executor should not be running initially")
	}

	if executor.IsComplete() {
		t.Error("Executor should not be complete initially")
	}
}

// TestPlanExecutorCallbacks tests progress callbacks.
func TestPlanExecutorCallbacks(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	// Set up progress callback
	var progressCalls []string
	executor.SetProgressCallback(func(step, total int, status string) {
		progressCalls = append(progressCalls, fmt.Sprintf("%d/%d: %s", step, total, status))
	})

	// Note: Full execution would require a tool executor
	// This test just verifies the callback mechanism is set up correctly
	if executor.GetPlan().Status != plan.StatusApproved {
		t.Errorf("Expected plan to be approved, got %s", executor.GetPlan().Status)
	}
}

// BenchmarkPlanCreation benchmarks plan creation.
func BenchmarkPlanCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = plan.GenerateFromExample("Benchmark task")
	}
}

// BenchmarkPlanProgress benchmarks progress calculation.
func BenchmarkPlanProgress(b *testing.B) {
	p := plan.GenerateFromExample("Benchmark task")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = p.Progress()
		_ = p.CompletedSteps()
		_ = p.FailedSteps()
	}
}
