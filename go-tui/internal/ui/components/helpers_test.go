// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"testing"
)

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestToStr(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
		{1000, "1000"},
		{-1, "-1"},
		{-123, "-123"},
		{-9223372036854775808, "-9223372036854775808"}, // MinInt64 special case
	}

	for _, tc := range tests {
		got := toStr(tc.input)
		if got != tc.want {
			t.Errorf("toStr(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFmtNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1234567890, "1,234,567,890"},
		{-1, "-1"},
		{-999, "-999"},
		{-1000, "-1,000"},
		{-1234, "-1,234"},
		{-123456, "-123,456"},
		{-9223372036854775808, "-9,223,372,036,854,775,808"}, // MinInt64
	}

	for _, tc := range tests {
		got := fmtNumber(tc.input)
		if got != tc.want {
			t.Errorf("fmtNumber(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFmtPercent(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0.0%"},
		{1.0, "1.0%"},
		{50.0, "50.0%"},
		{99.9, "99.9%"},
		{100.0, "100.0%"},
		{0.5, "0.5%"},
		{12.3, "12.3%"},
		{87.6, "87.6%"},
		{33.333, "33.3%"}, // Truncates to one decimal
	}

	for _, tc := range tests {
		got := fmtPercent(tc.input)
		if got != tc.want {
			t.Errorf("fmtPercent(%f) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestToStrEdgeCases(t *testing.T) {
	// Test MinInt64 special case
	minInt64 := -9223372036854775808
	result := toStr(minInt64)
	expected := "-9223372036854775808"
	if result != expected {
		t.Errorf("toStr(MinInt64) = %q, want %q", result, expected)
	}

	// Test zero
	result = toStr(0)
	if result != "0" {
		t.Errorf("toStr(0) = %q, want %q", result, "0")
	}
}

func TestFmtNumberEdgeCases(t *testing.T) {
	// Test MinInt64 special case
	minInt64 := -9223372036854775808
	result := fmtNumber(minInt64)
	expected := "-9,223,372,036,854,775,808"
	if result != expected {
		t.Errorf("fmtNumber(MinInt64) = %q, want %q", result, expected)
	}

	// Test numbers less than 1000 (no comma)
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{999, "999"},
		{-1, "-1"},
		{-99, "-99"},
		{-999, "-999"},
	}

	for _, tc := range tests {
		got := fmtNumber(tc.input)
		if got != tc.want {
			t.Errorf("fmtNumber(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFmtPercentEdgeCases(t *testing.T) {
	// Test negative percentages
	tests := []struct {
		input float64
		want  string
	}{
		{-1.0, "-1.0%"},
		{-10.5, "-10.5%"},
		{-0.1, "-0.1%"}, // Small negative preserves sign
	}

	for _, tc := range tests {
		got := fmtPercent(tc.input)
		if got != tc.want {
			t.Errorf("fmtPercent(%f) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// =============================================================================
// PERFORMANCE AND CORRECTNESS TESTS
// =============================================================================

func TestFmtNumberLargeNumbers(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{9999999, "9,999,999"},
		{10000000, "10,000,000"},
		{100000000, "100,000,000"},
		{1000000000, "1,000,000,000"},
		{2147483647, "2,147,483,647"}, // MaxInt32
	}

	for _, tc := range tests {
		got := fmtNumber(tc.input)
		if got != tc.want {
			t.Errorf("fmtNumber(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFmtPercentPrecision(t *testing.T) {
	// Test one decimal place precision
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0.0%"},
		{0.9, "0.9%"},
		{1.1, "1.1%"},
		{9.9, "9.9%"},
		{10.0, "10.0%"},
		{50.5, "50.5%"},
		{99.9, "99.9%"},
	}

	for _, tc := range tests {
		got := fmtPercent(tc.input)
		if got != tc.want {
			t.Errorf("fmtPercent(%f) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkToStr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = toStr(123456789)
	}
}

func BenchmarkFmtNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmtNumber(123456789)
	}
}

func BenchmarkFmtPercent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmtPercent(87.654)
	}
}
