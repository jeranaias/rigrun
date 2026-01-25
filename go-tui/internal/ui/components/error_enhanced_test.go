// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"
	"testing"
)

// TestErrorCategories verifies that error categories are properly defined.
func TestErrorCategories(t *testing.T) {
	categories := []ErrorCategory{
		CategoryNetwork,
		CategoryModel,
		CategoryTool,
		CategoryConfig,
		CategoryPermission,
		CategoryContext,
		CategoryTimeout,
		CategoryResource,
		CategoryParse,
		CategoryUnknown,
	}

	for _, cat := range categories {
		if string(cat) == "" {
			t.Errorf("Category should not be empty")
		}
	}
}

// TestEnhancedErrorPattern verifies that enhanced error patterns have all fields.
func TestEnhancedErrorPattern(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	// Test with a known error pattern
	testCases := []struct {
		errMsg      string
		expectMatch bool
		expectCat   ErrorCategory
	}{
		{"Cannot connect to ollama", true, CategoryNetwork},
		{"model not found", true, CategoryModel},
		{"context exceeded", true, CategoryContext},
		{"request timeout", true, CategoryTimeout},
		{"permission denied", true, CategoryPermission},
		{"out of disk space", true, CategoryResource},
		{"parse error", true, CategoryParse},
		{"some random error", false, CategoryUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.errMsg, func(t *testing.T) {
			display := matcher.Match(tc.errMsg)

			if tc.expectMatch {
				if display == nil {
					t.Errorf("Expected match for '%s' but got nil", tc.errMsg)
					return
				}

				if display.category != tc.expectCat {
					t.Errorf("Expected category %s, got %s", tc.expectCat, display.category)
				}

				// Check that enhanced fields are populated
				if display.title == "" {
					t.Error("Title should not be empty")
				}
				if display.message == "" {
					t.Error("Message should not be empty")
				}
				if len(display.suggestions) == 0 {
					t.Error("Suggestions should not be empty")
				}
				// DocsURL and LogHint are optional, but should be populated for most patterns
			} else {
				if display != nil {
					t.Errorf("Expected no match for '%s' but got: %+v", tc.errMsg, display)
				}
			}
		})
	}
}

// TestNewEnhancedError verifies enhanced error creation.
func TestNewEnhancedError(t *testing.T) {
	pattern := ErrorPattern{
		Keywords:    []string{"test"},
		Category:    CategoryNetwork,
		Title:       "Test Error",
		Suggestions: []string{"Suggestion 1", "Suggestion 2"},
		DocsURL:     "https://example.com/docs",
		LogHint:     "Check logs for details",
	}

	display := NewEnhancedError(pattern, "test error message")

	if display.category != CategoryNetwork {
		t.Errorf("Expected category %s, got %s", CategoryNetwork, display.category)
	}
	if display.title != "Test Error" {
		t.Errorf("Expected title 'Test Error', got '%s'", display.title)
	}
	if display.message != "test error message" {
		t.Errorf("Expected message 'test error message', got '%s'", display.message)
	}
	if len(display.suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(display.suggestions))
	}
	if display.docsURL != "https://example.com/docs" {
		t.Errorf("Expected docsURL 'https://example.com/docs', got '%s'", display.docsURL)
	}
	if display.logHint != "Check logs for details" {
		t.Errorf("Expected logHint 'Check logs for details', got '%s'", display.logHint)
	}
	if !display.visible {
		t.Error("Expected error to be visible")
	}
	if !display.showDocsHint {
		t.Error("Expected showDocsHint to be true")
	}
	if display.logsPath == "" {
		t.Error("Expected logsPath to be set")
	}
}

// TestNewEnhancedErrorWithContext verifies context addition.
func TestNewEnhancedErrorWithContext(t *testing.T) {
	pattern := ErrorPattern{
		Category:    CategoryModel,
		Title:       "Test Error",
		Suggestions: []string{"Fix it"},
		DocsURL:     "https://example.com",
		LogHint:     "Check logs",
	}

	context := "While initializing model 'llama3.2'"
	display := NewEnhancedErrorWithContext(pattern, "error message", context)

	if display.context != context {
		t.Errorf("Expected context '%s', got '%s'", context, display.context)
	}
}

// TestGetLogsPath verifies logs path generation.
func TestGetLogsPath(t *testing.T) {
	path := getLogsPath()

	if path == "" {
		t.Error("Logs path should not be empty")
	}

	// Should contain .rigrun and logs
	if !strings.Contains(path, ".rigrun") {
		t.Error("Logs path should contain '.rigrun'")
	}
	if !strings.Contains(path, "logs") {
		t.Error("Logs path should contain 'logs'")
	}
	if !strings.Contains(path, "rigrun.log") {
		t.Error("Logs path should contain 'rigrun.log'")
	}
}

// TestErrorDisplaySetters verifies setter methods.
func TestErrorDisplaySetters(t *testing.T) {
	display := NewErrorDisplay()

	display.SetCategory(CategoryNetwork)
	display.SetTitle("Test Title")
	display.SetMessage("Test Message")
	display.SetContext("Test Context")
	display.SetSuggestions([]string{"Suggestion 1"})
	display.SetDocsURL("https://example.com")
	display.SetLogHint("Check logs")

	if display.category != CategoryNetwork {
		t.Error("SetCategory failed")
	}
	if display.title != "Test Title" {
		t.Error("SetTitle failed")
	}
	if display.message != "Test Message" {
		t.Error("SetMessage failed")
	}
	if display.context != "Test Context" {
		t.Error("SetContext failed")
	}
	if len(display.suggestions) != 1 {
		t.Error("SetSuggestions failed")
	}
	if display.docsURL != "https://example.com" {
		t.Error("SetDocsURL failed")
	}
	if display.logHint != "Check logs" {
		t.Error("SetLogHint failed")
	}
	if !display.showDocsHint {
		t.Error("showDocsHint should be true when DocsURL is set")
	}
}

// TestErrorPatternPriority verifies pattern matching priority (most specific first).
func TestErrorPatternPriority(t *testing.T) {
	matcher := NewErrorPatternMatcher()

	// "ollama is not running" should match the specific Ollama pattern, not general connection
	display := matcher.Match("ollama is not running")
	if display == nil {
		t.Fatal("Expected match for 'ollama is not running'")
	}
	if display.title != "Ollama Not Running" {
		t.Errorf("Expected 'Ollama Not Running', got '%s'", display.title)
	}

	// General connection error
	display = matcher.Match("connection refused to unknown host")
	if display == nil {
		t.Fatal("Expected match for general connection error")
	}
	if display.title != "Connection Error" {
		t.Errorf("Expected 'Connection Error', got '%s'", display.title)
	}
}

// TestViewBoxRendering verifies that viewBox renders without errors.
func TestViewBoxRendering(t *testing.T) {
	pattern := ErrorPattern{
		Category:    CategoryNetwork,
		Title:       "Test Error",
		Suggestions: []string{"Suggestion 1", "Suggestion 2"},
		DocsURL:     "https://example.com/docs",
		LogHint:     "Check logs for connection issues",
	}

	display := NewEnhancedErrorWithContext(pattern, "Connection failed", "While connecting to server")
	display.SetSize(80, 24)

	view := display.viewBox()

	// Basic checks
	if view == "" {
		t.Error("viewBox should not return empty string")
	}

	// Should contain key elements (using ASCII indicators)
	expectedElements := []string{
		"Test Error",
		"Connection failed",
		"Context:",
		"While connecting to server",
		"Suggestions:",
		"Suggestion 1",
		"[DOC] Docs:",
		"[LOG] Logs:",
		"Check logs for connection issues",
		"[Enter] Dismiss",
		"[c] Copy error",
		"[d] Open docs",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(view, elem) {
			t.Errorf("viewBox should contain '%s'", elem)
		}
	}
}
