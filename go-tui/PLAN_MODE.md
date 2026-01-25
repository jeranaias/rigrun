# Plan Mode Feature Documentation

## Overview

Plan Mode is a major feature addition to rigrun Go TUI that enables users to create and execute multi-step execution plans for complex tasks. It provides a structured approach to breaking down large tasks into manageable steps with progress tracking and flow control.

## Features

### Core Capabilities

1. **Task Planning**: Break down complex tasks into 3-7 actionable steps
2. **Approval Workflow**: Review and approve plans before execution
3. **Progress Tracking**: Real-time progress display (Step 2/5: Running tests)
4. **Flow Control**: Pause, resume, and cancel plan execution
5. **Step Editing**: Modify plan steps before execution
6. **Error Handling**: Continue on error or fail-fast modes

## Usage

### Creating a Plan

Use the `/plan` command followed by a task description:

```bash
/plan Refactor auth to use RBAC
```

The system will:
1. Generate a multi-step plan using an LLM or template
2. Display the plan in Draft status
3. Show all steps with descriptions and tool calls
4. Wait for user approval

### Plan Workflow

#### 1. Review Plan (Draft Status)

```
Execution Plan
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Complete task: Refactor auth to use RBAC
Status: Draft

Steps:
  ○ 1. Analyze current code structure
     → search_files: Find all Go files in the project
  ○ 2. Identify refactoring opportunities
     → read_file: Read main implementation
  ○ 3. Create new structure
     → write_file: Create refactored code
  ○ 4. Update imports and references
     → edit_file: Update imports
  ○ 5. Run tests to verify refactoring
     → execute_command: Verify all tests pass

Actions: [a]pprove | [e]dit | [c]ancel
```

**Available Actions:**
- `[a]` - Approve the plan and make it ready for execution
- `[e]` - Edit steps (modify descriptions, tool calls, or order)
- `[c]` - Cancel the plan

#### 2. Execute Plan (Running Status)

After approval, press `[s]` to start execution:

```
Execution Plan
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Complete task: Refactor auth to use RBAC
Status: Running | Progress: 2/5

Steps:
  ● 1. Analyze current code structure (2.3s)
  ◐ 2. Identify refactoring opportunities
  ○ 3. Create new structure
  ○ 4. Update imports and references
  ○ 5. Run tests to verify refactoring

Actions: [p]ause | [c]ancel
```

**Step Icons:**
- `○` - Pending (not yet started)
- `◐` - Running (currently executing)
- `●` - Complete (finished successfully)
- `✗` - Failed (encountered an error)
- `⊘` - Skipped (skipped due to failure or user choice)

#### 3. Pause Execution (Paused Status)

Press `[p]` to pause:

```
Status: Paused | Progress: 2/5

Actions: [r]esume | [e]dit | [c]ancel
```

#### 4. Completed Plan

```
Status: Complete | Progress: 5/5

Steps:
  ● 1. Analyze current code structure (2.3s)
  ● 2. Identify refactoring opportunities (1.8s)
  ● 3. Create new structure (4.2s)
  ● 4. Update imports and references (1.5s)
  ● 5. Run tests to verify refactoring (8.7s)
     ✓ All tests passed

Actions: [n]ew plan | [c]lose
```

### Status Bar Integration

During plan execution, the status bar shows compact progress:

```
Model: opus-4   Mode: hybrid   Plan: Step 2/5 - Identify refactoring opportunities
```

## Technical Architecture

### Core Components

#### Plan Package (`internal/plan/`)

1. **plan.go** - Core types and data structures
   - `Plan`: Multi-step execution plan
   - `PlanStep`: Individual step with tool calls
   - `PlanStatus`: Draft, Approved, Running, Paused, Complete, Failed, Cancelled
   - `StepStatus`: Pending, Running, Complete, Failed, Skipped

2. **executor.go** - Execution engine
   - `PlanExecutor`: Step-by-step execution
   - Progress callbacks for UI updates
   - Flow control (pause, resume, cancel)
   - Error handling strategies

3. **generator.go** - Plan generation
   - `Generator`: LLM-based plan creation
   - `LLMClient`: Interface for LLM integration
   - `GenerateFromExample()`: Template-based plans

#### UI Components

1. **plan_view.go** - Plan display component
   - Full plan view with steps
   - Color-coded status indicators
   - Progress tracking
   - Action hints

### Integration Points

#### Commands

Added to `internal/commands/registry.go`:
```go
/plan <task>  - Create and execute a multi-step plan
```

#### Message Types

New Bubble Tea messages in `internal/commands/handlers.go`:
- `CreatePlanMsg` - Trigger plan creation
- `PlanCreatedMsg` - Plan ready for approval
- `ApprovePlanMsg` - Approve and prepare execution
- `PausePlanMsg` - Pause execution
- `ResumePlanMsg` - Resume execution
- `CancelPlanMsg` - Cancel plan
- `PlanProgressMsg` - Progress updates
- `PlanCompleteMsg` - Execution finished

## Examples

### Example 1: Simple Refactoring Task

```bash
/plan Refactor auth to use RBAC
```

Generated plan:
1. Analyze current code structure
2. Identify refactoring opportunities
3. Create new structure
4. Update imports and references
5. Run tests to verify refactoring

### Example 2: Feature Implementation

```bash
/plan Add user profile API endpoint
```

Generated plan:
1. Gather requirements and analyze the task
2. Implement the core functionality
3. Add tests for the new functionality
4. Run tests and verify functionality

### Example 3: Complex Multi-File Task

```bash
/plan Migrate database from SQLite to PostgreSQL
```

Generated plan might include:
1. Analyze current database schema and queries
2. Create PostgreSQL migration scripts
3. Update database connection configuration
4. Modify SQL queries for PostgreSQL compatibility
5. Run integration tests with PostgreSQL
6. Update documentation

## Testing

### Run Plan Tests

```bash
cd c:\rigrun\go-tui
go test -v ./internal/plan
```

### Example Test Output

```
=== RUN   TestPlanLifecycle
--- PASS: TestPlanLifecycle (0.00s)
=== RUN   TestPlanProgress
--- PASS: TestPlanProgress (0.00s)
=== RUN   TestPlanStepOperations
--- PASS: TestPlanStepOperations (0.00s)
=== RUN   TestPlanExecutor
--- PASS: TestPlanExecutor (0.00s)
PASS
```

### Build Verification

```bash
cd c:\rigrun\go-tui
go build ./internal/plan
```

## API Reference

### Plan

```go
type Plan struct {
    ID          string
    Description string
    Steps       []PlanStep
    Status      PlanStatus
    CurrentStep int
    // ... timestamps and error
}
```

**Methods:**
- `Progress() string` - Returns "2/5" format progress
- `Approve()` - Marks plan as approved
- `Start()` - Begins execution
- `Pause()` - Pauses execution
- `Complete()` - Marks as complete
- `Cancel()` - Cancels execution

### PlanExecutor

```go
executor := plan.NewPlanExecutor(plan, toolExecutor)
executor.SetProgressCallback(func(step, total int, status string) {
    // Update UI
})
err := executor.Execute() // Run to completion
```

**Methods:**
- `Execute() error` - Run entire plan
- `ExecuteNext() (bool, error)` - Execute one step
- `Pause()` - Pause execution
- `Resume() error` - Resume from pause
- `Cancel()` - Cancel execution

### Generator

```go
generator := plan.NewGenerator(llmClient)
plan, err := generator.Generate(ctx, "task description")
```

## Future Enhancements

### Planned Features

1. **Plan Templates**
   - Pre-defined templates for common tasks
   - Template library with categories
   - Custom template creation

2. **Step Dependencies**
   - Explicit dependency declarations
   - DAG-based execution order
   - Parallel execution of independent steps

3. **Plan History**
   - Save completed plans
   - Reload and reuse previous plans
   - Plan analytics and metrics

4. **Advanced Editing**
   - Rich text editor for steps
   - Drag-and-drop reordering
   - Conditional steps

5. **LLM Integration**
   - Full LLM-based plan generation
   - Intelligent step breakdown
   - Context-aware tool selection

6. **Collaboration**
   - Share plans with team members
   - Plan versioning
   - Comments and annotations

## Troubleshooting

### Plan Won't Execute

**Symptom:** Plan stays in Approved status after pressing `[s]`

**Solution:** Ensure the tool executor is properly initialized in the chat model.

### Steps Not Progressing

**Symptom:** Execution appears stuck on one step

**Solution:** Check the tool executor logs. The step may be waiting for tool execution to complete.

### Plan Generation Fails

**Symptom:** `/plan` command returns an error

**Solution:**
1. Verify LLM client is configured (or use example generator)
2. Check network connectivity for cloud LLM
3. Review error message for specific issues

## Implementation Checklist

✅ Core plan types and structures
✅ Plan executor with step-by-step execution
✅ Plan generator with LLM interface
✅ UI component for plan display
✅ Command registration (/plan)
✅ Message types for plan operations
✅ Comprehensive tests
✅ Documentation

### Pending Integration

⏳ Chat model integration
⏳ LLM client implementation
⏳ Tool executor integration
⏳ Keyboard shortcuts for plan actions
⏳ Plan persistence/storage

## Files Created

1. `c:\rigrun\go-tui\internal\plan\plan.go` - Core types (272 lines)
2. `c:\rigrun\go-tui\internal\plan\executor.go` - Execution engine (249 lines)
3. `c:\rigrun\go-tui\internal\plan\generator.go` - Plan generation (267 lines)
4. `c:\rigrun\go-tui\internal\ui\components\plan_view.go` - UI component (293 lines)
5. `c:\rigrun\go-tui\internal\plan\example_test.go` - Tests and examples (222 lines)
6. `c:\rigrun\go-tui\internal\plan\README.md` - Package documentation

## Files Modified

1. `c:\rigrun\go-tui\internal\commands\registry.go` - Added /plan command
2. `c:\rigrun\go-tui\internal\commands\handlers.go` - Added plan handler and messages

---

**Total Implementation:** ~1,300 lines of production code + tests + documentation

**Build Status:** ✅ All packages build successfully
**Test Status:** ✅ All tests pass
