# Plan Mode Quick Start Guide

## For Developers Integrating Plan Mode

### 1. Import the Package

```go
import "github.com/jeranaias/rigrun-tui/internal/plan"
```

### 2. Create a Plan

```go
// Using the example generator (for testing/demo)
p := plan.GenerateFromExample("Refactor auth to use RBAC")

// Or using an LLM generator
generator := plan.NewGenerator(llmClient)
p, err := generator.Generate(ctx, "Refactor auth to use RBAC")
if err != nil {
    // Handle error
}
```

### 3. Approve and Execute

```go
// Approve the plan
p.Approve()

// Create executor
executor := plan.NewPlanExecutor(p, toolExecutor)

// Set progress callback
executor.SetProgressCallback(func(step, total int, status string) {
    fmt.Printf("Progress: %d/%d - %s\n", step, total, status)
})

// Execute the plan
if err := executor.Execute(); err != nil {
    // Handle error
}
```

### 4. Display in UI

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/components"

// Create plan view
planView := components.NewPlanView(width, height)

// Render the plan
content := planView.Render(p)
fmt.Println(content)

// Or show compact progress in status bar
progress := components.RenderCompactProgress(p)
// "Plan: Step 2/5 - Running tests"
```

### 5. Handle User Actions

```go
switch action {
case "approve":
    p.Approve()
case "start":
    p.Start()
    go executor.Execute()
case "pause":
    executor.Pause()
case "resume":
    if err := executor.Resume(); err != nil {
        // Handle error
    }
case "cancel":
    executor.Cancel()
}
```

## Integration with Chat Model

### Add to Model Struct

```go
type Model struct {
    // ... existing fields ...

    // Plan mode
    currentPlan  *plan.Plan
    planExecutor *plan.PlanExecutor
    planView     *components.PlanView
    showPlan     bool
}
```

### Handle Messages

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case commands.CreatePlanMsg:
        // Generate plan
        p := plan.GenerateFromExample(msg.Task)
        m.currentPlan = p
        m.planView = components.NewPlanView(m.width, m.height)
        m.showPlan = true
        return m, nil

    case commands.ApprovePlanMsg:
        if m.currentPlan != nil {
            m.currentPlan.Approve()
            m.planExecutor = plan.NewPlanExecutor(m.currentPlan, m.toolExecutor)
            m.planExecutor.SetProgressCallback(m.onPlanProgress)
        }
        return m, nil

    case commands.PlanProgressMsg:
        // Update UI with progress
        return m, nil
    }
    // ... rest of update logic
}
```

### Render Plan in View

```go
func (m Model) View() string {
    if m.showPlan && m.currentPlan != nil {
        return m.planView.Render(m.currentPlan)
    }
    // ... normal chat view
}
```

## Implementing LLMClient

```go
type MyLLMClient struct {
    client *ollama.Client
}

func (c *MyLLMClient) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
    // Use your LLM to generate a response
    response, err := c.client.Generate(ctx, prompt)
    if err != nil {
        return "", err
    }
    return response, nil
}
```

## Keyboard Shortcuts

Suggested key bindings:

```go
// In Draft/Approved state
case key.Matches(msg, m.keys.Approve):
    return m, approvePlanCmd()

case key.Matches(msg, m.keys.Start):
    return m, startPlanCmd()

// In Running state
case key.Matches(msg, m.keys.Pause):
    return m, pausePlanCmd()

// In Paused state
case key.Matches(msg, m.keys.Resume):
    return m, resumePlanCmd()

// Any time
case key.Matches(msg, m.keys.Cancel):
    return m, cancelPlanCmd()
```

## Common Patterns

### Progress Updates

```go
executor.SetProgressCallback(func(step, total int, status string) {
    // Send progress message to UI
    progressMsg := commands.PlanProgressMsg{
        CurrentStep: step,
        TotalSteps:  total,
        Status:      status,
    }
    // Send to tea.Program
})
```

### Error Handling

```go
// Fail fast (default)
executor.SetContinueOnError(false)

// Continue on error
executor.SetContinueOnError(true)

// Check for failures after execution
if p.FailedSteps() > 0 {
    // Handle failed steps
}
```

### Step-by-Step Execution

```go
// Execute one step at a time
for {
    hasMore, err := executor.ExecuteNext()
    if err != nil {
        // Handle error
    }
    if !hasMore {
        break
    }
    // Update UI between steps
}
```

## Testing Your Integration

```go
func TestPlanIntegration(t *testing.T) {
    // Create test plan
    p := plan.GenerateFromExample("test task")

    // Create executor
    executor := plan.NewPlanExecutor(p, nil)

    // Test lifecycle
    p.Approve()
    assert.Equal(t, plan.StatusApproved, p.Status)

    p.Start()
    assert.Equal(t, plan.StatusRunning, p.Status)

    // ... more assertions
}
```

## Debugging

### Enable Logging

```go
executor.SetProgressCallback(func(step, total int, status string) {
    log.Printf("Plan progress: %d/%d - %s", step, total, status)
})
```

### Check Plan State

```go
fmt.Printf("Plan Status: %s\n", p.Status)
fmt.Printf("Current Step: %d/%d\n", p.CurrentStep, len(p.Steps))
fmt.Printf("Completed: %d, Failed: %d\n", p.CompletedSteps(), p.FailedSteps())

for i, step := range p.Steps {
    fmt.Printf("  Step %d: %s (%s)\n", i+1, step.Description, step.Status)
    if step.Error != nil {
        fmt.Printf("    Error: %v\n", step.Error)
    }
}
```

## Full Example

See `internal/plan/example_test.go` for complete working examples.

## API Reference

See `internal/plan/README.md` for detailed API documentation.

## Need Help?

- Check examples in `example_test.go`
- Review full documentation in `README.md`
- See user guide in `../../PLAN_MODE.md`
