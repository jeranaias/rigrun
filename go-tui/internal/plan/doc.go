// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
//
// This package enables the LLM to break down complex tasks into steps,
// execute them in order, and track progress through the plan.
//
// # Key Types
//
//   - Plan: Multi-step execution plan with steps and status
//   - Step: Single step with description and tool invocation
//   - PlanStatus: Plan status enumeration (Pending, Running, etc.)
//   - Executor: Executes plan steps with progress tracking
//   - Generator: Creates plans from natural language requests
//
// # Usage
//
// Generate a plan from a request:
//
//	generator := plan.NewGenerator(client)
//	p, err := generator.Generate(ctx, "Refactor the user service")
//
// Execute a plan:
//
//	executor := plan.NewExecutor(tools)
//	for result := range executor.Execute(ctx, p) {
//	    fmt.Printf("Step %d: %s\n", result.StepIndex, result.Status)
//	}
//
// # Plan Format
//
// Plans are structured as ordered steps with dependencies,
// allowing for complex multi-step operations with rollback support.
package plan
