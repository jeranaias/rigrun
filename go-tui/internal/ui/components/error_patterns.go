// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"runtime"
	"strings"
	"sync"
)

// =============================================================================
// ERROR CATEGORIES
// =============================================================================

// ErrorCategory represents the type of error for better organization and display.
type ErrorCategory string

const (
	// CategoryNetwork represents network and connectivity errors
	CategoryNetwork ErrorCategory = "Network"
	// CategoryModel represents model-related errors (not found, incompatible, etc.)
	CategoryModel ErrorCategory = "Model"
	// CategoryTool represents tool execution errors
	CategoryTool ErrorCategory = "Tool"
	// CategoryConfig represents configuration and settings errors
	CategoryConfig ErrorCategory = "Config"
	// CategoryPermission represents permission and authentication errors
	CategoryPermission ErrorCategory = "Permission"
	// CategoryContext represents context window and memory errors
	CategoryContext ErrorCategory = "Context"
	// CategoryTimeout represents timeout and performance errors
	CategoryTimeout ErrorCategory = "Timeout"
	// CategoryResource represents resource exhaustion errors (disk, GPU, etc.)
	CategoryResource ErrorCategory = "Resource"
	// CategoryParse represents parsing and format errors
	CategoryParse ErrorCategory = "Parse"
	// CategoryUnknown represents unclassified errors
	CategoryUnknown ErrorCategory = "Error"
)

// =============================================================================
// ERROR PATTERN MATCHER
// =============================================================================

// ErrorPattern defines a pattern to match against error strings and provide suggestions.
type ErrorPattern struct {
	// Keywords to match in the error message (case-insensitive, any match triggers)
	Keywords []string

	// Category classifies the error type
	Category ErrorCategory

	// Title for the error display
	Title string

	// Suggestions to help resolve the error
	Suggestions []string

	// DocsURL links to documentation for complex errors (optional)
	DocsURL string

	// LogHint tells users what to look for in logs (optional)
	LogHint string
}

// ErrorPatternMatcher analyzes error strings and provides smart suggestions.
type ErrorPatternMatcher struct {
	mu       sync.RWMutex
	patterns []ErrorPattern
}

// Singleton instance for default pattern matcher
var (
	defaultMatcher     *ErrorPatternMatcher
	defaultMatcherOnce sync.Once
)

// GetDefaultMatcher returns the singleton pattern matcher instance.
// This is thread-safe and avoids re-creating the matcher on every error.
func GetDefaultMatcher() *ErrorPatternMatcher {
	defaultMatcherOnce.Do(func() {
		defaultMatcher = NewErrorPatternMatcher()
	})
	return defaultMatcher
}

// NewErrorPatternMatcher creates a new error pattern matcher with default patterns.
func NewErrorPatternMatcher() *ErrorPatternMatcher {
	matcher := &ErrorPatternMatcher{
		patterns: make([]ErrorPattern, 0),
	}

	// Register default patterns
	matcher.registerDefaultPatterns()

	return matcher
}

// registerDefaultPatterns registers common error patterns with actionable suggestions.
// IMPORTANT: Patterns are registered from MOST SPECIFIC to LEAST SPECIFIC.
// The first matching pattern wins, so specific patterns must come before general ones.
func (m *ErrorPatternMatcher) registerDefaultPatterns() {
	// =========================================================================
	// MOST SPECIFIC PATTERNS FIRST
	// =========================================================================

	// Ollama not running (very specific - must be before general ollama/connection)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"ollama is not running", "ollama not available",
			"ollama service", "start ollama",
		},
		Category:    CategoryNetwork,
		Title:       "Ollama Not Running",
		Suggestions: getPlatformSpecificOllamaSuggestions(),
		DocsURL:     "https://rigrun.dev/docs/troubleshooting/ollama-connection",
		LogHint:     "Check for connection attempts and service status",
	})

	// Ollama-specific connection errors (before general connection errors)
	// Uses compound keywords to avoid matching "permission denied: /var/ollama" etc.
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"ollama connection", "connect to ollama",
			"localhost:11434", "http://localhost:11434",
			"ollama refused", "refused to ollama", "ollama dial", "ollama timeout",
		},
		Category: CategoryNetwork,
		Title:    "Ollama Connection Error",
		Suggestions: []string{
			"Start Ollama: ollama serve",
			"Check if Ollama is installed: ollama --version",
			"Verify Ollama is running on localhost:11434",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/ollama-connection",
		LogHint: "Look for 'connection refused' or 'dial tcp' errors",
	})

	// Model not found errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"model not found", "model does not exist",
			"no such model", "unknown model",
			"' not found", // Matches "model 'xyz' not found"
		},
		Category: CategoryModel,
		Title:    "Model Not Found",
		Suggestions: []string{
			"List available models: ollama list",
			"Pull the model: ollama pull <model-name>",
			"Check model name spelling",
		},
		DocsURL: "https://rigrun.dev/docs/models/installation",
		LogHint: "Check for model name and pull status",
	})

	// Context/Memory exceeded errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"context length", "context exceeded",
			"maximum context", "context window",
			"out of memory", "memory limit",
		},
		Category: CategoryContext,
		Title:    "Context Exceeded",
		Suggestions: []string{
			"Start new conversation: /new",
			"Clear history: /clear",
			"Use shorter messages or reduce context",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/context-limits",
		LogHint: "Check conversation length and token counts",
	})

	// Request Timeout (must be before general network errors)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"request timeout", "operation timed out",
			"context deadline exceeded",
		},
		Category: CategoryTimeout,
		Title:    "Request Timeout",
		Suggestions: []string{
			"Try again - the service may be temporarily busy",
			"Use a smaller or faster model",
			"Check server load and resources",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/timeouts",
		LogHint: "Look for timeout duration and server response times",
	})

	// Rate limiting errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"rate limit", "too many requests",
			"quota exceeded", "429",
			"throttled", "rate exceeded",
		},
		Category: CategoryNetwork,
		Title:    "Rate Limit Exceeded",
		Suggestions: []string{
			"Wait a moment and retry",
			"Switch to a local model to avoid limits",
			"Check your API quota and usage",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/rate-limits",
		LogHint: "Check request frequency and quota status",
	})

	// Permission/Authentication errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"permission denied", "access denied",
			"unauthorized", "forbidden", "403",
			"authentication failed", "invalid credentials",
			"api key", "invalid token",
		},
		Category:    CategoryPermission,
		Title:       "Permission Denied",
		Suggestions: getPlatformSpecificPermissionSuggestions(),
		DocsURL:     "https://rigrun.dev/docs/troubleshooting/permissions",
		LogHint:     "Check file permissions and authentication status",
	})

	// File not found errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"file not found", "no such file",
			"cannot find file", "path not found", "enoent",
		},
		Category: CategoryConfig,
		Title:    "File Not Found",
		Suggestions: []string{
			"Check the file path spelling",
			"Use an absolute path instead of relative",
			"Verify the file exists in the expected location",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/file-access",
		LogHint: "Check the full path being accessed",
	})

	// GPU/CUDA errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"cuda", "gpu", "vram", "out of gpu memory",
			"cuda error", "gpu error",
		},
		Category: CategoryResource,
		Title:    "GPU Error",
		Suggestions: []string{
			"Try a smaller model that fits in GPU memory",
			"Use CPU mode if GPU is unavailable",
			"Check GPU drivers and CUDA installation",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/gpu-issues",
		LogHint: "Check GPU memory usage and CUDA version",
	})

	// Disk space errors (specific)
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"no space left", "disk full",
			"out of disk space", "enospc",
		},
		Category: CategoryResource,
		Title:    "Disk Space Error",
		Suggestions: []string{
			"Free up disk space on your system",
			"Remove unused models: ollama rm <model>",
			"Clear temporary files and caches",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/disk-space",
		LogHint: "Check available disk space and model sizes",
	})

	// =========================================================================
	// MEDIUM SPECIFICITY PATTERNS
	// =========================================================================

	// Configuration errors
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"invalid config", "missing config", "parse config",
			"configuration error",
		},
		Category: CategoryConfig,
		Title:    "Configuration Error",
		Suggestions: []string{
			"Check configuration file syntax",
			"Verify all required fields are present",
			"Use default configuration: /reset",
		},
		DocsURL: "https://rigrun.dev/docs/configuration",
		LogHint: "Check config file path and validation errors",
	})

	// JSON/Parse errors
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"unmarshal", "parse error",
			"invalid json", "syntax error",
		},
		Category: CategoryParse,
		Title:    "Parse Error",
		Suggestions: []string{
			"Check for malformed JSON or data",
			"Verify the format matches expectations",
			"Try again with simpler input",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/parse-errors",
		LogHint: "Check input format and validation errors",
	})

	// =========================================================================
	// GENERAL/FALLBACK PATTERNS (LEAST SPECIFIC - LAST)
	// =========================================================================

	// Tool execution errors
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"tool failed", "tool execution",
			"tool error", "tool not found",
		},
		Category: CategoryTool,
		Title:    "Tool Error",
		Suggestions: []string{
			"Check tool availability and permissions",
			"Verify tool arguments are correct",
			"Review tool output for specific errors",
		},
		DocsURL: "https://rigrun.dev/docs/tools",
		LogHint: "Check tool execution logs and return codes",
	})

	// General network/connection errors (fallback - must be LAST)
	// NOTE: Does NOT include "timeout" - that's handled by Request Timeout above
	m.AddPattern(ErrorPattern{
		Keywords: []string{
			"connection refused", "connect: connection refused",
			"dial tcp", "no such host", "network unreachable",
			"connection reset", "broken pipe",
			"cannot connect", "failed to connect",
		},
		Category: CategoryNetwork,
		Title:    "Connection Error",
		Suggestions: []string{
			"Check your network connection",
			"Verify the service is running and accessible",
			"Try using offline mode if available",
		},
		DocsURL: "https://rigrun.dev/docs/troubleshooting/network",
		LogHint: "Check network connectivity and service status",
	})
}

// AddPattern adds a custom error pattern to the matcher.
// This allows extending the pattern matcher with application-specific patterns.
// Thread-safe.
func (m *ErrorPatternMatcher) AddPattern(pattern ErrorPattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns = append(m.patterns, pattern)
}

// Match analyzes an error string and returns an ErrorDisplay with smart suggestions.
// Returns nil if no pattern matches. Thread-safe.
func (m *ErrorPatternMatcher) Match(errMsg string) *ErrorDisplay {
	if errMsg == "" {
		return nil
	}

	errLower := strings.ToLower(errMsg)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try each pattern in order (most specific first)
	for _, pattern := range m.patterns {
		if m.matchesPattern(errLower, pattern) {
			// Create enhanced error display with all details
			display := NewEnhancedError(pattern, errMsg)
			return &display
		}
	}

	// No pattern matched - return generic error
	return nil
}

// MatchOrDefault analyzes an error string and returns an ErrorDisplay with smart suggestions.
// If no pattern matches, returns a generic error display with the given title and message.
func (m *ErrorPatternMatcher) MatchOrDefault(title, errMsg string) ErrorDisplay {
	if matched := m.Match(errMsg); matched != nil {
		return *matched
	}

	// No pattern matched - return default error
	return NewError(title, errMsg)
}

// matchesPattern checks if an error message matches a pattern's keywords.
func (m *ErrorPatternMatcher) matchesPattern(errMsg string, pattern ErrorPattern) bool {
	for _, keyword := range pattern.Keywords {
		if strings.Contains(errMsg, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// =============================================================================
// PLATFORM-SPECIFIC HELPERS
// =============================================================================

// getPlatformSpecificPermissionSuggestions returns permission suggestions based on the OS.
func getPlatformSpecificPermissionSuggestions() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"Check file permissions in Properties > Security",
			"Run as Administrator if needed",
			"Verify API key or credentials are set",
		}
	case "darwin": // macOS
		return []string{
			"Check file permissions: ls -l <file>",
			"Grant access in System Preferences > Security",
			"Verify API key or credentials are set",
		}
	default: // Linux and others
		return []string{
			"Check file permissions: ls -l <file>",
			"Grant permissions: chmod +r <file>",
			"Verify API key or credentials are set",
		}
	}
}

// getPlatformSpecificOllamaSuggestions returns Ollama startup suggestions based on the OS.
func getPlatformSpecificOllamaSuggestions() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"Start Ollama: ollama serve",
			"Check installation: %LOCALAPPDATA%\\Programs\\Ollama\\",
			"Verify Ollama is in your PATH",
		}
	case "darwin": // macOS
		return []string{
			"Start Ollama: ollama serve",
			"Or launch Ollama.app from Applications",
			"Check if Ollama is installed: which ollama",
		}
	default: // Linux
		return []string{
			"Start Ollama: ollama serve",
			"Or: sudo systemctl start ollama",
			"Check if Ollama is installed: which ollama",
		}
	}
}

// =============================================================================
// SMART ERROR CREATION
// =============================================================================

// SmartError creates an error display with auto-detected pattern matching.
// This is the recommended way to create errors with intelligent suggestions.
func SmartError(title, message string) ErrorDisplay {
	matcher := GetDefaultMatcher()
	return matcher.MatchOrDefault(title, message)
}

// SmartErrorFromError creates an error display from a Go error with pattern matching.
func SmartErrorFromError(title string, err error) ErrorDisplay {
	if err == nil {
		return NewError(title, "Unknown error")
	}
	return SmartError(title, err.Error())
}
