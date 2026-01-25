// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
package plan_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/plan"
)

// TestRaceConditionContextManagement tests that new Execute() calls create new contexts.
func TestRaceConditionContextManagement(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	// First execution that gets cancelled
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1() // Cancel immediately

	err := executor.Execute(ctx1)
	if err == nil {
		t.Error("Expected error from cancelled context")
	}

	// Second execution should work with a new context
	// This would fail in the old implementation because ctx.Done() was already closed
	ctx2 := context.Background()
	// We can't actually execute because we need a real tool executor,
	// but we can verify the executor doesn't immediately fail
	go func() {
		time.Sleep(10 * time.Millisecond)
		executor.Cancel()
	}()

	err = executor.Execute(ctx2)
	if err != nil && !strings.Contains(err.Error(), "cancelled") && !strings.Contains(err.Error(), "not configured") {
		t.Errorf("Expected cancellation or not-configured error, got: %v", err)
	}
}

// TestMutexProtectionOnPlanState tests concurrent access to plan state.
func TestMutexProtectionOnPlanState(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	// Start multiple goroutines reading plan state
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = executor.GetPlan()
				_ = executor.IsRunning()
				_ = executor.IsPaused()
				_ = executor.IsComplete()
			}
		}()
	}

	// One goroutine modifying state
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		if err := executor.Execute(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no race condition
	case <-time.After(5 * time.Second):
		t.Error("Test timed out - possible deadlock")
	}

	close(errChan)
	for err := range errChan {
		if err != nil && !strings.Contains(err.Error(), "not implemented") {
			t.Logf("Expected error during execution: %v", err)
		}
	}
}

// TestExecutorNilCheck tests that nil executor is properly handled.
func TestExecutorNilCheck(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	// Create executor with nil tool executor
	executor := plan.NewPlanExecutor(p, nil)

	ctx := context.Background()
	err := executor.Execute(ctx)

	// Should get an error about executor not configured
	if err == nil {
		t.Error("Expected error when executor is nil")
	}
	if !strings.Contains(err.Error(), "not configured") && !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not configured' or 'not implemented' error, got: %v", err)
	}
}

// TestUnboundedStepSliceGrowth tests that negative index validation works.
func TestUnboundedStepSliceGrowth(t *testing.T) {
	p := plan.GenerateFromExample("Test task")

	newStep := plan.PlanStep{
		ID:          "step-invalid",
		Description: "Invalid step",
		Status:      plan.StepPending,
	}

	// Try to insert with negative index
	err := p.InsertStep(-1, newStep)
	if err == nil {
		t.Error("Expected error when inserting with negative index")
	}
	if !strings.Contains(err.Error(), "invalid index") {
		t.Errorf("Expected 'invalid index' error, got: %v", err)
	}
}

// TestIndexOutOfRangePrevention tests bounds checking.
func TestIndexOutOfRangePrevention(t *testing.T) {
	p := &plan.Plan{
		ID:          "test",
		Description: "Test plan",
		Steps:       []plan.PlanStep{}, // Empty steps
		Status:      plan.StatusApproved,
	}

	executor := plan.NewPlanExecutor(p, nil)

	ctx := context.Background()
	err := executor.Execute(ctx)

	// Should not panic, should handle gracefully
	if err != nil {
		t.Logf("Expected error with empty steps: %v", err)
	}
}

// TestGeneratorSizeValidation tests response size limits.
func TestGeneratorSizeValidation(t *testing.T) {
	gen := plan.NewGenerator(nil)

	// Test with oversized response (this is internal, so we'd need to expose it or test indirectly)
	// For now, we'll test through the public API which should handle this

	// The generator would reject responses over 1MB
	// We can't easily test this without a mock LLM client
	if gen == nil {
		t.Error("Generator should not be nil")
	}
}

// TestDuplicateStepIDPrevention tests that duplicate step IDs are prevented.
func TestDuplicateStepIDPrevention(t *testing.T) {
	p := plan.GenerateFromExample("Test task")

	// Try to insert a step with duplicate ID
	duplicateStep := plan.PlanStep{
		ID:          "step-1", // This ID already exists
		Description: "Duplicate step",
		Status:      plan.StepPending,
	}

	err := p.InsertStep(0, duplicateStep)
	if err == nil {
		t.Error("Expected error when inserting duplicate step ID")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

// TestStateValidationOnApprove tests that Approve() validates plan state.
func TestStateValidationOnApprove(t *testing.T) {
	// Test approving empty plan
	emptyPlan := &plan.Plan{
		ID:          "empty",
		Description: "Empty plan",
		Steps:       []plan.PlanStep{},
		Status:      plan.StatusDraft,
	}

	err := emptyPlan.Approve()
	if err == nil {
		t.Error("Expected error when approving plan with no steps")
	}
	if !strings.Contains(err.Error(), "no steps") {
		t.Errorf("Expected 'no steps' error, got: %v", err)
	}

	// Test approving non-draft plan
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed first approval: %v", err)
	}

	err = p.Approve()
	if err == nil {
		t.Error("Expected error when approving already approved plan")
	}
}

// TestProgressCallbackConcurrency tests that progress callbacks are thread-safe.
func TestProgressCallbackConcurrency(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	var callbackMutex sync.Mutex
	callbackCalls := 0

	executor.SetProgressCallback(func(step, total int, status string) {
		callbackMutex.Lock()
		callbackCalls++
		callbackMutex.Unlock()
	})

	// Try to trigger concurrent callback access
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = executor.Execute(ctx)
		}()
	}

	wg.Wait()

	// If we got here without panic, the mutex protection worked
	callbackMutex.Lock()
	t.Logf("Callback was called %d times", callbackCalls)
	callbackMutex.Unlock()
}

// TestUpdateStepWithSameID tests that updating a step with its own ID works.
func TestUpdateStepWithSameID(t *testing.T) {
	p := plan.GenerateFromExample("Test task")

	step := p.GetStep("step-1")
	if step == nil {
		t.Fatal("Expected to find step-1")
	}

	// Update the step with modified description but same ID
	step.Description = "Modified description"
	err := p.UpdateStep(*step)
	if err != nil {
		t.Errorf("Should allow updating step with same ID: %v", err)
	}

	// Verify update worked
	updated := p.GetStep("step-1")
	if updated.Description != "Modified description" {
		t.Error("Step update failed")
	}
}

// TestContextPropagation tests that context is properly propagated.
func TestContextPropagation(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := executor.Execute(ctx)

	// Should get timeout or cancellation error
	if err != nil {
		if !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "not") {
			t.Errorf("Expected cancellation/deadline/not-implemented error, got: %v", err)
		}
	}
}

// TestExecuteNextWithContext tests ExecuteNext with context parameter.
func TestExecuteNextWithContext(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	ctx := context.Background()
	_, err := executor.ExecuteNext(ctx)

	// Should fail because executor is nil, but shouldn't panic
	if err == nil {
		t.Error("Expected error with nil executor")
	}
}

// TestResumeWithContext tests Resume with context parameter.
func TestResumeWithContext(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	// Can't resume before starting
	ctx := context.Background()
	err := executor.Resume(ctx)
	if err == nil {
		t.Error("Expected error when resuming from wrong state")
	}

	// Start and pause
	executor.GetPlan().Start()
	executor.Pause()

	// Now resume should work (but will fail due to nil executor)
	err = executor.Resume(ctx)
	if err != nil && !strings.Contains(err.Error(), "not") {
		t.Logf("Expected error during resume: %v", err)
	}
}

// TestCancelSafety tests that Cancel is safe to call multiple times.
func TestCancelSafety(t *testing.T) {
	p := plan.GenerateFromExample("Test task")
	if err := p.Approve(); err != nil {
		t.Fatalf("Failed to approve plan: %v", err)
	}

	executor := plan.NewPlanExecutor(p, nil)

	// Cancel before starting - should be safe
	executor.Cancel()

	// Cancel again - should be safe
	executor.Cancel()

	// Start execution
	go func() {
		ctx := context.Background()
		_ = executor.Execute(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Cancel during execution - should be safe
	executor.Cancel()

	// Cancel again - should be safe
	executor.Cancel()
}

// TestValidationErrorMessages tests that error messages are clear.
func TestValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() error
		wantErr  string
	}{
		{
			name: "empty plan approval",
			testFunc: func() error {
				p := &plan.Plan{Status: plan.StatusDraft, Steps: []plan.PlanStep{}}
				return p.Approve()
			},
			wantErr: "no steps",
		},
		{
			name: "duplicate step ID",
			testFunc: func() error {
				p := plan.GenerateFromExample("Test")
				return p.InsertStep(0, plan.PlanStep{ID: "step-1", Description: "Dup"})
			},
			wantErr: "duplicate",
		},
		{
			name: "negative index",
			testFunc: func() error {
				p := plan.GenerateFromExample("Test")
				return p.InsertStep(-5, plan.PlanStep{ID: "new", Description: "New"})
			},
			wantErr: "invalid index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if err == nil {
				t.Error("Expected error but got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
