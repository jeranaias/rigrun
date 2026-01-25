// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package util provides utility functions for the go-tui application.
package util

// UNICODE: Rune-aware truncation preserves multi-byte characters.
// These functions handle strings correctly regardless of character encoding,
// preventing mid-character truncation that would corrupt UTF-8 strings.

// TruncateRunes truncates a string to a maximum number of runes (characters).
// This is safe for UTF-8 strings as it counts characters, not bytes.
// If the string is truncated, "..." is appended.
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// TruncateRunesNoEllipsis truncates a string to a maximum number of runes
// without appending an ellipsis.
func TruncateRunesNoEllipsis(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// TruncateWidth truncates a string to a maximum display width.
// This accounts for double-width characters (CJK) that take 2 columns.
// For now, this provides a basic implementation; for full CJK support,
// consider using github.com/mattn/go-runewidth.
func TruncateWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	width := 0
	for i, r := range runes {
		charWidth := runeWidth(r)
		if width+charWidth > maxWidth {
			if maxWidth >= 3 && width >= 3 {
				return string(runes[:i]) + "..."
			}
			return string(runes[:i])
		}
		width += charWidth
	}
	return s
}

// SafeSubstring returns a substring using rune indices (not byte indices).
// This prevents splitting multi-byte UTF-8 characters.
func SafeSubstring(s string, start, end int) string {
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		return ""
	}
	if end < 0 || end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// StringWidth returns the display width of a string.
// Double-width characters (CJK) count as 2 columns.
func StringWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeWidth(r)
	}
	return width
}

// runeWidth returns the display width of a rune.
// Returns 2 for common CJK characters, 1 for others.
// For full support, use github.com/mattn/go-runewidth.
func runeWidth(r rune) int {
	// Common CJK ranges (simplified check)
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return 2
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return 2
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return 2
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return 2
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return 2
	}
	// Fullwidth Forms
	if r >= 0xFF00 && r <= 0xFFEF {
		return 2
	}
	return 1
}

// RuneLen returns the number of runes (characters) in a string.
// This is safer than len() for UTF-8 strings.
func RuneLen(s string) int {
	return len([]rune(s))
}
