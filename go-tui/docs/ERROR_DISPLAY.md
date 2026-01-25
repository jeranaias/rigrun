# Enhanced Error Display System

## Overview

The enhanced error display system in rigrun Go TUI provides informative, actionable error messages that help users understand and resolve issues quickly. Errors are categorized, include context and suggestions, link to documentation, and show where to find detailed logs.

## Key Features

### 1. Error Categories

Errors are automatically categorized for better organization:

- **Network**: Connection issues, timeouts, rate limits
- **Model**: Model not found, incompatible versions
- **Tool**: Tool execution failures
- **Config**: Configuration and file access errors
- **Permission**: Authentication and access denied errors
- **Context**: Context window overflow, memory limits
- **Timeout**: Request timeouts and performance issues
- **Resource**: Disk space, GPU memory exhaustion
- **Parse**: JSON/data parsing errors
- **Error**: Uncategorized errors

### 2. Enhanced Display Elements

Each error can include:

- **Category**: Shown in the border title (e.g., "Network Error")
- **Title**: Brief description (e.g., "Ollama Connection Error")
- **Message**: The actual error message
- **Context**: When/where the error occurred (optional)
- **Suggestions**: Actionable steps to resolve the issue
- **Docs Link**: URL to relevant documentation (optional)
- **Logs Path**: Where to find detailed logs
- **Log Hint**: What to look for in the logs (optional)

### 3. Visual Design

```
┌─ Network Error ───────────────────────────────────────┐
│                                                        │
│ ✗ Cannot connect to Ollama                            │
│                                                        │
│ Cannot connect to Ollama service at localhost:11434   │
│                                                        │
│ Context:                                               │
│ While initializing chat with model 'llama3.2:7b'      │
│                                                        │
│ Suggestions:                                           │
│   • Start Ollama: ollama serve                        │
│   • Check if Ollama is installed: ollama --version    │
│   • Verify Ollama is running on localhost:11434       │
│                                                        │
│ 📖 Docs: https://rigrun.dev/docs/troubleshooting      │
│ 📋 Logs: ~/.rigrun/logs/rigrun.log                    │
│    → Look for 'connection refused' or 'dial tcp'      │
│                                                        │
│ [Enter] Dismiss    [c] Copy error    [d] Open docs    │
└───────────────────────────────────────────────────────┘
```

### 4. Color-Coded Borders

Borders change color based on error category:

- **Red**: Network, Model, Tool, Permission errors
- **Amber**: Config, Parse, Context, Resource errors
- **Blue**: Timeout errors

This provides visual cues about error severity while maintaining accessibility.

## Usage

### Creating Enhanced Errors

#### Automatic Pattern Matching

The simplest way to create enhanced errors is to use the automatic pattern matcher:

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/components"

// Let the pattern matcher detect the error type and provide suggestions
matcher := components.GetDefaultMatcher()
display := matcher.Match("Cannot connect to Ollama service")
if display != nil {
    // Error was matched and enhanced with suggestions, docs, etc.
    display.SetContext("While starting new conversation")
    display.Show()
}
```

#### Using SmartErrorMsg

For message-based errors in the chat system:

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/chat"

// Automatically detects patterns and provides suggestions
errMsg := chat.SmartErrorMsg("Connection Failed", err.Error())
```

#### Manual Creation

For custom errors with specific details:

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/components"

// Create a pattern
pattern := components.ErrorPattern{
    Keywords:    []string{"custom error", "specific issue"},
    Category:    components.CategoryTool,
    Title:       "Custom Tool Error",
    Suggestions: []string{
        "Check tool configuration",
        "Verify tool is installed",
        "Review tool output",
    },
    DocsURL:     "https://rigrun.dev/docs/tools/custom",
    LogHint:     "Look for tool execution logs and exit codes",
}

// Create the error
display := components.NewEnhancedErrorWithContext(
    pattern,
    "Tool execution failed with code 127",
    "While running custom_analyzer tool on file.txt",
)
display.Show()
```

### Handling User Actions

The error display supports interactive actions:

```go
display, cmd := errorDisplay.Update(msg)

switch msg := msg.(type) {
case components.ErrorCopyRequestMsg:
    // User pressed 'c' to copy error
    copyToClipboard(msg.Title + "\n" + msg.Message)

case components.ErrorDocsRequestMsg:
    // User pressed 'd' to open docs
    openBrowser(msg.URL)
}
```

### Adding Custom Patterns

You can extend the pattern matcher with application-specific patterns:

```go
matcher := components.GetDefaultMatcher()

// Add a custom pattern
matcher.AddPattern(components.ErrorPattern{
    Keywords:    []string{"my custom error"},
    Category:    components.CategoryConfig,
    Title:       "Custom Configuration Error",
    Suggestions: []string{
        "Check your custom.toml file",
        "Verify custom settings",
    },
    DocsURL:     "https://docs.example.com/custom",
    LogHint:     "Check configuration validation errors",
})

// Now the matcher will recognize this pattern
display := matcher.Match("my custom error occurred")
```

## Error Pattern Priority

Patterns are matched from **most specific to least specific**. When adding new patterns, consider:

1. **Specific patterns first**: "ollama is not running" before general "connection"
2. **Compound keywords**: Use multiple related keywords to avoid false positives
3. **Fallback patterns last**: General patterns like "connection refused" should be last

Example ordering:

```
1. "ollama is not running"           (very specific)
2. "ollama connection failed"        (service-specific)
3. "model 'xyz' not found"          (specific error type)
4. "connection refused"              (general - fallback)
```

## Best Practices

### 1. Always Provide Context

```go
display.SetContext("While loading conversation history")
```

Context helps users understand **when** and **why** the error occurred.

### 2. Keep Suggestions Actionable

Good suggestions:
- ✅ "Start Ollama: ollama serve"
- ✅ "Check configuration: cat ~/.rigrun/config.toml"
- ✅ "Free up disk space: df -h"

Bad suggestions:
- ❌ "Fix the problem"
- ❌ "Try again"
- ❌ "Contact support"

### 3. Link to Relevant Documentation

Only provide docs links for complex errors that need detailed explanation:

```go
pattern.DocsURL = "https://rigrun.dev/docs/troubleshooting/ollama-connection"
```

### 4. Provide Specific Log Hints

Tell users **what to look for** in logs:

```go
pattern.LogHint = "Look for 'connection refused' or network timeout messages"
```

Not just "check the logs".

### 5. Test Your Patterns

Verify that patterns match the intended errors:

```go
func TestMyErrorPattern(t *testing.T) {
    matcher := components.NewErrorPatternMatcher()
    display := matcher.Match("my specific error message")

    if display == nil {
        t.Fatal("Pattern should match")
    }
    if display.category != components.CategoryNetwork {
        t.Error("Wrong category")
    }
}
```

## Accessibility

The error display system is designed with accessibility in mind:

1. **High Contrast Colors**: Uses high-contrast color schemes for colorblind users
2. **Icons with Text**: Error icons (✗) supplement color coding
3. **Clear Hierarchy**: Information is structured logically
4. **Keyboard Navigation**: All actions accessible via keyboard
5. **Screen Reader Friendly**: Text-based display works with screen readers

## Examples

See `error_examples.go` for complete examples of each error category:

- Network errors (Ollama connection)
- Model errors (model not found)
- Context errors (token limit exceeded)
- Timeout errors (request timeout)
- Permission errors (access denied)
- Resource errors (GPU memory)
- Config errors (invalid configuration)
- Tool errors (execution failures)
- Rate limit errors (API limits)

## Future Enhancements

Potential future improvements:

1. **Error History**: Track and review past errors
2. **Quick Actions**: One-click fixes for common errors
3. **Error Analytics**: Track error frequency and patterns
4. **Contextual Help**: AI-powered error suggestions
5. **Error Templates**: User-defined error responses

## Contributing

When adding new error patterns:

1. Identify the error category
2. Create specific keywords that uniquely identify the error
3. Provide 2-4 actionable suggestions
4. Add documentation link if the error needs detailed explanation
5. Specify what to look for in logs
6. Test the pattern with real error messages
7. Ensure it doesn't conflict with existing patterns

## References

- `error.go`: Main error display component
- `error_patterns.go`: Pattern matching system
- `error_examples.go`: Example implementations
- `error_enhanced_test.go`: Test suite

## Logs Location

Logs are stored at:

- **Windows**: `%USERPROFILE%\.rigrun\logs\rigrun.log`
- **macOS/Linux**: `~/.rigrun/logs/rigrun.log`

The error display automatically shows the correct path for the user's platform.
