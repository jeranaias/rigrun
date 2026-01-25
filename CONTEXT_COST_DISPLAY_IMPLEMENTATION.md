# Context Cost Display Implementation

## Overview
This document describes the implementation of the real-time context cost display feature for the rigrun Go TUI. This feature shows users token count estimates and cloud cost estimates before they send messages with @mentions.

## Requirements Implemented

### 1. Dynamic Token Count Display ✅
- Shows context token count in status bar when user has @mentions in input
- Updates dynamically as user types
- Format examples:
  - `+2.5k tok` (for 2,500 tokens)
  - `+125 tok` (for small contexts)
  - `+15k tok` (for large contexts)

### 2. Warning System ✅
- **Yellow warning (50k+ tokens)**: Shows "! " prefix with amber color
- **Red warning (100k+ tokens)**: Shows "! " prefix with rose color
- Color-coded warnings are accessible (uses high contrast colors and prefix symbols)

### 3. Cloud Cost Estimate ✅
- Shows estimated cost based on token count and model tier
- Format: `~$0.02` or `~0.5c` (adaptive formatting)
- Only displayed when cost is meaningful (> 0.001 cents)
- Uses current routing tier or defaults to Auto tier

### 4. Status Bar Integration ✅
- Displays info in status bar area (non-blocking)
- Responsive design that adapts to terminal width
- Progressive disclosure: shown when space available, hidden when terminal is narrow
- Positioned in right section after context bar, before shortcuts

## Files Modified

### 1. `internal/ui/chat/model.go`
**Changes:**
- Added two new fields to `Model` struct:
  - `contextTokenEstimate int`: Estimated tokens for current @mentions
  - `contextCostEstimate float64`: Estimated cost in cents

- Added `updateContextCostEstimate()` method:
  - Called on every keystroke when input changes
  - Quick check for @mentions using `ctxmention.HasMentions()`
  - Uses `contextExpander.EstimateContextSize()` for accurate token count
  - Estimates cost using `router.EstimateCost()` based on routing tier

- Added getter methods:
  - `GetContextTokenEstimate() int`
  - `GetContextCostEstimate() float64`

- Integrated into Update handler:
  - Calls `updateContextCostEstimate()` after text input updates

**Location:** Lines 117-119 (struct fields), Lines 1621-1670 (methods), Line 615 (Update integration)

### 2. `internal/ui/chat/view.go`
**Changes:**
- Added `renderContextCostInfo()` method:
  - Renders context cost information when @mentions detected
  - Color-coded by token count (cyan → amber → rose)
  - Shows warning prefix ("! ") for high token counts
  - Adaptive token formatting (125 tok, 2.5k tok, 15k tok)
  - Appends cost estimate when meaningful

- Modified `renderStatusBar()` method:
  - Added `contextCostInfo` variable to collect cost display
  - Updated `buildStatusBar()` function signature to include `showContextCost` parameter
  - Updated `statusConfig` struct with `showContextCost bool` field
  - Modified configurations to progressively show/hide context cost based on terminal width
  - Added `hasContextMentions` check to only show when relevant

- Progressive disclosure logic:
  - Full config: Shows context cost when @mentions present
  - Medium config: Shows context cost if space available
  - Compact config: Hides context cost to save space
  - Right section construction: `contextBar + contextCostInfo + shortcuts`

**Location:** Lines 1052 (variable), Lines 1057-1127 (buildStatusBar update), Lines 1133-1174 (statusConfig update), Lines 1444-1499 (renderContextCostInfo method)

## Technical Details

### Performance Optimization
The implementation is designed for fast, real-time updates:

1. **Quick @mention detection**: Uses `ctxmention.HasMentions()` for fast string checking
2. **No-op when no @mentions**: Returns immediately if input has no @mentions
3. **Reuses existing infrastructure**: Leverages `contextExpander.EstimateContextSize()` which is already optimized

### Token Estimation
Uses the existing `EstimateContextSize()` method from `internal/context/expander.go`:
- Expands @mentions to get actual content size
- Calculates tokens using rough approximation: `(len(expandedMessage) + 3) / 4`
- Accurate enough for cost estimation purposes

### Cost Estimation
Uses the existing `router.EstimateCost()` function from `internal/router/cost.go`:
- Takes input token count and tier
- Assumes 3:1 output:input ratio (typical for queries)
- Uses tier-specific pricing per 1K tokens
- Returns total estimated cost in cents

### Routing Tier Selection
For cost estimation, the implementation:
1. Uses `lastRouting.Tier` if available (from previous routing decision)
2. Falls back to `router.TierAuto` for new sessions
3. Updates dynamically as routing mode changes

### Warning Thresholds
- **50,000 tokens**: Yellow/amber warning (Claude API context warning threshold)
- **100,000 tokens**: Red/rose warning (approaching typical model limits)

### Display Format
Token count formatting is adaptive:
- `< 1,000 tokens`: Shows exact count (e.g., "+125 tok")
- `1,000 - 9,999 tokens`: Shows with 1 decimal place (e.g., "+2.5k tok")
- `≥ 10,000 tokens`: Shows rounded to nearest thousand (e.g., "+15k tok")

Cost formatting uses existing `formatCost()` function:
- `< 1 cent`: Shows with 2 decimal places (e.g., "0.05c")
- `1-99 cents`: Shows with 1 decimal place (e.g., "5.2c")
- `≥ 100 cents`: Converts to dollars (e.g., "$1.23")

## Integration with Existing Features

### Context Expansion System
- Integrates with `internal/context/expander.go`
- Uses `EstimateContextSize()` for token counting
- Respects @mention types (file, git, codebase, error, clipboard)

### Router System
- Integrates with `internal/router/cost.go`
- Uses `EstimateCost()` for cost calculation
- Uses tier-specific pricing from `GetTierPricing()`
- Respects routing mode (local/cloud/auto)

### Status Bar System
- Integrates with existing status bar responsive design
- Follows progressive disclosure pattern
- Uses same color scheme and styling conventions
- Adapts to terminal width constraints

## Testing Checklist

### Manual Testing
- [ ] Type `@file` mention - should show token estimate
- [ ] Type multiple @mentions - should show cumulative estimate
- [ ] Remove @mentions - estimate should disappear
- [ ] Test with 50k+ tokens - should show yellow warning
- [ ] Test with 100k+ tokens - should show red warning
- [ ] Test in cloud mode - should show cost estimate
- [ ] Test in local mode - should show token count only (no cost)
- [ ] Resize terminal - context cost should hide/show appropriately
- [ ] Test with very narrow terminal - should not break layout

### Edge Cases
- [ ] Empty input - should show nothing
- [ ] Invalid @mention - should handle gracefully
- [ ] Very large files (> 1MB) - should estimate correctly
- [ ] Multiple large contexts - should warn appropriately

## Future Enhancements

### Potential Improvements
1. **Lightweight estimation mode**: Add fast estimation without full expansion for better performance on large files
2. **Cached estimates**: Cache token counts for frequently referenced files
3. **Breakdown display**: Show token breakdown by mention type (e.g., "3 files: 2.5k tok")
4. **Smart throttling**: Only update estimate every 500ms instead of every keystroke
5. **Model-specific limits**: Show warnings based on specific model context limits
6. **Cost breakdown**: Show separate input/output cost estimates

### Performance Optimization
If performance becomes an issue with large contexts:
1. Debounce estimation updates (300-500ms delay)
2. Add lightweight pre-check before full expansion
3. Cache expansion results for unchanged @mentions
4. Show "Calculating..." during slow expansions

## Dependencies
- `internal/context` - Context expansion and @mention detection
- `internal/router` - Cost estimation and tier pricing
- `github.com/charmbracelet/lipgloss` - Terminal styling

## Related Files
- `internal/context/expander.go` - Token estimation logic
- `internal/router/cost.go` - Cost calculation logic
- `internal/router/types.go` - Tier definitions and pricing
- `internal/ui/chat/utils.go` - Formatting utilities (formatCost)
- `internal/ui/styles/colors.go` - Color scheme definitions
