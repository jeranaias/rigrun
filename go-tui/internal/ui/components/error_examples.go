// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

// This file contains example functions demonstrating the enhanced error display system.
// These can be used for testing and documentation purposes.

// ExampleNetworkError demonstrates a network error with full context.
func ExampleNetworkError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("Cannot connect to Ollama service at localhost:11434")
	if display == nil {
		return NewError("Network Error", "Connection failed")
	}
	display.SetContext("While initializing chat with model 'llama3.2:7b'")
	return *display
}

// ExampleModelError demonstrates a model not found error.
func ExampleModelError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("model 'llama3.5:70b' not found")
	if display == nil {
		return NewError("Model Error", "Model not found")
	}
	display.SetContext("Attempting to load model for conversation")
	return *display
}

// ExampleContextError demonstrates a context exceeded error.
func ExampleContextError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("context length exceeded maximum of 4096 tokens")
	if display == nil {
		return NewError("Context Error", "Context exceeded")
	}
	display.SetContext("After 47 messages in current conversation")
	return *display
}

// ExampleTimeoutError demonstrates a timeout error.
func ExampleTimeoutError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("request timeout after 30 seconds")
	if display == nil {
		return NewError("Timeout", "Request timed out")
	}
	display.SetContext("Generating response with model 'llama3.1:405b'")
	return *display
}

// ExamplePermissionError demonstrates a permission error.
func ExamplePermissionError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("permission denied accessing file /etc/config")
	if display == nil {
		return NewError("Permission Denied", "Access denied")
	}
	display.SetContext("Attempting to read configuration file")
	return *display
}

// ExampleResourceError demonstrates a resource exhaustion error.
func ExampleResourceError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("out of GPU memory - available: 2GB, required: 8GB")
	if display == nil {
		return NewError("Resource Error", "Insufficient resources")
	}
	display.SetContext("Loading model 'llama3.1:70b' to GPU")
	return *display
}

// ExampleConfigError demonstrates a configuration error.
func ExampleConfigError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("invalid config: missing required field 'api_key'")
	if display == nil {
		return NewError("Config Error", "Configuration invalid")
	}
	display.SetContext("Parsing configuration file ~/.rigrun/config.toml")
	return *display
}

// ExampleToolError demonstrates a tool execution error.
func ExampleToolError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("tool execution failed: command not found 'git'")
	if display == nil {
		return NewError("Tool Error", "Tool execution failed")
	}
	display.SetContext("Executing tool 'git_status' for repository analysis")
	return *display
}

// ExampleRateLimitError demonstrates a rate limit error.
func ExampleRateLimitError() ErrorDisplay {
	matcher := GetDefaultMatcher()
	display := matcher.Match("rate limit exceeded: 100 requests per minute")
	if display == nil {
		return NewError("Rate Limit", "Too many requests")
	}
	display.SetContext("Calling cloud API endpoint")
	return *display
}

// AllExampleErrors returns a slice of all example errors for testing/demo.
func AllExampleErrors() []ErrorDisplay {
	return []ErrorDisplay{
		ExampleNetworkError(),
		ExampleModelError(),
		ExampleContextError(),
		ExampleTimeoutError(),
		ExamplePermissionError(),
		ExampleResourceError(),
		ExampleConfigError(),
		ExampleToolError(),
		ExampleRateLimitError(),
	}
}
