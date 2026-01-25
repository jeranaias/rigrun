# Progress Indicators for Agentic Loops - Implementation Guide

## Overview

This implementation adds comprehensive progress tracking for multi-step agentic operations in the rigrun Go TUI. The progress indicators show:
- Current step number (e.g., "Step 2 of 5")
- Step title (e.g., "Running tests")
- Current tool being executed (e.g., "Bash (pytest tests/)")
- Elapsed time per step
- Overall elapsed time
- Progress bar showing completion percentage
- Cancellation support with Ctrl+C

## Files Created/Modified

### Created Files:
1. **`C:\rigrun\go-tui\internal\ui\components\progress.go`** (700+ lines)
   - `ProgressIndicator` - Main progress tracking component
   - `MultiStepProgress` - Multi-step progress tracker with callbacks
   - Full/Compact rendering modes
   - Time tracking and formatting
   - Status management (Running, Paused, Complete, Canceled, Error)

### Modified Files:
1. **`C:\rigrun\go-tui\internal\ui\chat\model.go`**
   - Added progress indicator fields to Model struct
   - Added progress message handlers (handleProgressStart, handleProgressStep, etc.)
   - Integrated progress tracking into Update function

2. **`C:\rigrun\go-tui\internal\ui\chat\view.go`**
   - Added renderProgressIndicator() function
   - Updated vertical layout stacking to include progress bar
   - Updated height calculations to account for progress indicator

3. **`C:\rigrun\go-tui\internal\ui\chat\messages.go`**
   - Added progress-related message types:
     - `ProgressStartMsg`
     - `ProgressStepMsg`
     - `ProgressUpdateMsg`
     - `ProgressCompleteMsg`
     - `ProgressCanceledMsg`
     - `ProgressErrorMsg`

## Usage Example

### Basic Multi-Step Operation

```go
// Start a multi-step operation
func executeAgenticPlan(model *chat.Model, steps []string) tea.Cmd {
	return func() tea.Msg {
		return chat.ProgressStartMsg{
			TotalSteps: len(steps),
			Title:      "Executing Plan",
		}
	}
}

// Update current step
func executeStep(step int, title string, tool string, args string) tea.Cmd {
	return func() tea.Msg {
		return chat.ProgressStepMsg{
			CurrentStep: step,
			TotalSteps:  5,
			StepTitle:   title,
			Tool:        tool,
			ToolArgs:    args,
		}
	}
}

// Complete the operation
func completePlan(success bool) tea.Cmd {
	return func() tea.Msg {
		return chat.ProgressCompleteMsg{
			Success: success,
			Message: "All tests passed successfully",
		}
	}
}
```

### Using MultiStepProgress Tracker

```go
// Create a multi-step progress tracker
steps := []string{
	"Analyzing codebase",
	"Running tests",
	"Generating report",
	"Uploading results",
}

progress := components.NewMultiStepProgress(steps)

// Set callbacks
progress.OnStepComplete = func(step int, info components.StepInfo) {
	log.Printf("Step %d completed in %s", step, info.EndTime.Sub(info.StartTime))
}

progress.OnAllComplete = func() {
	log.Println("All steps completed!")
}

progress.OnError = func(step int, err error) {
	log.Printf("Error at step %d: %v", step, err)
}

// Execute steps
for i := 0; i < len(steps); i++ {
	// Set tool being used for current step
	progress.SetToolForCurrentStep("Bash", "pytest tests/")

	// Do the work...
	err := performStep(i)

	if err != nil {
		progress.MarkError(err)
		break
	}

	// Move to next step
	progress.NextStep()
}

// Render current state
rendered := progress.Render()
```

## UI Design

### Full Mode (Default)
```
┌─ Executing Plan ──────────────────┐
│ Step 2 of 5: Running tests        │
│ ████████░░░░░░░░░░░░░ 40%        │
│ Tool: Bash (pytest tests/)        │
│ Elapsed: 12s | Total: 45s         │
│ Press Ctrl+C to cancel            │
└───────────────────────────────────┘
```

### Compact Mode (Single-Line)
```
[2/5] Running tests | Bash (pytest) | 12s | 40% ████░░░░░░
```

## Component Features

### ProgressIndicator

**Properties:**
- `CurrentStep`, `TotalSteps` - Step tracking
- `StepTitle` - Current step description
- `CurrentTool`, `CurrentToolArgs` - Tool being executed
- `StepStartTime`, `OverallStart` - Time tracking
- `Status` - Current status (Running, Paused, Complete, Canceled, Error)
- `Width` - Rendering width
- `ShowCancelHint` - Show "Press Ctrl+C to cancel"
- `Compact` - Use single-line compact mode

**Methods:**
- `StartStep(step, title)` - Begin new step
- `SetTool(name, args)` - Update current tool
- `Complete()`, `Cancel()`, `Pause()`, `Resume()`, `Error()` - Status updates
- `GetStepElapsed()`, `GetOverallElapsed()` - Time tracking
- `GetPercent()` - Progress percentage
- `Render()` - Render the indicator

### MultiStepProgress

**Properties:**
- `Steps` - Array of StepInfo
- `CurrentStepIdx` - Current step index
- `Indicator` - Embedded ProgressIndicator
- Callbacks: `OnStepComplete`, `OnAllComplete`, `OnError`

**Methods:**
- `NextStep()` - Advance to next step
- `SetToolForCurrentStep(tool, args)` - Set tool for current step
- `MarkError(err)` - Mark current step as errored
- `Cancel()` - Cancel the operation
- `Render()` - Render current state
- `GetSummary()` - Get summary of all steps

## Integration with Chat Model

The progress indicator is automatically rendered in the status area when active:

1. When a `ProgressStartMsg` is received, the indicator is created and shown
2. `ProgressStepMsg` updates the current step and tool information
3. `ProgressCompleteMsg` or `ProgressErrorMsg` hides the indicator
4. Ctrl+C during execution sends `ProgressCanceledMsg`

The layout automatically adjusts heights to accommodate the progress indicator without breaking the viewport.

## Cancellation Support

The progress indicator includes built-in cancellation support:
- Shows "Press Ctrl+C to cancel" hint when running
- Ctrl+C during operation sends `ProgressCanceledMsg`
- Status changes to "Canceled" with visual indication
- Operation can clean up and exit gracefully

## Color Coding

Progress indicator uses color to convey status:
- **Purple** - Running (default)
- **Amber** - Paused
- **Emerald** - Complete
- **Rose** - Error
- **Muted** - Canceled

The progress bar color also changes based on status for visual feedback.

## Time Formatting

Time is displayed in human-readable format:
- `<1s` - Shows milliseconds (e.g., "250ms")
- `<60s` - Shows seconds (e.g., "45s")
- `<60m` - Shows minutes:seconds (e.g., "2m 30s")
- `>=60m` - Shows hours:minutes (e.g., "1h 15m")

## Production-Ready Features

✅ **Unicode-safe** - Properly handles multi-byte characters
✅ **Responsive** - Adapts to terminal width
✅ **Accessible** - Clear status indicators beyond just color
✅ **Thread-safe** - Safe to update from goroutines via Bubble Tea messages
✅ **Cancellable** - Proper Ctrl+C handling
✅ **Comprehensive** - Tracks all necessary metrics (time, progress, tool info)
✅ **Flexible** - Supports both full and compact rendering modes
✅ **Well-documented** - Clear examples and usage patterns

## Testing

To test the progress indicator:

```go
// In your Update function
case tea.KeyMsg:
	if msg.String() == "p" {
		// Start a test multi-step operation
		return m, func() tea.Msg {
			return ProgressStartMsg{
				TotalSteps: 5,
				Title:      "Test Operation",
			}
		}
	}
```

## Future Enhancements

Potential improvements for future versions:
- [ ] Estimated time remaining based on average step duration
- [ ] Step-by-step replay/history view
- [ ] Parallel step execution tracking
- [ ] Progress persistence across sessions
- [ ] Customizable progress bar characters
- [ ] Sound/notification on completion
- [ ] Integration with logging system

## API Reference

See `C:\rigrun\go-tui\internal\ui\components\progress.go` for complete API documentation.

---

**Implementation Status:** ✅ Complete and ready for integration with agentic loop execution system.
