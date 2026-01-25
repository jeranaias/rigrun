// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"testing"
)

func TestErrorPatternMatcher(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	tests := []struct {
		name            string
		errorMsg        string
		expectedTitle   string
		shouldMatch     bool
		minSuggestions  int
	}{
		{
			name:            "Connection refused",
			errorMsg:        "dial tcp 127.0.0.1:11434: connect: connection refused",
			expectedTitle:   "Connection Error",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Ollama not running",
			errorMsg:        "Cannot connect to Ollama at localhost:11434",
			expectedTitle:   "Ollama Connection Error",
			shouldMatch:     true,
			minSuggestions:  3,
		},
		{
			name:            "Model not found",
			errorMsg:        "model 'llama2:13b' not found",
			expectedTitle:   "Model Not Found",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Permission denied",
			errorMsg:        "permission denied: cannot access /var/ollama",
			expectedTitle:   "Permission Denied",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Context exceeded",
			errorMsg:        "context length exceeded: maximum is 4096 tokens",
			expectedTitle:   "Context Exceeded",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Request timeout",
			errorMsg:        "request timeout: operation timed out after 30s",
			expectedTitle:   "Request Timeout",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Rate limit",
			errorMsg:        "rate limit exceeded: too many requests",
			expectedTitle:   "Rate Limit Exceeded",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "File not found",
			errorMsg:        "file not found: /path/to/config.yaml",
			expectedTitle:   "File Not Found",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "GPU error",
			errorMsg:        "CUDA error: out of GPU memory",
			expectedTitle:   "GPU Error",
			shouldMatch:     true,
			minSuggestions:  2,
		},
		{
			name:            "Ollama-specific connection error",
			errorMsg:        "connection refused to ollama server",
			expectedTitle:   "Ollama Connection Error",
			shouldMatch:     true,
			minSuggestions:  3,
		},
		{
			name:            "Unknown error",
			errorMsg:        "some random unknown error",
			expectedTitle:   "",
			shouldMatch:     false,
			minSuggestions:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(tt.errorMsg)

			if tt.shouldMatch {
				if result == nil {
					t.Errorf("Expected pattern to match, but got nil")
					return
				}

				if result.title != tt.expectedTitle {
					t.Errorf("Expected title %q, got %q", tt.expectedTitle, result.title)
				}

				if len(result.suggestions) < tt.minSuggestions {
					t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(result.suggestions))
				}

				// Verify suggestions are limited to 3
				if len(result.suggestions) > 3 {
					t.Errorf("Expected at most 3 suggestions, got %d", len(result.suggestions))
				}
			} else {
				if result != nil {
					t.Errorf("Expected no match, but got title %q", result.title)
				}
			}
		})
	}
}

func TestErrorPatternMatcherMatchOrDefault(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	tests := []struct {
		name           string
		title          string
		errorMsg       string
		expectCustom   bool
		expectTitle    string
	}{
		{
			name:         "Matched pattern",
			title:        "Connection Issue",
			errorMsg:     "connection refused",
			expectCustom: true,
			expectTitle:  "Connection Error", // Pattern's title takes precedence
		},
		{
			name:         "No match - use default",
			title:        "Custom Error",
			errorMsg:     "something went wrong",
			expectCustom: false,
			expectTitle:  "Custom Error", // Uses provided title
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.MatchOrDefault(tt.title, tt.errorMsg)

			if result.title != tt.expectTitle {
				t.Errorf("Expected title %q, got %q", tt.expectTitle, result.title)
			}

			if tt.expectCustom && len(result.suggestions) == 0 {
				t.Error("Expected suggestions for matched pattern, got none")
			}
		})
	}
}

func TestSmartError(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		message      string
		expectSuggs  bool
	}{
		{
			name:        "Connection error gets suggestions",
			title:       "Error",
			message:     "dial tcp: connection refused",
			expectSuggs: true,
		},
		{
			name:        "Ollama error gets suggestions",
			title:       "Error",
			message:     "cannot connect to ollama",
			expectSuggs: true,
		},
		{
			name:        "Generic error has no suggestions",
			title:       "Error",
			message:     "something unexpected happened",
			expectSuggs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SmartError(tt.title, tt.message)

			if tt.expectSuggs && len(result.suggestions) == 0 {
				t.Error("Expected suggestions but got none")
			}

			if !tt.expectSuggs && len(result.suggestions) > 0 {
				t.Errorf("Expected no suggestions but got %d", len(result.suggestions))
			}
		})
	}
}

func TestAddCustomPattern(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	// Add a custom pattern
	customPattern := ErrorPattern{
		Keywords:    []string{"custom error", "my special error"},
		Title:       "Custom Error",
		Suggestions: []string{"Do this", "Do that"},
	}
	matcher.AddPattern(customPattern)

	// Test that it matches
	result := matcher.Match("This is a custom error message")
	if result == nil {
		t.Fatal("Expected custom pattern to match")
	}

	if result.title != "Custom Error" {
		t.Errorf("Expected title %q, got %q", "Custom Error", result.title)
	}

	if len(result.suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(result.suggestions))
	}
}

func TestPlatformSpecificSuggestions(t *testing.T) {
	// Test that platform-specific suggestions are returned
	permissions := getPlatformSpecificPermissionSuggestions()
	if len(permissions) == 0 {
		t.Error("Expected platform-specific permission suggestions")
	}

	ollama := getPlatformSpecificOllamaSuggestions()
	if len(ollama) == 0 {
		t.Error("Expected platform-specific Ollama suggestions")
	}
}

func TestCaseInsensitiveMatching(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	tests := []struct {
		errorMsg    string
		shouldMatch bool
	}{
		{"CONNECTION REFUSED", true},
		{"Connection Refused", true},
		{"connection refused", true},
		{"CoNnEcTiOn ReFuSeD", true},
	}

	for _, tt := range tests {
		t.Run(tt.errorMsg, func(t *testing.T) {
			result := matcher.Match(tt.errorMsg)
			matched := result != nil

			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v", tt.shouldMatch, matched)
			}
		})
	}
}
