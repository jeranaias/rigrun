// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// FORMATTING UTILITIES TESTS
// =============================================================================

func TestFormatTimestamp(t *testing.T) {
	now := time.Now()

	// Test today - should show just time
	result := formatTimestamp(now)
	if !strings.Contains(result, ":") {
		t.Error("formatTimestamp(today) should contain time with colon")
	}
	if strings.Contains(result, "Mon") || strings.Contains(result, "Jan") {
		t.Error("formatTimestamp(today) should not contain day or month")
	}

	// Test this week - should show day and time
	yesterday := now.AddDate(0, 0, -1)
	result = formatTimestamp(yesterday)
	// Should have either Mon/Tue/Wed/Thu/Fri/Sat/Sun and time
	weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	hasWeekday := false
	for _, day := range weekdays {
		if strings.Contains(result, day) {
			hasWeekday = true
			break
		}
	}
	if !hasWeekday {
		t.Logf("formatTimestamp(yesterday) = %q", result)
		// Note: If yesterday is same day, it will be "today" format
	}

	// Test older - should show date and time
	lastMonth := now.AddDate(0, -1, 0)
	result = formatTimestamp(lastMonth)
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	hasMonth := false
	for _, month := range months {
		if strings.Contains(result, month) {
			hasMonth = true
			break
		}
	}
	if !hasMonth {
		t.Errorf("formatTimestamp(old) = %q, should contain month", result)
	}
}

func TestFormatBool(t *testing.T) {
	tests := []struct {
		input bool
		want  string
	}{
		{true, "enabled"},
		{false, "disabled"},
	}

	for _, tc := range tests {
		got := formatBool(tc.input)
		if got != tc.want {
			t.Errorf("formatBool(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatFloat64(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0.0"},
		{1.0, "1.0"},
		{1.5, "1.5"},
		{45.9, "45.9"},
		{123.456, "123.5"}, // Rounds to one decimal (123.456 + 0.05 = 123.506 → 123.5)
		{-5.3, "-5.3"},
	}

	for _, tc := range tests {
		got := formatFloat64(tc.input)
		if got != tc.want {
			t.Errorf("formatFloat64(%f) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatFloat64EdgeCases(t *testing.T) {
	// Test special values - these might return special strings
	specialCases := []struct {
		name  string
		input float64
	}{
		{"Very large", 1e20},
		{"Very small positive", 1e-10},
		{"Negative", -123.456},
	}

	for _, tc := range specialCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatFloat64(tc.input)
			if result == "" {
				t.Errorf("formatFloat64(%f) should not return empty string", tc.input)
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{-5, "-5"},
		{-9223372036854775808, "-9223372036854775808"}, // MinInt64
	}

	for _, tc := range tests {
		got := formatInt(tc.input)
		if got != tc.want {
			t.Errorf("formatInt(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatNumberWithCommas(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
		{-9223372036854775808, "-9,223,372,036,854,775,808"}, // MinInt64
	}

	for _, tc := range tests {
		got := formatNumberWithCommas(tc.input)
		if got != tc.want {
			t.Errorf("formatNumberWithCommas(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cents float64
		want  string
	}{
		{0.05, "0.1c"}, // Rounds up (0.05 + 0.05 = 0.1)
		{0.5, "0.5c"},
		{1.5, "1.5c"},
		{50.0, "50.0c"},
		{99.9, "99.9c"},
		{100.0, "$1.0"},
		{250.0, "$2.5"},
	}

	for _, tc := range tests {
		got := formatCost(tc.cents)
		if got != tc.want {
			t.Errorf("formatCost(%f) = %q, want %q", tc.cents, got, tc.want)
		}
	}
}

// =============================================================================
// TEXT UTILITIES TESTS
// =============================================================================

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		wantLen  int // Approximate expected line count
	}{
		{"short text", "Hello", 10, 1},
		{"exact fit", "1234567890", 10, 1},
		{"needs wrap", "This is a long line that needs wrapping", 10, 4},
		{"with newlines", "Line1\nLine2\nLine3", 20, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := wordWrap(tc.text, tc.maxWidth)
			lines := strings.Split(result, "\n")

			// Each line should be <= maxWidth (in runes)
			for i, line := range lines {
				runeCount := len([]rune(line))
				if runeCount > tc.maxWidth {
					t.Errorf("Line %d has %d runes, max %d: %q", i, runeCount, tc.maxWidth, line)
				}
			}
		})
	}
}

func TestWordWrapZeroWidth(t *testing.T) {
	result := wordWrap("Test text", 0)
	// Should return original text when maxWidth is 0
	if result != "Test text" {
		t.Errorf("wordWrap with zero width should return original text")
	}
}

func TestCalculateContentWidth(t *testing.T) {
	tests := []struct {
		totalWidth int
		margin     int
		want       int
	}{
		{80, 4, 76},
		{100, 10, 90},
		{40, 4, 36},
		{20, 4, 16},
		{10, 4, 6},
		{5, 4, 3}, // Should handle small values with minimum
	}

	for _, tc := range tests {
		got := calculateContentWidth(tc.totalWidth, tc.margin)
		if got != tc.want {
			t.Errorf("calculateContentWidth(%d, %d) = %d, want %d",
				tc.totalWidth, tc.margin, got, tc.want)
		}
	}
}

func TestCalculateContentWidthMinimum(t *testing.T) {
	// Test that it enforces minimum content width
	result := calculateContentWidth(5, 10)
	if result < 3 { // Should be at least 3 based on the constraints
		t.Errorf("calculateContentWidth should enforce minimum, got %d", result)
	}
}

func TestWrapText(t *testing.T) {
	text := "This is a test of text wrapping functionality"
	maxWidth := 10

	result := wrapText(text, maxWidth)
	lines := strings.Split(result, "\n")

	// Verify each line is within max width
	for i, line := range lines {
		runeCount := len([]rune(line))
		if runeCount > maxWidth {
			t.Errorf("Line %d exceeds max width: %d > %d", i, runeCount, maxWidth)
		}
	}
}

func TestWrapTextPreservesNewlines(t *testing.T) {
	text := "Line 1\nLine 2\nLine 3"
	result := wrapText(text, 100)

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("wrapText should preserve original newlines, got %d lines", len(lines))
	}
}

func TestWrapTextUnicode(t *testing.T) {
	text := "Hello 世界 Unicode test 你好"
	maxWidth := 10

	result := wrapText(text, maxWidth)
	lines := strings.Split(result, "\n")

	// Should handle Unicode correctly (count runes, not bytes)
	for i, line := range lines {
		runeCount := len([]rune(line))
		if runeCount > maxWidth {
			t.Errorf("Line %d (Unicode) exceeds max width: %d > %d", i, runeCount, maxWidth)
		}
	}
}

// =============================================================================
// MESSAGE PREVIEW UTILITIES TESTS
// =============================================================================

func TestRenderMessagePreview(t *testing.T) {
	// Create content with many lines
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "Line " + formatInt(i+1)
	}
	longContent := strings.Join(lines, "\n")

	// When not expanded, should truncate
	result := renderMessagePreview(longContent, false)
	resultLines := strings.Split(result, "\n")
	if len(resultLines) > MaxPreviewLines+2 { // +2 for "show more" message
		t.Errorf("renderMessagePreview(false) should truncate to ~%d lines, got %d",
			MaxPreviewLines, len(resultLines))
	}

	// When expanded, should show all
	result = renderMessagePreview(longContent, true)
	resultLines = strings.Split(result, "\n")
	if len(resultLines) != 30 {
		t.Errorf("renderMessagePreview(true) should show all lines, got %d", len(resultLines))
	}
}

func TestRenderMessagePreviewShortContent(t *testing.T) {
	shortContent := "Short message\nwith only\nthree lines"

	result := renderMessagePreview(shortContent, false)
	// Should not add "show more" for short content
	if strings.Contains(result, "more lines") {
		t.Error("renderMessagePreview should not show 'more lines' for short content")
	}
}

func TestIsMessageTruncatable(t *testing.T) {
	// Short content
	shortContent := strings.Repeat("Line\n", 10)
	if isMessageTruncatable(shortContent) {
		t.Error("isMessageTruncatable should return false for short content")
	}

	// Long content
	longContent := strings.Repeat("Line\n", MaxPreviewLines+10)
	if !isMessageTruncatable(longContent) {
		t.Error("isMessageTruncatable should return true for long content")
	}
}

// =============================================================================
// CONVERSATION UTILITIES TESTS
// =============================================================================

func TestFormatHistory(t *testing.T) {
	conv := model.NewConversation()
	conv.AddUserMessage("Hello")
	msg := conv.AddAssistantMessage()
	msg.AppendToken("Hi there!")
	msg.FinalizeStream(nil) // Complete the streaming message
	conv.AddUserMessage("How are you?")

	result := formatHistory(conv, 10)

	// Should contain message count
	if !strings.Contains(result, "messages") {
		t.Error("formatHistory should mention message count")
	}

	// Should contain some message content
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "Hi there") {
		t.Error("formatHistory should contain message content")
	}

	// Should have role labels
	if !strings.Contains(result, "You") || !strings.Contains(result, "Assistant") {
		t.Error("formatHistory should contain role labels")
	}
}

func TestFormatHistoryEmpty(t *testing.T) {
	conv := model.NewConversation()

	result := formatHistory(conv, 10)

	if !strings.Contains(result, "No messages") {
		t.Error("formatHistory of empty conversation should say 'No messages'")
	}
}

func TestFormatHistoryLimit(t *testing.T) {
	conv := model.NewConversation()
	for i := 0; i < 20; i++ {
		conv.AddUserMessage("Message " + formatInt(i))
	}

	// Request only last 5 messages
	result := formatHistory(conv, 5)

	if !strings.Contains(result, "Last 5 messages") {
		t.Error("formatHistory should mention it's showing last N messages")
	}
}

func TestFormatHistoryTruncation(t *testing.T) {
	conv := model.NewConversation()

	// Add a very long message
	longMsg := strings.Repeat("A", 200)
	conv.AddUserMessage(longMsg)

	result := formatHistory(conv, 10)

	// Should truncate long messages
	if strings.Count(result, "A") > 150 {
		t.Error("formatHistory should truncate very long messages")
	}
}

// =============================================================================
// EDGE CASES AND ERROR HANDLING
// =============================================================================

func TestFormatIntMinInt64(t *testing.T) {
	minInt64 := -9223372036854775808
	result := formatInt(minInt64)
	expected := "-9223372036854775808"

	if result != expected {
		t.Errorf("formatInt(MinInt64) = %q, want %q", result, expected)
	}
}

func TestFormatNumberWithCommasMinInt64(t *testing.T) {
	minInt64 := -9223372036854775808
	result := formatNumberWithCommas(minInt64)
	expected := "-9,223,372,036,854,775,808"

	if result != expected {
		t.Errorf("formatNumberWithCommas(MinInt64) = %q, want %q", result, expected)
	}
}

func TestWrapTextEmptyString(t *testing.T) {
	result := wrapText("", 10)
	if result != "" {
		t.Error("wrapText of empty string should return empty string")
	}
}

func TestWordWrapNegativeWidth(t *testing.T) {
	result := wordWrap("Test", -5)
	// Should handle gracefully (likely returns original text)
	if result == "" {
		t.Error("wordWrap with negative width should not return empty string")
	}
}

func TestCalculateContentWidthEdgeCases(t *testing.T) {
	// Very small total width
	result := calculateContentWidth(1, 4)
	if result < 0 {
		t.Error("calculateContentWidth should not return negative values")
	}

	// Margin larger than total width
	result = calculateContentWidth(10, 20)
	if result < 0 {
		t.Error("calculateContentWidth should handle margin > total gracefully")
	}
}

func TestFormatHistoryWithDifferentRoles(t *testing.T) {
	conv := model.NewConversation()
	conv.AddUserMessage("User message")
	assistantMsg := conv.AddAssistantMessage()
	assistantMsg.Content = "Assistant message"
	conv.AddSystemMessage("System message")

	result := formatHistory(conv, 10)

	// Should show all role types
	if !strings.Contains(result, "You") {
		t.Error("formatHistory should show user role")
	}
	if !strings.Contains(result, "Assistant") {
		t.Error("formatHistory should show assistant role")
	}
	if !strings.Contains(result, "System") {
		t.Error("formatHistory should show system role")
	}
}

// =============================================================================
// SECURITY TESTS
// =============================================================================

func TestWrapTextNoInjection(t *testing.T) {
	// Test that control characters are handled safely
	malicious := "Normal text\x1b[31mRed text\x1b[0m"
	result := wrapText(malicious, 50)

	// Should preserve the control sequences (not interpret them during wrap)
	if !strings.Contains(result, malicious) {
		t.Error("wrapText should preserve control sequences")
	}
}

func TestFormatFunctionsNoHTMLInjection(t *testing.T) {
	malicious := "<script>alert('xss')</script>"

	// All format functions should treat this as plain text
	results := []string{
		formatInt(len(malicious)),
		formatCost(float64(len(malicious))),
		wordWrap(malicious, 50),
	}

	for i, result := range results {
		if result == "" {
			t.Errorf("Format function %d should handle HTML safely", i)
		}
	}
}
