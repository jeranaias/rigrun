// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package util provides utility functions for the go-tui application.
//
// This package contains common helper functions used throughout the application
// for string manipulation, type conversion, and file operations.
//
// # Key Functions
//
// String Utilities:
//   - TruncateRunes: UTF-8 safe string truncation with ellipsis
//   - TruncateBytes: Byte-based string truncation
//
// Type Conversion:
//   - IntToString, Int64ToString: Numeric to string conversion
//   - StringToInt, StringToInt64: String to numeric conversion
//
// File Operations:
//   - AtomicWriteFile: Crash-safe file writing with fsync
//
// # Usage
//
//	// Truncate long strings safely for display
//	display := util.TruncateRunes(longText, 50)
//
//	// Convert integers to strings efficiently
//	s := util.IntToString(42)
//
//	// Write files atomically to prevent data loss
//	err := util.AtomicWriteFile(path, data, 0644)
package util
