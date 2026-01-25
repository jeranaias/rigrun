// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"strings"
	"testing"
)

// TestCompleterComplete tests basic completion functionality
func TestCompleterComplete(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&Command{
		Name:        "/help",
		Description: "Show help",
	})
	registry.Register(&Command{
		Name:        "/history",
		Description: "Show history",
	})
	registry.Register(&Command{
		Name:        "/model",
		Description: "Switch model",
		Args: []ArgDef{
			{Name: "model", Type: ArgTypeModel, Required: true},
		},
	})

	completer := NewCompleter(registry)

	tests := []struct {
		name        string
		input       string
		cursorPos   int
		wantMinimum int // minimum expected completions
		wantPrefix  string // expected prefix of first completion
	}{
		{
			name:        "empty input",
			input:       "/",
			cursorPos:   1,
			wantMinimum: 3, // At least /help, /history, /model
			wantPrefix:  "/",
		},
		{
			name:        "partial command",
			input:       "/h",
			cursorPos:   2,
			wantMinimum: 2, // /help and /history
			wantPrefix:  "/h",
		},
		{
			name:        "complete command with space",
			input:       "/model ",
			cursorPos:   7,
			wantMinimum: 1, // At least one model completion
		},
		{
			name:        "no match",
			input:       "/xyz",
			cursorPos:   4,
			wantMinimum: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions := completer.Complete(tt.input, tt.cursorPos)
			if len(completions) < tt.wantMinimum {
				t.Errorf("Complete() got %d completions, want at least %d", len(completions), tt.wantMinimum)
			}
			if tt.wantPrefix != "" && len(completions) > 0 {
				if !strings.HasPrefix(completions[0].Value, tt.wantPrefix) {
					t.Errorf("First completion %q doesn't start with %q", completions[0].Value, tt.wantPrefix)
				}
			}
		})
	}
}

// TestCompleterMentions tests @ mention completion
func TestCompleterMentions(t *testing.T) {
	registry := NewRegistry()
	completer := NewCompleter(registry)

	tests := []struct {
		name      string
		input     string
		cursorPos int
		wantCount int
	}{
		{
			name:      "start mention",
			input:     "@",
			cursorPos: 1,
			wantCount: 5, // @file:, @clipboard, @git, @codebase, @error
		},
		{
			name:      "partial file mention",
			input:     "@f",
			cursorPos: 2,
			wantCount: 1, // @file:
		},
		{
			name:      "complete file mention",
			input:     "@file:",
			cursorPos: 6,
			wantCount: 0, // No files in test (would need FilesFn callback)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions := completer.Complete(tt.input, tt.cursorPos)
			if len(completions) != tt.wantCount {
				t.Errorf("Complete() got %d completions, want %d", len(completions), tt.wantCount)
			}
		})
	}
}

// TestCalculateScore tests the scoring algorithm
func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		partial    string
		wantHigher bool // true if score should be > 100
	}{
		{
			name:       "exact match",
			value:      "help",
			partial:    "help",
			wantHigher: true,
		},
		{
			name:       "prefix match",
			value:      "help",
			partial:    "hel",
			wantHigher: true,
		},
		{
			name:       "no match",
			value:      "help",
			partial:    "xyz",
			wantHigher: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateScore(tt.value, tt.partial)
			if tt.wantHigher && score <= 100 {
				t.Errorf("calculateScore() = %d, want > 100", score)
			}
			if !tt.wantHigher && score > 100 {
				t.Errorf("calculateScore() = %d, want <= 100", score)
			}
		})
	}
}

// TestSortCompletions tests that completions are sorted by score
func TestSortCompletions(t *testing.T) {
	completions := []Completion{
		{Value: "a", Score: 50},
		{Value: "b", Score: 150},
		{Value: "c", Score: 100},
	}

	sortCompletions(completions)

	// Should be sorted by score descending
	if completions[0].Value != "b" {
		t.Errorf("First completion should be 'b' (highest score), got %q", completions[0].Value)
	}
	if completions[1].Value != "c" {
		t.Errorf("Second completion should be 'c', got %q", completions[1].Value)
	}
	if completions[2].Value != "a" {
		t.Errorf("Third completion should be 'a' (lowest score), got %q", completions[2].Value)
	}
}

// TestTruncate tests the truncation helper
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncate with ellipsis",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "unicode truncation no ellipsis",
			input:  "你好世界",
			maxLen: 4,
			want:   "你好世界",
		},
		{
			name:   "unicode truncation with ellipsis",
			input:  "你好世界!",
			maxLen: 4,
			want:   "你...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatFileSize tests file size formatting
func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want string
	}{
		{
			name: "bytes",
			size: 100,
			want: "100 B",
		},
		{
			name: "kilobytes",
			size: 1024,
			want: "1 KB",
		},
		{
			name: "kilobytes with decimal",
			size: 1536,
			want: "1.5 KB",
		},
		{
			name: "megabytes",
			size: 1024 * 1024,
			want: "1 MB",
		},
		{
			name: "gigabytes",
			size: 1024 * 1024 * 1024,
			want: "1 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileSize(tt.size)
			if got != tt.want {
				t.Errorf("formatFileSize() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCompletionState tests the CompletionState navigation
func TestCompletionState(t *testing.T) {
	cs := NewCompletionState()

	// Initially empty
	if cs.Visible {
		t.Error("New CompletionState should not be visible")
	}

	// Add completions
	completions := []Completion{
		{Value: "a"},
		{Value: "b"},
		{Value: "c"},
	}
	cs.Update("test", completions)

	if !cs.Visible {
		t.Error("CompletionState should be visible after Update")
	}

	if cs.Selected != 0 {
		t.Errorf("Initial selection should be 0, got %d", cs.Selected)
	}

	// Test Next
	cs.Next()
	if cs.Selected != 1 {
		t.Errorf("After Next(), selection should be 1, got %d", cs.Selected)
	}

	// Test wrapping
	cs.Next()
	cs.Next() // Should wrap to 0
	if cs.Selected != 0 {
		t.Errorf("After wrapping, selection should be 0, got %d", cs.Selected)
	}

	// Test Prev
	cs.Prev() // Should wrap to last
	if cs.Selected != 2 {
		t.Errorf("After Prev() from 0, selection should be 2, got %d", cs.Selected)
	}

	// Test Accept
	accepted := cs.Accept()
	if accepted != "c" {
		t.Errorf("Accept() should return 'c', got %q", accepted)
	}

	// Test Clear
	cs.Clear()
	if cs.Visible {
		t.Error("CompletionState should not be visible after Clear")
	}
}

// TestCompleterCallbacks tests custom completion callbacks
func TestCompleterCallbacks(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&Command{
		Name: "/test",
		Args: []ArgDef{
			{Name: "model", Type: ArgTypeModel},
			{Name: "tool", Type: ArgTypeTool},
		},
	})

	completer := NewCompleter(registry)

	// Set custom model completion
	completer.ModelsFn = func() []string {
		return []string{"custom-model-1", "custom-model-2"}
	}

	// Set custom tool completion
	completer.ToolsFn = func() []string {
		return []string{"CustomTool"}
	}

	// Test model completion
	completions := completer.Complete("/test c", 7)
	if len(completions) != 2 {
		t.Errorf("Model completion should return 2 results, got %d", len(completions))
	}

	// Test tool completion (second arg)
	completions = completer.Complete("/test model1 C", 14)
	if len(completions) != 1 {
		t.Errorf("Tool completion should return 1 result, got %d", len(completions))
	}
}
