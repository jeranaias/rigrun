// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

// =============================================================================
// SHARED HELPER FUNCTIONS
// =============================================================================

// toStr converts an integer to a string without using fmt package.
func toStr(n int) string {
	if n == 0 {
		return "0"
	}

	if n == -9223372036854775808 { // math.MinInt64
		return "-9223372036854775808"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// fmtNumber formats a number with thousand separators.
func fmtNumber(n int) string {
	// Handle math.MinInt64 specially since -math.MinInt64 overflows
	if n == -9223372036854775808 {
		return "-9,223,372,036,854,775,808"
	}

	// Handle negative numbers by processing absolute value and prepending -
	if n < 0 {
		return "-" + fmtNumber(-n)
	}

	if n < 1000 {
		return toStr(n)
	}

	// Build from right to left
	s := toStr(n)
	result := ""
	count := 0

	for i := len(s) - 1; i >= 0; i-- {
		if count > 0 && count%3 == 0 {
			result = "," + result
		}
		result = string(s[i]) + result
		count++
	}

	return result
}

// fmtPercent formats a percentage with one decimal place (with rounding).
func fmtPercent(p float64) string {
	negative := p < 0
	absP := p
	if negative {
		absP = -p
	}

	// Add 0.05 for proper rounding
	rounded := absP + 0.05
	whole := int(rounded)
	frac := int((rounded - float64(whole)) * 10)

	result := toStr(whole) + "." + toStr(frac) + "%"
	if negative {
		result = "-" + result
	}
	return result
}
