# Context Cost Display - Visual Examples

## Status Bar Display Examples

### Example 1: Small Context (No Warning)
```
Input: "Explain this code @file main.go"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5-coder:14b | ~ AUTO | Ctx: [####------] +1.2k tok  ^R=mode | ^C=stop │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Cyan/blue color for token count
- No warning prefix
- No cost estimate (local routing)

### Example 2: Medium Context with Cloud Routing
```
Input: "Review @file server.go @file client.go"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5-coder | * CLOUD | Ctx: [######----] +4.5k tok ~$0.03  ^R | ^C       │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Cyan color for token count
- Shows cost estimate in amber (~$0.03)
- Compact display for narrow terminal

### Example 3: Large Context (Yellow Warning)
```
Input: "Analyze @file large_file.py @git HEAD~5..HEAD"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5 | * CLOUD | [########--] ! +52k tok ~$1.15  ^R=mode | ^C=stop       │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Yellow/amber color for token count
- Warning prefix "! " before token count
- Shows significant cost estimate
- Model name truncated to save space

### Example 4: Very Large Context (Red Warning)
```
Input: "@codebase @file huge.json"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen | * CLOUD | [##########] ! +125k tok ~$3.50  ^C                        │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Red/rose color for token count
- Warning prefix "! " before token count
- High cost estimate
- Ultra-compact display (model name heavily truncated)
- Context bar shows full (100%)

### Example 5: Progressive Disclosure - Wide Terminal
```
Terminal Width: 120 columns
Input: "@file service.ts @file types.ts"
Status Bar:
┌──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5-coder:14b | ~ AUTO | Cloud: enabled | 1,234 tok | $0.45 | Saved: $12.30 | Ctx: [####------] +3.2k tok ~$0.05  ^R=mode | ^C=stop │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```
- Full display with all information
- Session stats shown (total tokens, cost, savings)
- Context cost info shown in right section
- Full keyboard shortcuts

### Example 6: Progressive Disclosure - Narrow Terminal
```
Terminal Width: 60 columns
Input: "@file main.go"
Status Bar:
┌────────────────────────────────────────────────────────┐
│ qwen | ~ AUTO | [####--] +1.2k  ^C                     │
└────────────────────────────────────────────────────────┘
```
- Minimal display for narrow terminal
- Context cost shown without cost estimate (no space)
- Shortcuts reduced to bare minimum
- Model name truncated heavily

### Example 7: No @mentions
```
Input: "What is Go?"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5-coder:14b | ~ AUTO | Ctx: [----------]  ^R=mode | ^C=stop           │
└─────────────────────────────────────────────────────────────────────────────┘
```
- No context cost info shown (no @mentions detected)
- Empty context bar
- Normal status bar display

### Example 8: Multiple Large Files
```
Input: "@file src/app.ts @file src/utils.ts @file src/components/header.tsx"
Status Bar:
┌─────────────────────────────────────────────────────────────────────────────┐
│ qwen2.5-coder | * CLOUD | [######----] +8.7k tok ~$0.15  ^R=mode | ^C       │
└─────────────────────────────────────────────────────────────────────────────┘
```
- Cumulative token count for all @mentions
- Cost reflects total context size
- Cyan color (below warning threshold)

## Color Scheme Reference

### Token Count Colors
- **Cyan (< 50k tokens)**: Normal context size, no concern
- **Amber (50k-99k tokens)**: Warning - large context, may hit limits
- **Rose (≥ 100k tokens)**: Critical - very large context, approaching model limits

### Cost Colors
- **Amber**: All cost estimates shown in amber/yellow
- Helps distinguish cost from token count
- Consistent with "money" theme

### Warning Indicators
- **"! " prefix**: Added before token count when ≥ 50k tokens
- **Bold text**: Token count always bold for emphasis
- **High contrast**: Uses accessibility-friendly colors

## Responsive Behavior

### Terminal Width Breakpoints
1. **≥ 100 columns (Wide)**: Full display with all info
2. **60-99 columns (Medium)**: Progressive hiding of session stats, then context cost
3. **< 60 columns (Narrow)**: Minimal display, context cost hidden

### Progressive Hiding Order
When terminal gets narrower, elements are hidden in this order:
1. Session savings display
2. Session cost display
3. Session token count
4. Full mode text (fallback to icon)
5. **Context cost estimate** (keeps token count)
6. Full context bar (fallback to compact)
7. Full shortcuts (fallback to minimal)
8. Model name truncation (progressively more aggressive)

## Format Variations

### Token Count Formats
| Range | Format | Example |
|-------|--------|---------|
| < 1,000 | +NNN tok | +125 tok |
| 1,000 - 9,999 | +N.Nk tok | +2.5k tok |
| ≥ 10,000 | +NNk tok | +15k tok |

### Cost Formats
| Range | Format | Example |
|-------|--------|---------|
| < 1¢ | N.NNc | 0.05c |
| 1-99¢ | N.Nc | 5.2c |
| ≥ $1 | $N.NN | $1.23 |

### Warning Prefixes
| Threshold | Prefix | Color |
|-----------|--------|-------|
| < 50k tokens | (none) | Cyan |
| 50k-99k tokens | "! " | Amber |
| ≥ 100k tokens | "! " | Rose |

## User Experience Flow

### Typing Flow
1. User starts typing message: `"Check @f"`
   - No estimate shown (incomplete @mention)

2. User completes @mention: `"Check @file main.go"`
   - Estimate appears: `+1.2k tok`
   - Updates on every keystroke as user continues typing

3. User adds second file: `"Check @file main.go @file utils.go"`
   - Estimate updates: `+3.5k tok`
   - Shows cumulative total

4. User changes routing mode (Ctrl+R): Cloud mode selected
   - Cost estimate appears: `+3.5k tok ~$0.06`

5. User removes an @mention: `"Check @file main.go"`
   - Estimate updates: `+1.2k tok`
   - Cost remains visible

6. User removes all @mentions: `"Check the code"`
   - Estimate disappears
   - Status bar returns to normal

### Warning Flow
1. User adds large file: `"@file huge_dataset.json"`
   - Shows: `! +75k tok` (amber warning)
   - User is aware of large context before sending

2. User decides to proceed anyway: Presses Enter
   - Message sent with full context
   - Cost tracked in session stats

3. Alternative: User removes large file
   - Warning disappears
   - Uses smaller context instead

## Implementation Notes

### Performance
- Estimates update on every keystroke (real-time)
- Fast @mention detection using string search
- Token estimation reuses existing expansion logic
- No blocking or lag in typing experience

### Accuracy
- Token count: ±10% accuracy (uses 4 chars/token heuristic)
- Cost estimate: Based on estimated tokens and tier pricing
- More accurate than no estimate, less accurate than actual API usage
- Good enough for user awareness and decision-making

### Accessibility
- High contrast colors (follows WCAG guidelines)
- Bold text for emphasis
- Symbol prefix ("! ") in addition to color
- Progressive disclosure maintains readability
