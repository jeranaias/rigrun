# Plan Mode Implementation

Plan Mode is a major feature for the rigrun Go TUI that enables users to create and execute multi-step plans for complex tasks.

## Overview

Plan Mode allows users to:
- Break down complex tasks into manageable steps
- Review and approve plans before execution
- Monitor progress through step-by-step execution
- Pause, resume, and modify plans during execution

## Architecture

### Core Components

#### 1. `plan.go` - Core Types
- **Plan**: Represents a multi-step execution plan
  - ID, Description, Status, Steps
  - Progress tracking (CurrentStep, timestamps)
  - Methods for state management (Approve, Start, Pause, Complete, etc.)

- **PlanStep**: Represents a single step in a plan
  - ID, Description, Status, ToolCalls
  - Result, Error, Duration tracking
  - Dependencies between steps

- **PlanStatus**: Draft, Approved, Running, Paused, Complete, Failed, Cancelled

- **StepStatus**: Pending, Running, Complete, Failed, Skipped

#### 2. `executor.go` - Plan Execution
- **PlanExecutor**: Executes plans step-by-step
  - Execute(): Run entire plan to completion
  - ExecuteNext(): Execute one step at a time
  - Pause(), Resume(), Cancel(): Flow control
  - Progress callbacks for UI updates
  - Error handling (continue on error or fail fast)

#### 3. `generator.go` - Plan Generation
- **Generator**: Creates plans from task descriptions using LLM
  - LLMClient interface for flexibility
  - Prompt engineering for structured plan output
  - JSON parsing of LLM responses
  - GenerateFromExample(): Demo/testing utility

#### 4. `plan_view.go` - UI Component
- **PlanView**: Renders plans in the TUI
  - Header with status and progress
  - Step list with icons and colors
  - Interactive footer with actions
  - Compact progress indicator for status bar

## Usage

### Command

```bash
/plan <task description>
```

Example:
```bash
/plan Refactor auth to use RBAC
```

### Workflow

1. **Create Plan**: User types `/plan <task>`
   - System sends CreatePlanMsg
   - Generator creates plan using LLM or example
   - Plan shown in Draft status

2. **Review Plan**: User reviews steps
   - Can edit steps if needed
   - Can modify descriptions or tool calls
   - Plan remains in Draft until approved

3. **Approve Plan**: User approves with `[a]`
   - Plan status changes to Approved
   - Ready for execution

4. **Execute Plan**: User starts with `[s]`
   - Plan status changes to Running
   - Steps execute sequentially
   - Progress shown: "Step 2/5: Running tests"

5. **Monitor Progress**: Real-time updates
   - Current step highlighted
   - Duration tracking for completed steps
   - Results/errors displayed inline

6. **Flow Control**:
   - Pause: `[p]` - Pause execution
   - Resume: `[r]` - Continue from paused state
   - Cancel: `[c]` - Abort execution

## Integration Points

### Commands Integration

Added to `internal/commands/registry.go`:
```go
r.Register(&Command{
    Name:        "/plan",
    Description: "Create and execute a multi-step plan",
    Usage:       "/plan <task>",
    Args: []ArgDef{
        {Name: "task", Required: true, Type: ArgTypeString, Description: "Task description to plan"},
    },
    Category: "Tools",
    Handler:  handlePlan,
})
```

### Message Types

Added to `internal/commands/handlers.go`:
- `CreatePlanMsg`: Trigger plan creation
- `PlanCreatedMsg`: Plan created, ready for approval
- `ApprovePlanMsg`: Approve plan
- `PausePlanMsg`: Pause execution
- `ResumePlanMsg`: Resume execution
- `CancelPlanMsg`: Cancel execution
- `EditPlanMsg`: Open plan editor
- `PlanProgressMsg`: Progress update
- `PlanCompleteMsg`: Execution complete

### Chat Model Integration

The chat model needs to:
1. Handle CreatePlanMsg and create/display plan
2. Store current plan state
3. Process plan control messages (approve, pause, resume, cancel)
4. Update UI based on PlanProgressMsg
5. Show compact progress in status bar during execution

Example chat model fields:
```go
type Model struct {
    // ... existing fields ...

    // Plan mode
    currentPlan     *plan.Plan
    planExecutor    *plan.PlanExecutor
    showPlanView    bool
    planView        *components.PlanView
}
```

## Example Plan Structure

```json
{
  "id": "plan-123",
  "description": "Refactor authentication to use RBAC",
  "status": "Running",
  "current_step": 2,
  "steps": [
    {
      "id": "step-1",
      "description": "Analyze current auth implementation",
      "status": "Complete",
      "tool_calls": [
        {
          "name": "search_files",
          "arguments": {"pattern": "auth*.go"},
          "description": "Find auth files"
        }
      ],
      "result": "Found 5 auth files"
    },
    {
      "id": "step-2",
      "description": "Design RBAC structure",
      "status": "Running",
      "tool_calls": [
        {
          "name": "write_file",
          "arguments": {"path": "rbac.go"},
          "description": "Create RBAC types"
        }
      ]
    },
    {
      "id": "step-3",
      "description": "Migrate existing auth to RBAC",
      "status": "Pending"
    },
    {
      "id": "step-4",
      "description": "Update tests",
      "status": "Pending"
    },
    {
      "id": "step-5",
      "description": "Run full test suite",
      "status": "Pending",
      "tool_calls": [
        {
          "name": "execute_command",
          "arguments": {"command": "go test ./..."},
          "description": "Verify all tests pass"
        }
      ]
    }
  ]
}
```

## UI Display

```
Execution Plan
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Refactor authentication to use RBAC
Status: Running | Progress: 2/5

Steps:
  ● 1. Analyze current auth implementation (2.3s)
  ◐ 2. Design RBAC structure
     → write_file: Create RBAC types
  ○ 3. Migrate existing auth to RBAC
  ○ 4. Update tests
  ○ 5. Run full test suite
     → execute_command: Verify all tests pass

Actions: [p]ause | [c]ancel
```

## Status Bar Integration

During execution, show compact progress:
```
Plan: Step 2/5 - Design RBAC structure
```

## Testing

Build verification:
```bash
cd c:\rigrun\go-tui
go build ./internal/plan
```

## Future Enhancements

1. **Plan Templates**: Pre-defined plans for common tasks
2. **Step Dependencies**: Explicit dependency graph
3. **Parallel Execution**: Run independent steps in parallel
4. **Plan History**: Save and reload previous plans
5. **Plan Sharing**: Export/import plans as JSON
6. **LLM Integration**: Full LLM-based plan generation
7. **Interactive Editing**: Rich editor for modifying steps
8. **Rollback Support**: Undo completed steps

## Implementation Status

✅ Core plan types and structures
✅ Plan executor with step-by-step execution
✅ Plan generator with LLM interface
✅ UI component for plan display
✅ Command registration (/plan)
✅ Message types for plan operations
⏳ Chat model integration (pending)
⏳ LLM client implementation (pending)
⏳ Tool executor integration (pending)

## Files Created

1. `c:\rigrun\go-tui\internal\plan\plan.go` - Core types
2. `c:\rigrun\go-tui\internal\plan\executor.go` - Execution engine
3. `c:\rigrun\go-tui\internal\plan\generator.go` - Plan generation
4. `c:\rigrun\go-tui\internal\ui\components\plan_view.go` - UI component

## Files Modified

1. `c:\rigrun\go-tui\internal\commands\registry.go` - Added /plan command
2. `c:\rigrun\go-tui\internal\commands\handlers.go` - Added plan messages and handler
