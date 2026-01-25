# Error Pattern Matching System

## Overview

The Error Pattern Matching system provides intelligent error handling with context-aware suggestions for the rigrun Go TUI. Instead of showing generic errors, the system analyzes error messages and provides actionable suggestions to help users resolve issues quickly.

## Features

- **Pattern-based Error Detection**: Automatically detects error types from error message strings
- **Smart Suggestions**: Provides up to 3 actionable suggestions per error
- **Platform-Specific Advice**: Adapts suggestions based on the operating system (Windows, macOS, Linux)
- **Extensible Design**: Easy to add new error patterns without modifying existing code
- **Zero Dependencies**: Uses only standard library and existing project dependencies

## Architecture

### Components

1. **ErrorPattern** (`error_patterns.go`)
   - Defines keywords to match against error messages
   - Specifies the error title and suggestions
   - Supports case-insensitive keyword matching

2. **ErrorPatternMatcher** (`error_patterns.go`)
   - Maintains a registry of error patterns
   - Matches error strings against patterns
   - Returns enhanced `ErrorDisplay` with suggestions

3. **SmartErrorMsg** (`messages.go`)
   - Convenience function for creating error messages
   - Automatically applies pattern matching
   - Falls back to generic error if no pattern matches

## Usage

### Basic Usage

```go
// Instead of:
errMsg := NewErrorMsg("Error", err.Error())

// Use:
errMsg := SmartErrorMsg("Error", err.Error())
```

The smart error function will automatically:
1. Analyze the error message
2. Match it against known patterns
3. Add relevant suggestions
4. Return an enhanced error message

### Adding Custom Patterns

You can extend the pattern matcher with application-specific patterns:

```go
matcher := NewErrorPatternMatcher()

// Add a custom pattern
matcher.AddPattern(ErrorPattern{
    Keywords: []string{"custom error", "special case"},
    Title: "Custom Error",
    Suggestions: []string{
        "Do this first",
        "Then try this",
        "Finally, do this",
    },
})

// Use the matcher
display := matcher.Match(errorMessage)
```

### Using the Components Package Directly

For advanced use cases, you can use the pattern matcher from the components package:

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/components"

// Create error with pattern matching
display := components.SmartError("Title", "error message here")

// Or create from a Go error
display := components.SmartErrorFromError("Title", err)
```

## Supported Error Patterns

### Network & Connection Errors
**Keywords**: connection refused, dial tcp, no such host, timeout
**Suggestions**:
- Check your network connection
- Verify the service is running
- Try using offline mode if available

### Ollama Connection Errors
**Keywords**: ollama, localhost:11434
**Suggestions**:
- Start Ollama: ollama serve
- Check if Ollama is installed: ollama --version
- Verify Ollama is running on localhost:11434

### Model Not Found
**Keywords**: model not found, model does not exist, 404
**Suggestions**:
- List available models: ollama list
- Pull the model: ollama pull <model-name>
- Check model name spelling

### Permission Denied
**Keywords**: permission denied, unauthorized, forbidden, 403
**Suggestions** (Platform-specific):
- **Windows**: Check file permissions in Properties > Security
- **macOS**: Check file permissions: ls -l <file>
- **Linux**: Grant permissions: chmod +r <file>

### Context Exceeded
**Keywords**: context exceeded, too long, maximum context
**Suggestions**:
- Start new conversation: /new
- Clear history: /clear
- Use shorter messages

### Request Timeout
**Keywords**: timeout, timed out, deadline exceeded
**Suggestions**:
- Try again
- Use a smaller model
- Check server load

### Rate Limit Exceeded
**Keywords**: rate limit, too many requests, 429
**Suggestions**:
- Wait a moment and retry
- Switch to a local model
- Check your API quota

### File Not Found
**Keywords**: file not found, no such file, path not found
**Suggestions**:
- Check the file path spelling
- Use an absolute path instead of relative
- Verify the file exists

### GPU Errors
**Keywords**: cuda, gpu, vram, out of gpu memory
**Suggestions**:
- Try a smaller model that fits in GPU memory
- Use CPU mode if GPU is unavailable
- Check GPU drivers and CUDA installation

### Configuration Errors
**Keywords**: config, configuration, invalid config
**Suggestions**:
- Check configuration file syntax
- Verify all required fields are present
- Use default configuration: /reset

### Parse Errors
**Keywords**: json, unmarshal, parse error
**Suggestions**:
- Check for malformed JSON or data
- Verify the format matches expectations
- Try again with simpler input

### Disk Space Errors
**Keywords**: no space left, disk full, enospc
**Suggestions**:
- Free up disk space on your system
- Remove unused models: ollama rm <model>
- Clear temporary files and caches

## Implementation Details

### Pattern Matching Algorithm

1. **Error Message Normalization**: Convert to lowercase for case-insensitive matching
2. **Keyword Scanning**: Check if any pattern's keywords appear in the error message
3. **First Match Wins**: Patterns are checked in order, first match is returned
4. **Suggestion Limiting**: Automatically limits to 3 suggestions maximum

### Platform Detection

The system uses `runtime.GOOS` to detect the operating system and provide platform-specific suggestions for:
- Permission errors
- Ollama startup instructions
- File path conventions

### Performance Considerations

- **Lazy Initialization**: Matcher is created on-demand
- **Linear Search**: Fast enough for ~10-15 patterns (typical use case)
- **No Regex**: Uses simple string matching for speed
- **Reusable Matcher**: Can create once and reuse for multiple errors

## Testing

The system includes comprehensive tests in `error_patterns_test.go`:

```bash
cd internal/ui/components
go test -v -run TestErrorPatternMatcher
```

Test coverage includes:
- All default error patterns
- Custom pattern addition
- Case-insensitive matching
- Suggestion limiting
- Platform-specific suggestions
- MatchOrDefault behavior

## Examples

### Example 1: Connection Error

```go
// Error message
err := errors.New("dial tcp 127.0.0.1:11434: connect: connection refused")

// Create smart error
errMsg := SmartErrorMsg("Connection Failed", err.Error())

// Results in ErrorDisplay with:
// Title: "Connection Error"
// Message: "dial tcp 127.0.0.1:11434: connect: connection refused"
// Suggestions:
//   - Check your network connection
//   - Verify the service is running
//   - Try using offline mode if available
```

### Example 2: Model Not Found

```go
// Error message
err := errors.New("model 'llama2:70b' not found")

// Create smart error
errMsg := SmartErrorMsg("Model Error", err.Error())

// Results in ErrorDisplay with:
// Title: "Model Not Found"
// Message: "model 'llama2:70b' not found"
// Suggestions:
//   - List available models: ollama list
//   - Pull the model: ollama pull <model-name>
//   - Check model name spelling
```

### Example 3: Custom Pattern

```go
// Create matcher with custom pattern
matcher := NewErrorPatternMatcher()
matcher.AddPattern(ErrorPattern{
    Keywords: []string{"database", "connection pool"},
    Title: "Database Connection Error",
    Suggestions: []string{
        "Check database connection string",
        "Verify database is running",
        "Increase connection pool size",
    },
})

// Match error
err := errors.New("database connection pool exhausted")
display := matcher.Match(err.Error())

// Results in custom error with appropriate suggestions
```

## Integration Points

### Current Integrations

1. **Chat Model** (`internal/ui/chat/model.go`)
   - `handleStreamError()`: Uses SmartErrorMsg for streaming errors
   - `handleOllamaStatus()`: Enhanced Ollama connection errors
   - `handleOllamaModels()`: Better model listing errors
   - `handleModelSwitched()`: Improved model switching errors

2. **Update Handlers** (`internal/ui/chat/update.go`)
   - `HandleOllamaError()`: Centralized error handling with smart suggestions

3. **Error Messages** (`internal/ui/chat/messages.go`)
   - `SmartErrorMsg()`: Primary function for creating enhanced errors
   - `detectErrorSuggestions()`: Inline pattern matching (avoids circular deps)

## Future Enhancements

### Potential Improvements

1. **Regex Support**: For more complex pattern matching
2. **Error History**: Track common errors and suggest permanent fixes
3. **Interactive Fixes**: Automatically execute suggested commands
4. **Localization**: Multi-language error messages and suggestions
5. **Telemetry**: Track which errors occur most frequently
6. **Context-Aware**: Suggestions based on current application state

### Adding New Patterns

To add new error patterns, edit `error_patterns.go`:

```go
// In registerDefaultPatterns()
m.AddPattern(ErrorPattern{
    Keywords: []string{"your", "keywords", "here"},
    Title: "Your Error Title",
    Suggestions: []string{
        "Suggestion 1",
        "Suggestion 2",
        "Suggestion 3",
    },
})
```

## Best Practices

1. **Use SmartErrorMsg by Default**: Replace all `NewErrorMsg` calls with `SmartErrorMsg`
2. **Keep Suggestions Actionable**: Focus on what users can DO, not what went wrong
3. **Limit to 3 Suggestions**: Too many options overwhelm users
4. **Order by Likelihood**: Put most common fixes first
5. **Include Commands**: When possible, provide exact commands to run
6. **Be Platform-Aware**: Use platform-specific helpers for OS-dependent suggestions

## Troubleshooting

### Pattern Not Matching

If a pattern isn't matching:
1. Check keyword case (should be lowercase)
2. Verify error message contains the keyword
3. Check pattern order (earlier patterns take precedence)
4. Add debug logging to see which patterns are checked

### Too Many/Few Suggestions

- **Too many**: Increase specificity of keywords
- **Too few**: Add more general patterns or relax keyword matching

### Wrong Platform Suggestions

Verify `runtime.GOOS` returns expected value:
- Windows: "windows"
- macOS: "darwin"
- Linux: "linux"

## Related Files

- `error_patterns.go`: Pattern matcher implementation
- `error_patterns_test.go`: Comprehensive test suite
- `error.go`: ErrorDisplay component
- `messages.go`: SmartErrorMsg helper
- `model.go`: Chat model error handlers
- `update.go`: Error handler helpers

## License

Part of the rigrun Go TUI project.
