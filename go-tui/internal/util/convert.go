// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package util provides utility functions for the go-tui application.
package util

import "strconv"

// IntToString converts an int to string.
// Uses strconv.Itoa for optimal performance.
func IntToString(i int) string {
	return strconv.Itoa(i)
}

// IntToStr is an alias for IntToString for backward compatibility.
// Uses strconv.Itoa for optimal performance.
func IntToStr(i int) string {
	return strconv.Itoa(i)
}

// Int64ToString converts an int64 to string.
// Uses strconv.FormatInt for optimal performance.
func Int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

// FloatToString converts a float64 to string with 2 decimal places.
// Uses strconv.FormatFloat for optimal performance.
func FloatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// FloatToStringPrec converts a float64 to string with specified decimal precision.
func FloatToStringPrec(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}
