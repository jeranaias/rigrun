# Error Display Enhancements - Changelog

## Version: Enhanced Error System v2.0

**Date**: 2026-01-24

### Summary

Complete overhaul of the error display system to make errors more informative, actionable, and user-friendly. Errors now include categories, context, suggestions, documentation links, and log paths.

---

## New Features

### 1. Error Categorization System

Added 10 error categories for better organization:

- `CategoryNetwork`: Connection issues, timeouts, rate limits
- `CategoryModel`: Model-related errors
- `CategoryTool`: Tool execution failures
- `CategoryConfig`: Configuration errors
- `CategoryPermission`: Authentication/access errors
- `CategoryContext`: Context window overflow
- `CategoryTimeout`: Request timeouts
- `CategoryResource`: Resource exhaustion (disk, GPU)
- `CategoryParse`: Parsing errors
- `CategoryUnknown`: Fallback category

**Impact**: Users can quickly understand the type of error at a glance.

### 2. Enhanced Error Pattern Structure

Extended `ErrorPattern` with new fields:

```go
type ErrorPattern struct {
    Keywords    []string
    Category    ErrorCategory  // NEW
    Title       string
    Suggestions []string
    DocsURL     string         // NEW
    LogHint     string         // NEW
}
```

**Impact**: Provides comprehensive error information in a structured format.

### 3. Context Support

Added optional context field to show when/where errors occurred:

```go
display.SetContext("While initializing chat with model 'llama3.2:7b'")
```

**Example Output**:
```
Context:
While initializing chat with model 'llama3.2:7b'
```

**Impact**: Users understand the circumstances that triggered the error.

### 4. Documentation Links

Errors can now link to relevant documentation:

```go
DocsURL: "https://rigrun.dev/docs/troubleshooting/ollama-connection"
```

**Display**: 📖 Docs: https://rigrun.dev/docs/troubleshooting

**Action**: Press `[d]` to open docs in browser

**Impact**: Users get detailed help for complex errors.

### 5. Logs Path and Hints

Errors now show where to find detailed logs and what to look for:

```go
LogHint: "Look for 'connection refused' or 'dial tcp' errors"
```

**Display**:
```
📋 Logs: ~/.rigrun/logs/rigrun.log
   → Look for 'connection refused' or 'dial tcp' errors
```

**Impact**: Users can quickly locate and understand log files.

### 6. Interactive Error Actions

New keyboard shortcuts:

- `[Enter]` - Dismiss error
- `[c]` - Copy error details to clipboard
- `[d]` - Open documentation in browser

**Impact**: Users can take immediate action on errors.

### 7. Color-Coded Borders

Border colors indicate error severity:

- **Red**: Critical errors (Network, Model, Tool, Permission)
- **Amber**: Warning-level errors (Config, Parse, Resource)
- **Blue**: Informational (Timeout)

**Impact**: Visual cues about error importance while maintaining accessibility.

### 8. Category Labels in Border

Error category is displayed in the border title:

```
┌─ Network Error ──────────────┐
```

**Impact**: Immediate identification of error type.

---

## Enhanced Error Patterns

### Updated Patterns

All existing patterns now include:
- Error category
- Documentation URL
- Log hints

### New Patterns

Added pattern for tool errors:

```go
ErrorPattern{
    Keywords:    []string{"tool failed", "tool execution", "tool error"},
    Category:    CategoryTool,
    Title:       "Tool Error",
    Suggestions: []string{
        "Check tool availability and permissions",
        "Verify tool arguments are correct",
        "Review tool output for specific errors",
    },
    DocsURL:     "https://rigrun.dev/docs/tools",
    LogHint:     "Check tool execution logs and return codes",
}
```

### Pattern Coverage

Now covers 15+ error scenarios:
1. Ollama not running
2. Ollama connection errors
3. Model not found
4. Context exceeded
5. Request timeout
6. Rate limiting
7. Permission denied
8. File not found
9. GPU/CUDA errors
10. Disk space errors
11. Configuration errors
12. JSON/Parse errors
13. Tool execution errors
14. General network errors
15. General connection errors

---

## API Changes

### New Functions

#### `NewEnhancedError`
Creates error with full pattern details:

```go
func NewEnhancedError(pattern ErrorPattern, message string) ErrorDisplay
```

#### `NewEnhancedErrorWithContext`
Creates error with additional context:

```go
func NewEnhancedErrorWithContext(
    pattern ErrorPattern,
    message string,
    context string,
) ErrorDisplay
```

#### `getLogsPath`
Returns platform-specific logs path:

```go
func getLogsPath() string
// Windows: %USERPROFILE%\.rigrun\logs\rigrun.log
// Unix: ~/.rigrun/logs/rigrun.log
```

### New Setters

- `SetCategory(ErrorCategory)` - Set error category
- `SetContext(string)` - Set contextual information
- `SetDocsURL(string)` - Set documentation link
- `SetLogHint(string)` - Set log search hint

### New Message Types

#### `ErrorCopyRequestMsg`
Sent when user presses `[c]` to copy error:

```go
type ErrorCopyRequestMsg struct {
    Title   string
    Message string
    Context string
}
```

#### `ErrorDocsRequestMsg`
Sent when user presses `[d]` to open docs:

```go
type ErrorDocsRequestMsg struct {
    URL string
}
```

---

## Modified Files

### Core Files

1. **`error_patterns.go`**
   - Added error categories (10 constants)
   - Extended `ErrorPattern` structure
   - Updated all 13+ patterns with categories, docs, and log hints
   - Added tool error pattern
   - Updated `Match()` to return enhanced errors

2. **`error.go`**
   - Extended `ErrorDisplay` structure with 6 new fields
   - Added `NewEnhancedError()` and `NewEnhancedErrorWithContext()`
   - Added 4 new setter methods
   - Completely rewrote `viewBox()` for enhanced display
   - Added keyboard handlers for copy (`c`) and docs (`d`)
   - Added 2 new message types
   - Added `getLogsPath()` helper
   - Added `addTitleToBox()` helper

3. **`update.go`**
   - Already using `SmartErrorMsg()` - no changes needed
   - System automatically benefits from enhancements

### New Files

1. **`error_enhanced_test.go`**
   - 10+ comprehensive tests
   - Tests for categories, patterns, setters, rendering
   - Pattern priority tests
   - Visual rendering verification

2. **`error_examples.go`**
   - 9 example error scenarios
   - Demonstrates all error categories
   - Can be used for testing and demos

3. **`docs/ERROR_DISPLAY.md`**
   - Complete documentation
   - Usage examples
   - Best practices
   - API reference

4. **`docs/CHANGELOG_ERROR_ENHANCEMENTS.md`**
   - This file

---

## Migration Guide

### For Existing Code

#### Before
```go
display := components.NewError("Connection Failed", errMsg)
```

#### After (Automatic Enhancement)
```go
matcher := components.GetDefaultMatcher()
display := matcher.Match(errMsg)
if display != nil {
    display.SetContext("While connecting to Ollama")
}
```

### Backward Compatibility

All existing error creation functions still work:

- `NewError(title, message)` ✅
- `NewErrorWithSuggestions(title, message, suggestions)` ✅
- `NewToastError(message)` ✅
- `SmartError(title, message)` ✅

They now include:
- Default category (`CategoryUnknown`)
- Automatic logs path
- Copy functionality
- Enhanced display

### Adding Custom Patterns

```go
matcher := components.GetDefaultMatcher()

matcher.AddPattern(components.ErrorPattern{
    Keywords:    []string{"my custom error"},
    Category:    components.CategoryConfig,
    Title:       "My Custom Error",
    Suggestions: []string{"Fix it", "Try this"},
    DocsURL:     "https://example.com/docs",
    LogHint:     "Look for validation errors",
})
```

---

## Performance Impact

### Minimal Overhead

- Pattern matching: O(n) where n = number of patterns (~15)
- Patterns checked in order (most specific first)
- Thread-safe with RWMutex
- Singleton pattern matcher (no repeated initialization)

### Memory Impact

- Additional 4 fields per error (~64 bytes)
- Logs path cached on creation
- No impact on non-error paths

---

## Testing

### Test Coverage

- ✅ Error categories defined and valid
- ✅ Pattern matching works correctly
- ✅ Enhanced error creation
- ✅ Context addition
- ✅ Logs path generation
- ✅ Setter methods
- ✅ Pattern priority (specific before general)
- ✅ Visual rendering (contains expected elements)

### Manual Testing Scenarios

1. Start TUI with Ollama stopped → See enhanced connection error
2. Try non-existent model → See model not found with suggestions
3. Fill context window → See context exceeded with action hints
4. Press `[c]` on error → Copy to clipboard
5. Press `[d]` on error → Open docs (if URL provided)

---

## Documentation

### New Documentation

1. **ERROR_DISPLAY.md**: Complete user and developer guide
2. **CHANGELOG_ERROR_ENHANCEMENTS.md**: This document
3. Inline code comments for all new functions

### Updated Documentation

- None required - system is backward compatible

---

## Future Enhancements

### Planned

1. **Error History**: View past errors with `/errors` command
2. **Quick Actions**: One-click fixes (e.g., "Start Ollama" button)
3. **Error Search**: Search errors by category or keyword
4. **Error Export**: Export errors to file for bug reports

### Potential

1. **AI Error Suggestions**: Use LLM to provide context-specific help
2. **Error Templates**: User-defined error responses
3. **Error Analytics**: Track error patterns over time
4. **Telemetry**: Optional error reporting (with user consent)

---

## Breaking Changes

**None** - All changes are backward compatible.

---

## Credits

Developed as part of the rigrun Go TUI enhancement initiative.

### Design Goals

1. ✅ Make errors informative, not scary
2. ✅ Provide actionable suggestions
3. ✅ Link to documentation for complex issues
4. ✅ Show where to find logs for debugging
5. ✅ Categorize for better organization
6. ✅ Maintain accessibility (high contrast, icons)
7. ✅ Preserve backward compatibility

---

## Examples

### Before Enhancement
```
┌─ Error ──────────────────────┐
│                               │
│ ✗ Connection Error            │
│                               │
│ connection refused            │
│                               │
│ Press Esc to dismiss          │
└───────────────────────────────┘
```

### After Enhancement
```
┌─ Network Error ────────────────────────────────────────┐
│                                                         │
│ ✗ Ollama Connection Error                              │
│                                                         │
│ Cannot connect to Ollama service at localhost:11434    │
│                                                         │
│ Context:                                                │
│ While initializing chat with model 'llama3.2:7b'       │
│                                                         │
│ Suggestions:                                            │
│   • Start Ollama: ollama serve                         │
│   • Check if Ollama is installed: ollama --version     │
│   • Verify Ollama is running on localhost:11434        │
│                                                         │
│ 📖 Docs: https://rigrun.dev/docs/troubleshooting       │
│ 📋 Logs: ~/.rigrun/logs/rigrun.log                     │
│    → Look for 'connection refused' or 'dial tcp' errors│
│                                                         │
│ [Enter] Dismiss    [c] Copy error    [d] Open docs     │
└────────────────────────────────────────────────────────┘
```

**Impact**: Users immediately understand:
- What went wrong (connection error)
- Why it happened (Ollama not running)
- When it happened (during model initialization)
- How to fix it (3 actionable steps)
- Where to get more help (docs + logs)

---

## Conclusion

The enhanced error display system transforms error messages from frustrating roadblocks into helpful guides. By providing context, suggestions, documentation links, and log paths, users can quickly understand and resolve issues independently.

The system maintains backward compatibility while adding powerful new features, ensuring a smooth transition for existing code and an improved experience for all users.
