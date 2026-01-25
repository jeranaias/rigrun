// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package plan provides plan creation and execution for multi-step tasks.
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// LLM CLIENT INTERFACE
// =============================================================================

// LLMClient is an interface for generating plans using an LLM.
type LLMClient interface {
	// GenerateCompletion generates a text completion from the LLM
	GenerateCompletion(ctx context.Context, prompt string) (string, error)
}

// =============================================================================
// PLAN GENERATOR
// =============================================================================

// Generator generates execution plans from task descriptions using an LLM.
type Generator struct {
	llmClient LLMClient
}

// NewGenerator creates a new plan generator.
func NewGenerator(llmClient LLMClient) *Generator {
	return &Generator{
		llmClient: llmClient,
	}
}

// Generate creates a plan from a task description.
func (g *Generator) Generate(ctx context.Context, task string) (*Plan, error) {
	if g.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	// Generate the prompt for the LLM
	prompt := g.buildPrompt(task)

	// Get completion from LLM
	response, err := g.llmClient.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %w", err)
	}

	// Parse the LLM response into a plan
	plan, err := g.parsePlanResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	// Set metadata
	plan.ID = uuid.New().String()
	plan.OriginalTask = task
	plan.CreatedAt = time.Now()
	plan.Status = StatusDraft

	return plan, nil
}

// buildPrompt constructs the prompt for the LLM to generate a plan.
func (g *Generator) buildPrompt(task string) string {
	return fmt.Sprintf(`You are a task planning assistant. Break down the following task into a clear, executable plan.

Task: %s

Generate a plan with the following structure:
1. A brief description of the overall goal
2. A list of 3-7 steps that accomplish the task
3. For each step, include:
   - A clear description of what needs to be done
   - Any tool calls needed (e.g., file operations, code execution)

Available tools:
- read_file: Read content from a file
- write_file: Write content to a file
- edit_file: Edit specific parts of a file
- execute_command: Run a shell command
- search_files: Search for files matching a pattern
- search_content: Search file contents using regex

Format your response as JSON with this structure:
{
  "description": "Brief description of the plan",
  "steps": [
    {
      "description": "What this step does",
      "tool_calls": [
        {
          "name": "tool_name",
          "arguments": {"arg1": "value1"},
          "description": "Why this tool call is needed"
        }
      ]
    }
  ]
}

Respond with ONLY the JSON, no additional text.`, task)
}

// parsePlanResponse parses the LLM's JSON response into a Plan.
func (g *Generator) parsePlanResponse(response string) (*Plan, error) {
	// Validate response size to prevent memory issues
	const maxResponseSize = 1024 * 1024 // 1MB limit
	if len(response) > maxResponseSize {
		return nil, fmt.Errorf("response too large: %d bytes (max: %d)", len(response), maxResponseSize)
	}

	// Clean up the response - remove markdown code blocks if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON
	var planData struct {
		Description string `json:"description"`
		Steps       []struct {
			Description string `json:"description"`
			ToolCalls   []struct {
				Name        string                 `json:"name"`
				Arguments   map[string]interface{} `json:"arguments"`
				Description string                 `json:"description"`
			} `json:"tool_calls"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(response), &planData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate step count
	const minSteps = 1
	const maxSteps = 50
	if len(planData.Steps) < minSteps {
		return nil, fmt.Errorf("plan must have at least %d step(s), got %d", minSteps, len(planData.Steps))
	}
	if len(planData.Steps) > maxSteps {
		return nil, fmt.Errorf("plan has too many steps: %d (max: %d)", len(planData.Steps), maxSteps)
	}

	// Validate description
	if strings.TrimSpace(planData.Description) == "" {
		return nil, fmt.Errorf("plan description cannot be empty")
	}

	// Convert to Plan structure
	plan := &Plan{
		Description: planData.Description,
		Steps:       make([]PlanStep, 0, len(planData.Steps)),
	}

	for i, stepData := range planData.Steps {
		step := PlanStep{
			ID:          fmt.Sprintf("step-%d", i+1),
			Description: stepData.Description,
			Status:      StepPending,
			Editable:    true,
			ToolCalls:   make([]ToolCall, 0, len(stepData.ToolCalls)),
		}

		for _, tcData := range stepData.ToolCalls {
			toolCall := ToolCall{
				Name:        tcData.Name,
				Arguments:   tcData.Arguments,
				Description: tcData.Description,
			}
			step.ToolCalls = append(step.ToolCalls, toolCall)
		}

		plan.Steps = append(plan.Steps, step)
	}

	return plan, nil
}

// GenerateFromExample creates a plan from an example (for testing/demo).
func GenerateFromExample(task string) *Plan {
	// Create a sample plan based on the task
	plan := &Plan{
		ID:           uuid.New().String(),
		Description:  fmt.Sprintf("Complete task: %s", task),
		OriginalTask: task,
		CreatedAt:    time.Now(),
		Status:       StatusDraft,
		CurrentStep:  0,
	}

	// Generate example steps based on common patterns
	if strings.Contains(strings.ToLower(task), "refactor") {
		plan.Steps = []PlanStep{
			{
				ID:          "step-1",
				Description: "Analyze current code structure",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "search_files",
						Arguments:   map[string]interface{}{"pattern": "*.go"},
						Description: "Find all Go files in the project",
					},
				},
			},
			{
				ID:          "step-2",
				Description: "Identify refactoring opportunities",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "read_file",
						Arguments:   map[string]interface{}{"path": "main.go"},
						Description: "Read main implementation",
					},
				},
			},
			{
				ID:          "step-3",
				Description: "Create new structure",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "write_file",
						Arguments:   map[string]interface{}{"path": "new_structure.go"},
						Description: "Create refactored code",
					},
				},
			},
			{
				ID:          "step-4",
				Description: "Update imports and references",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "edit_file",
						Arguments:   map[string]interface{}{"path": "main.go"},
						Description: "Update imports",
					},
				},
			},
			{
				ID:          "step-5",
				Description: "Run tests to verify refactoring",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "execute_command",
						Arguments:   map[string]interface{}{"command": "go test ./..."},
						Description: "Verify all tests pass",
					},
				},
			},
		}
	} else {
		// Generic multi-step plan
		plan.Steps = []PlanStep{
			{
				ID:          "step-1",
				Description: "Gather requirements and analyze the task",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "search_files",
						Arguments:   map[string]interface{}{"pattern": "*"},
						Description: "Survey the codebase",
					},
				},
			},
			{
				ID:          "step-2",
				Description: "Implement the core functionality",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "write_file",
						Arguments:   map[string]interface{}{"path": "implementation.go"},
						Description: "Write the implementation",
					},
				},
			},
			{
				ID:          "step-3",
				Description: "Add tests for the new functionality",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "write_file",
						Arguments:   map[string]interface{}{"path": "implementation_test.go"},
						Description: "Write tests",
					},
				},
			},
			{
				ID:          "step-4",
				Description: "Run tests and verify functionality",
				Status:      StepPending,
				Editable:    true,
				ToolCalls: []ToolCall{
					{
						Name:        "execute_command",
						Arguments:   map[string]interface{}{"command": "go test ./..."},
						Description: "Execute test suite",
					},
				},
			},
		}
	}

	return plan
}
