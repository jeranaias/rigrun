# Error Messages Reference

This document provides examples of all error messages in rigrun after Phase 4.3 improvements.

## Error Message Format

All errors in rigrun follow a consistent format:

```
[✗] Error Title

[Context information if applicable]

Possible causes:
  - Cause 1
  - Cause 2
  - Cause 3

Try these fixes:
  1. Fix command or action 1
  2. Fix command or action 2
  3. Fix command or action 3

Need help? https://github.com/jeranaias/rigrun/issues
```

## Ollama Errors

### Failed to Connect to Ollama

```
[✗] Failed to connect to Ollama

Cannot connect to Ollama at http://localhost:11434.

Possible causes:
  - Ollama service not running
  - Ollama not installed
  - Wrong Ollama URL in config
  - Firewall blocking connection

Try these fixes:
  1. Start Ollama: ollama serve
  2. Check if installed: ollama --version
  3. Verify service: curl http://localhost:11434/api/tags
  4. Check config: rigrun config show

Need help? https://github.com/jeranaias/rigrun/issues
```

### Request Timed Out

```
[✗] Request timed out

Generation request timed out after 300 seconds.

Possible causes:
  - Model is too large for available resources
  - System running out of RAM or VRAM
  - Network latency to Ollama server
  - Ollama server overloaded

Try these fixes:
  1. Use a smaller model: ollama pull qwen2.5-coder:1.5b
  2. Close other applications to free memory
  3. Check system resources: rigrun doctor
  4. Increase timeout if using remote Ollama

Need help? https://github.com/jeranaias/rigrun/issues
```

### Model Not Found

```
[✗] Model not found: llama3.2:latest

Possible causes:
  - Model not downloaded yet
  - Model name misspelled
  - Model deleted from local storage

Try these fixes:
  1. Pull the model: ollama pull llama3.2:latest
  2. List available models: ollama list
  3. Check model name spelling
  4. Pull a popular model: ollama pull qwen2.5-coder:7b

Need help? https://github.com/jeranaias/rigrun/issues
```

### Ollama API Error

```
[✗] Ollama API error

Ollama returned an error: invalid model format

Possible causes:
  - Incompatible Ollama version
  - Corrupted model files
  - Invalid request parameters
  - Ollama server error

Try these fixes:
  1. Update Ollama: curl -fsSL https://ollama.ai/install.sh | sh
  2. Check Ollama version: ollama --version
  3. Reinstall model: ollama rm <model> && ollama pull <model>
  4. Check Ollama logs for errors

Need help? https://github.com/jeranaias/rigrun/issues
```

### Network Error

```
[✗] Network error

Failed to connect: connection refused

Possible causes:
  - Network connection interrupted
  - Proxy or VPN interference
  - DNS resolution failure
  - Firewall blocking connections

Try these fixes:
  1. Check internet connection
  2. Disable VPN temporarily
  3. Check firewall settings
  4. Verify DNS: ping localhost

Need help? https://github.com/jeranaias/rigrun/issues
```

## OpenRouter Errors

### OpenRouter Not Configured

```
[✗] OpenRouter not configured

API key is not set.

Possible causes:
  - OPENROUTER_API_KEY environment variable not set
  - API key not added to rigrun config
  - API key was deleted or reset

Try these fixes:
  1. Get an API key: https://openrouter.ai/keys
  2. Set it: export OPENROUTER_API_KEY=sk-or-...
  3. Or configure: rigrun config set openrouter_api_key sk-or-...
  4. Verify: rigrun config show

Need help? https://github.com/jeranaias/rigrun/issues
```

### Authentication Failed

```
[✗] Authentication failed

Invalid API key.

Possible causes:
  - Invalid or expired API key
  - API key was revoked
  - Incorrect API key format
  - Account suspended

Try these fixes:
  1. Verify your API key at: https://openrouter.ai/keys
  2. Generate a new key if needed
  3. Update config: rigrun config set openrouter_api_key sk-or-...
  4. Check account status: https://openrouter.ai/account

Need help? https://github.com/jeranaias/rigrun/issues
```

### Rate Limit Exceeded

```
[✗] Rate limit exceeded

Too many requests.

Possible causes:
  - Too many requests in short time
  - Free tier limit reached
  - Account quota exceeded
  - Shared IP address issue

Try these fixes:
  1. Wait 60 seconds and try again
  2. Add credits to account: https://openrouter.ai/credits
  3. Use a slower model to reduce costs
  4. Check your usage: https://openrouter.ai/activity

Need help? https://github.com/jeranaias/rigrun/issues
```

### Model Not Found (OpenRouter)

```
[✗] Model not found: anthropic/claude-3-haiku

Possible causes:
  - Model name misspelled
  - Model was deprecated or removed
  - Model ID format incorrect
  - Model not available in your region

Try these fixes:
  1. List available models: rigrun models
  2. Check model name spelling
  3. Browse models: https://openrouter.ai/models
  4. Try a popular model: anthropic/claude-3-haiku

Need help? https://github.com/jeranaias/rigrun/issues
```

### OpenRouter API Error

```
[✗] OpenRouter API error

API error: HTTP 500 - Internal server error

Possible causes:
  - OpenRouter service temporarily down
  - Invalid request format
  - Model overloaded or unavailable
  - Account issue

Try these fixes:
  1. Check OpenRouter status: https://status.openrouter.ai
  2. Try a different model
  3. Wait a moment and retry
  4. Check account: https://openrouter.ai/account

Need help? https://github.com/jeranaias/rigrun/issues
```

### Network Error (OpenRouter)

```
[✗] Network error

Failed to connect to OpenRouter: connection timeout

Possible causes:
  - No internet connection
  - DNS resolution failure
  - Firewall blocking HTTPS
  - Proxy or VPN interference

Try these fixes:
  1. Check internet connection
  2. Verify DNS: ping openrouter.ai
  3. Check firewall settings
  4. Disable VPN temporarily

Need help? https://github.com/jeranaias/rigrun/issues
```

## Using the Error Utilities

### Basic Error Formatting

```rust
use rigrun::error::format_error;

let error = format_error(
    "Operation failed",
    &[
        "Resource not available",
        "Permission denied",
    ],
    &[
        "Check resource status",
        "Verify permissions",
    ],
);
println!("{}", error);
```

### Error Builder

```rust
use rigrun::error::ErrorBuilder;

let error = ErrorBuilder::new("Database connection failed")
    .cause("Database server not running")
    .cause("Invalid credentials")
    .fix("Start database: sudo systemctl start postgresql")
    .fix("Check credentials in config")
    .build();

println!("{}", error);
```

### Error Macro

```rust
use rigrun::error_msg;

let error = error_msg!(
    "File not found",
    causes: [
        "File was deleted",
        "Wrong path specified",
    ],
    fixes: [
        "Check file exists: ls -la path/to/file",
        "Verify path in config",
    ]
);
```

## Design Principles

1. **Clarity**: Error title clearly states what went wrong
2. **Context**: Provides relevant context information
3. **Actionability**: Lists specific causes and fixes
4. **Discoverability**: Includes links to documentation and support
5. **Consistency**: All errors follow the same format
6. **Commands**: Fixes include exact commands users can run
7. **Multiple Options**: Provides several troubleshooting approaches

## Benefits

- **Reduced Support Burden**: Users can self-diagnose and fix common issues
- **Improved UX**: Clear, actionable error messages reduce frustration
- **Faster Resolution**: Step-by-step fixes help users resolve issues quickly
- **Better Documentation**: Error messages serve as inline documentation
- **Consistent Experience**: Same format across all error types

## Related Documentation

- [Troubleshooting Guide](troubleshooting.md)
- [Configuration Reference](configuration.md)
- [API Documentation](api-reference.md)
- [GitHub Issues](https://github.com/jeranaias/rigrun/issues)
